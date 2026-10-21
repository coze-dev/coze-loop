// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// IExptNaturalRunFinisher keeps the broad manager and old test doubles compatible.
//
//go:generate mockgen -destination=mocks/expt_run_finish.go -package=mocks . IExptNaturalRunFinisher
type IExptNaturalRunFinisher interface {
	FinishCurrentRun(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error)
	FinishCurrentRunWithHeldTickLock(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error)
}

func (e *ExptMangerImpl) FinishCurrentRun(ctx context.Context, event *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	if event == nil || e.manualRetryLocker == nil {
		return nil, errno.NewMQRetryErr("finish tick locker missing")
	}
	locker := e.manualRetryLocker()
	if locker == nil {
		return nil, errno.NewMQRetryErr("finish tick locker missing")
	}
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", event.ExptID, event.ExptRunID)
	locked, lockCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if !locked {
		return nil, errno.NewMQRetryErr("finish tick lock busy")
	}
	defer cancel()
	if err := lockCtx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if e.exptRepo == nil {
		return nil, errno.NewMQRetryErr("finish experiment repository missing")
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(lockCtx), event.ExptID, event.SpaceID)
	if err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if current == nil {
		return nil, errno.NewMQRetryErr("finish experiment missing")
	}
	if current.LatestRunID != event.ExptRunID || entity.IsExptFinished(current.Status) {
		return e.FinishCurrentRunWithHeldTickLock(lockCtx, event)
	}
	if calibration, ok := e.exptResultService.(ExptManualRetryFilterService); ok {
		ready, err := reconcileManualRetryFilterBeforeFinish(lockCtx, calibration, event.SpaceID, event.ExptID, event.ExptRunID)
		if err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		if !ready {
			return &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishPending}, nil
		}
	} else {
		return nil, errno.NewMQRetryErr("finish calibration service missing")
	}
	return e.FinishCurrentRunWithHeldTickLock(lockCtx, event)
}

// The scheduler calls this only after calibration under its existing tick lease.
func (e *ExptMangerImpl) FinishCurrentRunWithHeldTickLock(ctx context.Context, event *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	if event == nil || e.runFinishRepo == nil {
		return nil, errno.NewMQRetryErr("current run finish repository missing")
	}
	if err := ctx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	result, err := e.runFinishRepo.FinishCurrentRun(ctx, &entity.ExptRunFinishRequest{SpaceID: event.SpaceID, ExptID: event.ExptID, RunID: event.ExptRunID})
	if err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if result == nil {
		return nil, errno.NewMQRetryErr("finish returned no disposition")
	}
	switch result.Disposition {
	case entity.ExptRunFinishPending, entity.ExptRunFinishNonCurrent:
		return result, nil
	case entity.ExptRunFinishFinished, entity.ExptRunFinishAlreadyFinished:
		if err := e.cleanupNaturallyFinishedRun(ctx, event, result.Experiment); err != nil {
			return result, errno.WrapMQRetryErr(err)
		}
		return result, nil
	default:
		return nil, errno.NewMQRetryErr("invalid finish disposition")
	}
}

func (e *ExptMangerImpl) cleanupNaturallyFinishedRun(ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment) error {
	if finished == nil {
		return fmt.Errorf("finished experiment missing")
	}
	if e.manualRetryLocker == nil {
		return fmt.Errorf("finish lifecycle locker missing")
	}
	locker := e.manualRetryLocker()
	if locker == nil {
		return fmt.Errorf("finish lifecycle locker missing")
	}
	key := e.makeExptMutexLockKey(event.ExptID)
	// Remove only this run's original value, then acquire the same lifecycle key.
	// If R2 wins the gap we do no cleanup. A legacy random holder is never forced.
	if _, err := locker.UnlockWithValue(ctx, key, strconv.FormatInt(event.ExptRunID, 10)); err != nil {
		return err
	}
	locked, cleanupCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return err
	}
	if !locked {
		return errno.NewMQRetryErr("finish lifecycle owner unavailable; legacy random holders require expiry")
	}
	defer cancel()
	if err := cleanupCtx.Err(); err != nil {
		return err
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(cleanupCtx), event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("finished experiment disappeared")
	}
	if current.LatestRunID != event.ExptRunID {
		return nil
	}
	if !entity.IsExptFinished(current.Status) || current.EndAt == nil {
		return fmt.Errorf("finish cleanup observed active experiment")
	}
	// Every external action remains under this lifecycle lease, which also blocks
	// new LogRun/LogRetryItemsRun. The SQL finish transaction has already committed.
	if err := NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(cleanupCtx, event.ExptID, event.SpaceID, event.Session); err != nil {
		return err
	}
	if err := cleanupCtx.Err(); err != nil {
		return err
	}
	if e.exptResultService != nil {
		incomplete, err := e.exptResultService.GetIncompleteTurns(cleanupCtx, event.ExptID, event.SpaceID, event.Session)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(incomplete))
		seen := map[int64]bool{}
		for _, turn := range incomplete {
			if !seen[turn.ItemID] {
				ids = append(ids, turn.ItemID)
				seen[turn.ItemID] = true
			}
		}
		if len(ids) > 0 {
			e.terminateSandboxExecutesForCancelledItems(cleanupCtx, event.SpaceID, current.TargetSpaceID, event.ExptID, &event.ExptRunID, ids)
		}
	}
	if err := cleanupCtx.Err(); err != nil {
		return err
	}

	// Resources above remain replayable. Only the completed notification segment
	// is deduplicated, under the same lifecycle lease that serializes retries.
	if e.idem == nil {
		return errno.NewMQRetryErr("finish notification idempotency service missing")
	}
	notificationKey := fmt.Sprintf("finish-cleanup:%d:%d", event.ExptID, event.ExptRunID)
	seen, err := e.idem.Exist(cleanupCtx, notificationKey)
	if err != nil {
		return err
	}
	if seen {
		return cleanupCtx.Err()
	}

	if current.ExptTemplateMeta != nil && current.ExptTemplateMeta.ID > 0 && e.templateManager != nil {
		if err := e.templateManager.UpdateExptInfo(cleanupCtx, current.ExptTemplateMeta.ID, event.SpaceID, event.ExptID, current.Status, 0, nil); err != nil {
			return err
		}
	}
	if e.exptAggrResultService != nil {
		if err := e.exptAggrResultService.PublishExptAggrResultEvent(cleanupCtx, &entity.AggrCalculateEvent{ExperimentID: event.ExptID, SpaceID: event.SpaceID, CalculateMode: entity.CreateAllFields}, gptr.Of(3*time.Second)); err != nil {
			return err
		}
	}
	if err := cleanupCtx.Err(); err != nil {
		return err
	}
	if e.publisher != nil {
		// Stable Processing -> final key survives cleanup retries without DB markers.
		lifecycle := &entity.ExptLifecycleEvent{ExptID: event.ExptID, ExptRunID: &event.ExptRunID, SpaceID: event.SpaceID, FromStatus: entity.ExptStatus_Processing, ToStatus: current.Status, ExptType: current.ExptType, SourceType: current.SourceType}
		lifecycle.IdempotentKey = fmt.Sprintf("expt_%d_%d_%d_%d", event.ExptID, event.ExptRunID, lifecycle.FromStatus, lifecycle.ToStatus)
		if err := e.publisher.PublishExptLifecycleEvent(cleanupCtx, lifecycle, gptr.Of(3*time.Second), lifecycle.IdempotentKey); err != nil {
			return err
		}
	}
	if err := cleanupCtx.Err(); err != nil {
		return err
	}
	if current.SourceType == entity.SourceType_Workflow {
		e.notifyWorkflowPipelineOnExptFinished(cleanupCtx, current, event.SpaceID, current.Status)
	}

	if e.mtr != nil {
		e.mtr.EmitExptExecResult(event.SpaceID, int64(current.ExptType), int64(current.Status), gptr.Indirect(current.StartAt))
	}
	e.emitSandboxAgentExperimentFinished(cleanupCtx, current, current.Status, gptr.Indirect(current.EndAt))
	if err := cleanupCtx.Err(); err != nil {
		return err
	}
	duration := 3 * time.Minute
	if e.configer != nil {
		duration = time.Duration(e.configer.GetExptExecConf(cleanupCtx, event.SpaceID).GetZombieIntervalSecond()) * time.Second * 2
	}
	// A crash or Set failure after RPC success can replay this segment; the key
	// bounds normal duplicates, it does not provide distributed exactly-once.
	return e.idem.Set(cleanupCtx, notificationKey, duration)
}
