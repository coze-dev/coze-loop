// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sort"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func (e *ExptMangerImpl) LogRetryItemsRunFromSource(ctx context.Context, exptID, expectedSourceRunID, spaceID int64, itemIDs []int64, session *entity.Session) (*entity.RetryItemsRunReceipt, error) {
	if exptID <= 0 || expectedSourceRunID <= 0 || spaceID <= 0 || len(itemIDs) == 0 {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
	}
	unique := make(map[int64]bool, len(itemIDs))
	for _, id := range itemIDs {
		if id <= 0 {
			return nil, errorx.NewByCode(errno.CommonInvalidParamCode)
		}
		unique[id] = true
	}
	ids := make([]int64, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	receipt := &entity.RetryItemsRunReceipt{}
	runID, _, err := e.logRetryItemsRun(ctx, exptID, entity.EvaluationModeRetryItems, spaceID, ids, session, &expectedSourceRunID, receipt)
	if err != nil {
		return nil, err
	}
	receipt.RunID = runID
	return receipt, nil
}

// Both the lifecycle and source completion locks are held until the new run is logged.
func (e *ExptMangerImpl) prepareSourceRetryItems(ctx context.Context, exptID, expectedRunID, spaceID int64, itemIDs []int64, receipt *entity.RetryItemsRunReceipt) error {
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return err
	}
	if expt == nil || expt.ID != exptID || expt.SpaceID != spaceID || expt.LatestRunID != expectedRunID {
		return errorx.NewByCode(errno.ManualRetryRunConflictCode)
	}
	if expt.ExptType != entity.ExptType_Offline || expt.Status != entity.ExptStatus_Failed {
		return errorx.NewByCode(errno.ManualRetryRunFinishedCode)
	}
	run, err := e.runLogRepo.Get(ctx, exptID, expectedRunID)
	if err != nil {
		return err
	}
	if run == nil || run.SpaceID != spaceID || run.ExptID != exptID || run.ExptRunID != expectedRunID ||
		run.Status != int64(entity.ExptStatus_Failed) || entity.ExptRunMode(run.Mode) < entity.EvaluationModeSubmit || entity.ExptRunMode(run.Mode) > entity.EvaluationModeTrialRun {
		return errorx.NewByCode(errno.ManualRetryRunConflictCode)
	}
	receipt.SnapshotAtMS = time.Now().UnixMilli()
	dispatchByRun := make(map[int64]map[int64]*repo.ExptDispatchObservation)
	for start := 0; start < len(itemIDs); start += 200 {
		ids := itemIDs[start:min(start+200, len(itemIDs))]
		refs, err := e.itemRefRepo.MGetByExptIDAndItemIDs(ctx, spaceID, exptID, ids)
		if err != nil {
			return err
		}
		refByID := make(map[int64]*entity.ExptItemRef, len(refs))
		for _, ref := range refs {
			refByID[ref.ItemID] = ref
		}
		items, err := e.itemResultRepo.MGetItemResults(ctx, exptID, ids, spaceID)
		if err != nil {
			return err
		}
		itemByID := make(map[int64]*entity.ExptItemResult, len(items))
		idsByRun := make(map[int64][]int64)
		queueRuns := make(map[int64]bool)
		for _, item := range items {
			itemByID[item.ItemID] = item
			if item.ExptRunID > 0 {
				idsByRun[item.ExptRunID] = append(idsByRun[item.ExptRunID], item.ItemID)
				queueRuns[item.ExptRunID] = queueRuns[item.ExptRunID] || item.Status == entity.ItemRunState_Queueing
			}
		}
		logByID := make(map[int64]*entity.ExptItemResultRunLog, len(items))
		for runID, ids := range idsByRun {
			logs, err := e.itemResultRepo.MGetItemRunLog(ctx, exptID, runID, ids, spaceID)
			if err != nil {
				return err
			}
			for _, log := range logs {
				logByID[log.ItemID] = log
			}
			if queueRuns[runID] && dispatchByRun[runID] == nil {
				observations, err := e.itemResultRepo.ScanIncompleteDispatchObservations(ctx, spaceID, exptID, runID)
				if err != nil {
					return err
				}
				dispatchByRun[runID] = make(map[int64]*repo.ExptDispatchObservation, len(observations))
				for _, observation := range observations {
					dispatchByRun[runID][observation.ItemID] = observation
				}
			}
		}
		for _, id := range ids {
			ref, item, log := refByID[id], itemByID[id], logByID[id]
			reason := ""
			switch {
			case ref == nil || ref.ID <= 0 || ref.SpaceID != spaceID || ref.ExptID != exptID || ref.EvalSetID <= 0 || ref.EvalSetVersionID <= 0 || (expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig && ref.ItemConfig == nil):
				reason = "item_ref_incomplete"
			case item == nil || item.SpaceID != spaceID || item.ExptID != exptID || item.ExptRunID <= 0 || item.ItemVersionID != ref.ItemVersionID:
				reason = "item_result_incomplete"
			case log == nil || log.SpaceID != spaceID || log.ExptID != exptID || log.ExptRunID != item.ExptRunID || log.ItemVersionID != item.ItemVersionID || log.Status != int32(item.Status):
				reason = "item_run_incomplete"
			case item.Status == entity.ItemRunState_Fail:
			case entity.IsNeverStartedQueuedItem(item, log):
				observation := dispatchByRun[item.ExptRunID][id]
				if observation == nil || observation.Status != int32(entity.ItemRunState_Queueing) || observation.RequestID != "" || observation.QuotaReservationState != entity.QuotaReservationStateNone {
					reason = "item_dispatch_not_pristine"
				}
			default:
				reason = "item_not_retryable"
			}
			if reason != "" {
				receipt.RejectedItems = append(receipt.RejectedItems, &entity.ManualRetryRejectedItem{ItemID: id, Reason: reason})
			} else {
				receipt.AcceptedItemIDs = append(receipt.AcceptedItemIDs, id)
			}
		}
	}
	return nil
}
