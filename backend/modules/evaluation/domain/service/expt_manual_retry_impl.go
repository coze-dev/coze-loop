// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

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
				return nil, fmt.Errorf("invalid retry item id")
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
		itemLogs, err := m.itemResultRepo.MGetItemRunLog(ctx, expt.ID, runID, []int64{item.ItemID}, expt.SpaceID)
		if err != nil {
			return nil, err
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
		prepared.FrozenItemRef, err = m.itemRefRepo.GetByExptIDAndItemID(ctx, expt.SpaceID, expt.ID, item.ItemID)
		if err != nil {
			return nil, err
		}
		ref := prepared.FrozenItemRef
		if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID || ref.ItemID != item.ItemID || ref.ItemVersionID != item.ItemVersionID {
			return nil, fmt.Errorf("frozen retry item version mismatch")
		}
		prepared.ItemVersionID = ref.ItemVersionID
		if err := m.prepareManualRetryTurns(ctx, expt, runID, mode, prepared, records); err != nil {
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

func (m *ExptMangerImpl) prepareManualRetryTurns(ctx context.Context, expt *entity.Experiment, runID int64, mode entity.ExptRunMode, item *entity.PreparedRetryItem, records EvaluatorRecordService) error {
	turns, err := m.turnResultRepo.GetItemTurnResults(ctx, expt.ID, item.SourceItem.ItemID, expt.SpaceID)
	if err != nil {
		return err
	}
	if len(turns) == 0 {
		return fmt.Errorf("frozen retry turns missing")
	}
	logs, err := m.turnResultRepo.GetItemTurnRunLogs(ctx, expt.ID, runID, item.SourceItem.ItemID, expt.SpaceID)
	if err != nil {
		return err
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
		if turn == nil || turn.SpaceID != expt.SpaceID || turn.ExptID != expt.ID || turn.ItemID != item.SourceItem.ItemID || turn.ExptRunID != item.PreviousRunID || turn.ItemVersionID != item.ItemVersionID {
			return fmt.Errorf("invalid frozen retry turn")
		}
		ids = append(ids, turn.ID)
	}
	refs, err := m.turnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, expt.SpaceID, ids)
	if err != nil {
		return err
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
				if cfg := item.FrozenItemRef.ItemConfig; cfg != nil && cfg.TargetSourceSpaceID > 0 {
					targetSpace = cfg.TargetSourceSpaceID
				}
				targetVersion := expt.TargetVersionID
				if cfg := item.FrozenItemRef.ItemConfig; cfg != nil && cfg.EvalTargetConf != nil {
					targetVersion = cfg.EvalTargetConf.TargetVersionID
				}
				targetRequired := !shouldSkipTargetNode(expt)
				if cfg := item.FrozenItemRef.ItemConfig; cfg != nil && cfg.EvalTargetConf != nil {
					targetRequired = targetVersion > 0
				}
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
