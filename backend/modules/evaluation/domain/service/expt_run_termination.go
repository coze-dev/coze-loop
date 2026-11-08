// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// IExptCurrentRunTerminator accepts the execution source before a worker leaves
// its item lease. The caller must release that lease before calling this port.
//
//go:generate mockgen -destination=mocks/expt_run_termination.go -package=mocks . IExptCurrentRunTerminator
type IExptCurrentRunTerminator interface {
	TerminateCurrentRun(context.Context, int64, int64, int64, *entity.Session, *entity.ExptCurrentItemRunSource, ...entity.CompleteExptOptionFn) (bool, error)
}

func (e *ExptMangerImpl) terminationRepo() (repo.IExptRunTerminationRepo, error) {
	port, ok := e.runFinishRepo.(repo.IExptRunTerminationRepo)
	if !ok || e.manualRetryLocker == nil {
		return nil, errno.NewMQRetryErr("current run termination provider missing")
	}
	return port, nil
}

func (e *ExptMangerImpl) withTerminationRunLock(ctx context.Context, expt, run int64, f func(context.Context) error) (err error) {
	if e.manualRetryLocker == nil {
		return errno.NewMQRetryErr("termination tick locker missing")
	}
	locker := e.manualRetryLocker()
	if locker == nil {
		return errno.NewMQRetryErr("termination tick locker missing")
	}
	key := fmt.Sprintf("expt_run_exec_lock:%d:%d", expt, run)
	locked, lockCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !locked {
		return errno.NewMQRetryErr("termination tick lock busy")
	}
	defer func() {
		cancel()
		_, unlockErr := locker.Unlock(key)
		err = errors.Join(err, unlockErr)
	}()
	if err := lockCtx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	return f(lockCtx)
}

func (e *ExptMangerImpl) beginCurrentRunTermination(ctx context.Context, expt, run, space int64) error {
	port, err := e.terminationRepo()
	if err != nil {
		return err
	}
	begin := func(ctx context.Context) error {
		result, err := port.TerminateCurrentRun(ctx, &entity.ExptRunTerminationRequest{SpaceID: space, ExptID: expt, RunID: run, Begin: true})
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if result == nil {
			return errno.NewMQRetryErr("termination begin result missing")
		}
		if result.NonCurrent {
			return errno.NewMQRetryErr("cancellation run replaced")
		}
		return nil
	}
	if run != 0 {
		return e.withTerminationRunLock(ctx, expt, run, begin)
	}
	// 首次 LogRun 也持有此实验锁；零 run 的取消不得与登记新 run 交错。
	locker := e.manualRetryLocker()
	if locker == nil {
		return errno.NewMQRetryErr("termination lifecycle locker missing")
	}
	key := e.makeExptMutexLockKey(expt)
	locked, leaseCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !locked {
		return errno.NewMQRetryErr("initial run admission in progress")
	}
	err = leaseCtx.Err()
	if err == nil {
		err = begin(leaseCtx)
	}
	cancel()
	_, unlockErr := locker.Unlock(key)
	return errors.Join(err, unlockErr)
}

func (e *ExptMangerImpl) captureTerminationSources(ctx context.Context, port repo.IExptRunTerminationRepo, space, expt, run int64, ids []int64) ([]*entity.ExptCurrentItemRunSource, error) {
	seen := map[int64]bool{}
	ordered := []int64{}
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid termination item")
		}
		if !seen[id] {
			ordered = append(ordered, id)
			seen[id] = true
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	sources := make([]*entity.ExptCurrentItemRunSource, 0, len(ordered))
	for _, id := range ordered {
		s, err := port.GetTerminationItemSource(ctx, space, expt, run, id)
		if err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		if s == nil {
			return nil, errno.NewMQRetryErr("termination source missing")
		}
		sources = append(sources, s)
	}
	return sources, nil
}

func (e *ExptMangerImpl) commitTerminationItems(ctx context.Context, port repo.IExptRunTerminationRepo, request *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
	opCtx, stop := context.WithCancel(ctx)
	defer stop()
	for _, source := range request.Items {
		locker := e.manualRetryLocker()
		if locker == nil {
			return nil, errno.NewMQRetryErr("termination item locker missing")
		}
		if err := opCtx.Err(); err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", request.ExptID, source.Current.ItemID)
		locked, itemCtx, cancel, err := locker.LockWithRenew(opCtx, key, 5*time.Second, 5*time.Minute)
		if err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		if !locked {
			return nil, errno.NewMQRetryErr("termination item lease busy")
		}
		defer cancel()
		unregister := context.AfterFunc(itemCtx, stop)
		defer unregister()
		if err := itemCtx.Err(); err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		if source.Current.Status == entity.ItemRunState_Success || source.Current.Status == entity.ItemRunState_Fail {
			continue
		}
		write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Terminal}
		if err := prepareMissingRawTurnLogs(opCtx, e.idgenerator, source, write); err != nil {
			return nil, err
		}
		request.MissingTurnLogs = append(request.MissingTurnLogs, write.MissingTurnLogs...)
	}
	result, err := port.TerminateCurrentRun(opCtx, request)
	if err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if result == nil {
		return nil, errno.NewMQRetryErr("termination result missing")
	}
	return result, nil
}

func (e *ExptMangerImpl) terminateCurrentItems(ctx context.Context, expt, run, space int64, ids []int64, session *entity.Session) error {
	port, err := e.terminationRepo()
	if err != nil {
		return err
	}
	// Preserve the request's original source even when admission wins the run lease.
	sources, err := e.captureTerminationSources(ctx, port, space, expt, run, ids)
	if err != nil {
		return err
	}
	return e.withTerminationRunLock(ctx, expt, run, func(ctx context.Context) error {
		current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if current == nil {
			return errno.NewMQRetryErr("termination experiment missing")
		}
		if current.LatestRunID != run {
			return nil
		}
		if current.ExptType != entity.ExptType_Offline || current.Status != entity.ExptStatus_Processing {
			return errno.NewMQRetryErr("row termination requires processing offline experiment")
		}
		result, err := e.commitTerminationItems(ctx, port, &entity.ExptRunTerminationRequest{SpaceID: space, ExptID: expt, RunID: run, Items: sources})
		if err != nil {
			return err
		}
		if result.NonCurrent {
			return nil
		}
		resourceCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		defer cancel()
		if err := e.releaseTerminationResources(resourceCtx, current, run, sources, false, session); err != nil {
			return err
		}
		return nil
	})
}

func (e *ExptMangerImpl) TerminateCurrentRun(ctx context.Context, expt, run, space int64, session *entity.Session, expected *entity.ExptCurrentItemRunSource, opts ...entity.CompleteExptOptionFn) (bool, error) {
	port, err := e.terminationRepo()
	if err != nil {
		return false, err
	}
	opt := &entity.CompleteExptOption{Status: entity.ExptStatus_Terminated}
	for _, fn := range opts {
		fn(opt)
	}
	if run == 0 {
		if err := e.beginCurrentRunTermination(ctx, expt, run, space); err != nil {
			return false, err
		}
	}
	applied := false
	err = e.withTerminationRunLock(ctx, expt, run, func(ctx context.Context) error {
		current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if current == nil {
			return errno.NewMQRetryErr("termination experiment missing")
		}
		if current.LatestRunID != run {
			return nil
		}
		if expected != nil {
			s, err := port.GetTerminationItemSource(ctx, space, expt, run, expected.Current.ItemID)
			if err != nil {
				return errno.WrapMQRetryErr(err)
			}
			if !sameExecutionIdentity(expected, s) {
				return nil
			}
		}
		// Include terminal logs on replay, so residual resource debt remains visible.
		var ids []int64
		var cursor int64
		for {
			logs, next, err := e.itemResultRepo.ScanItemRunLogs(contexts.WithCtxWriteDB(ctx), expt, run, &entity.ExptItemRunLogFilter{}, cursor, 500, space)
			if err != nil {
				return errno.WrapMQRetryErr(err)
			}
			for _, log := range logs {
				if log == nil {
					return errno.NewMQRetryErr("termination log missing")
				}
				ids = append(ids, log.ItemID)
			}
			if next == 0 || len(logs) == 0 {
				break
			}
			if next <= cursor {
				return errno.NewMQRetryErr("termination scan cursor invalid")
			}
			cursor = next
		}
		sources, err := e.captureTerminationSources(ctx, port, space, expt, run, ids)
		if err != nil {
			return err
		}
		result, err := e.commitTerminationItems(ctx, port, &entity.ExptRunTerminationRequest{SpaceID: space, ExptID: expt, RunID: run, Finalize: true, Status: opt.Status, StatusMessage: opt.StatusMessage, NoCompleteItemTurn: opt.NoCompleteItemTurn, ExpectedSource: expected, Items: sources})
		if err != nil {
			return err
		}
		if result.NonCurrent {
			if expected == nil {
				return errno.NewMQRetryErr("termination source advanced while acquiring item leases")
			}
			return nil
		}
		if result.Experiment == nil {
			return errno.NewMQRetryErr("terminated experiment missing")
		}
		applied = true
		return e.cleanupTerminatedRun(ctx, &entity.ExptScheduleEvent{SpaceID: space, ExptID: expt, ExptRunID: run, Session: session}, result.Experiment, sources, opt)
	})
	return applied, err
}

func (e *ExptMangerImpl) releaseTerminationResources(ctx context.Context, expt *entity.Experiment, run int64, sources []*entity.ExptCurrentItemRunSource, whole bool, session *entity.Session) error {
	seen := map[int64]bool{}
	records := []int64{}
	for _, source := range sources {
		if source == nil {
			return fmt.Errorf("termination resource source missing")
		}
		if !whole && (source.Current.Status == entity.ItemRunState_Success || source.Current.Status == entity.ItemRunState_Fail) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if e.centralGuard != nil && entity.IsCentralDispatch(expt.ExptDispatchMode) {
			if expt.SchedulerScope == "" {
				return fmt.Errorf("termination scope missing")
			}
			// Each ledger IO has its own budget; a healthy large run must reach
			// its tail. The caller's lease/cancellation still bounds the loop.
			releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			err := e.centralGuard.Release(releaseCtx, expt.SchedulerScope, run, source.Current.ItemID, source.Current.RequestID, "current run item terminated")
			if err == nil {
				err = releaseCtx.Err()
			}
			cancel()
			if err != nil {
				return errno.WrapMQRetryErr(err)
			}
		}
		if whole || !entity.IsItemRunFinished(source.Current.Status) || source.Current.Status == entity.ItemRunState_Terminal {
			frozen := frozenTargetRecordIDs(source)
			for _, turn := range source.CanonicalTurns {
				if turn != nil && turn.TargetResultID > 0 {
					frozen = append(frozen, turn.TargetResultID)
				}
			}
			for _, id := range frozen {
				if !seen[id] {
					records = append(records, id)
					seen[id] = true
				}
			}
		}
	}
	if e.evalTargetService != nil && len(records) > 0 {
		releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		_ = e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(releaseCtx, resolveLoadSpaceID(expt.SpaceID, expt.TargetSpaceID), records, int32(errno.AsyncEvalTargetTerminatedCode), "async eval target terminated: experiment cancelled", false)
		err := releaseCtx.Err()
		cancel()
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
	}
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if whole && e.quotaRepo != nil {
		releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(releaseCtx, expt.ID, expt.SpaceID, session)
	}
	return nil
}

func (e *ExptMangerImpl) cleanupTerminatedRun(ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment, sources []*entity.ExptCurrentItemRunSource, opt *entity.CompleteExptOption) error {
	// Only resources can outlive cancellation. SQL, scores and MQ retain ctx.
	resourceCtx, resourceCancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
	defer resourceCancel()
	if finished == nil || finished.ID != event.ExptID || finished.LatestRunID != event.ExptRunID {
		return errno.NewMQRetryErr("termination cleanup frozen experiment missing")
	}
	locker := e.manualRetryLocker()
	if locker == nil {
		return errno.NewMQRetryErr("termination lifecycle locker missing")
	}
	key := e.makeExptMutexLockKey(event.ExptID)
	if err := e.releasePreviousLifecycleHolder(resourceCtx, locker, key, event); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if err := ctx.Err(); err != nil {
		return e.cleanupCancelledTerminationResources(ctx, event, finished, sources)
	}
	locked, leaseCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !locked {
		return errno.NewMQRetryErr("termination lifecycle owner unavailable")
	}
	defer cancel()
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(leaseCtx), event.ExptID, event.SpaceID)
	if err != nil {
		if ctx.Err() != nil {
			cancel()
			if _, unlockErr := locker.Unlock(key); unlockErr != nil {
				return errno.WrapMQRetryErr(unlockErr)
			}
			return e.cleanupCancelledTerminationResources(ctx, event, finished, sources)
		}
		return errno.WrapMQRetryErr(err)
	}
	if current == nil {
		return errno.NewMQRetryErr("termination cleanup experiment missing")
	}
	if current.LatestRunID != event.ExptRunID {
		return nil
	}
	if !entity.IsExptFinished(current.Status) {
		return errno.NewMQRetryErr("termination cleanup observed active run")
	}
	if err := e.releaseTerminationResources(leaseCtx, current, event.ExptRunID, sources, true, event.Session); err != nil {
		if ctx.Err() != nil {
			cancel()
			if _, unlockErr := locker.Unlock(key); unlockErr != nil {
				return errno.WrapMQRetryErr(unlockErr)
			}
			return e.cleanupCancelledTerminationResources(ctx, event, current, sources)
		}
		return err
	}
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	opCtx, stop := context.WithCancel(ctx)
	defer stop()
	unregister := context.AfterFunc(leaseCtx, stop)
	defer unregister()
	if err := opCtx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !opt.NoCompleteItemTurn {
		if err := e.refreshTerminatedRunFilters(opCtx, event, sources); err != nil {
			return errno.WrapMQRetryErr(err)
		}
	}
	if e.idem == nil {
		return errno.NewMQRetryErr("termination notification idempotency service missing")
	}
	notificationKey := fmt.Sprintf("finish-cleanup:%d:%d", event.ExptID, event.ExptRunID)
	seen, err := e.idem.Exist(opCtx, notificationKey)
	if err != nil {
		return err
	}
	if seen {
		return nil
	}
	if current.ExptTemplateMeta != nil && current.ExptTemplateMeta.ID > 0 && e.templateManager != nil {
		if err := e.templateManager.UpdateExptInfo(opCtx, current.ExptTemplateMeta.ID, event.SpaceID, event.ExptID, current.Status, 0, nil); err != nil {
			return err
		}
	}
	if !opt.NoAggrCalculate && e.exptAggrResultService != nil {
		if err := e.exptAggrResultService.PublishExptAggrResultEvent(opCtx, &entity.AggrCalculateEvent{ExperimentID: event.ExptID, SpaceID: event.SpaceID, CalculateMode: entity.CreateAllFields}, gptr.Of(3*time.Second)); err != nil {
			return err
		}
	}
	if err := opCtx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if e.publisher != nil {
		lifecycle := &entity.ExptLifecycleEvent{SpaceID: event.SpaceID, ExptID: event.ExptID, ExptRunID: &event.ExptRunID, FromStatus: entity.ExptStatus_Processing, ToStatus: current.Status, ExptType: current.ExptType, SourceType: current.SourceType}
		lifecycle.IdempotentKey = fmt.Sprintf("expt_%d_%d_%d_%d", event.ExptID, event.ExptRunID, lifecycle.FromStatus, lifecycle.ToStatus)
		if err := e.publisher.PublishExptLifecycleEvent(opCtx, lifecycle, gptr.Of(3*time.Second), lifecycle.IdempotentKey); err != nil {
			return err
		}
	}
	if current.SourceType == entity.SourceType_Workflow {
		e.notifyWorkflowPipelineOnExptFinished(opCtx, current, event.SpaceID, current.Status)
	}
	if e.mtr != nil {
		e.mtr.EmitExptExecResult(event.SpaceID, int64(current.ExptType), int64(current.Status), gptr.Indirect(current.StartAt))
	}
	e.emitSandboxAgentExperimentFinished(opCtx, current, current.Status, gptr.Indirect(current.EndAt))
	if err := opCtx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	return e.idem.Set(opCtx, notificationKey, 3*time.Minute)
}

func (e *ExptMangerImpl) cleanupCancelledTerminationResources(ctx context.Context, event *entity.ExptScheduleEvent, frozen *entity.Experiment, sources []*entity.ExptCurrentItemRunSource) error {
	// Cancellation permits only a primary ownership read and frozen resource IO.
	// Scores and MQ retain ctx. A separate holder prevents the cancelled
	// operation's asynchronous unlock from deleting this lease.
	resourceCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
	defer stop()
	locker := e.manualRetryLocker()
	if locker == nil {
		return errno.NewMQRetryErr("termination resource locker missing")
	}
	locked, leaseCtx, cancel, err := locker.LockWithRenew(resourceCtx, e.makeExptMutexLockKey(event.ExptID), 5*time.Second, 8*time.Second)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !locked {
		return errno.NewMQRetryErr("termination resource owner unavailable")
	}
	defer cancel()
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(leaseCtx), event.ExptID, event.SpaceID)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if current == nil {
		return errno.NewMQRetryErr("termination resource experiment missing")
	}
	if current.LatestRunID != event.ExptRunID {
		return errno.WrapMQRetryErr(ctx.Err())
	}
	if err := e.releaseTerminationResources(leaseCtx, frozen, event.ExptRunID, sources, true, event.Session); err != nil {
		return err
	}
	return errno.WrapMQRetryErr(ctx.Err())
}

func (e *ExptMangerImpl) refreshTerminatedRunFilters(ctx context.Context, event *entity.ExptScheduleEvent, sources []*entity.ExptCurrentItemRunSource) error {
	port, err := e.terminationRepo()
	if err != nil {
		return err
	}
	for _, original := range sources {
		if original == nil {
			return errno.NewMQRetryErr("termination filter source missing")
		}
		if original.Current.Status == entity.ItemRunState_Success || original.Current.Status == entity.ItemRunState_Fail {
			continue
		}
		local, ok := e.exptResultService.(*ExptResultServiceImpl)
		if !ok {
			if e.exptResultService == nil {
				return errno.NewMQRetryErr("termination filter service missing")
			}
			if err := e.exptResultService.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, []int64{original.Current.ItemID}); err != nil {
				return err
			}
			continue
		}
		// The termination transaction has released all item leases. Reacquire
		// independently and rebuild the payload from primary canonical rows.
		if err := local.withFilterItemLock(ctx, event.ExptID, original.Current.ItemID, func(ctx context.Context) error {
			current, err := port.GetTerminationItemSource(ctx, event.SpaceID, event.ExptID, event.ExptRunID, original.Current.ItemID)
			if err != nil {
				return err
			}
			if current == nil || current.Current.RequestID != original.Current.RequestID || current.Current.ItemVersionID != original.Current.ItemVersionID || current.Current.Mode != original.Current.Mode || current.Current.Status != entity.ItemRunState_Terminal {
				return errno.NewMQRetryErr("termination filter current source changed")
			}
			filters, err := local.buildCurrentItemFilterLocked(ctx, event.SpaceID, event.ExptID, original.Current.ItemID)
			if err != nil {
				return err
			}
			if len(filters) == 0 || local.exptTurnResultFilterRepo == nil {
				return errno.NewMQRetryErr("termination filter canonical turns missing")
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			return local.exptTurnResultFilterRepo.Save(ctx, filters)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (e *ExptMangerImpl) cleanupCurrentUnscheduledRun(ctx context.Context, expt, run, space int64, cause error) (err error) {
	if run <= 0 {
		return fmt.Errorf("invalid cleanup run")
	}
	// Always release the captured admission value, including read/write failures.
	// Compare-and-delete cannot release a replacement run's lifecycle lock.
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_, releaseErr := e.mutex.UnlockWithValue(releaseCtx, e.makeExptMutexLockKey(expt), strconv.FormatInt(run, 10))
		err = errors.Join(err, releaseErr)
	}()
	stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), exptRunLogPersistTimeout)
	defer cancel()
	terminate := false
	err = e.withTerminationRunLock(stateCtx, expt, run, func(ctx context.Context) error {
		current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
		if err != nil {
			return err
		}
		if current == nil {
			return fmt.Errorf("cleanup experiment missing")
		}
		if current.LatestRunID != run {
			return nil
		}
		if current.ExptType == entity.ExptType_Online {
			e.cleanupLegacyUnscheduledRun(ctx, expt, run, space, cause)
			return nil
		}
		if current.Status == entity.ExptStatus_Pending {
			port, err := e.terminationRepo()
			if err != nil {
				return err
			}
			_, err = port.TerminateCurrentRun(ctx, &entity.ExptRunTerminationRequest{SpaceID: space, ExptID: expt, RunID: run, SetupAbort: true, StatusMessage: cause.Error()})
			if err != nil {
				return err
			}
			return e.releaseUnscheduledRunQuota(ctx, expt, run, space)
		}
		if entity.IsExptFinished(current.Status) {
			runLog, err := e.runLogRepo.Get(ctx, expt, run)
			if err != nil {
				return err
			}
			if runLog == nil {
				return fmt.Errorf("cleanup run missing")
			}
			if runLog.Status == int64(entity.ExptStatus_Pending) {
				if err := e.runLogRepo.Update(ctx, expt, run, space, map[string]any{"status": int64(entity.ExptStatus_Failed), "status_message": []byte(cause.Error())}); err != nil {
					return err
				}
			}
			return e.releaseUnscheduledRunQuota(ctx, expt, run, space)
		}
		terminate = true
		return nil
	})
	if err != nil || !terminate {
		return err
	}
	_, err = e.TerminateCurrentRun(ctx, expt, run, space, &entity.Session{}, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(cause.Error()), entity.NoAggrCalculate())
	return err
}

// 额度按实验记账；持有原生命周期锁并主库复核，不能释放随后替代 run 的额度。
func (e *ExptMangerImpl) releaseUnscheduledRunQuota(ctx context.Context, expt, run, space int64) (err error) {
	ctx, stop := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
	defer stop()
	locker := e.manualRetryLocker()
	if locker == nil {
		return errno.NewMQRetryErr("setup quota lifecycle locker missing")
	}
	key := e.makeExptMutexLockKey(expt)
	event := &entity.ExptScheduleEvent{SpaceID: space, ExptID: expt, ExptRunID: run}
	if err := e.releasePreviousLifecycleHolder(ctx, locker, key, event); err != nil {
		return err
	}
	locked, leaseCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 8*time.Second)
	if err != nil {
		return err
	}
	if !locked {
		return errno.NewMQRetryErr("setup quota lifecycle owner changed")
	}
	defer func() {
		cancel()
		_, unlockErr := locker.Unlock(key)
		err = errors.Join(err, unlockErr)
	}()
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(leaseCtx), expt, space)
	if err != nil {
		return err
	}
	if current == nil || current.LatestRunID != run {
		return nil
	}
	return NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(leaseCtx, expt, space, &entity.Session{})
}

func (e *ExptMangerImpl) releaseCurrentSupersededRunQuota(ctx context.Context, expt, space, newRun int64, mode entity.ExptRunMode) error {
	if !isRetryRunMode(mode) {
		return nil
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if current == nil {
		return errno.NewMQRetryErr("superseded experiment missing")
	}
	if !entity.IsCentralDispatch(current.ExptDispatchMode) || current.LatestRunID == 0 || current.LatestRunID == newRun {
		return nil
	}
	if entity.IsExptFinished(current.Status) {
		if err := NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(ctx, expt, space, &entity.Session{}); err != nil {
			return errno.WrapMQRetryErr(err)
		}
	}
	if e.centralGuard == nil || current.SchedulerScope == "" {
		return errno.NewMQRetryErr("superseded quota provider or scope missing")
	}
	port, err := e.terminationRepo()
	if err != nil {
		return err
	}
	run := current.LatestRunID
	var sources []*entity.ExptCurrentItemRunSource
	var cursor int64
	for {
		logs, next, err := e.itemResultRepo.ScanItemRunLogs(contexts.WithCtxWriteDB(ctx), expt, run, &entity.ExptItemRunLogFilter{}, cursor, 500, space)
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
		for _, log := range logs {
			if log == nil {
				return errno.NewMQRetryErr("superseded quota log missing")
			}
			source, err := port.GetTerminationItemSource(ctx, space, expt, run, log.ItemID)
			if err != nil {
				return errno.WrapMQRetryErr(err)
			}
			if source == nil {
				return errno.NewMQRetryErr("superseded quota source missing")
			}
			sources = append(sources, source)
		}
		if next == 0 || len(logs) == 0 {
			break
		}
		if next <= cursor {
			return errno.NewMQRetryErr("superseded quota cursor invalid")
		}
		cursor = next
	}
	// The new lifecycle holder excludes old run cleanup until migration completes.
	for _, source := range sources {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		err := e.centralGuard.Release(releaseCtx, current.SchedulerScope, run, source.Current.ItemID, source.Current.RequestID, "superseded run residual reservation")
		cancel()
		if err != nil {
			return errno.WrapMQRetryErr(err)
		}
	}
	return ctx.Err()
}

// IExptNewRunItemResetter protects only canonical migration; the scheduler owns
// the run tick lease around the surrounding new-run initializer.
//
//go:generate mockgen -destination=mocks/expt_new_run_reset.go -package=mocks . IExptNewRunItemResetter
type IExptNewRunItemResetter interface {
	ResetItemsForNewRun(context.Context, int64, int64, int64, []int64, map[string]any) error
}

// IExptNewRunItemLocker shares new-run ownership and terminal reservation handling
// with ResetItemsForNewRun while letting the retry initializer use its transaction.
type IExptNewRunItemLocker interface {
	WithNewRunItemLocks(context.Context, int64, int64, int64, []int64, func(context.Context, int64) error) error
}

func (e *ExptMangerImpl) withNewRunItemLocks(ctx context.Context, space, expt, newRun int64, ids []int64, unlock bool, write func(context.Context, int64) error) error {
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if current == nil {
		return errno.NewMQRetryErr("new-run reset experiment missing")
	}
	if current.LatestRunID != newRun {
		return errno.NewMQRetryErr("new-run reset replaced")
	}
	selected := map[int64]bool{}
	ordered := []int64{}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("invalid new-run reset item")
		}
		if !selected[id] {
			ordered = append(ordered, id)
			selected[id] = true
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, id := range ordered {
		err := func() error {
			if e.manualRetryLocker == nil {
				return errno.NewMQRetryErr("new-run reset item locker missing")
			}
			locker := e.manualRetryLocker()
			if locker == nil {
				return errno.NewMQRetryErr("new-run reset item locker missing")
			}
			key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", expt, id)
			locked, itemCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, time.Minute)
			if err != nil {
				return errno.WrapMQRetryErr(err)
			}
			if !locked {
				return errno.NewMQRetryErr("new-run reset item lock busy")
			}
			defer cancel()
			if unlock {
				defer func() {
					if _, err := locker.Unlock(key); err != nil {
						logs.CtxWarn(ctx, "unlock new-run reset item %v: %v", key, err)
					}
				}()
			}
			if err := itemCtx.Err(); err != nil {
				return errno.WrapMQRetryErr(err)
			}
			items, err := e.itemResultRepo.BatchGet(contexts.WithCtxWriteDB(itemCtx), space, expt, []int64{id})
			if err != nil {
				return errno.WrapMQRetryErr(err)
			}
			if len(items) != 1 || items[0] == nil {
				return errno.NewMQRetryErr("new-run reset canonical item missing")
			}
			item := items[0]
			if item.ExptRunID != newRun && entity.IsCentralDispatch(current.ExptDispatchMode) {
				if e.centralGuard == nil || current.SchedulerScope == "" {
					return errno.NewMQRetryErr("new-run reset quota provider missing")
				}
				previous, err := entity.ParseManualRetryContext(item.Ext)
				if err != nil {
					return errno.WrapMQRetryErr(err)
				}
				token := ""
				if previous != nil {
					if previous.RunID != item.ExptRunID {
						return errno.NewMQRetryErr("new-run reset token run mismatch")
					}
					token = previous.RequestID
				}
				result, err := e.centralGuard.ReleaseTerminalReservation(itemCtx, current.SchedulerScope, item.ExptRunID, id, token)
				if err != nil {
					return errno.WrapMQRetryErr(err)
				}
				if result != component.TerminalReservationReleased && result != component.TerminalReservationAbsent {
					return errno.NewMQRetryErr("new-run reset reservation source mismatch")
				}
			}
			if err := itemCtx.Err(); err != nil {
				return errno.WrapMQRetryErr(err)
			}
			return write(itemCtx, id)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *ExptMangerImpl) WithNewRunItemLocks(ctx context.Context, space, expt, newRun int64, ids []int64, write func(context.Context, int64) error) error {
	if e.runFinishRepo == nil {
		return errno.NewMQRetryErr("new-run reset lifecycle guard missing")
	}
	return e.withNewRunItemLocks(ctx, space, expt, newRun, ids, true, write)
}

func (e *ExptMangerImpl) ResetItemsForNewRun(ctx context.Context, space, expt, newRun int64, ids []int64, fields map[string]any) error {
	if e.runFinishRepo == nil {
		return e.itemResultRepo.UpdateItemsResult(ctx, space, expt, ids, fields)
	}
	return e.withNewRunItemLocks(ctx, space, expt, newRun, ids, false, func(itemCtx context.Context, id int64) error {
		return e.itemResultRepo.UpdateItemsResult(itemCtx, space, expt, []int64{id}, fields)
	})
}

func updateNewRunItemResults(ctx context.Context, manager IExptManager, items repo.IExptItemResultRepo, event *entity.ExptScheduleEvent, ids []int64, fields map[string]any) error {
	if reset, ok := manager.(IExptNewRunItemResetter); ok {
		return reset.ResetItemsForNewRun(ctx, event.SpaceID, event.ExptID, event.ExptRunID, ids, fields)
	}
	return items.UpdateItemsResult(ctx, event.SpaceID, event.ExptID, ids, fields)
}
