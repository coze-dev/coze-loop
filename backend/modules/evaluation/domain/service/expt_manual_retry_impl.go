// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// 与 FinishCurrentRun / 终止链路共用 expt_run_exec_lock，持锁上限必须一致：
// 1 分钟到期会 cancel opCtx，失败项一多整批重试就会回滚。
const manualRetryLockMaxHold = 5 * time.Minute

type manualRetryPrepareBatch struct {
	itemLogs    map[int64][]*entity.ExptItemResultRunLog
	itemRefs    map[int64]*entity.ExptItemRef
	turnResults map[int64][]*entity.ExptTurnResult
	turnLogs    map[int64][]*entity.ExptTurnResultRunLog
	refsByTurn  map[int64][]*entity.ExptTurnEvaluatorResultRef
}

func chunkManualRetryIDs(ids []int64, size int) [][]int64 {
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

// loadManualRetryPrepareBatch batches the frozen-source reads that used to be
// issued once per item. The per-item loop below keeps the exact validation
// order and rejection reasons, but every table is now read in bounded IN
// chunks and reused through maps.
func (m *ExptMangerImpl) loadManualRetryPrepareBatch(ctx context.Context, expt *entity.Experiment, runID int64, items []*entity.ExptItemResult) (*manualRetryPrepareBatch, error) {
	batch := &manualRetryPrepareBatch{
		itemLogs:    make(map[int64][]*entity.ExptItemResultRunLog, len(items)),
		itemRefs:    make(map[int64]*entity.ExptItemRef, len(items)),
		turnResults: make(map[int64][]*entity.ExptTurnResult, len(items)),
		turnLogs:    make(map[int64][]*entity.ExptTurnResultRunLog, len(items)),
		refsByTurn:  make(map[int64][]*entity.ExptTurnEvaluatorResultRef, len(items)),
	}
	itemIDs := make([]int64, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ItemID)
	}
	needRefs := expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig
	turnResultIDs := make([]int64, 0, len(items))
	for _, chunk := range chunkManualRetryIDs(itemIDs, 2000) {
		logs, err := m.itemResultRepo.MGetItemRunLog(ctx, expt.ID, runID, chunk, expt.SpaceID)
		if err != nil {
			return nil, err
		}
		for _, log := range logs {
			if log != nil {
				batch.itemLogs[log.ItemID] = append(batch.itemLogs[log.ItemID], log)
			}
		}
		if needRefs {
			refs, err := m.itemRefRepo.MGetByExptIDAndItemIDs(ctx, expt.SpaceID, expt.ID, chunk)
			if err != nil {
				return nil, err
			}
			for _, ref := range refs {
				if ref != nil {
					batch.itemRefs[ref.ItemID] = ref
				}
			}
		}
		turns, err := m.itemResultRepo.MGetItemTurnResults(ctx, expt.SpaceID, expt.ID, chunk)
		if err != nil {
			return nil, err
		}
		for _, turn := range turns {
			if turn != nil {
				batch.turnResults[turn.ItemID] = append(batch.turnResults[turn.ItemID], turn)
				turnResultIDs = append(turnResultIDs, turn.ID)
			}
		}
		turnLogs, err := m.turnResultRepo.MGetItemTurnRunLogs(ctx, expt.ID, runID, chunk, expt.SpaceID)
		if err != nil {
			return nil, err
		}
		for _, log := range turnLogs {
			if log != nil {
				batch.turnLogs[log.ItemID] = append(batch.turnLogs[log.ItemID], log)
			}
		}
	}
	for _, chunk := range chunkManualRetryIDs(turnResultIDs, 2000) {
		refs, err := m.turnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, expt.SpaceID, chunk)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if ref != nil {
				batch.refsByTurn[ref.ExptTurnResultID] = append(batch.refsByTurn[ref.ExptTurnResultID], ref)
			}
		}
	}
	return batch, nil
}

func (m *ExptMangerImpl) prepareTerminalCandidatesAndReusableRefs(ctx context.Context, expt *entity.Experiment, runID int64, mode entity.ExptRunMode, itemIDs []int64, records EvaluatorRecordService) (*entity.ManualRetryPreparation, error) {
	ctx = contexts.WithCtxWriteDB(ctx)
	plan := &entity.ManualRetryPreparation{SnapshotAtMS: time.Now().UnixMilli()}
	var items []*entity.ExptItemResult
	if mode == entity.EvaluationModeFailRetry {
		if len(itemIDs) != 0 {
			return nil, fmt.Errorf("failure retry cannot select items")
		}
		var cursor int64
		for {
			page, next, err := m.itemResultRepo.ScanItemResults(ctx, expt.ID, cursor, 500, []int32{int32(entity.ItemRunState_Fail)}, expt.SpaceID)
			if err != nil {
				return nil, err
			}
			items = append(items, page...)
			if next == 0 || len(page) == 0 {
				break
			}
			if next <= cursor {
				return nil, fmt.Errorf("invalid failure retry cursor")
			}
			cursor = next
		}
	} else if mode == entity.EvaluationModeRetryItems {
		if len(itemIDs) == 0 {
			return nil, fmt.Errorf("manual retry selection is empty")
		}
		selected := make(map[int64]bool, len(itemIDs))
		for _, id := range itemIDs {
			if id <= 0 {
				return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid retry item id"))
			}
			selected[id] = true
		}
		itemIDs = make([]int64, 0, len(selected))
		for id := range selected {
			itemIDs = append(itemIDs, id)
		}
		sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
		var err error
		items, err = m.itemResultRepo.BatchGet(ctx, expt.SpaceID, expt.ID, itemIDs)
		if err != nil {
			return nil, err
		}
		found := make(map[int64]bool)
		for _, item := range items {
			if item != nil {
				found[item.ItemID] = true
			}
		}
		for _, id := range itemIDs {
			if !found[id] {
				plan.RejectedItems = append(plan.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: id, Reason: "not_found"})
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported manual retry mode %d", mode)
	}
	for _, item := range items {
		if item == nil {
			return nil, fmt.Errorf("nil retry item")
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ItemID < items[j].ItemID })
	candidates := make([]*entity.ExptItemResult, 0, len(items))
	for _, item := range items {
		if manualRetryTerminalReason(item.Status) == "" {
			candidates = append(candidates, item)
		}
	}
	// A single item needs no prefetch; larger selections take one batched
	// snapshot per table and reuse it in the per-item validation loop below.
	var preparedBatch *manualRetryPrepareBatch
	if len(candidates) > 1 {
		var err error
		preparedBatch, err = m.loadManualRetryPrepareBatch(ctx, expt, runID, candidates)
		if err != nil {
			return nil, err
		}
	}
	for _, item := range items {
		reason := manualRetryTerminalReason(item.Status)
		if reason != "" {
			plan.RejectedItems = append(plan.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.ItemID, Reason: reason})
			continue
		}
		if item.SpaceID != expt.SpaceID || item.ExptID != expt.ID || item.ExptRunID <= 0 {
			return nil, fmt.Errorf("invalid retry item source")
		}
		previous, err := entity.ParseManualRetryContext(item.Ext)
		if err != nil {
			return nil, err
		}
		prepared := &entity.PreparedRetryItem{SourceItem: item, PreviousRunID: item.ExptRunID}
		if previous != nil {
			if previous.RunID != item.ExptRunID {
				return nil, fmt.Errorf("manual retry source run mismatch")
			}
			prepared.PreviousRequestID = previous.RequestID
		}
		var itemLogs []*entity.ExptItemResultRunLog
		if preparedBatch != nil {
			itemLogs = preparedBatch.itemLogs[item.ItemID]
		} else {
			itemLogs, err = m.itemResultRepo.MGetItemRunLog(ctx, expt.ID, runID, []int64{item.ItemID}, expt.SpaceID)
			if err != nil {
				return nil, err
			}
		}
		if len(itemLogs) > 1 {
			return nil, fmt.Errorf("duplicate current item log")
		}
		if len(itemLogs) == 1 {
			prepared.CurrentItemLog = itemLogs[0]
		}

		if log := prepared.CurrentItemLog; log != nil {
			if log.SpaceID != expt.SpaceID || log.ExptID != expt.ID || log.ExptRunID != runID || log.ItemID != item.ItemID {
				return nil, fmt.Errorf("invalid current item log")
			}
			reason = manualRetryTerminalReason(entity.ItemRunState(log.Status))
			if reason == "" && log.ResultState != int32(entity.ExptItemResultStateResulted) {
				reason = "result_not_ready"
			}
			if reason != "" {
				plan.RejectedItems = append(plan.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.ItemID, Reason: reason})
				continue
			}
		} else if item.ExptRunID == runID {
			return nil, fmt.Errorf("current retry item log missing")
		}
		switch expt.EvalSetSourceType {
		case 0, entity.ExptEvalSetSourceType_SingleSet:
			// A committed experiment binding and the existing item/turn rows are
			// the frozen single-set source. Never fall back to reading a live draft.
			if expt.EvalSetID <= 0 || resolveLoadSpaceID(expt.SpaceID, expt.EvalSetSpaceID) <= 0 || resolveSetReadVersionID(expt.EvalSetID, expt.EvalSetVersionID) == nil || item.ItemVersionID < 0 {
				return nil, fmt.Errorf("frozen single-set retry source missing")
			}
			if log := prepared.CurrentItemLog; log != nil && log.ItemVersionID != item.ItemVersionID {
				return nil, fmt.Errorf("frozen retry item version mismatch")
			}
			prepared.SingleSetSource = &entity.PreparedRetrySingleSetSource{
				SourceType: expt.EvalSetSourceType, EvalSetID: expt.EvalSetID,
				EvalSetVersionID: expt.EvalSetVersionID, EvalSetSpaceID: expt.EvalSetSpaceID,
				TargetID: expt.TargetID, TargetVersionID: expt.TargetVersionID,
				TargetSpaceID: expt.TargetSpaceID, TargetType: expt.TargetType,
			}
		case entity.ExptEvalSetSourceType_MultiSetConfig:
			if preparedBatch != nil {
				prepared.FrozenItemRef = preparedBatch.itemRefs[item.ItemID]
			}
			if prepared.FrozenItemRef == nil {
				// Keep the original missing-ref error for the rare drift path;
				// the normal path is fully served by the batch read above.
				prepared.FrozenItemRef, err = m.itemRefRepo.GetByExptIDAndItemID(ctx, expt.SpaceID, expt.ID, item.ItemID)
				if err != nil {
					return nil, err
				}
			}
			ref := prepared.FrozenItemRef
			if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID || ref.ItemID != item.ItemID || ref.ItemVersionID != item.ItemVersionID {
				return nil, fmt.Errorf("frozen retry item version mismatch")
			}
		default:
			return nil, fmt.Errorf("invalid retry evaluation set source type")
		}
		prepared.ItemVersionID = item.ItemVersionID
		if err := m.prepareManualRetryTurns(ctx, expt, runID, mode, prepared, records, preparedBatch); err != nil {
			return nil, err
		}
		plan.Items = append(plan.Items, prepared)
	}
	sort.Slice(plan.RejectedItems, func(i, j int) bool { return plan.RejectedItems[i].ItemID < plan.RejectedItems[j].ItemID })
	return plan, nil
}

func manualRetryTerminalReason(status entity.ItemRunState) string {
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

func (m *ExptMangerImpl) prepareManualRetryTurns(ctx context.Context, expt *entity.Experiment, runID int64, mode entity.ExptRunMode, item *entity.PreparedRetryItem, records EvaluatorRecordService, batches ...*manualRetryPrepareBatch) error {
	var batch *manualRetryPrepareBatch
	if len(batches) > 0 {
		batch = batches[0]
	}
	var turns []*entity.ExptTurnResult
	var logs []*entity.ExptTurnResultRunLog
	var err error
	if batch != nil {
		turns = batch.turnResults[item.SourceItem.ItemID]
		logs = batch.turnLogs[item.SourceItem.ItemID]
	} else {
		turns, err = m.turnResultRepo.GetItemTurnResults(ctx, expt.ID, item.SourceItem.ItemID, expt.SpaceID)
		if err != nil {
			return err
		}
		if len(turns) == 0 {
			return fmt.Errorf("frozen retry turns missing")
		}
		logs, err = m.turnResultRepo.GetItemTurnRunLogs(ctx, expt.ID, runID, item.SourceItem.ItemID, expt.SpaceID)
		if err != nil {
			return err
		}
	}
	if len(turns) == 0 {
		return fmt.Errorf("frozen retry turns missing")
	}
	byTurn := make(map[int64]*entity.ExptTurnResultRunLog)
	for _, log := range logs {
		if log == nil || log.SpaceID != expt.SpaceID || log.ExptID != expt.ID || log.ExptRunID != runID || log.ItemID != item.SourceItem.ItemID || byTurn[log.TurnID] != nil {
			return fmt.Errorf("invalid retry turn log")
		}
		byTurn[log.TurnID] = log
	}
	ids := make([]int64, 0, len(turns))
	for _, turn := range turns {
		if turn == nil || turn.SpaceID != expt.SpaceID || turn.ExptID != expt.ID || turn.ItemID != item.SourceItem.ItemID || turn.ItemVersionID != item.ItemVersionID {
			return fmt.Errorf("invalid frozen retry turn")
		}
		// FailRetry 保留的历史成功 turn 仍属于旧 run，这是既有语义；
		// 与 expt_current_result.go / expt_item_dispatch.go 的读侧规则保持一致。
		if turn.ExptRunID != item.PreviousRunID &&
			(mode != entity.EvaluationModeFailRetry || entity.TurnRunState(turn.Status) != entity.TurnRunState_Success) {
			return fmt.Errorf("invalid frozen retry turn")
		}
		ids = append(ids, turn.ID)
	}
	var refs []*entity.ExptTurnEvaluatorResultRef
	if batch != nil {
		for _, id := range ids {
			refs = append(refs, batch.refsByTurn[id]...)
		}
	} else {
		refs, err = m.turnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, expt.SpaceID, ids)
		if err != nil {
			return err
		}
	}
	refsByTurn := make(map[int64][]*entity.ExptTurnEvaluatorResultRef)
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID {
			return fmt.Errorf("invalid retry evaluator reference")
		}
		refsByTurn[ref.ExptTurnResultID] = append(refsByTurn[ref.ExptTurnResultID], ref)
	}
	source := &retryEvaluatorSourceLogs{repo: m.turnResultRepo, event: &entity.ExptItemEvalEvent{SpaceID: expt.SpaceID, ExptID: expt.ID, ExptRunID: runID, EvalSetItemID: item.SourceItem.ItemID}, logsByRun: map[int64]map[int64]*entity.ExptTurnResultRunLog{runID: byTurn}}
	for _, turn := range turns {
		log := byTurn[turn.TurnID]
		if item.CurrentItemLog != nil && log == nil {
			return fmt.Errorf("frozen current retry turn log missing")
		}
		if log != nil && log.ItemVersionID != item.ItemVersionID {
			return fmt.Errorf("retry turn log version mismatch")
		}
		prepared := &entity.PreparedRetryTurn{SourceTurn: turn, CurrentRunLog: log, SourceRefs: refsByTurn[turn.ID]}
		if mode == entity.EvaluationModeFailRetry {
			if entity.TurnRunState(turn.Status) == entity.TurnRunState_Success {
				prepared.PreserveSuccess = true
				prepared.TargetResultID = turn.TargetResultID
				prepared.EvaluatorResults = retryEvaluatorResultsFromRefs(turn.ID, prepared.SourceRefs)
			} else {
				targetSpace := expt.TargetSpaceID
				if item.FrozenItemRef != nil {
					if cfg := item.FrozenItemRef.ItemConfig; cfg != nil && cfg.TargetSourceSpaceID > 0 {
						targetSpace = cfg.TargetSourceSpaceID
					}
				}
				targetVersion := expt.TargetVersionID
				targetRequired := !shouldSkipTargetNode(expt)
				targetReader := &manualRetryTargetReader{IEvalTargetService: m.evalTargetService, versionID: targetVersion, source: source, turn: turn}
				prepared.TargetResultID, prepared.EvaluatorResults, err = failRetrySelectTurnRunLogRefs(ctx, resolveLoadSpaceID(expt.SpaceID, targetSpace), targetRequired, turn, targetReader, records, prepared.SourceRefs, source)
				if err != nil {
					return err
				}
			}
			for _, ref := range prepared.SourceRefs {
				if prepared.PreserveSuccess || manualRetryResultContains(prepared.EvaluatorResults, ref) {
					prepared.KeepRefIDs = append(prepared.KeepRefIDs, ref.ID)
				}
			}
		}
		item.Turns = append(item.Turns, prepared)
	}
	for _, byTurn := range source.logsByRun {
		for _, log := range byTurn {
			item.SourceTurnLogs = append(item.SourceTurnLogs, log)
		}
	}
	sort.Slice(item.SourceTurnLogs, func(i, j int) bool {
		a, b := item.SourceTurnLogs[i], item.SourceTurnLogs[j]
		if a.ExptRunID != b.ExptRunID {
			return a.ExptRunID < b.ExptRunID
		}
		return a.TurnID < b.TurnID
	})
	return nil
}

func manualRetryResultContains(results *entity.EvaluatorResults, ref *entity.ExptTurnEvaluatorResultRef) bool {
	if results == nil {
		return false
	}
	if ref.SourceType == int32(entity.EvaluatorRecordSourceTypeInline) {
		for _, r := range results.Inline {
			if r != nil && r.RecordID == ref.EvaluatorResultID && r.InlineKey == ref.InlineKey {
				return true
			}
		}
		return false
	}
	for _, r := range results.Registered {
		if r != nil && r.RecordID == ref.EvaluatorResultID && r.VersionID == ref.EvaluatorVersionID && r.Alias == ref.Alias {
			return true
		}
	}
	return ref.Alias == "" && results.EvalVerIDToResID[ref.EvaluatorVersionID] == ref.EvaluatorResultID
}

type manualRetryTargetReader struct {
	IEvalTargetService
	versionID int64
	source    *retryEvaluatorSourceLogs
	turn      *entity.ExptTurnResult
}

func (r *manualRetryTargetReader) GetRecordByID(ctx context.Context, spaceID, recordID int64) (*entity.EvalTargetRecord, error) {
	if r.IEvalTargetService == nil {
		return nil, fmt.Errorf("manual retry target service missing")
	}
	record, err := r.IEvalTargetService.GetRecordByID(ctx, spaceID, recordID)
	if err != nil || record == nil {
		return record, err
	}
	if record.SpaceID != spaceID || record.ID != recordID || record.TargetVersionID != r.versionID || record.ExperimentRunID <= 0 || record.ItemID != r.turn.ItemID || record.TurnID != r.turn.TurnID {
		return nil, fmt.Errorf("manual retry target source mismatch")
	}
	if record.Status != nil && *record.Status == entity.EvalTargetRunStatusSuccess {
		byTurn, err := r.source.loadRun(ctx, record.ExperimentRunID)
		if err != nil {
			return nil, err
		}
		origin := byTurn[r.turn.TurnID]
		if origin == nil || origin.TargetResultID != recordID {
			return nil, fmt.Errorf("manual retry target source log mismatch")
		}
	}
	return record, nil
}

func normalizeCurrentRunRetry(req *entity.RetryInCurrentRunRequest) (*entity.RetryInCurrentRunRequest, error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid manual retry request"))
	}
	normalized := *req
	normalized.Ext = maps.Clone(req.Ext)
	selected := make(map[int64]bool, len(req.ItemIDs))
	for _, id := range req.ItemIDs {
		selected[id] = true
	}
	normalized.ItemIDs = make([]int64, 0, len(selected))
	for id := range selected {
		normalized.ItemIDs = append(normalized.ItemIDs, id)
	}
	sort.Slice(normalized.ItemIDs, func(i, j int) bool { return normalized.ItemIDs[i] < normalized.ItemIDs[j] })
	return &normalized, nil
}

func (m *ExptMangerImpl) RetryInCurrentRun(ctx context.Context, req *entity.RetryInCurrentRunRequest) (*entity.ManualRetryReceipt, error) {
	normalized, err := normalizeCurrentRunRetry(req)
	if err != nil {
		return nil, err
	}
	if m.manualRetryRepo == nil {
		return nil, fmt.Errorf("manual retry repository unavailable")
	}
	if normalized.ExpectedRunID == 0 {
		current, err := m.exptRepo.GetByID(ctx, normalized.ExptID, normalized.SpaceID)
		if err != nil {
			return nil, err
		}
		if current == nil || current.LatestRunID <= 0 {
			return nil, errorx.NewByCode(errno.ManualRetryRunConflictCode)
		}
		normalized.ExpectedRunID = current.LatestRunID
	}
	if err := validateCurrentRunRetry(normalized); err != nil {
		return nil, err
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	expt, err := m.exptRepo.GetByID(ctx, normalized.ExptID, normalized.SpaceID)
	if err != nil {
		return nil, err
	}
	if err := validateCurrentRetryExperiment(expt, normalized); err != nil {
		return nil, err
	}
	plan, err := m.prepareTerminalCandidatesAndReusableRefs(ctx, expt, normalized.ExpectedRunID, normalized.Mode, normalized.ItemIDs, m.evaluatorRecords)
	if err != nil {
		return nil, err
	}
	if m.manualRetryLocker == nil {
		return nil, fmt.Errorf("manual retry locker unavailable")
	}
	locker := m.manualRetryLocker()
	if locker == nil {
		return nil, fmt.Errorf("manual retry locker unavailable")
	}
	locked, lockCtx, cancel, err := locker.LockWithRenew(ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", expt.ID, normalized.ExpectedRunID), 5*time.Second, manualRetryLockMaxHold)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errorx.NewByCode(errno.ManualRetryBusyCode)
	}
	defer cancel()
	if err := lockCtx.Err(); err != nil {
		return nil, err
	}
	expt, err = m.exptRepo.GetByID(lockCtx, normalized.ExptID, normalized.SpaceID)
	if err != nil {
		return nil, err
	}
	if err := validateCurrentRetryExperiment(expt, normalized); err != nil {
		return nil, err
	}
	run, err := m.runLogRepo.Get(lockCtx, expt.ID, normalized.ExpectedRunID)
	if err != nil {
		return nil, err
	}
	if run == nil {
		return nil, fmt.Errorf("manual retry run missing")
	}
	if run.SpaceID != expt.SpaceID || run.ExptID != expt.ID || run.ExptRunID != normalized.ExpectedRunID {
		return nil, errorx.NewByCode(errno.ManualRetryRunConflictCode)
	}
	if run.Status != int64(entity.ExptStatus_Pending) && run.Status != int64(entity.ExptStatus_Processing) {
		return nil, errorx.NewByCode(errno.ManualRetryRunFinishedCode)
	}
	opCtx, stop := context.WithCancel(lockCtx)
	defer stop()
	held := make([]context.Context, 0, len(plan.Items))
	candidates := make([]*entity.PreparedRetryItem, 0, len(plan.Items))
	for _, item := range plan.Items {
		if err := opCtx.Err(); err != nil {
			return nil, err
		}
		acquired, itemCtx, itemCancel, err := locker.LockWithRenew(opCtx, fmt.Sprintf("expt_item_eval_run_lock:%d:%d", expt.ID, item.SourceItem.ItemID), 5*time.Second, manualRetryLockMaxHold)
		if err != nil {
			return nil, err
		}
		if !acquired {
			plan.RejectedItems = append(plan.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.SourceItem.ItemID, Reason: "busy"})
			continue
		}
		defer itemCancel()
		unregister := context.AfterFunc(itemCtx, stop)
		defer unregister()
		held = append(held, itemCtx)
		if err := itemCtx.Err(); err != nil {
			return nil, err
		}
		candidates = append(candidates, item)
	}
	checkAllLocks := func() error {
		if err := opCtx.Err(); err != nil {
			return err
		}
		for _, ctx := range held {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		return nil
	}
	// The acceptance loop can run tens of thousands of releases. A full lock
	// sweep per item is O(n^2); opCtx is canceled as soon as any per-item lease
	// is lost, and a full sweep still guards every commit boundary.
	releaseChecks := 0
	checkLocks := func() error {
		if err := opCtx.Err(); err != nil {
			return err
		}
		releaseChecks++
		if releaseChecks%1024 == 0 {
			return checkAllLocks()
		}
		return nil
	}
	if err := checkAllLocks(); err != nil {
		return nil, err
	}
	operator := ""
	if normalized.Session != nil {
		operator = normalized.Session.UserID
	}
	commit := &entity.RequeueTerminalItemsRequest{SpaceID: expt.SpaceID, ExptID: expt.ID, RunID: normalized.ExpectedRunID, RequestID: uuid.NewString(), Operator: operator, Mode: normalized.Mode, SnapshotAtMS: plan.SnapshotAtMS, Items: candidates, RejectedItems: plan.RejectedItems}
	checked, err := m.manualRetryRepo.PreflightTerminalItems(opCtx, commit)
	if err != nil {
		return nil, err
	}
	commit.Items = nil
	commit.RejectedItems = checked.RejectedItems
	for _, item := range checked.Items {
		if err := checkLocks(); err != nil {
			return nil, err
		}
		if expt.ExptDispatchMode == entity.ExptDispatchModeEnforce {
			if m.centralGuard == nil || expt.SchedulerScope == "" {
				return nil, fmt.Errorf("terminal reservation release unavailable")
			}
			released, err := m.centralGuard.ReleaseTerminalReservation(opCtx, expt.SchedulerScope, item.PreviousRunID, item.SourceItem.ItemID, item.PreviousRequestID)
			if err != nil {
				return nil, err
			}
			switch released {
			case component.TerminalReservationReleased, component.TerminalReservationAbsent:
			case component.TerminalReservationMismatch:
				commit.RejectedItems = append(commit.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: item.SourceItem.ItemID, Reason: "reservation_mismatch"})
				continue
			default:
				return nil, fmt.Errorf("invalid terminal reservation release result")
			}
		}
		commit.Items = append(commit.Items, item)
	}
	if err := checkAllLocks(); err != nil {
		return nil, err
	}
	receipt, err := m.manualRetryRepo.RequeueTerminalItems(opCtx, commit)
	if err != nil {
		return nil, err
	}
	cancel()
	stop()
	if err := m.wakeCurrentRetryRun(ctx, expt, run); err != nil {
		logs.CtxWarn(ctx, "wake current retry run failed, expt_id=%d, run_id=%d, err=%v", expt.ID, run.ExptRunID, err)
	}
	if receipt != nil {
		receipt.RunID = normalized.ExpectedRunID
	}
	return receipt, nil
}

func validateCurrentRunRetry(req *entity.RetryInCurrentRunRequest) error {
	// ExpectedRunID 允许为 0：调用方不关心具体 run 时按「当前 run」处理，由 RetryInCurrentRun 解析。
	if req.SpaceID <= 0 || req.ExptID <= 0 || req.ExpectedRunID < 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid manual retry identity"))
	}
	if req.Mode != entity.EvaluationModeFailRetry && req.Mode != entity.EvaluationModeRetryItems {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("unsupported manual retry mode"))
	}
	if (req.Mode == entity.EvaluationModeFailRetry && len(req.ItemIDs) != 0) || (req.Mode == entity.EvaluationModeRetryItems && len(req.ItemIDs) == 0) {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid manual retry selection"))
	}
	for _, id := range req.ItemIDs {
		if id <= 0 {
			return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("invalid retry item id"))
		}
	}
	if _, exists := req.Ext[entity.ManualRetryExtKey]; exists {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("manual retry ext is internal"))
	}
	return nil
}

func validateCurrentRetryExperiment(expt *entity.Experiment, req *entity.RetryInCurrentRunRequest) error {
	if expt == nil {
		return fmt.Errorf("manual retry experiment missing")
	}
	if expt.ID != req.ExptID || expt.SpaceID != req.SpaceID || expt.LatestRunID != req.ExpectedRunID {
		return errorx.NewByCode(errno.ManualRetryRunConflictCode)
	}
	if expt.ExptType != entity.ExptType_Offline {
		return errorx.NewByCode(errno.ManualRetryUnsupportedTypeCode)
	}
	if expt.Status != entity.ExptStatus_Processing {
		return errorx.NewByCode(errno.ManualRetryRunFinishedCode)
	}
	return nil
}

func (m *ExptMangerImpl) wakeCurrentRetryRun(ctx context.Context, expt *entity.Experiment, _ *entity.ExptRunLog) error {
	if expt.ExptDispatchMode == entity.ExptDispatchModeEnforce {
		if m.currentRunWaker == nil {
			return fmt.Errorf("central current run wake unavailable")
		}
		return m.currentRunWaker.WakeCurrentRun(ctx, expt.SchedulerScope)
	}
	// The original periodic tick retains the frozen Ext and Session.
	return fmt.Errorf("original schedule event snapshot unavailable")
}
