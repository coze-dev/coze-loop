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

func (e *ExptMangerImpl) withTerminationRunLock(ctx context.Context, expt, run int64, f func(context.Context) error) error {
	lockCtx, cancel, err := e.completeTick(ctx, expt, run)
	if err != nil {
		return err
	}
	defer cancel()
	return f(lockCtx)
}

func (e *ExptMangerImpl) beginCurrentRunTermination(ctx context.Context, expt, run, space int64) error {
	port, err := e.terminationRepo()
	if err != nil {
		return err
	}
	return e.withTerminationRunLock(ctx, expt, run, func(ctx context.Context) error {
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
	})
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
		debt, releaseErr := e.releaseTerminationResourcesWithDebt(resourceCtx, current, run, sources, false, session)
		if len(debt) > 0 {
			// 行级终止后实验仍在跑：把欠释放的 item 落成持久标记，由后续调度拍对账。
			e.retainTerminalResourceDebt(ctx, current, run, debt)
		}
		if releaseErr != nil {
			return releaseErr
		}
		return nil
	})
}

// Compatibility port for item error/cancellation callers carrying an original
// execution source. The standard Complete boundary validates it again in SQL.
func (e *ExptMangerImpl) TerminateCurrentRun(ctx context.Context, expt, run, space int64, session *entity.Session, expected *entity.ExptCurrentItemRunSource, opts ...entity.CompleteExptOptionFn) (bool, error) {
	if _, err := e.terminationRepo(); err != nil {
		return false, err
	}
	result := &entity.ExptRunFinishResult{}
	ctx = context.WithValue(ctx, completeCaptureKey{}, result)
	if expected != nil {
		ctx = context.WithValue(ctx, completeExpectedSourceKey{}, expected)
	}
	options := append([]entity.CompleteExptOptionFn{entity.WithStatus(entity.ExptStatus_Terminated)}, opts...)
	err := e.CompleteRun(ctx, expt, run, space, session, options...)
	accepted := result.Experiment != nil && result.Disposition != entity.ExptRunFinishNonCurrent
	if err != nil {
		return accepted, errno.WrapMQRetryErr(err)
	}
	// Offline completion captures its SQL boundary and already calls CompleteExpt.
	// Online's legacy run-only branch still needs the original completion pair.
	if result.Disposition != 0 {
		return accepted, nil
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if current == nil {
		return false, errno.NewMQRetryErr("termination experiment missing")
	}
	if current.ExptType != entity.ExptType_Online {
		return accepted, nil
	}
	if err := e.CompleteExpt(ctx, expt, &run, space, session, options...); err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	return true, nil
}

// releaseTerminationResources 保持原有"只返回错误"的契约（存量调用方与单测依赖它）。
// 需要拿到"仍欠释放"的 item 的调用方用 releaseTerminationResourcesWithDebt。
func (e *ExptMangerImpl) releaseTerminationResources(ctx context.Context, expt *entity.Experiment, run int64, sources []*entity.ExptCurrentItemRunSource, whole bool, session *entity.Session) error {
	_, err := e.releaseTerminationResourcesWithDebt(ctx, expt, run, sources, whole, session)
	return err
}

// releaseTerminationResourcesWithDebt 释放已提交终态的 item 所持有的外部资源（额度账本 + 沙箱）。
//
// 第二个返回值是**本次没能释放成功、因而仍然欠一次释放**的 item 来源。调用方必须把它
// 落成持久标记（见 retainTerminalResourceDebt）：DB 提交已经发生，错误不能只回给调用方
// 就被忘掉，否则账本 reservation 会随进程一起消失（R8-A / R8-B2）。
func (e *ExptMangerImpl) releaseTerminationResourcesWithDebt(ctx context.Context, expt *entity.Experiment, run int64, sources []*entity.ExptCurrentItemRunSource, whole bool, session *entity.Session) ([]*entity.ExptCurrentItemRunSource, error) {
	seen := map[int64]bool{}
	records := []int64{}
	// eligible 保持与 sources 相同的顺序；出错时从当前位置起的剩余项都是"欠释放"。
	eligible := make([]*entity.ExptCurrentItemRunSource, 0, len(sources))
	for _, source := range sources {
		if source == nil {
			return nil, fmt.Errorf("termination resource source missing")
		}
		if !whole && (source.Current.Status == entity.ItemRunState_Success || source.Current.Status == entity.ItemRunState_Fail) {
			continue
		}
		eligible = append(eligible, source)
	}
	for index, source := range eligible {
		if err := ctx.Err(); err != nil {
			return eligible[index:], errno.WrapMQRetryErr(err)
		}
		if e.centralGuard != nil && entity.IsCentralDispatch(expt.ExptDispatchMode) {
			if expt.SchedulerScope == "" {
				return eligible[index:], fmt.Errorf("termination scope missing")
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
				// 不回滚、不跳过：后面的 item 也还没释放，全部交给对账。
				return eligible[index:], errno.WrapMQRetryErr(err)
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
		targetErr := e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(releaseCtx, resolveLoadSpaceID(expt.SpaceID, expt.TargetSpaceID), records, int32(errno.AsyncEvalTargetTerminatedCode), "async eval target terminated: experiment cancelled", false)
		releaseErr := releaseCtx.Err()
		cancel()
		if targetErr != nil {
			// 以前这里是 `_ =`：错误既不进日志也不触发任何重试，沙箱只能靠平台侧巡检。
			// 现在至少留痕，并把整批标记成"仍欠资源"，由调度拍重放。
			logs.CtxWarn(ctx, "[Termination] terminate async target records failed, sandbox cleanup deferred to reconcile, expt_id: %v, expt_run_id: %v, records: %v, err: %v",
				expt.ID, run, records, targetErr)
			// 契约不变：沙箱是 best-effort，不让它把整次终止判失败；
			// 但错误不再被丢弃 —— 债务标记 + Warn 才是可观测、可重放的信号。
			return eligible, nil
		}
		if releaseErr != nil {
			return eligible, errno.WrapMQRetryErr(releaseErr)
		}
	}
	if err := ctx.Err(); err != nil {
		return eligible, errno.WrapMQRetryErr(err)
	}
	if whole && e.quotaRepo != nil {
		releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		if err := NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(releaseCtx, expt.ID, expt.SpaceID, session); err != nil {
			return eligible, err
		}
	}
	return nil, nil
}

// retainTerminalResourceDebt 把"终态行仍欠一次资源释放"写成持久标记。
//
// 为什么必须落库而不是只记日志：终止事务已经提交（status=Terminal），此后不会再有
// 任何 item 事件，账本和沙箱都不会再有人看这条 item。标记就是终止路径与调度拍之间
// 唯一的交接物。写入本身带 request identity + 终态 + CAS，写不进去说明该行已经被
// 新 run / 新 manual attempt 取代，此时**不能**碰它的投影。
func (e *ExptMangerImpl) retainTerminalResourceDebt(ctx context.Context, expt *entity.Experiment, run int64, sources []*entity.ExptCurrentItemRunSource) {
	// 只有带账本的中心调度实验才需要这个标记。legacy 从不预占，给它打 reserved 会让
	// ScanEvalItems 永久把该行算成 incomplete，而对账入口对 legacy / 无 scope 直接 no-op
	// —— 那会把"资源没释放"变成"run 永不结束"，比原问题更糟。
	// 无 scheduler_scope 时同样无处可释放，标记也无人可清，一律不打。
	if len(sources) == 0 || expt == nil || !entity.IsCentralDispatch(expt.ExptDispatchMode) || expt.SchedulerScope == "" {
		return
	}
	markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()
	for _, source := range sources {
		if source == nil {
			continue
		}
		marked, err := e.itemResultRepo.SetTerminalQuotaReservationState(markCtx, source.Current.SpaceID, source.Current.ExptID, run, source.Current.ItemID, source.Current.RequestID, entity.QuotaReservationStateReserved)
		if err != nil {
			logs.CtxWarn(ctx, "[Termination] retain terminal resource debt failed, item will rely on ledger expiry, expt_id: %v, expt_run_id: %v, item_id: %v, err: %v",
				source.Current.ExptID, run, source.Current.ItemID, err)
			continue
		}
		logs.CtxWarn(ctx, "[Termination] terminal resource debt retained for scheduler reconcile, expt_id: %v, expt_run_id: %v, item_id: %v, request_id: %v, marked: %v",
			source.Current.ExptID, run, source.Current.ItemID, source.Current.RequestID, marked)
	}
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

func (e *ExptMangerImpl) cleanupCurrentUnscheduledRun(ctx context.Context, expt, run, space int64, cause error, ordinaryAdmission ...bool) error {
	// Pre-MQ failures have no confirmed successful space quota reservation.
	return e.cleanupCurrentUnscheduledRunWithQuota(ctx, expt, run, space, cause, false, ordinaryAdmission...)
}

func (e *ExptMangerImpl) cleanupCurrentUnscheduledRunWithQuota(ctx context.Context, expt, run, space int64, cause error, quotaReserved bool, ordinaryAdmission ...bool) (err error) {
	if run <= 0 {
		return fmt.Errorf("invalid cleanup run")
	}
	capturedOrdinary := len(ordinaryAdmission) > 0 && ordinaryAdmission[0]
	capturedNumeric := len(ordinaryAdmission) > 0 && !ordinaryAdmission[0]
	admissionTick, hasAdmissionTick := ctx.Value(completeTickKey{}).(completeTickIdentity)
	primaryReadFailed := false
	ordinaryReleaseAllowed := false
	releaseAdmission := !quotaReserved
	cleanupCtx := context.WithoutCancel(ctx)
	if _, held := ctx.Value(completeTickKey{}).(completeTickIdentity); held {
		// New-run admission holds the previous run tick through compensation.
		// Its actual lease loss must stop fixed admission/shared quota cleanup.
		if _, owned := ctx.Value(completeTickOwnerKey{}).(context.Context); !owned {
			return errno.NewMQRetryErr("admission cleanup tick owner missing")
		}
		linked, stop := bindCompleteTickOwner(ctx)
		defer stop()
		cleanupCtx = linked
	}
	defer func() {
		if !releaseAdmission {
			return
		}
		releaseCtx, cancel := context.WithTimeout(cleanupCtx, 2*time.Second)
		defer cancel()
		if releaseCtx.Err() != nil {
			err = errors.Join(err, errno.WrapMQRetryErr(releaseCtx.Err()))
			return
		}
		var releaseErr error
		if capturedOrdinary {
			if _, held := ctx.Value(completeTickKey{}).(completeTickIdentity); held {
				_, releaseErr = e.mutex.UnlockWithContext(releaseCtx, e.makeExptMutexLockKey(expt))
			} else {
				_, releaseErr = e.mutex.Unlock(e.makeExptMutexLockKey(expt))
			}
		} else if capturedNumeric {
			_, releaseErr = e.mutex.UnlockWithValue(releaseCtx, e.makeExptMutexLockKey(expt), strconv.FormatInt(run, 10))
		} else if ordinaryReleaseAllowed {
			logged, readErr := e.runLogRepo.Get(releaseCtx, expt, run)
			if readErr == nil && logged != nil && entity.ExptRunMode(logged.Mode) == entity.EvaluationModeRetryItems {
				_, releaseErr = e.mutex.UnlockWithValue(releaseCtx, e.makeExptMutexLockKey(expt), strconv.FormatInt(run, 10))
			} else if readErr == nil || primaryReadFailed {
				if _, held := ctx.Value(completeTickKey{}).(completeTickIdentity); held {
					_, releaseErr = e.mutex.UnlockWithContext(releaseCtx, e.makeExptMutexLockKey(expt))
				} else {
					_, releaseErr = e.mutex.Unlock(e.makeExptMutexLockKey(expt))
				}
			} else {
				releaseErr = readErr
			}
		}
		err = errors.Join(err, releaseErr)
	}()
	stateCtx, cancel := context.WithTimeout(cleanupCtx, exptRunLogPersistTimeout)
	defer cancel()
	terminate := false
	err = e.withTerminationRunLock(stateCtx, expt, run, func(ctx context.Context) error {
		current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), expt, space)
		if err != nil {
			// Synchronous pre-MQ cleanup may still identify the ordinary run from
			// its captured run log. A confirmed replacement below never grants this.
			ordinaryReleaseAllowed = true
			primaryReadFailed = true
			return err
		}
		if current == nil {
			return fmt.Errorf("cleanup experiment missing")
		}
		if current.LatestRunID != run {
			if capturedOrdinary || capturedNumeric {
				if hasAdmissionTick && current.LatestRunID != admissionTick.run {
					releaseAdmission = false
				}
				return e.runLogRepo.Update(ctx, expt, run, space, map[string]any{"status": int64(entity.ExptStatus_Failed), "status_message": []byte(cause.Error())})
			}
			return nil
		}
		ordinaryReleaseAllowed = true
		if current.ExptType == entity.ExptType_Online {
			releaseAdmission = false // the synchronous legacy path owns its one release
			e.cleanupLegacyUnscheduledRun(ctx, expt, run, space, cause)
			return nil
		}
		if current.Status == entity.ExptStatus_Pending {
			port, err := e.terminationRepo()
			if err != nil {
				return err
			}
			_, err = port.TerminateCurrentRun(ctx, &entity.ExptRunTerminationRequest{SpaceID: space, ExptID: expt, RunID: run, SetupAbort: true, StatusMessage: cause.Error()})
			return err
		}
		if entity.IsExptFinished(current.Status) {
			runLog, err := e.runLogRepo.Get(ctx, expt, run)
			if err != nil {
				return err
			}
			if runLog == nil {
				return fmt.Errorf("cleanup run missing")
			}
			if runLog.Status != int64(entity.ExptStatus_Pending) {
				return nil
			}
			if quotaReserved {
				// CompleteRun-only compensation follows a successful prepareRun.
				// Release before marking the run terminal so a release error leaves
				// Pending and admission intact for the scheduler to retry this path.
				if err := NewQuotaService(e.quotaRepo, e.configer).ReleaseExptRun(ctx, expt, space, &entity.Session{}); err != nil {
					return errno.WrapMQRetryErr(err)
				}
				if err := e.releaseCompleteAdmissionForMode(ctx, expt, run, entity.ExptRunMode(runLog.Mode)); err != nil {
					return errno.WrapMQRetryErr(err)
				}
			}
			if err := e.runLogRepo.Update(ctx, expt, run, space, map[string]any{"status": int64(entity.ExptStatus_Failed), "status_message": []byte(cause.Error())}); err != nil {
				return err
			}
			releaseAdmission = !quotaReserved
			return nil
		}
		terminate = true
		releaseAdmission = false // termination retains and releases lifecycle itself
		return nil
	})
	if err != nil || !terminate {
		return err
	}
	_, err = e.TerminateCurrentRun(ctx, expt, run, space, &entity.Session{}, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(cause.Error()), entity.NoAggrCalculate())
	return err
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
		if err := ctx.Err(); err != nil {
			return errno.WrapMQRetryErr(err)
		}
		releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := e.centralGuard.Release(releaseCtx, current.SchedulerScope, run, source.Current.ItemID, source.Current.RequestID, "superseded run residual reservation")
		if err == nil {
			err = releaseCtx.Err()
		}
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

// ITerminalResourceDebtReconciler 让调度拍在不新增异步任务系统的前提下，
// 对账"终止事务已提交、但资源没释放成功"的终态行。
//
// 为什么入口在调度拍而不是 item 事件：终态 item 不会再有事件，MQ 重投也不会回来。
// 调度拍是唯一还会为这个 run 周期性运行的执行者。
//
// rows 是本次拍已经校验过的当前 run 观测（ScanCurrentRequests 的结果），
// 复用它可以避免同一拍再扫一遍主表。返回被成功清掉债务的 itemID；调用方据此
// 把本拍内存里的投影改成 none，从而在同一拍就能收口（B1 不再永久 incomplete）。
type ITerminalResourceDebtReconciler interface {
	ReconcileTerminalResourceDebt(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, rows []*entity.ExptItemCurrentRequest) []int64
}

// ReconcileTerminalResourceDebt 释放并清除终态行残留的额度预占 / 沙箱资源。
//
// 触发条件只有一种：终态行仍然投影为 reserved。reserved 在这里的含义被扩成
// "该行仍欠一次资源释放"——
//   - 已准入未启动就被终止的行本来是 reserved（B1）；
//   - 运行中被终止（投影 none）但 Release 失败的行，由终止路径用
//     retainTerminalResourceDebt 主动打回 reserved（B2），否则账本泄漏无从发现。
//
// 幂等与身份保证：
//   - guard.Release 自带 expectedRequestID 原子比对，重复拍是 no-op，不会重复扣减；
//   - 清投影走 SetTerminalQuotaReservationState：同 run + 同 request identity + 终态 + CAS，
//     新 run / 新 manual attempt 的行不会被误改；
//   - 只有账本释放与沙箱重放都成功才清投影；任何一步失败都保留标记，下一拍再来。
func (e *ExptMangerImpl) ReconcileTerminalResourceDebt(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, rows []*entity.ExptItemCurrentRequest) []int64 {
	if e.centralGuard == nil || event == nil || expt == nil || !entity.IsCentralDispatch(expt.ExptDispatchMode) {
		return nil
	}
	if expt.SchedulerScope == "" {
		logs.CtxWarn(ctx, "[TerminationRecovery] enforce experiment without scheduler_scope, cannot reconcile, expt_id: %v, expt_run_id: %v", event.ExptID, event.ExptRunID)
		return nil
	}
	var cleared []int64
	for _, row := range rows {
		if row == nil || row.ObservationUnstable || row.RunID != event.ExptRunID {
			continue
		}
		if !entity.IsItemRunFinished(row.Status) || !row.QuotaReservationState.IsQuotaReserved() {
			continue
		}
		if err := e.releaseTerminalResourceDebt(ctx, expt, row); err != nil {
			logs.CtxWarn(ctx, "[TerminationRecovery] terminal resource debt not cleared, retained for next tick, expt_id: %v, expt_run_id: %v, item_id: %v, request_id: %v, err: %v",
				event.ExptID, event.ExptRunID, row.ItemID, row.RequestID, err)
			continue
		}
		cleared = append(cleared, row.ItemID)
	}
	if len(cleared) > 0 {
		logs.CtxWarn(ctx, "[TerminationRecovery] terminal resource debt reconciled, expt_id: %v, expt_run_id: %v, items: %v", event.ExptID, event.ExptRunID, cleared)
	}
	return cleared
}

func (e *ExptMangerImpl) releaseTerminalResourceDebt(ctx context.Context, expt *entity.Experiment, row *entity.ExptItemCurrentRequest) error {
	// 1) 账本：identity 比对在 guard 内部原子完成，重复调用是 no-op。
	releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	err := e.centralGuard.Release(releaseCtx, expt.SchedulerScope, row.RunID, row.ItemID, row.RequestID, "terminal item residual reservation")
	if err == nil {
		err = releaseCtx.Err()
	}
	cancel()
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	// 2) 沙箱：sweepTerminatedSandboxItems 只巡检 Processing 的 item，终态行永远进不去，
	//    所以沙箱销毁失败在终止路径之外没有第二个执行者。这里补上。
	if e.evalTargetService != nil && isSandboxAgentExpt(expt) {
		if err := e.replayTerminalSandboxCleanup(ctx, expt, row); err != nil {
			return err
		}
	}
	// 3) 两者都确认完成才清标记。CAS 不命中（行已推进 / 已被清）不算失败，但也不当作成功，
	//    避免把内存投影改错。
	cleared, err := e.itemResultRepo.SetTerminalQuotaReservationState(ctx, row.SpaceID, row.ExptID, row.RunID, row.ItemID, row.RequestID, entity.QuotaReservationStateNone)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if !cleared {
		return errno.NewMQRetryErr("terminal reservation projection advanced or already cleared")
	}
	return nil
}

// replayTerminalSandboxCleanup 对终态 item 关联的异步评测对象重放一次销毁。
//
// 为什么用 waitForDestroy=true：初始终止调用是 best-effort（wait=false），
// TerminateAsyncRecordsAndDestroySandbox 只对 AsyncInvoking 的记录生效，且异步销毁的
// 失败被内部 goroutine 吞掉 —— 那一跳既拿不到错误、也没有记录状态可供下一拍判断。
// 同步路径（wait=true）相反：销毁失败直接返回错误且**不落** Fail，记录保持 AsyncInvoking，
// 因此下一拍真的会再试一次。这正是"真实重放"，代价是每次最多一个重试预算（10s），
// 只发生在极少数带债务标记的行上。
//
// 重复重放安全：已落 Fail 的记录被该接口整体跳过；仍未销毁的记录重复销毁是平台侧的幂等操作。
func (e *ExptMangerImpl) replayTerminalSandboxCleanup(ctx context.Context, expt *entity.Experiment, row *entity.ExptItemCurrentRequest) error {
	if e.turnResultRepo == nil {
		return errno.NewMQRetryErr("terminal sandbox replay source missing")
	}
	turnLogs, err := e.turnResultRepo.MGetItemTurnRunLogs(ctx, row.ExptID, row.RunID, []int64{row.ItemID}, row.SpaceID)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	seen := map[int64]bool{}
	records := make([]int64, 0, len(turnLogs))
	for _, turnLog := range turnLogs {
		if turnLog == nil || turnLog.TargetResultID <= 0 || seen[turnLog.TargetResultID] {
			continue
		}
		seen[turnLog.TargetResultID] = true
		records = append(records, turnLog.TargetResultID)
	}
	if len(records) == 0 {
		return nil
	}
	err = e.evalTargetService.TerminateAsyncRecordsAndDestroySandbox(ctx, resolveLoadSpaceID(row.SpaceID, expt.TargetSpaceID), records, int32(errno.AsyncEvalTargetTerminatedCode), "async eval target terminated: experiment cancelled", false, true)
	if err != nil {
		logs.CtxWarn(ctx, "[TerminationRecovery] sandbox destroy replay failed, keep debt marker, expt_id: %v, expt_run_id: %v, item_id: %v, records: %v, err: %v",
			row.ExptID, row.RunID, row.ItemID, records, err)
		return errno.WrapMQRetryErr(err)
	}
	return nil
}
