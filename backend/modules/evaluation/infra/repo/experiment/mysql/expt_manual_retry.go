// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type ExptManualRetryDAOImpl struct {
	provider db.Provider
	idgen    idgen.IIDGenerator
}

var _ repo.IExptManualRetryRepo = (*ExptManualRetryDAOImpl)(nil)

func NewExptManualRetryDAO(provider db.Provider, generator idgen.IIDGenerator) repo.IExptManualRetryRepo {
	return &ExptManualRetryDAOImpl{provider: provider, idgen: generator}
}

func (d *ExptManualRetryDAOImpl) FindReceipt(ctx context.Context, spaceID, exptID, runID int64, requestID, hash string) (*entity.ManualRetryReceipt, error) {
	run, err := NewExptRunLogDAO(d.provider).Get(ctx, exptID, runID, db.WithMaster())
	if err != nil {
		return nil, err
	}
	if run.SpaceID != spaceID {
		return nil, errorx.NewByCode(errno.ManualRetryRunConflictCode)
	}
	chunks, err := decodeManualRetryRunItems(run)
	if err != nil {
		return nil, err
	}
	return findManualRetryReceipt(chunks, requestID, hash)
}

func decodeManualRetryRunItems(run *model.ExptRunLog) ([]entity.ExptRunLogItems, error) {
	var chunks []entity.ExptRunLogItems
	if raw := gptr.Indirect(run.ItemIds); len(raw) > 0 {
		if err := json.Unmarshal(raw, &chunks); err != nil {
			return nil, fmt.Errorf("invalid run item metadata: %w", err)
		}
	}
	return chunks, nil
}

func findManualRetryReceipt(chunks []entity.ExptRunLogItems, requestID, hash string) (*entity.ManualRetryReceipt, error) {
	var found *entity.ManualRetryReceipt
	for _, chunk := range chunks {
		receipt := chunk.ManualRetryReceipt
		if receipt == nil || receipt.RequestID != requestID {
			continue
		}
		if receipt.PayloadHash != hash {
			return nil, errorx.NewByCode(errno.ManualRetryRequestConflictCode)
		}
		if found != nil {
			return nil, fmt.Errorf("duplicate manual retry receipt")
		}
		found = receipt
	}
	return found, nil
}

type manualRetryLogIDs struct {
	itemID  int64
	turnIDs map[int64]int64
}

func (d *ExptManualRetryDAOImpl) allocateRetryLogIDs(ctx context.Context, items []*entity.PreparedRetryItem) (map[*entity.PreparedRetryItem]*manualRetryLogIDs, error) {
	result := make(map[*entity.PreparedRetryItem]*manualRetryLogIDs, len(items))
	for _, item := range items {
		if item == nil || item.SourceItem == nil {
			return nil, fmt.Errorf("invalid prepared retry item")
		}
		ids := &manualRetryLogIDs{turnIDs: make(map[int64]int64)}
		generate := func() (int64, error) {
			if d.idgen == nil {
				return 0, fmt.Errorf("manual retry IDGen missing")
			}
			id, err := d.idgen.GenID(ctx)
			if err != nil {
				return 0, err
			}
			if id <= 0 {
				return 0, fmt.Errorf("invalid manual retry log id")
			}
			return id, nil
		}
		var err error
		if item.CurrentItemLog == nil {
			ids.itemID, err = generate()
			if err != nil {
				return nil, err
			}
		}
		for _, turn := range item.Turns {
			if turn == nil || turn.SourceTurn == nil {
				return nil, fmt.Errorf("invalid prepared retry turn")
			}
			if turn.CurrentRunLog == nil {
				id, err := generate()
				if err != nil {
					return nil, err
				}
				ids.turnIDs[turn.SourceTurn.TurnID] = id
			}
		}
		result[item] = ids
	}
	return result, nil
}

type lockedManualRetryItem struct {
	prepared *entity.PreparedRetryItem
	item     *model.ExptItemResult
	log      *model.ExptItemResultRunLog
	ext      []byte
	turns    []*lockedManualRetryTurn
}

type lockedManualRetryTurn struct {
	prepared  *entity.PreparedRetryTurn
	turn      *model.ExptTurnResult
	log       *model.ExptTurnResultRunLog
	sourceLog *model.ExptTurnResultRunLog
}

func (d *ExptManualRetryDAOImpl) RequeueTerminalItems(ctx context.Context, request *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	if request == nil || request.SpaceID <= 0 || request.ExptID <= 0 || request.RunID <= 0 || request.RequestID == "" || request.PayloadHash == "" || (request.Mode != entity.EvaluationModeFailRetry && request.Mode != entity.EvaluationModeRetryItems) {
		return nil, fmt.Errorf("invalid manual retry request")
	}
	receipt, err := d.FindReceipt(ctx, request.SpaceID, request.ExptID, request.RunID, request.RequestID, request.PayloadHash)
	if err != nil || receipt != nil {
		return receipt, err
	}
	logIDs, err := d.allocateRetryLogIDs(ctx, request.Items)
	if err != nil {
		return nil, err
	}
	err = d.provider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		expt, err := NewExptDAO(d.provider).GetByID(ctx, request.ExptID, opts...)
		if err != nil {
			return err
		}
		run, err := NewExptRunLogDAO(d.provider).Get(ctx, request.ExptID, request.RunID, opts...)
		if err != nil {
			return err
		}
		if expt.SpaceID != request.SpaceID || run.SpaceID != request.SpaceID {
			return errorx.NewByCode(errno.ManualRetryRunConflictCode)
		}
		chunks, err := decodeManualRetryRunItems(run)
		if err != nil {
			return err
		}
		receipt, err = findManualRetryReceipt(chunks, request.RequestID, request.PayloadHash)
		if err != nil || receipt != nil {
			return err
		}
		if expt.LatestRunID != request.RunID {
			return errorx.NewByCode(errno.ManualRetryRunConflictCode)
		}
		if expt.Status != int32(entity.ExptStatus_Processing) || (gptr.Indirect(run.Status) != int64(entity.ExptStatus_Pending) && gptr.Indirect(run.Status) != int64(entity.ExptStatus_Processing)) || expt.ExptType != int32(entity.ExptType_Offline) {
			return errorx.NewByCode(errno.ManualRetryRunFinishedCode)
		}
		receipt = &entity.ManualRetryReceipt{RequestID: request.RequestID, PayloadHash: request.PayloadHash, SnapshotAtMS: request.SnapshotAtMS, RejectedItems: append([]*entity.ManualRetryRejectedItem(nil), request.RejectedItems...)}
		prepared := append([]*entity.PreparedRetryItem(nil), request.Items...)
		for _, item := range prepared {
			if item == nil || item.SourceItem == nil {
				return fmt.Errorf("invalid prepared retry item")
			}
		}
		sort.Slice(prepared, func(i, j int) bool { return prepared[i].SourceItem.ItemID < prepared[j].SourceItem.ItemID })
		locked := make([]*lockedManualRetryItem, 0, len(prepared))
		for _, rejected := range receipt.RejectedItems {
			if rejected == nil || rejected.ItemID <= 0 {
				return fmt.Errorf("invalid rejected retry item")
			}
		}
		for index, item := range prepared {
			if index > 0 && item.SourceItem.ItemID == prepared[index-1].SourceItem.ItemID {
				return fmt.Errorf("duplicate prepared retry item")
			}
			current, reason, err := d.lockManualRetryItem(ctx, request, item, opts)
			if err != nil {
				return err
			}
			if reason != "" {
				receipt.RejectedItems = append(receipt.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.SourceItem.ItemID, Reason: reason})
				continue
			}
			locked = append(locked, current)
		}
		accepted := make([]*lockedManualRetryItem, 0, len(locked))
		for _, item := range locked {
			reason, err := d.lockManualRetryTurns(ctx, request, item, opts)
			if err != nil {
				return err
			}
			if reason != "" {
				receipt.RejectedItems = append(receipt.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.item.ItemID, Reason: reason})
				continue
			}
			accepted = append(accepted, item)
		}
		receipt.AcceptedCount = int32(len(accepted))
		sort.Slice(receipt.RejectedItems, func(i, j int) bool { return receipt.RejectedItems[i].ItemID < receipt.RejectedItems[j].ItemID })
		business := (&entity.ExptRunLog{ItemIds: chunks}).GetItemIDs()
		contained := make(map[int64]bool, len(business))
		for _, id := range business {
			contained[id] = true
		}
		added := make([]int64, 0)
		for _, item := range accepted {
			if item.log == nil && !contained[item.item.ItemID] {
				contained[item.item.ItemID] = true
				added = append(added, item.item.ItemID)
			}
		}
		if len(added) > 0 {
			chunks = append(chunks, entity.ExptRunLogItems{ItemIDs: added, CreateAt: gptr.Of(time.Now().Unix())})
		}
		chunks = append(chunks, entity.ExptRunLogItems{CreateAt: gptr.Of(time.Now().Unix()), ManualRetryReceipt: receipt})
		encoded, err := entity.EncodeExptRunLogItems(chunks)
		if errors.Is(err, entity.ErrExptRunLogItemsCapacity) {
			return errorx.WrapByCode(err, errno.ManualRetryCapacityCode, errorx.WithExtraMsg(err.Error()))
		}
		if err != nil {
			return err
		}
		var stats *model.ExptStats
		if len(accepted) > 0 {
			stats, err = NewExptStatsDAO(d.provider).Get(ctx, request.ExptID, request.SpaceID, opts...)
			if err != nil {
				return err
			}
			if stats == nil {
				return fmt.Errorf("manual retry stats missing")
			}
		}
		deltas := make(map[entity.ItemRunState]int)
		for _, item := range accepted {
			deltas[entity.ItemRunState(item.item.Status)]--
			deltas[entity.ItemRunState_Queueing]++
		}
		if stats != nil {
			for status, delta := range deltas {
				count := map[entity.ItemRunState]int32{entity.ItemRunState_Queueing: stats.PendingCnt, entity.ItemRunState_Success: stats.SuccessCnt, entity.ItemRunState_Fail: stats.FailCnt, entity.ItemRunState_Terminal: stats.TerminatedCnt}[status]
				if int(count)+delta < 0 {
					return fmt.Errorf("manual retry stats underflow")
				}
			}
		}
		for _, item := range accepted {
			if err := d.writeManualRetryItem(ctx, request, item, logIDs[item.prepared], opts); err != nil {
				return err
			}
		}
		if len(accepted) > 0 {
			if err := NewExptStatsDAO(d.provider).ArithOperateCount(ctx, request.ExptID, request.SpaceID, &entity.StatsCntArithOp{OpStatusCnt: deltas}, opts...); err != nil {
				return err
			}
		}
		fields := map[string]any{"item_ids": encoded}
		if len(accepted) > 0 {
			type statusCount struct {
				Status int32
				Count  int32
			}
			var counts []statusCount
			if err := d.provider.NewSession(ctx, opts...).Model(&model.ExptItemResultRunLog{}).Select("status, COUNT(*) AS count").Where("space_id = ? AND expt_id = ? AND expt_run_id = ?", request.SpaceID, request.ExptID, request.RunID).Group("status").Scan(&counts).Error; err != nil {
				return err
			}
			for _, field := range []string{"pending_cnt", "processing_cnt", "success_cnt", "fail_cnt", "terminated_cnt"} {
				fields[field] = int32(0)
			}
			for _, count := range counts {
				if field := ItemRunStateStatsField(entity.ItemRunState(count.Status)); field != "" {
					fields[field] = count.Count
				}
			}
		}
		return NewExptRunLogDAO(d.provider).Update(ctx, request.ExptID, request.RunID, request.SpaceID, fields, opts...)
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (d *ExptManualRetryDAOImpl) lockManualRetryItem(ctx context.Context, r *entity.RequeueTerminalItemsRequest, p *entity.PreparedRetryItem, opts []db.Option) (*lockedManualRetryItem, string, error) {
	rows, err := NewExptItemResultDAO(d.provider).BatchGet(ctx, r.SpaceID, r.ExptID, []int64{p.SourceItem.ItemID}, opts...)
	if err != nil {
		return nil, "", err
	}
	if len(rows) == 0 {
		return nil, "not_found", nil
	}
	current := rows[0]
	ext, err := entity.ParseExptItemResultExt(gptr.Indirect(current.Ext))
	if err != nil {
		return nil, "", err
	}
	retry, err := entity.ParseManualRetryContext(ext)
	if err != nil {
		return nil, "", err
	}
	previousRequest := ""
	if retry != nil {
		if retry.RunID != current.ExptRunID {
			return nil, "", fmt.Errorf("manual retry source run mismatch")
		}
		previousRequest = retry.RequestID
	}
	if reason := retryTerminalReason(entity.ItemRunState(current.Status)); reason != "" {
		return nil, reason, nil
	}
	source := convert.NewExptItemResultConvertor().PO2DO(current)
	source.Ext = ext
	if r.Mode == entity.EvaluationModeFailRetry && current.Status != int32(entity.ItemRunState_Fail) {
		return nil, "source_changed", nil
	}
	if current.ItemVersionID != p.ItemVersionID {
		return nil, "source_changed", nil
	}
	if current.ExptRunID != p.PreviousRunID || previousRequest != p.PreviousRequestID || !reflect.DeepEqual(source, p.SourceItem) {
		return nil, "source_changed", nil
	}
	log, err := NewExptItemResultDAO(d.provider).GetItemRunLog(ctx, r.ExptID, r.RunID, current.ItemID, r.SpaceID, opts...)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log = nil
	}
	if log == nil {
		if p.CurrentItemLog != nil || current.ExptRunID == r.RunID {
			return nil, "source_changed", nil
		}
	} else {
		if reason := retryTerminalReason(entity.ItemRunState(log.Status)); reason != "" {
			return nil, reason, nil
		}
		if gptr.Indirect(log.ResultState) != int32(entity.ExptItemResultStateResulted) {
			return nil, "result_not_ready", nil
		}
		if !reflect.DeepEqual(convert.NewExptItemResultRunLogConverter().PO2DO(log), p.CurrentItemLog) {
			return nil, "source_changed", nil
		}
	}
	merged, err := entity.MergeManualRetryContext(gptr.Indirect(current.Ext), &entity.ManualRetryContext{RunID: r.RunID, RequestID: r.RequestID, Mode: r.Mode, Operator: r.Operator, AcceptedAtMS: r.SnapshotAtMS})
	if err != nil {
		return nil, "", err
	}
	return &lockedManualRetryItem{prepared: p, item: current, log: log, ext: merged}, "", nil
}

func retryTerminalReason(status entity.ItemRunState) string {
	switch status {
	case entity.ItemRunState_Success, entity.ItemRunState_Fail, entity.ItemRunState_Terminal:
		return ""
	case entity.ItemRunState_Queueing:
		return "queueing"
	case entity.ItemRunState_Processing:
		return "processing"
	default:
		return "not_terminal"
	}
}

func (d *ExptManualRetryDAOImpl) lockManualRetryTurns(ctx context.Context, r *entity.RequeueTerminalItemsRequest, item *lockedManualRetryItem, opts []db.Option) (string, error) {
	frozen, err := NewExptItemRefDAO(d.provider).GetByExptIDAndItemID(ctx, r.SpaceID, r.ExptID, item.item.ItemID, opts...)
	if err != nil {
		return "", err
	}
	if frozen.ItemConfig != nil {
		var cfg *entity.ExptItemConfig
		if err := json.Unmarshal(*frozen.ItemConfig, &cfg); err != nil {
			return "", err
		}
	}
	if !reflect.DeepEqual(convert.NewExptItemRefConvertor().PO2DO(frozen), item.prepared.FrozenItemRef) || frozen.ItemVersionID != item.prepared.ItemVersionID {
		return "source_changed", nil
	}
	turns, err := NewExptTurnResultDAO(d.provider).GetItemTurnResults(ctx, r.ExptID, item.item.ItemID, r.SpaceID, opts...)
	if err != nil {
		return "", err
	}
	if len(turns) != len(item.prepared.Turns) || len(turns) == 0 {
		return "source_changed", nil
	}
	expected := make(map[int64]*entity.PreparedRetryTurn)
	for _, turn := range item.prepared.Turns {
		if turn == nil || turn.SourceTurn == nil || expected[turn.SourceTurn.TurnID] != nil {
			return "", fmt.Errorf("invalid prepared retry turn")
		}
		expected[turn.SourceTurn.TurnID] = turn
	}
	sort.Slice(turns, func(i, j int) bool { return turns[i].ID < turns[j].ID })
	for _, turn := range turns {
		p := expected[turn.TurnID]
		if p == nil || !reflect.DeepEqual(convert.NewExptTurnResultConvertor().PO2DO(turn, p.SourceTurn.EvaluatorResults), p.SourceTurn) {
			return "source_changed", nil
		}
		var log model.ExptTurnResultRunLog
		err := d.provider.NewSession(ctx, opts...).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ? AND turn_id = ?", r.SpaceID, r.ExptID, r.RunID, item.item.ItemID, turn.TurnID).First(&log).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
		var currentLog *model.ExptTurnResultRunLog
		if err == nil {
			currentLog = &log
		} else if p.CurrentRunLog != nil || item.log != nil {
			return "source_changed", nil
		}
		var logDO *entity.ExptTurnResultRunLog
		if currentLog != nil {
			if err := validateManualRetryTurnExt(currentLog.Ext); err != nil {
				return "", err
			}
			logDO, err = convert.NewExptTurnResultRunLogConvertor().PO2DO(currentLog)
		} else {
			err = nil
		}
		if err != nil {
			return "", err
		}
		if !reflect.DeepEqual(logDO, p.CurrentRunLog) {
			return "source_changed", nil
		}
		refs, err := NewExptTurnEvaluatorResultRefDAO(d.provider).BatchGet(ctx, r.SpaceID, []int64{turn.ID}, opts...)
		if err != nil {
			return "", err
		}
		actual := make(map[int64]*entity.ExptTurnEvaluatorResultRef)
		for _, ref := range refs {
			actual[ref.ID] = convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(ref)
		}
		if len(actual) != len(p.SourceRefs) {
			return "source_changed", nil
		}
		for _, ref := range p.SourceRefs {
			if ref == nil || !reflect.DeepEqual(actual[ref.ID], ref) {
				return "source_changed", nil
			}
		}
		if r.Mode == entity.EvaluationModeRetryItems && (p.PreserveSuccess || p.TargetResultID != 0 || p.EvaluatorResults != nil || len(p.KeepRefIDs) > 0) {
			return "", fmt.Errorf("invalid whole-row retry plan")
		}
		if p.PreserveSuccess && (turn.Status != int32(entity.TurnRunState_Success) || p.TargetResultID != turn.TargetResultID || len(p.KeepRefIDs) != len(actual)) {
			return "", fmt.Errorf("invalid successful retry turn plan")
		}
		for _, id := range p.KeepRefIDs {
			if actual[id] == nil {
				return "", fmt.Errorf("invalid retained retry reference")
			}
		}
		item.turns = append(item.turns, &lockedManualRetryTurn{prepared: p, turn: turn, log: currentLog})
	}
	for _, source := range item.prepared.SourceTurnLogs {
		if source == nil || source.SpaceID != r.SpaceID || source.ExptID != r.ExptID || source.ItemID != item.item.ItemID {
			return "", fmt.Errorf("invalid frozen retry source log")
		}
		var row model.ExptTurnResultRunLog
		if err := d.provider.NewSession(ctx, opts...).Where("space_id=? AND expt_id=? AND expt_run_id=? AND id=?", r.SpaceID, r.ExptID, source.ExptRunID, source.ID).First(&row).Error; err != nil {
			return "", err
		}
		if err := validateManualRetryTurnExt(row.Ext); err != nil {
			return "", err
		}
		got, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&row)
		if err != nil {
			return "", err
		}
		if !reflect.DeepEqual(got, source) {
			return "source_changed", nil
		}
	}
	if item.log == nil && item.prepared.PreviousRunID != r.RunID {
		for _, turn := range item.turns {
			if turn.log != nil {
				continue
			}
			var source model.ExptTurnResultRunLog
			err := d.provider.NewSession(ctx, opts...).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND turn_id=?", r.SpaceID, r.ExptID, item.prepared.PreviousRunID, item.item.ItemID, turn.turn.TurnID).First(&source).Error
			if err != nil {
				return "", err
			}
			if source.ItemVersionID != item.prepared.ItemVersionID {
				return "source_changed", nil
			}
			if err := validateManualRetryTurnExt(source.Ext); err != nil {
				return "", err
			}
			if _, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&source); err != nil {
				return "", err
			}
			turn.sourceLog = &source
		}
	}
	return "", nil
}

func (d *ExptManualRetryDAOImpl) writeManualRetryItem(ctx context.Context, r *entity.RequeueTerminalItemsRequest, item *lockedManualRetryItem, ids *manualRetryLogIDs, opts []db.Option) error {
	conn := func() *gorm.DB { return d.provider.NewSession(ctx, opts...) }
	if err := conn().Model(&model.ExptItemResult{}).Where("space_id = ? AND expt_id = ? AND id = ?", r.SpaceID, r.ExptID, item.item.ID).Updates(map[string]any{"status": int32(entity.ItemRunState_Queueing), "expt_run_id": r.RunID, "err_msg": nil, "log_id": "", "ext": item.ext}).Error; err != nil {
		return err
	}
	if item.log == nil {
		id := ids.itemID
		if err := conn().Create(&model.ExptItemResultRunLog{ID: id, SpaceID: r.SpaceID, ExptID: r.ExptID, ExptRunID: r.RunID, ItemID: item.item.ItemID, ItemVersionID: item.prepared.ItemVersionID, Status: int32(entity.ItemRunState_Queueing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault)), QuotaReservationState: int32(entity.QuotaReservationStateNone)}).Error; err != nil {
			return err
		}
	} else {
		if err := conn().Model(&model.ExptItemResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND id = ?", r.SpaceID, r.ExptID, r.RunID, item.log.ID).Updates(map[string]any{"status": int32(entity.ItemRunState_Queueing), "result_state": int32(entity.ExptItemResultStateDefault), "quota_reservation_state": int32(entity.QuotaReservationStateNone), "err_msg": nil, "log_id": ""}).Error; err != nil {
			return err
		}
	}

	for _, turn := range item.turns {
		p := turn.prepared
		canonicalFields := map[string]any{"expt_run_id": r.RunID}
		logFields := map[string]any{"expt_run_id": r.RunID}
		if !p.PreserveSuccess {
			canonicalFields["status"] = int32(entity.TurnRunState_Queueing)
			canonicalFields["target_result_id"] = p.TargetResultID
			canonicalFields["weighted_score"] = nil
			canonicalFields["err_msg"] = nil
			canonicalFields["log_id"] = ""
			canonicalFields["trace_id"] = int64(0)
			results, err := p.EvaluatorResults.Serialize()
			if err != nil {
				return err
			}
			for field, value := range canonicalFields {
				if field != "weighted_score" {
					logFields[field] = value
				}
			}
			logFields["evaluator_result_ids"] = results
		}
		if err := conn().Model(&model.ExptTurnResult{}).Where("space_id = ? AND expt_id = ? AND id = ?", r.SpaceID, r.ExptID, turn.turn.ID).Updates(canonicalFields).Error; err != nil {
			return err
		}
		if turn.log == nil {
			id := ids.turnIDs[turn.turn.TurnID]
			results, err := p.EvaluatorResults.Serialize()
			if err != nil {
				return err
			}
			row := &model.ExptTurnResultRunLog{ID: id, SpaceID: r.SpaceID, ExptID: r.ExptID, ExptRunID: r.RunID, ItemID: item.item.ItemID, ItemVersionID: item.prepared.ItemVersionID, TurnID: turn.turn.TurnID, Status: int32(entity.TurnRunState_Queueing), TargetResultID: p.TargetResultID, EvaluatorResultIds: &results}
			if turn.sourceLog != nil {
				row.Ext = turn.sourceLog.Ext
			}
			if p.PreserveSuccess {
				row.Status = turn.turn.Status
				row.TraceID = turn.turn.TraceID
				row.LogID = turn.turn.LogID
				row.ErrMsg = turn.turn.ErrMsg
			}
			if err := conn().Create(row).Error; err != nil {
				return err
			}
		} else {
			if err := conn().Model(&model.ExptTurnResultRunLog{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND id = ?", r.SpaceID, r.ExptID, r.RunID, turn.log.ID).Updates(logFields).Error; err != nil {
				return err
			}
		}

		if !p.PreserveSuccess {
			if err := NewExptTurnEvaluatorResultRefDAO(d.provider).DeleteByTurnResultIDs(ctx, r.SpaceID, r.ExptID, []int64{turn.turn.ID}, p.KeepRefIDs, opts...); err != nil {
				return err
			}
		}
	}
	return nil
}

// Preflight reuses final source validation without allocating IDs or writing rows.
func (d *ExptManualRetryDAOImpl) PreflightTerminalItems(ctx context.Context, request *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error) {
	if request == nil {
		return nil, fmt.Errorf("invalid manual retry request")
	}
	plan := &entity.ManualRetryPreparation{SnapshotAtMS: request.SnapshotAtMS, RejectedItems: append([]*entity.ManualRetryRejectedItem(nil), request.RejectedItems...)}
	err := d.provider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		expt, err := NewExptDAO(d.provider).GetByID(ctx, request.ExptID, opts...)
		if err != nil {
			return err
		}
		run, err := NewExptRunLogDAO(d.provider).Get(ctx, request.ExptID, request.RunID, opts...)
		if err != nil {
			return err
		}
		if expt.SpaceID != request.SpaceID || run.SpaceID != request.SpaceID || expt.LatestRunID != request.RunID {
			return errorx.NewByCode(errno.ManualRetryRunConflictCode)
		}
		if expt.Status != int32(entity.ExptStatus_Processing) || expt.ExptType != int32(entity.ExptType_Offline) || (gptr.Indirect(run.Status) != int64(entity.ExptStatus_Pending) && gptr.Indirect(run.Status) != int64(entity.ExptStatus_Processing)) {
			return errorx.NewByCode(errno.ManualRetryRunFinishedCode)
		}
		chunks, err := decodeManualRetryRunItems(run)
		if err != nil {
			return err
		}
		receipt, err := findManualRetryReceipt(chunks, request.RequestID, request.PayloadHash)
		if err != nil {
			return err
		}
		if receipt != nil {
			return errorx.NewByCode(errno.ManualRetryBusyCode, errorx.WithExtraMsg("manual retry receipt appeared during preflight"))
		}
		items := append([]*entity.PreparedRetryItem(nil), request.Items...)
		for _, item := range items {
			if item == nil || item.SourceItem == nil {
				return fmt.Errorf("invalid prepared retry item")
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].SourceItem.ItemID < items[j].SourceItem.ItemID })
		accepted := make([]*lockedManualRetryItem, 0, len(items))
		for i, item := range items {
			if i > 0 && item.SourceItem.ItemID == items[i-1].SourceItem.ItemID {
				return fmt.Errorf("duplicate prepared retry item")
			}
			current, reason, err := d.lockManualRetryItem(ctx, request, item, opts)
			if err != nil {
				return err
			}
			if reason == "" {
				reason, err = d.lockManualRetryTurns(ctx, request, current, opts)
				if err != nil {
					return err
				}
			}
			if reason != "" {
				plan.RejectedItems = append(plan.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.SourceItem.ItemID, Reason: reason})
				continue
			}
			for _, turn := range current.turns {
				raw, err := turn.prepared.EvaluatorResults.Serialize()
				if err != nil {
					return err
				}
				if len(raw) > 65535 {
					return errorx.NewByCode(errno.ManualRetryCapacityCode, errorx.WithExtraMsg("manual retry evaluator results exceeds blob capacity"))
				}
				if turn.sourceLog != nil && len(turn.sourceLog.Ext) > 65535 {
					return errorx.NewByCode(errno.ManualRetryCapacityCode, errorx.WithExtraMsg("manual retry turn ext exceeds blob capacity"))
				}
			}
			plan.Items = append(plan.Items, item)
			accepted = append(accepted, current)
		}
		if len(accepted) > 0 {
			stats, err := NewExptStatsDAO(d.provider).Get(ctx, request.ExptID, request.SpaceID, opts...)
			if err != nil {
				return err
			}
			if stats == nil {
				return fmt.Errorf("manual retry stats missing")
			}
			deltas := map[entity.ItemRunState]int{}
			for _, item := range accepted {
				deltas[entity.ItemRunState(item.item.Status)]--
			}
			counts := map[entity.ItemRunState]int32{entity.ItemRunState_Success: stats.SuccessCnt, entity.ItemRunState_Fail: stats.FailCnt, entity.ItemRunState_Terminal: stats.TerminatedCnt}
			for status, delta := range deltas {
				if int(counts[status])+delta < 0 {
					return fmt.Errorf("manual retry stats underflow")
				}
			}
		}
		// Reserve both the maximum business append and a rejection for every candidate.
		contained := map[int64]bool{}
		for _, id := range (&entity.ExptRunLog{ItemIds: chunks}).GetItemIDs() {
			contained[id] = true
		}
		var added []int64
		worstRejected := append([]*entity.ManualRetryRejectedItem(nil), plan.RejectedItems...)
		for _, item := range plan.Items {
			id := item.SourceItem.ItemID
			if item.CurrentItemLog == nil && !contained[id] {
				added = append(added, id)
				contained[id] = true
			}
			worstRejected = append(worstRejected, &entity.ManualRetryRejectedItem{ItemID: id, Reason: "reservation_mismatch"})
		}
		if len(added) > 0 {
			chunks = append(chunks, entity.ExptRunLogItems{ItemIDs: added, CreateAt: gptr.Of(time.Now().Unix())})
		}
		chunks = append(chunks, entity.ExptRunLogItems{CreateAt: gptr.Of(time.Now().Unix()), ManualRetryReceipt: &entity.ManualRetryReceipt{RequestID: request.RequestID, PayloadHash: request.PayloadHash, SnapshotAtMS: request.SnapshotAtMS, AcceptedCount: 2147483647, RejectedItems: worstRejected}})
		_, err = entity.EncodeExptRunLogItems(chunks)
		if errors.Is(err, entity.ErrExptRunLogItemsCapacity) {
			return errorx.WrapByCode(err, errno.ManualRetryCapacityCode, errorx.WithExtraMsg(err.Error()))
		}
		return err
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	sort.Slice(plan.RejectedItems, func(i, j int) bool { return plan.RejectedItems[i].ItemID < plan.RejectedItems[j].ItemID })
	return plan, nil
}

func validateManualRetryTurnExt(raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	var fields map[string]*string
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key, value := range fields {
		if value == nil {
			return fmt.Errorf("retry turn ext %q must be a string", key)
		}
	}
	if len(raw) > 65535 {
		return errorx.NewByCode(errno.ManualRetryCapacityCode, errorx.WithExtraMsg("retry turn ext exceeds blob capacity"))
	}
	return nil
}
