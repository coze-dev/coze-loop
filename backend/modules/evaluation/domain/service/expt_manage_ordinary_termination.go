// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// IOrdinaryManualCancellation is used only by KillExperiment and its recovery path.
type IOrdinaryManualCancellation interface {
	CanResumeManualCancellation(context.Context, entity.HookRunKey) (bool, error)
	StartManualCancellation(context.Context, entity.HookRunKey, *entity.Session) (bool, error)
	CompleteManualCancellation(context.Context, entity.HookRunKey, *entity.Session) error
}

func (e *ExptMangerImpl) readManualCancellationSource(ctx context.Context, key entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	if e.finalization == nil {
		return nil, nil
	}
	if ctx == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	source, err := e.finalization.Repository.ReadFinalizationSource(contexts.WithCtxWriteDB(ctx), key)
	if err != nil {
		return nil, err
	}
	if source == nil || source.Key != key || source.RunLog == nil || source.Experiment == nil ||
		source.RunLog.ID != key.RunID || source.RunLog.ExptRunID != key.RunID || source.RunLog.ExptID != key.ExperimentID || source.RunLog.SpaceID != key.WorkspaceID ||
		source.Experiment.ID != key.ExperimentID || source.Experiment.SpaceID != key.WorkspaceID {
		return nil, entity.ErrHookStoreCorrupt
	}
	if !source.Managed && source.Experiment.LatestRunID != key.RunID {
		return nil, entity.ErrHookStoreConflict
	}
	return source, nil
}

func (e *ExptMangerImpl) CanResumeManualCancellation(ctx context.Context, key entity.HookRunKey) (bool, error) {
	source, err := e.readManualCancellationSource(ctx, key)
	return manualCancellationPending(source), err
}

// Ordinary Terminating is written only by KillExperiment/StartManualCancellation;
// managed finalization has a separate marker and never enters this path.
func manualCancellationPending(source *entity.HookFinalizationSource) bool {
	return source != nil && !source.Managed && source.Experiment.Status == entity.ExptStatus_Terminating &&
		(source.RunLog.Status == int64(entity.ExptStatus_Terminating) || source.RunLog.Status == int64(entity.ExptStatus_Terminated))
}

func (e *ExptMangerImpl) StartManualCancellation(ctx context.Context, key entity.HookRunKey, session *entity.Session) (bool, error) {
	source, err := e.readManualCancellationSource(ctx, key)
	if err != nil {
		return true, err
	}
	if source == nil || source.Managed {
		return false, nil
	}
	if session == nil || hookExecutionNil(e.publisher) {
		return true, entity.ErrHookStoreCorrupt
	}
	err = e.withManualCancellationFence(ctx, key, func(ctx context.Context, bound *ExptMangerImpl) error {
		var err error
		source, err = bound.readManualCancellationSource(ctx, key)
		if err != nil {
			return err
		}
		if source.Experiment.Status == entity.ExptStatus_Processing &&
			(source.RunLog.Status == int64(entity.ExptStatus_Pending) || source.RunLog.Status == int64(entity.ExptStatus_Processing) || source.RunLog.Status == int64(entity.ExptStatus_Terminating)) {
			if err := bound.SetExptTerminating(ctx, key.ExperimentID, key.RunID, key.WorkspaceID, session); err != nil {
				return err
			}
			source, err = bound.readManualCancellationSource(ctx, key)
			if err != nil {
				return err
			}
		}
		if !manualCancellationPending(source) {
			return entity.ErrHookStoreConflict
		}
		return nil
	})
	if err != nil {
		return true, err
	}
	delay := schedulerHookWaitDelay
	return true, e.publisher.PublishExptScheduleEvent(ctx, &entity.ExptScheduleEvent{
		SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID,
		ExptRunMode: entity.ExptRunMode(source.RunLog.Mode), ExptType: source.Experiment.ExptType,
		Session: session, CreatedAt: time.Now().Unix(),
	}, &delay)
}

func (e *ExptMangerImpl) CompleteManualCancellation(ctx context.Context, key entity.HookRunKey, session *entity.Session) error {
	source, err := e.readManualCancellationSource(ctx, key)
	if err != nil {
		return err
	}
	if source != nil && !source.Managed && source.Experiment.Status == entity.ExptStatus_Terminated && source.RunLog.Status == int64(entity.ExptStatus_Terminated) {
		return nil
	}
	if !manualCancellationPending(source) {
		return entity.ErrHookStoreConflict
	}
	if err := e.closeManualCancellationItems(ctx, source); err != nil {
		return err
	}
	return e.withManualCancellationFence(ctx, key, func(ctx context.Context, bound *ExptMangerImpl) error {
		source, err := bound.readManualCancellationSource(ctx, key)
		if err != nil {
			return err
		}
		if !source.Managed && source.Experiment.Status == entity.ExptStatus_Terminated && source.RunLog.Status == int64(entity.ExptStatus_Terminated) {
			return nil
		}
		if !manualCancellationPending(source) {
			return entity.ErrHookStoreConflict
		}
		lockKey := bound.makeExptMutexLockKey(key.ExperimentID)
		owner, err := bound.finalization.Owners.ReadFinalizationOwner(ctx, lockKey)
		if err != nil {
			return err
		}
		if id, err := managerHookLockRunID(owner); err == nil && id != key.RunID {
			return entity.ErrHookStoreConflict
		} else if err != nil && strings.HasPrefix(owner, "hook_run:") {
			return err
		}
		// Save the Run before releasing creation's mutex; generic CompleteRun unlocks too early.
		run := *source.RunLog
		if err := bound.calculateRunLogStats(ctx, key.ExperimentID, key.RunID, &run, key.WorkspaceID, session); err != nil {
			return err
		}
		run.Status = int64(entity.ExptStatus_Terminated)
		if err := bound.runLogRepo.Save(ctx, &run); err != nil {
			return err
		}
		if err := bound.CompleteExpt(ctx, key.ExperimentID, &key.RunID, key.WorkspaceID, session,
			entity.WithStatus(entity.ExptStatus_Terminated), entity.NoAggrCalculate()); err != nil {
			return err
		}
		// Keep quota, template and notification effects inside the parent fence too.
		if owner != "" {
			_, err = bound.mutex.UnlockWithValue(ctx, lockKey, owner)
		}
		return err
	})
}

func (e *ExptMangerImpl) closeManualCancellationItems(ctx context.Context, source *entity.HookFinalizationSource) error {
	key := source.Key
	ctx = contexts.WithCtxWriteDB(ctx)
	// Release reservations before changing the run-log states used by the quota scan.
	e.releaseCentralQuotaForIncompleteItems(ctx, source.Experiment, &key.RunID, entity.ExptStatus_Terminated)
	const limit = 100
	for cursor := int64(0); ; {
		items, next, err := e.itemResultRepo.ScanItemRunLogs(ctx, key.ExperimentID, key.RunID, nil, cursor, limit, key.WorkspaceID)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item == nil || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID <= 0 {
				return entity.ErrHookStoreCorrupt
			}
			if entity.IsItemRunFinished(entity.ItemRunState(item.Status)) && item.ResultState == int32(entity.ExptItemResultStateResulted) {
				continue
			}
			if err := e.terminateOrdinaryItem(ctx, key, item.ItemID); err != nil {
				return err
			}
		}
		if len(items) < limit {
			return nil
		}
		if next <= cursor {
			return entity.ErrHookStoreCorrupt
		}
		cursor = next
	}
}

func (e *ExptMangerImpl) terminateOrdinaryItem(ctx context.Context, key entity.HookRunKey, itemID int64) (resultErr error) {
	if e.finalization.NewItemLocker == nil {
		return entity.ErrHookFinalizationUnsettled
	}
	locker := e.finalization.NewItemLocker()
	if hookExecutionNil(locker) {
		return entity.ErrHookFinalizationUnsettled
	}
	lockKey := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", key.ExperimentID, itemID)
	locked, lockedCtx, cancel, err := locker.LockWithRenew(ctx, lockKey, 5*time.Second, time.Hour)
	if err != nil {
		return err
	}
	if !locked {
		return entity.ErrHookFinalizationUnsettled
	}
	defer func() {
		cancel()
		if _, err := locker.Unlock(lockKey); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	return e.withManualCancellationFence(lockedCtx, key, func(ctx context.Context, bound *ExptMangerImpl) error {
		source, err := bound.readManualCancellationSource(ctx, key)
		if err != nil {
			return err
		}
		if !manualCancellationPending(source) {
			return entity.ErrHookStoreConflict
		}
		if err := bound.closeOrdinaryItemRun(ctx, key, itemID, entity.ExptRunMode(source.RunLog.Mode)); err != nil {
			return err
		}
		if _, err := bound.exptResultService.RecordItemRunLogs(ctx, key.ExperimentID, key.RunID, itemID, key.WorkspaceID, source.Experiment); err != nil {
			return err
		}
		bound.terminateSandboxExecutesForCancelledItems(ctx, key.WorkspaceID, source.Experiment.TargetSpaceID, key.ExperimentID, &key.RunID, []int64{itemID})
		if indexErr := bound.exptResultService.UpsertExptTurnResultFilter(ctx, key.WorkspaceID, key.ExperimentID, []int64{itemID}); indexErr != nil {
			logs.CtxWarn(ctx, "ordinary cancellation result filter update failed, expt_id: %v, run_id: %v, item_id: %v, err: %v", key.ExperimentID, key.RunID, itemID, indexErr)
		}
		return nil
	})
}

// Preserve finished turns within an unfinished item; row-level TerminateItems cancels all turns.
func (e *ExptMangerImpl) closeOrdinaryItemRun(ctx context.Context, key entity.HookRunKey, itemID int64, mode entity.ExptRunMode) error {
	item, err := e.itemResultRepo.GetItemRunLog(ctx, key.ExperimentID, key.RunID, itemID, key.WorkspaceID)
	if err != nil {
		return err
	}
	if item == nil || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != itemID {
		return entity.ErrHookStoreCorrupt
	}
	switch entity.ItemRunState(item.Status) {
	case entity.ItemRunState_Success, entity.ItemRunState_Fail:
		return nil
	case entity.ItemRunState_Queueing, entity.ItemRunState_Processing, entity.ItemRunState_Terminal:
	default:
		return entity.ErrHookStoreCorrupt
	}
	turns, err := e.itemResultRepo.GetItemTurnResults(ctx, key.WorkspaceID, key.ExperimentID, itemID)
	if err != nil {
		return err
	}
	logs, err := e.turnResultRepo.GetItemTurnRunLogs(ctx, key.ExperimentID, key.RunID, itemID, key.WorkspaceID)
	if err != nil {
		return err
	}
	byTurn := make(map[int64]*entity.ExptTurnResultRunLog, len(logs))
	for _, log := range logs {
		if log == nil || log.SpaceID != key.WorkspaceID || log.ExptID != key.ExperimentID || log.ExptRunID != key.RunID || log.ItemID != itemID || byTurn[log.TurnID] != nil {
			return entity.ErrHookStoreCorrupt
		}
		byTurn[log.TurnID] = log
	}
	message := errno.SerializeErr(errno.NewItemManuallyTerminatedErr())
	var updates []*entity.ExptTurnResultRunLog
	for _, turn := range turns {
		if turn == nil || turn.SpaceID != key.WorkspaceID || turn.ExptID != key.ExperimentID || turn.ItemID != itemID {
			return entity.ErrHookStoreConflict
		}
		historicalSuccess := mode == entity.EvaluationModeFailRetry && turn.ExptRunID > 0 && turn.ExptRunID != key.RunID && turn.Status == int32(entity.TurnRunState_Success)
		if turn.ExptRunID != key.RunID && !historicalSuccess {
			return entity.ErrHookStoreConflict
		}
		log := byTurn[turn.TurnID]
		if log == nil {
			if !historicalSuccess && turn.Status != int32(entity.TurnRunState_Queueing) && turn.Status != int32(entity.TurnRunState_Processing) {
				return entity.ErrHookFinalizationUnsettled
			}
			id, err := e.idgenerator.GenID(ctx)
			if err != nil {
				return err
			}
			// FailRetry's unexecuted turns still carry the reusable target/evaluator snapshot.
			log = turn.ToRunLogDO()
			log.ID, log.ExptRunID, log.ItemVersionID = id, key.RunID, item.ItemVersionID
		}
		if historicalSuccess {
			// PreEval marks reused turns Processing; keep its validated result references.
			copy := *log
			copy.Status, copy.ErrMsg = entity.TurnRunState_Success, turn.ErrMsg
			updates = append(updates, &copy)
			continue
		}
		switch log.Status {
		case entity.TurnRunState_Success, entity.TurnRunState_Fail, entity.TurnRunState_Terminal:
			continue
		case entity.TurnRunState_Queueing, entity.TurnRunState_Processing:
		default:
			return entity.ErrHookStoreCorrupt
		}
		copy := *log
		copy.Status, copy.ErrMsg = entity.TurnRunState_Terminal, message
		updates = append(updates, &copy)
	}
	if len(updates) > 0 {
		if err := e.turnResultRepo.SaveTurnRunLogs(ctx, updates); err != nil {
			return err
		}
	}
	// Publish the item as Logged only after all turn logs are complete, so retry can archive it.
	return e.itemResultRepo.UpdateItemRunLog(ctx, key.ExperimentID, key.RunID, []int64{itemID}, map[string]any{
		"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateLogged), "err_msg": []byte(message),
	}, key.WorkspaceID)
}
