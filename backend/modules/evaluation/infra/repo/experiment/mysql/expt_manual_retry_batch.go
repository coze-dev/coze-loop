// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

const (
	manualRetryIDChunk   = 1000
	manualRetryRowChunk  = 500
	manualRetryBlobChunk = 100
)

func manualRetryChunks(ids []int64, size int) [][]int64 {
	if len(ids) == 0 {
		return nil
	}
	chunks := make([][]int64, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[start:end])
	}
	return chunks
}

// manualRetryState is the per-transaction snapshot of every row the current
// whole-batch FailRetry revalidates. Loading it up front replaces the previous
// per-item/per-turn SELECTs while keeping FOR UPDATE and the existing
// DeepEqual/source checks inside the same transaction.
type manualRetryState struct {
	itemsByItemID        map[int64]*model.ExptItemResult
	itemLogsByItemID     map[int64]*model.ExptItemResultRunLog
	itemRefsByItemID     map[int64]*model.ExptItemRef
	turnsByItemID        map[int64][]*model.ExptTurnResult
	turnLogsByItemID     map[int64]map[int64]*model.ExptTurnResultRunLog
	refsByTurnResultID   map[int64][]*model.ExptTurnEvaluatorResultRef
	sourceLogsByID       map[int64]*model.ExptTurnResultRunLog
	previousLogsByItemID map[int64]map[int64]*model.ExptTurnResultRunLog
}

func (d *ExptManualRetryDAOImpl) loadManualRetryState(ctx context.Context, r *entity.RequeueTerminalItemsRequest, prepared []*entity.PreparedRetryItem, opts []db.Option) (*manualRetryState, error) {
	state := &manualRetryState{
		itemsByItemID:        make(map[int64]*model.ExptItemResult, len(prepared)),
		itemLogsByItemID:     make(map[int64]*model.ExptItemResultRunLog, len(prepared)),
		itemRefsByItemID:     make(map[int64]*model.ExptItemRef, len(prepared)),
		turnsByItemID:        make(map[int64][]*model.ExptTurnResult, len(prepared)),
		turnLogsByItemID:     make(map[int64]map[int64]*model.ExptTurnResultRunLog, len(prepared)),
		refsByTurnResultID:   make(map[int64][]*model.ExptTurnEvaluatorResultRef),
		sourceLogsByID:       make(map[int64]*model.ExptTurnResultRunLog),
		previousLogsByItemID: make(map[int64]map[int64]*model.ExptTurnResultRunLog),
	}
	seen := make(map[int64]bool, len(prepared))
	itemIDs := make([]int64, 0, len(prepared))
	needRefs := false
	for _, item := range prepared {
		if item == nil || item.SourceItem == nil {
			return nil, fmt.Errorf("invalid prepared retry item")
		}
		if !seen[item.SourceItem.ItemID] {
			seen[item.SourceItem.ItemID] = true
			itemIDs = append(itemIDs, item.SourceItem.ItemID)
		}
		if item.SingleSetSource == nil {
			needRefs = true
		}
	}
	itemDAO := NewExptItemResultDAO(d.provider)
	turnDAO := NewExptTurnResultDAO(d.provider)
	refDAO := NewExptTurnEvaluatorResultRefDAO(d.provider)
	turnResultIDs := make([]int64, 0, len(prepared)*2)
	for _, chunk := range manualRetryChunks(itemIDs, manualRetryIDChunk) {
		rows, err := itemDAO.MGetItemResults(ctx, r.SpaceID, r.ExptID, chunk, opts...)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row != nil {
				state.itemsByItemID[row.ItemID] = row
			}
		}
		logs, err := itemDAO.MGetItemRunLog(ctx, r.ExptID, r.RunID, chunk, r.SpaceID, opts...)
		if err != nil {
			return nil, err
		}
		for _, log := range logs {
			if log != nil {
				state.itemLogsByItemID[log.ItemID] = log
			}
		}
		if needRefs {
			refs, err := NewExptItemRefDAO(d.provider).MGetByExptIDAndItemIDs(ctx, r.SpaceID, r.ExptID, chunk, opts...)
			if err != nil {
				return nil, err
			}
			for _, ref := range refs {
				if ref != nil {
					state.itemRefsByItemID[ref.ItemID] = ref
				}
			}
		}
		turns, err := itemDAO.MGetItemTurnResults(ctx, r.SpaceID, r.ExptID, chunk, opts...)
		if err != nil {
			return nil, err
		}
		for _, turn := range turns {
			if turn != nil {
				state.turnsByItemID[turn.ItemID] = append(state.turnsByItemID[turn.ItemID], turn)
				turnResultIDs = append(turnResultIDs, turn.ID)
			}
		}
		turnLogs, err := turnDAO.MGetItemTurnRunLogs(ctx, r.ExptID, r.RunID, chunk, r.SpaceID, opts...)
		if err != nil {
			return nil, err
		}
		for _, log := range turnLogs {
			if log == nil {
				continue
			}
			byTurn := state.turnLogsByItemID[log.ItemID]
			if byTurn == nil {
				byTurn = make(map[int64]*model.ExptTurnResultRunLog)
				state.turnLogsByItemID[log.ItemID] = byTurn
			}
			byTurn[log.TurnID] = log
		}
	}
	for _, chunk := range manualRetryChunks(turnResultIDs, manualRetryIDChunk) {
		refs, err := refDAO.BatchGet(ctx, r.SpaceID, chunk, opts...)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if ref != nil {
				state.refsByTurnResultID[ref.ExptTurnResultID] = append(state.refsByTurnResultID[ref.ExptTurnResultID], ref)
			}
		}
	}
	sourceIDsByRun := make(map[int64][]int64)
	for _, item := range prepared {
		for _, source := range item.SourceTurnLogs {
			if source != nil {
				sourceIDsByRun[source.ExptRunID] = append(sourceIDsByRun[source.ExptRunID], source.ID)
			}
		}
	}
	for runID, ids := range sourceIDsByRun {
		for _, chunk := range manualRetryChunks(ids, manualRetryIDChunk) {
			var rows []model.ExptTurnResultRunLog
			if err := d.provider.NewSession(ctx, opts...).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND id IN ?", r.SpaceID, r.ExptID, runID, chunk).Find(&rows).Error; err != nil {
				return nil, err
			}
			for i := range rows {
				state.sourceLogsByID[rows[i].ID] = &rows[i]
			}
		}
	}
	previousIDsByRun := make(map[int64][]int64)
	for _, item := range prepared {
		if state.itemLogsByItemID[item.SourceItem.ItemID] != nil || item.PreviousRunID == r.RunID {
			continue
		}
		previousIDsByRun[item.PreviousRunID] = append(previousIDsByRun[item.PreviousRunID], item.SourceItem.ItemID)
	}
	for runID, ids := range previousIDsByRun {
		for _, chunk := range manualRetryChunks(ids, manualRetryIDChunk) {
			logs, err := turnDAO.MGetItemTurnRunLogs(ctx, r.ExptID, runID, chunk, r.SpaceID, opts...)
			if err != nil {
				return nil, err
			}
			for _, log := range logs {
				if log == nil {
					continue
				}
				byTurn := state.previousLogsByItemID[log.ItemID]
				if byTurn == nil {
					byTurn = make(map[int64]*model.ExptTurnResultRunLog)
					state.previousLogsByItemID[log.ItemID] = byTurn
				}
				byTurn[log.TurnID] = log
			}
		}
	}
	return state, nil
}

// writeManualRetryItems keeps the exact five-table write order per row
// class but collapses rows into bounded UPDATE ... CASE / multi-row INSERT
// statements. Every statement still runs inside the caller's single
// transaction; any failure rolls the whole batch back.
func (d *ExptManualRetryDAOImpl) writeManualRetryItems(ctx context.Context, r *entity.RequeueTerminalItemsRequest, accepted []*lockedManualRetryItem, logIDs map[*entity.PreparedRetryItem]*manualRetryLogIDs, opts []db.Option) error {
	provider := d.provider
	conn := func() *gorm.DB { return d.provider.NewSession(ctx, opts...) }
	for _, chunk := range manualRetryChunksLocked(accepted, manualRetryBlobChunk) {
		var cases strings.Builder
		exprArgs := make([]any, 0, len(chunk)*2)
		ids := make([]int64, 0, len(chunk))
		for _, item := range chunk {
			cases.WriteString(" WHEN ? THEN ?")
			exprArgs = append(exprArgs, item.item.ID, item.ext)
			ids = append(ids, item.item.ID)
		}
		if err := conn().Model(&model.ExptItemResult{}).Where("space_id = ? AND expt_id = ? AND id IN ?", r.SpaceID, r.ExptID, ids).Updates(map[string]any{
			"status":      int32(entity.ItemRunState_Queueing),
			"expt_run_id": r.RunID,
			"err_msg":     nil,
			"log_id":      "",
			"ext":         gorm.Expr("CASE id"+cases.String()+" ELSE ext END", exprArgs...),
		}).Error; err != nil {
			return err
		}
	}

	var createItemLogs []*model.ExptItemResultRunLog
	var updateItemLogIDs []int64
	for _, item := range accepted {
		if item.log == nil {
			createItemLogs = append(createItemLogs, &model.ExptItemResultRunLog{ID: logIDs[item.prepared].itemID, SpaceID: r.SpaceID, ExptID: r.ExptID, ExptRunID: r.RunID, ItemID: item.item.ItemID, ItemVersionID: item.prepared.ItemVersionID, Status: int32(entity.ItemRunState_Queueing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault)), QuotaReservationState: int32(entity.QuotaReservationStateNone)})
			continue
		}
		updateItemLogIDs = append(updateItemLogIDs, item.log.ID)
	}
	for start := 0; start < len(createItemLogs); start += manualRetryRowChunk {
		end := start + manualRetryRowChunk
		if end > len(createItemLogs) {
			end = len(createItemLogs)
		}
		if err := conn().Create(createItemLogs[start:end]).Error; err != nil {
			return err
		}
	}
	for _, chunk := range manualRetryChunks(updateItemLogIDs, manualRetryRowChunk) {
		if err := conn().Model(&model.ExptItemResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND id IN ?", r.SpaceID, r.ExptID, r.RunID, chunk).Updates(map[string]any{
			"status":                  int32(entity.ItemRunState_Queueing),
			"result_state":            int32(entity.ExptItemResultStateDefault),
			"quota_reservation_state": int32(entity.QuotaReservationStateNone),
			"err_msg":                 nil,
			"log_id":                  "",
		}).Error; err != nil {
			return err
		}
	}

	var preserveTurnIDs []int64
	var retryTurns []*lockedManualRetryTurn
	for _, item := range accepted {
		for _, turn := range item.turns {
			if turn.prepared.PreserveSuccess {
				preserveTurnIDs = append(preserveTurnIDs, turn.turn.ID)
				continue
			}
			retryTurns = append(retryTurns, turn)
		}
	}
	for _, chunk := range manualRetryChunks(preserveTurnIDs, manualRetryRowChunk) {
		if err := conn().Model(&model.ExptTurnResult{}).Where("space_id = ? AND expt_id = ? AND id IN ?", r.SpaceID, r.ExptID, chunk).Update("expt_run_id", r.RunID).Error; err != nil {
			return err
		}
	}
	for _, chunk := range manualRetryChunksLockedTurns(retryTurns, manualRetryBlobChunk) {
		var cases strings.Builder
		exprArgs := make([]any, 0, len(chunk)*2)
		ids := make([]int64, 0, len(chunk))
		for _, turn := range chunk {
			cases.WriteString(" WHEN ? THEN ?")
			exprArgs = append(exprArgs, turn.turn.ID, turn.prepared.TargetResultID)
			ids = append(ids, turn.turn.ID)
		}
		if err := conn().Model(&model.ExptTurnResult{}).Where("space_id = ? AND expt_id = ? AND id IN ?", r.SpaceID, r.ExptID, ids).Updates(map[string]any{
			"expt_run_id":      r.RunID,
			"status":           int32(entity.TurnRunState_Queueing),
			"target_result_id": gorm.Expr("CASE id"+cases.String()+" ELSE target_result_id END", exprArgs...),
			"weighted_score":   nil,
			"err_msg":          nil,
			"log_id":           "",
			"trace_id":         int64(0),
		}).Error; err != nil {
			return err
		}
	}

	var createTurnLogs []*model.ExptTurnResultRunLog
	var preserveLogIDs []int64
	var retryTurnLogs []*lockedManualRetryTurn
	for _, item := range accepted {
		for _, turn := range item.turns {
			if turn.log != nil {
				if turn.prepared.PreserveSuccess {
					preserveLogIDs = append(preserveLogIDs, turn.log.ID)
				} else {
					retryTurnLogs = append(retryTurnLogs, turn)
				}
				continue
			}
			results, err := turn.prepared.EvaluatorResults.Serialize()
			if err != nil {
				return err
			}
			row := &model.ExptTurnResultRunLog{ID: logIDs[item.prepared].turnIDs[turn.turn.TurnID], SpaceID: r.SpaceID, ExptID: r.ExptID, ExptRunID: r.RunID, ItemID: turn.turn.ItemID, ItemVersionID: item.prepared.ItemVersionID, TurnID: turn.turn.TurnID, Status: int32(entity.TurnRunState_Queueing), TargetResultID: turn.prepared.TargetResultID, EvaluatorResultIds: &results}
			if turn.sourceLog != nil {
				row.Ext = turn.sourceLog.Ext
			}
			if turn.prepared.PreserveSuccess {
				row.Status = turn.turn.Status
				row.TraceID = turn.turn.TraceID
				row.LogID = turn.turn.LogID
				row.ErrMsg = turn.turn.ErrMsg
			}
			createTurnLogs = append(createTurnLogs, row)
		}
	}
	for start := 0; start < len(createTurnLogs); start += manualRetryBlobChunk {
		end := start + manualRetryBlobChunk
		if end > len(createTurnLogs) {
			end = len(createTurnLogs)
		}
		if err := conn().Create(createTurnLogs[start:end]).Error; err != nil {
			return err
		}
	}
	for _, chunk := range manualRetryChunks(preserveLogIDs, manualRetryRowChunk) {
		if err := conn().Model(&model.ExptTurnResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND id IN ?", r.SpaceID, r.ExptID, r.RunID, chunk).Update("expt_run_id", r.RunID).Error; err != nil {
			return err
		}
	}
	for _, chunk := range manualRetryChunksLockedTurns(retryTurnLogs, manualRetryBlobChunk) {
		var targetCases, evaluatorCases strings.Builder
		var targetArgs, evaluatorArgs []any
		ids := make([]int64, 0, len(chunk))
		for _, turn := range chunk {
			results, err := turn.prepared.EvaluatorResults.Serialize()
			if err != nil {
				return err
			}
			targetCases.WriteString(" WHEN ? THEN ?")
			targetArgs = append(targetArgs, turn.log.ID, turn.prepared.TargetResultID)
			evaluatorCases.WriteString(" WHEN ? THEN ?")
			evaluatorArgs = append(evaluatorArgs, turn.log.ID, results)
			ids = append(ids, turn.log.ID)
		}
		if err := conn().Model(&model.ExptTurnResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND id IN ?", r.SpaceID, r.ExptID, r.RunID, ids).Updates(map[string]any{
			"expt_run_id":          r.RunID,
			"status":               int32(entity.TurnRunState_Queueing),
			"target_result_id":     gorm.Expr("CASE id"+targetCases.String()+" ELSE target_result_id END", targetArgs...),
			"evaluator_result_ids": gorm.Expr("CASE id"+evaluatorCases.String()+" ELSE evaluator_result_ids END", evaluatorArgs...),
			"err_msg":              nil,
			"log_id":               "",
			"trace_id":             int64(0),
		}).Error; err != nil {
			return err
		}
	}

	var deleteTurnIDs []int64
	for _, turn := range retryTurns {
		deleteTurnIDs = append(deleteTurnIDs, turn.turn.ID)
	}
	for _, chunk := range manualRetryChunks(deleteTurnIDs, manualRetryRowChunk) {
		chunkSet := make(map[int64]bool, len(chunk))
		for _, id := range chunk {
			chunkSet[id] = true
		}
		chunkKeep := make([]int64, 0)
		for _, turn := range retryTurns {
			if chunkSet[turn.turn.ID] {
				chunkKeep = append(chunkKeep, turn.prepared.KeepRefIDs...)
			}
		}
		if err := NewExptTurnEvaluatorResultRefDAO(provider).DeleteByTurnResultIDs(ctx, r.SpaceID, r.ExptID, chunk, chunkKeep, opts...); err != nil {
			return err
		}
	}
	return nil
}

func manualRetryChunksLocked(items []*lockedManualRetryItem, size int) [][]*lockedManualRetryItem {
	if len(items) == 0 {
		return nil
	}
	chunks := make([][]*lockedManualRetryItem, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}

func manualRetryChunksLockedTurns(items []*lockedManualRetryTurn, size int) [][]*lockedManualRetryTurn {
	if len(items) == 0 {
		return nil
	}
	chunks := make([][]*lockedManualRetryTurn, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[start:end])
	}
	return chunks
}
