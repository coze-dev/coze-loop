// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/xid"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
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

type lifecycleLockValueReader interface {
	GetLifecycleLockValue(context.Context, string) (string, bool, error)
	GetLifecycleLockTTL(context.Context, string) (time.Duration, bool, error)
}

const (
	// LegacyLifecycleMigrationGrace is the minimum holder age before a
	// pre-upgrade lifecycle holder may be migrated.
	LegacyLifecycleMigrationGrace = 5 * time.Minute
	// legacyStaticLifecycleHolder is the fixed value written by the pre-upgrade
	// shared evaluation locker (application.NewLock) for lifecycle keys. It is
	// kept here rather than infra/lock on purpose: it is a legacy evaluation
	// business vocabulary, not part of the generic lock API.
	legacyStaticLifecycleHolder = "evaluation"
)

// legacyLifecycleHolderAcquiredAt derives the acquisition time from a
// pre-upgrade "<hostname>-<xid>" holder. XID embeds an immutable creation
// timestamp, so age is independent of any configuration change between
// acquisition and migration.
func legacyLifecycleHolderAcquiredAt(holder string) (time.Time, bool) {
	sep := strings.LastIndex(holder, "-")
	if sep <= 0 || sep+1 >= len(holder) {
		return time.Time{}, false
	}
	id, err := xid.FromString(holder[sep+1:])
	if err != nil {
		return time.Time{}, false
	}
	return id.Time(), true
}

// releasePreviousLifecycleHolder migrates a pre-upgrade random holder only when
// the current run is already terminal. Numeric holders belong to another run,
// prefixed holders belong to a live new cleanup lease, and the fixed legacy
// "evaluation" holder carries no acquisition token at all; none are migrated.
func (e *ExptMangerImpl) releasePreviousLifecycleHolder(ctx context.Context, locker lock.ILocker, key string, event *entity.ExptScheduleEvent) error {
	if event == nil {
		return errno.NewMQRetryErr("legacy lifecycle event missing")
	}
	released, err := locker.UnlockWithValue(ctx, key, strconv.FormatInt(event.ExptRunID, 10))
	if err != nil || released {
		return err
	}
	reader, ok := locker.(lifecycleLockValueReader)
	if !ok {
		logs.CtxWarn(ctx, "[ExptLifecycle] TTL reader unavailable; keep legacy holder, expt_id: %v, run_id: %v", event.ExptID, event.ExptRunID)
		return nil
	}
	holder, exists, err := reader.GetLifecycleLockValue(ctx, key)
	if err != nil || !exists {
		return err
	}
	if _, err := strconv.ParseInt(holder, 10, 64); err == nil || strings.HasPrefix(holder, lock.LifecycleCleanupHolderPrefix) {
		return nil
	}
	// 无过期时间的锁永远等不到自然过期，迁移前先暴露出来。
	if _, hasTTL, err := reader.GetLifecycleLockTTL(ctx, key); err != nil {
		return err
	} else if !hasTTL {
		logs.CtxWarn(ctx, "[ExptLifecycle] legacy holder has no expiry or TTL is unavailable; keep lock, expt_id: %v, run_id: %v", event.ExptID, event.ExptRunID)
		return nil
	}
	if holder == legacyStaticLifecycleHolder {
		// 固定的旧 "evaluation" holder 没有 acquisition token：旧 LogRun 在上一
		// 个 run 的 holder 被释放后会以完全相同的 value 为下一个 run 重新 SETNX。
		// 终态 EndAt 只证明上一个 run 结束了多久，不证明当前 Redis owner 属于
		// 它；同值 CAS 也无法识别 R1/R2 换代。R2 可能已经 SETNX 但仍停在
		// Create/LatestRunID 之前，此时删除就是抹掉它已取得的排他保护。无法证明
		// 归属就不能删除，只能保留并让 AlreadyFinished 重放等到租约自然到期。
		logs.CtxWarn(ctx, "[ExptLifecycle] fixed legacy holder has no ownership token; keep lock, expt_id: %v, run_id: %v", event.ExptID, event.ExptRunID)
		return nil
	}
	acquiredAt, provable := legacyLifecycleHolderAcquiredAt(holder)
	if !provable {
		// 不认识的老 holder 格式：无法证明获取时间，宁可等自然过期也不删。
		logs.CtxWarn(ctx, "[ExptLifecycle] legacy holder age unverifiable; keep lock, expt_id: %v, run_id: %v", event.ExptID, event.ExptRunID)
		return nil
	}
	if time.Since(acquiredAt) < LegacyLifecycleMigrationGrace {
		return nil
	}
	if e.exptRepo == nil {
		return errno.NewMQRetryErr("legacy lifecycle ownership repository missing")
	}
	current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), event.ExptID, event.SpaceID)
	if err != nil {
		return err
	}
	if current == nil || current.LatestRunID != event.ExptRunID || !entity.IsExptFinished(current.Status) {
		return nil
	}
	released, err = locker.UnlockWithValue(ctx, key, holder)
	if err != nil {
		return err
	}
	if !released {
		return errno.NewMQRetryErr("legacy lifecycle holder changed during migration")
	}
	return nil
}

// acquireFinishedRunLifecycle retains the admission key through cleanup. The
// caller must recheck primary LatestRunID/terminal status before side effects.
// A nil release means SQL already confirms a replacement; the caller exits.
// Ordinary evaluation and existing numeric runs finish under their original
// holder; only an absent/legacy random key needs the existing cleanup lease.
// The fixed ordinary value intentionally retains the old protocol's guarantee:
// TTL expiry or an old force-unlock followed by same-value SETNX cannot be fenced.
func (e *ExptMangerImpl) acquireFinishedRunLifecycle(ctx context.Context, event *entity.ExptScheduleEvent) (context.Context, func() error, func(), error) {
	if e.manualRetryLocker == nil {
		return nil, nil, nil, errno.NewMQRetryErr("finish lifecycle locker missing")
	}
	locker := e.manualRetryLocker()
	if locker == nil {
		return nil, nil, nil, errno.NewMQRetryErr("finish lifecycle locker missing")
	}
	key := e.makeExptMutexLockKey(event.ExptID)
	if reader, ok := locker.(lifecycleLockValueReader); ok {
		holder, exists, err := reader.GetLifecycleLockValue(ctx, key)
		if err != nil {
			return nil, nil, nil, err
		}
		if exists && (holder == legacyStaticLifecycleHolder || holder == strconv.FormatInt(event.ExptRunID, 10)) {
			release := func() error {
				releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
				defer cancel()
				_, err := locker.UnlockWithValue(releaseCtx, key, holder)
				return err
			}
			return ctx, release, func() {}, nil
		}
		if exists && e.exptRepo != nil {
			current, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), event.ExptID, event.SpaceID)
			if err != nil {
				return nil, nil, nil, err
			}
			if current != nil && current.LatestRunID != event.ExptRunID {
				return ctx, nil, nil, nil
			}
		}
	}
	// Keep the prior compatibility handling for old random holders and lockers
	// without a value reader; numeric replacement holders are never migrated.
	if err := e.releasePreviousLifecycleHolder(ctx, locker, key, event); err != nil {
		return nil, nil, nil, err
	}
	locked, leaseCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return nil, nil, nil, err
	}
	if !locked {
		return nil, nil, nil, errno.NewMQRetryErr("finish lifecycle owner unavailable")
	}
	stop := sync.OnceFunc(cancel)
	return leaseCtx, func() error { stop(); return nil }, stop, nil
}

func (e *ExptMangerImpl) finishedRunCleanupSeen(ctx context.Context, event *entity.ExptScheduleEvent) (bool, error) {
	if e.idem == nil {
		return false, nil
	}
	return e.idem.Exist(ctx, fmt.Sprintf("finish-cleanup:%d:%d", event.ExptID, event.ExptRunID))
}

func (e *ExptMangerImpl) cleanupNaturallyFinishedRun(ctx context.Context, event *entity.ExptScheduleEvent, finished *entity.Experiment) error {
	if finished == nil {
		return fmt.Errorf("finished experiment missing")
	}
	if e.idem == nil {
		return errno.NewMQRetryErr("finish notification idempotency service missing")
	}
	// A completed replay exits before touching a subsequent same-value admission
	// or its shared run quota, including the next LogRun's pre-SQL window.
	seen, err := e.finishedRunCleanupSeen(ctx, event)
	if err != nil {
		return err
	}
	if seen {
		return nil
	}
	cleanupCtx, release, cancel, err := e.acquireFinishedRunLifecycle(ctx, event)
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}
	defer cancel()
	if err := cleanupCtx.Err(); err != nil {
		return err
	}
	if e.exptRepo == nil {
		return errno.NewMQRetryErr("finish cleanup repository missing")
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
			_ = e.terminateSandboxExecutesForCancelledItems(cleanupCtx, event.SpaceID, current.TargetSpaceID, event.ExptID, &event.ExptRunID, ids)
		}
	}
	if err := cleanupCtx.Err(); err != nil {
		return err
	}

	// Mark the whole successful cleanup before releasing admission. Errors leave
	// the original holder in place so resources and notifications remain replayable.
	notificationKey := fmt.Sprintf("finish-cleanup:%d:%d", event.ExptID, event.ExptRunID)

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
	duration := e.finishedRunCleanupTTL(cleanupCtx, event.SpaceID)
	// A crash or Set failure after RPC success can replay this segment; the key
	// bounds normal duplicates, it does not provide distributed exactly-once.
	if err := e.idem.Set(cleanupCtx, notificationKey, duration); err != nil {
		return err
	}
	if err := release(); err != nil {
		// A failed release must not turn a remaining admission into an ACK-only leak.
		return errors.Join(err, e.idem.Del(context.WithoutCancel(ctx), notificationKey))
	}
	return nil
}

func (e *ExptMangerImpl) finishedRunCleanupTTL(ctx context.Context, space int64) time.Duration {
	if e.configer == nil {
		return 3 * time.Minute
	}
	return time.Duration(e.configer.GetExptExecConf(ctx, space).GetZombieIntervalSecond()) * time.Second * 2
}
