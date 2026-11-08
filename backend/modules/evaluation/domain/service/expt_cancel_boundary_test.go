// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRunCancelAtomicBeginRollsBackBothStates(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	before := f.snapshot()
	require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("cancel_experiment_fault", func(tx *gorm.DB) {
		if tx.Statement.Table == "experiment" {
			_ = tx.AddError(errors.New("cancel experiment write fault"))
		}
	}))
	err := p.manager.SetExptTerminating(context.Background(), f.expt, f.run, f.space, p.event.Session)
	require.Error(t, err)
	require.Equal(t, before, f.snapshot(), "run and experiment must roll back together")
}

func TestRunCancelOldRunCannotTerminateReplacement(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
	replacement := f.run + 100
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: replacement, SpaceID: f.space, ExptID: f.expt, ExptRunID: replacement, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", replacement).Error)
	before := f.snapshot()
	require.Error(t, p.manager.SetExptTerminating(context.Background(), f.expt, f.run, f.space, p.event.Session))
	require.Equal(t, before, f.snapshot(), "an old cancellation cannot change R2 or R1")
}

func TestTerminateRowAtomicLogsRollback(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.manager.turnResultRepo = exptrepo.NewExptTurnResultRepo(p.manager.idgenerator, exptmysql.NewExptTurnResultDAO(p.f.provider), exptmysql.NewExptTurnEvaluatorResultRefDAO(p.f.provider))
	f := p.f
	before := f.snapshot()
	require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("cancel_item_fault", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" {
			_ = tx.AddError(errors.New("cancel item write fault"))
		}
	}))
	err := p.manager.TerminateItems(context.Background(), f.expt, f.run, f.space, []int64{f.item}, p.event.Session)
	require.Error(t, err)
	require.Equal(t, before, f.snapshot(), "turn and item logs must commit as a pair")
}

func TestRunCancelRetryAfterTerminationFailsWithoutRewriting(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.publishSuccess(t, false)
	receipt, err := p.manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{SpaceID: p.f.space, ExptID: p.f.expt, ExpectedRunID: p.f.run, Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{p.f.item}, Session: p.event.Session})
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	require.NoError(t, p.manager.SetExptTerminating(context.Background(), p.f.expt, p.f.run, p.f.space, p.event.Session))
	applied, err := p.manager.TerminateCurrentRun(context.Background(), p.f.expt, p.f.run, p.f.space, p.event.Session, nil, entity.WithStatus(entity.ExptStatus_Terminated), entity.NoAggrCalculate())
	require.NoError(t, err)
	require.True(t, applied)
	before := p.f.snapshot()
	replay, err := p.manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{SpaceID: p.f.space, ExptID: p.f.expt, ExpectedRunID: p.f.run, Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{p.f.item}, Session: p.event.Session})
	require.ErrorContains(t, err, "run_finished")
	require.Nil(t, replay)
	require.Empty(t, receipt.RequestID)
	require.Empty(t, receipt.PayloadHash)
	require.Positive(t, receipt.SnapshotAtMS)
	require.Equal(t, before, p.f.snapshot())
	_, err = p.manager.TerminateCurrentRun(context.Background(), p.f.expt, p.f.run, p.f.space, p.event.Session, nil, entity.WithStatus(entity.ExptStatus_Failed))
	require.NoError(t, err)
	require.Equal(t, before, p.f.snapshot(), "terminal replay preserves the original result")
	var experiment model.Experiment
	require.NoError(t, p.f.conn.Where("space_id=?", p.f.space).Take(&experiment).Error)
	require.Equal(t, int32(entity.ExptStatus_Terminated), experiment.Status)
	var item model.ExptItemResultRunLog
	require.NoError(t, p.f.conn.Where("space_id=?", p.f.space).Take(&item).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
}

func TestTerminateRowPublicationDoesNotCountTwice(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	require.NoError(t, p.manager.TerminateItems(context.Background(), p.f.expt, p.f.run, p.f.space, []int64{p.f.item}, p.event.Session))
	var stats model.ExptStats
	require.NoError(t, p.f.conn.Where("space_id=?", p.f.space).Take(&stats).Error)
	require.Zero(t, stats.PendingCnt)
	require.Equal(t, int32(1), stats.TerminatedCnt)
	_, err := p.result.RecordItemRunLogs(context.Background(), p.f.expt, p.f.run, p.f.item, p.f.space, p.expt)
	require.NoError(t, err)
	require.NoError(t, p.f.conn.Where("space_id=?", p.f.space).Take(&stats).Error)
	require.Zero(t, stats.PendingCnt)
	require.Equal(t, int32(1), stats.TerminatedCnt)
	before := p.f.snapshot()
	require.NoError(t, p.manager.TerminateItems(context.Background(), p.f.expt, p.f.run, p.f.space, []int64{p.f.item}, p.event.Session))
	require.Equal(t, before, p.f.snapshot())
	result, err := p.manager.FinishCurrentRun(context.Background(), p.event)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Success, result.Experiment.Status)
}

func TestRunCancelSchedulerOldErrorDoesNotEndNewRun(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	replacement := p.f.run + 100
	require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", p.f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
	require.NoError(t, p.f.conn.Create(&model.ExptRunLog{ID: replacement, SpaceID: p.f.space, ExptID: p.f.expt, ExptRunID: replacement, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=?", p.f.space).Update("latest_run_id", replacement).Error)
	before := p.f.snapshot()
	scheduler := &ExptSchedulerImpl{Manager: p.manager}
	err := scheduler.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return errors.New("business failure") })(context.Background(), p.event)
	require.NoError(t, err)
	require.Equal(t, before, p.f.snapshot())
	require.Zero(t, p.resources.Load())
	require.Zero(t, p.notifications.Load())
}

func TestCleanupUnscheduledOldRunPreservesSuccess(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", p.f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=?", p.f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": time.Now()}).Error)
	before := p.f.snapshot()
	p.manager.cleanupUnscheduledRun(context.Background(), p.f.expt, p.f.run, p.f.space, errors.New("late setup error"))
	require.Equal(t, before, p.f.snapshot(), "late initialization cleanup must preserve terminal success")
}

type cancelHookLocker struct {
	lock.ILocker
	hook func(string)
}

func (l *cancelHookLocker) GetLifecycleLockValue(ctx context.Context, key string) (string, bool, error) {
	return l.ILocker.(lifecycleLockValueReader).GetLifecycleLockValue(ctx, key)
}

func (l *cancelHookLocker) GetLifecycleLockTTL(ctx context.Context, key string) (time.Duration, bool, error) {
	return l.ILocker.(lifecycleLockValueReader).GetLifecycleLockTTL(ctx, key)
}

func (l *cancelHookLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	if l.hook != nil {
		l.hook(key)
	}
	return l.ILocker.LockWithRenew(ctx, key, ttl, hold)
}

func TestDebtOriginalSourceRecheckedAfterItemLeaseReleased(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			f := p.f
			source := f.source()
			event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: f.item, ExptRunMode: source.Current.Mode}
			config := confmocks.NewMockIConfiger(gomock.NewController(t))
			config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{IsInDebt: true})
			var once sync.Once
			factory := p.factory
			p.manager.manualRetryLocker = func() lock.ILocker {
				return &cancelHookLocker{ILocker: factory(), hook: func(key string) {
					if replace && strings.HasPrefix(key, "expt_run_exec_lock:") {
						once.Do(func() { f.token("aa6fe7ea-7db6-4d16-8f5b-2d1344556677") })
					}
				}}
			}
			svc := &ExptItemEventEvalServiceImpl{experimentRepo: p.manager.exptRepo, manager: p.manager, configer: config, itemLocker: func() lock.ILocker { return factory() }, exptItemResultRepo: f.itemRepo()}
			before := f.snapshot()
			err := svc.handleCurrentEventErr(context.Background(), event, errors.New("debt"), f.port, &currentItemExecution{Source: source})
			require.NoError(t, err)
			if replace {
				after := f.snapshot()
				for table, value := range before {
					if table != "expt_item_result" {
						require.Equal(t, value, after[table], table)
					}
				}
				require.Zero(t, p.resources.Load())
				require.Zero(t, p.notifications.Load())
			} else {
				var row model.Experiment
				require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&row).Error)
				require.Equal(t, int32(entity.ExptStatus_Terminated), row.Status)
			}
		})
	}
}

func TestRunCancelHeldWorkerItemLeaseDoesNotWaitForRun(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	worker := p.factory()
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", p.f.expt, p.f.item)
	locked, _, cancel, err := worker.LockWithRenew(context.Background(), key, time.Second*5, time.Minute)
	require.NoError(t, err)
	require.True(t, locked)
	before := p.f.snapshot()
	started := time.Now()
	err = p.manager.TerminateItems(context.Background(), p.f.expt, p.f.run, p.f.space, []int64{p.f.item}, p.event.Session)
	require.True(t, errno.NeedMQRetry(err))
	require.Less(t, time.Since(started), 8*time.Second)
	require.Equal(t, before, p.f.snapshot())
	cancel()
	require.NoError(t, p.manager.TerminateItems(context.Background(), p.f.expt, p.f.run, p.f.space, []int64{p.f.item}, p.event.Session))
}

func TestRunCancelFirstRejectsFreshRequest(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.publishSuccess(t, false)
	require.NoError(t, p.manager.SetExptTerminating(context.Background(), p.f.expt, p.f.run, p.f.space, p.event.Session))
	before := p.f.snapshot()
	_, err := p.manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{SpaceID: p.f.space, ExptID: p.f.expt, ExpectedRunID: p.f.run, Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{p.f.item}, Session: p.event.Session})
	require.Error(t, err)
	require.Equal(t, before, p.f.snapshot())
}

func TestTerminateRowOldSnapshotDoesNotTouchReadmittedRequest(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	original := p.factory
	var snapshot map[string]string
	var once sync.Once
	p.manager.manualRetryLocker = func() lock.ILocker {
		return &cancelHookLocker{ILocker: original(), hook: func(key string) {
			if strings.HasPrefix(key, "expt_run_exec_lock:") {
				once.Do(func() { f.token("aa6fe7ea-7db6-4d16-8f5b-2d1344556677"); snapshot = f.snapshot() })
			}
		}}
	}
	require.NoError(t, p.manager.TerminateItems(context.Background(), f.expt, f.run, f.space, []int64{f.item}, p.event.Session))
	require.Equal(t, snapshot, f.snapshot())
	require.Zero(t, p.resources.Load())
}

type cancelResourceGuard struct {
	*fakeGuard
	check func(context.Context, string)
}

func (g *cancelResourceGuard) Release(ctx context.Context, scope string, run, item int64, token, reason string) error {
	g.check(ctx, token)
	return g.fakeGuard.Release(ctx, scope, run, item, token, reason)
}

func TestRunCancelResourcesSurviveParentCancelAfterCommit(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	token := "aa6fe7ea-7db6-4d16-8f5b-2d1344556677"
	f.token(token)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel after the SQL port returned, before lifecycle cleanup starts.
	port := p.manager.runFinishRepo
	p.manager.runFinishRepo = &cancelAfterCommitPort{IExptRunFinishRepo: port, termination: port.(repo.IExptRunTerminationRepo), cancel: cancel}
	released := &fakeGuard{}
	quotaState := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: time.Now().Unix()}}
	quota := repomocks.NewMockQuotaRepo(gomock.NewController(t))
	quota.EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		next, _, err := update(quotaState)
		require.NoError(t, err)
		quotaState = next
		p.resources.Add(1)
		return err
	}).Times(1)
	p.manager.quotaRepo = quota
	p.manager.centralGuard = &cancelResourceGuard{fakeGuard: released, check: func(ctx context.Context, observed string) {
		require.NoError(t, ctx.Err())
		require.Equal(t, token, observed)
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 8*time.Second)
	}}
	_, err := p.manager.TerminateCurrentRun(parent, f.expt, f.run, f.space, p.event.Session, nil, entity.NoAggrCalculate())
	require.True(t, errno.NeedMQRetry(err))
	require.Len(t, released.releases(), 1)
	require.Zero(t, p.resources.Load(), "required filter recovery must precede shared quota release")
	require.Contains(t, quotaState.ExptID2RunTime, f.expt)
	require.Zero(t, p.notifications.Load())
	var row model.Experiment
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&row).Error)
	require.Equal(t, int32(entity.ExptStatus_Terminated), row.Status)
	ctx := context.Background()
	key := p.manager.makeExptMutexLockKey(f.expt)
	admitted, err := p.manager.mutex.Exists(ctx, key)
	require.NoError(t, err)
	require.True(t, admitted)
	cid := fmt.Sprintf("CompleteExpt:exptexec:onend:%d", f.run)
	marked, err := p.manager.idem.Exist(ctx, cid)
	require.NoError(t, err)
	require.False(t, marked)
	before := f.snapshot()
	p.manager.runFinishRepo = port
	_, err = p.manager.TerminateCurrentRun(ctx, f.expt, f.run, f.space, p.event.Session, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.Equal(t, before, f.snapshot(), "recovery preserves the committed stats, logs and end time")
	require.Equal(t, int32(1), p.resources.Load())
	require.NotContains(t, quotaState.ExptID2RunTime, f.expt)
	admitted, err = p.manager.mutex.Exists(ctx, key)
	require.NoError(t, err)
	require.False(t, admitted)
	marked, err = p.manager.idem.Exist(ctx, cid)
	require.NoError(t, err)
	require.True(t, marked)
	releaseCalls, notifications := len(released.releases()), p.notifications.Load()
	_, err = p.manager.TerminateCurrentRun(ctx, f.expt, f.run, f.space, p.event.Session, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.Equal(t, before, f.snapshot())
	require.Equal(t, int32(1), p.resources.Load())
	require.Len(t, released.releases(), releaseCalls)
	require.Equal(t, notifications, p.notifications.Load())
}

type cancelAfterCommitPort struct {
	repo.IExptRunFinishRepo
	termination repo.IExptRunTerminationRepo
	cancel      context.CancelFunc
}

func (p *cancelAfterCommitPort) GetTerminationItemSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptCurrentItemRunSource, error) {
	return p.termination.GetTerminationItemSource(ctx, space, expt, run, item)
}

func (p *cancelAfterCommitPort) TerminateCurrentRun(ctx context.Context, r *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
	result, err := p.termination.TerminateCurrentRun(ctx, r)
	if err == nil && result.Applied && r.Finalize {
		p.cancel()
	}
	return result, err
}

func TestNewRunRealInitializersAndResetClearOnlyManualExt(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems, entity.EvaluationModeFailRetry} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			f := p.f
			f.token("aa6fe7ea-7db6-4d16-8f5b-2d1344556677")
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": time.Now()}).Error)
			oldLog := f.snapshot()["expt_run_log"]
			released, err := p.manager.mutex.Unlock(p.manager.makeExptMutexLockKey(f.expt))
			require.NoError(t, err)
			require.True(t, released)
			next := f.run + 100
			ctrl := gomock.NewController(t)
			ids := idmocks.NewMockIIDGenerator(ctrl)
			ids.EXPECT().GenID(gomock.Any()).Return(next, nil).AnyTimes()
			var seq atomic.Int64
			ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, n int) ([]int64, error) {
				out := make([]int64, n)
				for i := range out {
					out[i] = f.base + 12000 + seq.Add(1)
				}
				return out, nil
			}).AnyTimes()
			p.manager.idgenerator = ids
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitExptExecRun(f.space, int64(mode))
			p.manager.mtr = metric
			if mode == entity.EvaluationModeRetryItems {
				run, joined, err := p.manager.LogRetryItemsRun(context.Background(), f.expt, mode, f.space, []int64{f.item}, p.event.Session)
				require.NoError(t, err)
				require.False(t, joined)
				require.Equal(t, next, run)
			} else {
				require.NoError(t, p.manager.LogRun(context.Background(), f.expt, next, mode, f.space, []int64{f.item}, p.event.Session))
			}
			var item model.ExptItemResult
			require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&item).Error)
			ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
			require.NoError(t, err)
			require.Contains(t, ext, entity.ManualRetryExtKey, "LogRun must not clear sources before migration")
			event := *p.event
			event.ExptRunID = next
			event.ExptRunMode = mode
			deps := retryItemResetDeps{manager: p.manager, exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), exptRepo: p.manager.exptRepo, idgenerator: ids}
			err = resetRetryItemsForNewRun(context.Background(), deps, &event, p.expt, []*entity.EvaluationSetItem{{ItemID: f.item, ItemVersionID: &f.version, Turns: []*entity.Turn{{ID: 0}}}}, map[int64]int64{f.item: f.version})
			require.NoError(t, err)
			require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&item).Error)
			ext, err = entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
			require.NoError(t, err)
			require.Equal(t, map[string]string{"other": "keep"}, ext)
			original, err := p.manager.runLogRepo.Get(context.Background(), f.expt, f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Success), original.Status)
			var oldRows []map[string]any
			require.NoError(t, f.conn.Table("expt_run_log").Where("space_id=? AND expt_run_id=?", f.space, f.run).Find(&oldRows).Error)
			encoded, err := json.Marshal(oldRows)
			require.NoError(t, err)
			require.Equal(t, oldLog, string(encoded))
			run, err := p.manager.runLogRepo.Get(context.Background(), f.expt, next)
			require.NoError(t, err)
			for _, chunk := range run.ItemIds {
				require.Nil(t, chunk.ManualRetryReceipt)
			}
		})
	}
}

func TestRunCancelAscendingItemLockOrder(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	for _, offset := range []int64{1, 2} {
		require.NoError(t, f.conn.Create(&model.ExptItemResult{ID: f.base + 13000 + offset, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + offset, ItemVersionID: 0, Status: int32(entity.ItemRunState_Queueing)}).Error)
		require.NoError(t, f.conn.Create(&model.ExptItemResultRunLog{ID: f.base + 14000 + offset, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + offset, ItemVersionID: 0, Status: int32(entity.ItemRunState_Queueing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	}
	factory := p.factory
	keys := []string{}
	p.manager.manualRetryLocker = func() lock.ILocker {
		return &cancelHookLocker{ILocker: factory(), hook: func(key string) { keys = append(keys, key) }}
	}
	require.NoError(t, p.manager.TerminateItems(context.Background(), f.expt, f.run, f.space, []int64{f.item + 2, f.item, f.item + 1, f.item}, p.event.Session))
	require.Equal(t, []string{fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run), fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item), fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item+1), fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item+2)}, keys)
}

type cancelLostLeaseLocker struct{ lock.ILocker }

func (l *cancelLostLeaseLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	child, cancel := context.WithCancel(ctx)
	cancel()
	return true, child, func() {}, nil
}

func TestRunCancelLostLeaseHasZeroStateWrites(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	before := p.f.snapshot()
	p.manager.manualRetryLocker = func() lock.ILocker { return &cancelLostLeaseLocker{ILocker: p.factory()} }
	require.True(t, errno.NeedMQRetry(p.manager.SetExptTerminating(context.Background(), p.f.expt, p.f.run, p.f.space, p.event.Session)))
	require.Equal(t, before, p.f.snapshot())
	require.Zero(t, p.resources.Load())
}

func TestRunCancelR2LifecycleOwnerPreventsAnyCleanup(t *testing.T) {
	for _, tc := range []struct {
		name               string
		r1Mode, r2Mode     entity.ExptRunMode
		replayBeforeCreate bool
	}{
		{"retry_items_admission_before_create", entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems, true},
		{"ordinary_admission_before_create", entity.EvaluationModeRetryItems, entity.EvaluationModeRetryAll, true},
		{"persisted_retry_items", entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems, false},
		// Ordinary and numeric replacements retain the same prior-tick protection.
		{"persisted_ordinary", entity.EvaluationModeSubmit, entity.EvaluationModeRetryAll, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runReplacementAdmissionFixture(t, tc.r1Mode, tc.r2Mode, tc.replayBeforeCreate, false)
		})
	}
}

// Record both resource APIs and delegate identity-checked IO to the existing
// bounded Redis compare/delete fixture. An absent old key remains idempotent;
// a foreign run or token is an error, never an unconditional success.
type replacementFixtureGuard struct {
	*fakeGuard
	terminal  *retryQuotaGuard
	sink      *cleanupLeaseSlowGuard
	scope     string
	run, item int64
}

func (g *replacementFixtureGuard) releaseOriginal(ctx context.Context, scope string, run, item int64, token string) error {
	if scope != g.scope || run != g.run || item != g.item {
		return fmt.Errorf("replacement fixture release changed original scope/run/item")
	}
	releaseCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return g.sink.Release(releaseCtx, scope, run, item, token, "original reservation")
}

func (g *replacementFixtureGuard) Release(ctx context.Context, scope string, run, item int64, token, reason string) error {
	if err := g.fakeGuard.Release(ctx, scope, run, item, token, reason); err != nil {
		return err
	}
	return g.releaseOriginal(ctx, scope, run, item, token)
}

func (g *replacementFixtureGuard) ReleaseTerminalReservation(ctx context.Context, scope string, run, item int64, token string) (component.TerminalReservationReleaseResult, error) {
	if _, err := g.terminal.ReleaseTerminalReservation(ctx, scope, run, item, token); err != nil {
		return 0, err
	}
	exists, err := g.sink.raw.Exists(ctx, g.sink.keys[g.item]).Result()
	if err != nil {
		return 0, err
	}
	if err := g.releaseOriginal(ctx, scope, run, item, token); err != nil {
		return 0, err
	}
	if exists == 0 {
		return component.TerminalReservationAbsent, nil
	}
	return component.TerminalReservationReleased, nil
}

// Exercise the actual admission -> old residual cleanup -> Create -> Latest
// sequence. A pending R1 cleanup may release R1 resources before Create; R2's
// space quota and item reservation are acquired only after Latest is confirmed.
func runReplacementAdmissionFixture(t *testing.T, r1Mode, r2Mode entity.ExptRunMode, replayBeforeCreate, pendingSetup bool) {
	t.Helper()
	p := newPendingRunFixture(t, r1Mode)
	f := p.f
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	const originalToken = "aa6fe7ea-7db6-4d16-8f5b-2d1344556677"
	f.token(originalToken) // R1 fixture identity, before the standard SQL completion.
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).
		Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)

	quotaState := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{}}
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		p.resources.Add(1)
		next, _, err := update(quotaState)
		if err == nil && next != nil {
			quotaState = next
		}
		return err
	}).AnyTimes()
	p.manager.quotaRepo = quota
	quotaService := NewQuotaService(quota, p.manager.configer)
	require.NoError(t, quotaService.AllowExptRun(ctx, f.expt, f.space, p.event.Session))
	require.Contains(t, quotaState.ExptID2RunTime, f.expt, "the pre-existing slot was acquired for R1")
	_, resourceServer := finishRealLockerFactory(t)
	raw := goredis.NewClient(&goredis.Options{Addr: resourceServer.Addr()})
	t.Cleanup(func() { require.NoError(t, raw.Close()) })
	originalReservation := fmt.Sprintf("r2-fixture-reservation:%d:%d", f.run, f.item)
	require.NoError(t, raw.Set(ctx, originalReservation, originalToken, time.Minute).Err())
	guard := &replacementFixtureGuard{
		fakeGuard: &fakeGuard{releaseErr: errors.New("R1 resource cleanup temporarily unavailable")},
		terminal:  &retryQuotaGuard{}, scope: testScope, run: f.run, item: f.item,
		sink: &cleanupLeaseSlowGuard{raw: raw, keys: map[int64]string{f.item: originalReservation}, expected: map[int64]string{f.item: originalToken}},
	}
	p.manager.centralGuard = guard

	originalCID := fmt.Sprintf("r2-fixture-r1:%d", f.run)
	complete := func(cid string) error {
		opts := []entity.CompleteExptOptionFn{entity.WithCID(cid), entity.WithStatus(entity.ExptStatus_Terminated), entity.NoAggrCalculate()}
		if pendingSetup {
			return p.manager.CompleteRun(ctx, f.expt, f.run, f.space, p.event.Session, opts...)
		}
		return p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session, opts...)
	}
	expectedExperiment, expectedRun, expectedItem := entity.ExptStatus_Terminated, entity.ExptStatus_Terminated, entity.ItemRunState_Terminal
	if pendingSetup {
		// R15 deliberately retains the old Pending run on an already finished
		// experiment. Publish a consistent Success item through the real ports,
		// but do not call CompleteRun or finish the old run during setup.
		guard.releaseErr = nil
		p.publishSuccess(t, false)
		endedAt := time.Now().Add(-time.Minute)
		require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).
			Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": endedAt}).Error)
		expectedExperiment, expectedRun, expectedItem = entity.ExptStatus_Success, entity.ExptStatus_Pending, entity.ItemRunState_Success
	} else {
		// R01 uses the standard SQL termination boundary. A real old resource
		// failure keeps Done pending after committing consistent terminal rows.
		require.True(t, errno.NeedMQRetry(complete(originalCID)))
	}
	original, err := p.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, expectedExperiment, original.Status)
	require.NotNil(t, original.EndAt)
	logged, err := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
	require.NoError(t, err)
	require.Equal(t, int64(expectedRun), logged.Status)
	termination, err := p.manager.terminationRepo()
	require.NoError(t, err)
	source, err := termination.GetTerminationItemSource(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Equal(t, expectedItem, source.Current.Status)
	require.Equal(t, f.run, source.Current.RunID)
	require.Equal(t, originalToken, source.Current.RequestID)
	var canonical model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Take(&canonical).Error)
	require.Equal(t, int32(expectedItem), canonical.Status)
	require.Equal(t, f.run, canonical.ExptRunID)
	require.Contains(t, quotaState.ExptID2RunTime, f.expt, "the original slot exists before its delayed cleanup")
	require.Equal(t, originalToken, raw.Get(ctx, originalReservation).Val())
	seen, err := p.manager.idem.Exist(ctx, fmt.Sprintf("CompleteExpt:r2-fixture-r1:%d", f.run))
	require.NoError(t, err)
	require.False(t, seen)
	require.Eventually(t, func() bool {
		_, exists, err := p.factory().(lifecycleLockValueReader).GetLifecycleLockValue(ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run))
		return err == nil && !exists
	}, 2*time.Second, 10*time.Millisecond)

	lockKey := p.manager.makeExptMutexLockKey(f.expt)
	// Explicitly model original admission expiry before R2 starts. R01 has
	// committed terminal SQL; R15 retains the Pending setup compensation state.
	var released bool
	if r1Mode == entity.EvaluationModeRetryItems {
		released, err = p.manager.mutex.UnlockWithValue(ctx, lockKey, fmt.Sprint(f.run))
	} else {
		released, err = p.manager.mutex.Unlock(lockKey)
	}
	require.NoError(t, err)
	require.True(t, released)
	guard.releaseErr = nil

	if pendingSetup && replayBeforeCreate {
		// CompleteRun's Pending setup compensation must actually execute before
		// R2 owns the previous tick. Once R2 enters Create, replay must retry.
		before := f.snapshot()
		callsBefore := p.resources.Load()
		releasesBefore := guard.releases()
		terminalBefore := append([]retryQuotaRelease(nil), guard.terminal.releases...)
		oldRun, readErr := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
		require.NoError(t, readErr)
		require.Equal(t, int64(entity.ExptStatus_Pending), oldRun.Status)
		finished, readErr := p.manager.exptRepo.GetByID(ctx, f.expt, f.space)
		require.NoError(t, readErr)
		require.Equal(t, entity.ExptStatus_Success, finished.Status)
		require.Equal(t, original.EndAt, finished.EndAt)
		t.Logf("R15_PENDING_COMPENSATION_BEFORE run_status=%d experiment_status=%d end_at=%v", oldRun.Status, finished.Status, finished.EndAt)
		require.NoError(t, complete(originalCID))
		require.Greater(t, p.resources.Load(), callsBefore, "actual Pending compensation releases R1 space quota")
		require.Empty(t, quotaState.ExptID2RunTime)
		after := f.snapshot()
		oldRun, readErr = p.manager.runLogRepo.Get(ctx, f.expt, f.run)
		require.NoError(t, readErr)
		require.Equal(t, int64(entity.ExptStatus_Failed), oldRun.Status)
		for table, rows := range before {
			if table != "expt_run_log" {
				require.Equal(t, rows, after[table], "Pending compensation preserves experiment/source table %s", table)
			}
		}
		unchangedSource, readErr := termination.GetTerminationItemSource(ctx, f.space, f.expt, f.run, f.item)
		require.NoError(t, readErr)
		require.Equal(t, source, unchangedSource)
		require.Equal(t, releasesBefore, guard.releases())
		require.Equal(t, terminalBefore, guard.terminal.releases)
		t.Logf("R15_PENDING_COMPENSATION_AFTER run_status=%d experiment_status=%d END_AT_AND_SOURCE_UNCHANGED", oldRun.Status, original.Status)
	}

	r2 := f.run + 100
	ids := idmocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenID(gomock.Any()).Return(r2, nil).Times(1)
	var sequence atomic.Int64
	ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, count int) ([]int64, error) {
		generated := make([]int64, count)
		for i := range generated {
			generated[i] = f.base + 12000 + sequence.Add(1)
		}
		return generated, nil
	}).AnyTimes()
	p.manager.idgenerator = ids // the original fixture's AnyTimes GenID returns R1.
	p.manager.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecRun(f.space, int64(r2Mode))
	barrier := &lifecycleBlockingRunLogRepo{IExptRunLogRepo: p.manager.runLogRepo, entered: make(chan struct{}), release: make(chan struct{})}
	p.manager.runLogRepo = barrier
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(barrier.release) }) }
	done := make(chan struct{})
	var newRun int64
	var joined bool
	var startErr error
	go func() {
		defer close(done)
		newRun, joined, startErr = logSchedulerRetryFixtureRun(ctx, p, r2Mode)
	}()
	t.Cleanup(func() {
		unblock()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("R2 entry did not finish after releasing Create barrier")
		}
	})
	select {
	case <-barrier.entered:
	case <-done:
		t.Fatalf("R2 never reached Create: %v", startErr)
	case <-time.After(10 * time.Second):
		t.Fatal("R2 did not reach Create barrier")
	}

	owner := legacyStaticHolderValue
	if r2Mode == entity.EvaluationModeRetryItems {
		owner = fmt.Sprint(r2)
	}
	assertAdmission := func() {
		actual, exists, readErr := p.factory().(lifecycleLockValueReader).GetLifecycleLockValue(ctx, lockKey)
		require.NoError(t, readErr)
		require.True(t, exists)
		require.Equal(t, owner, actual)
	}
	assertOriginalReleases := func() {
		calls := guard.releases()
		require.NotEmpty(t, calls)
		for _, call := range calls {
			require.Equal(t, testScope, call.Scope)
			require.Equal(t, f.run, call.RunID)
			require.Equal(t, f.item, call.ItemID)
			require.Equal(t, originalToken, call.RequestID, "old cleanup must keep the original token")
		}
		for _, call := range guard.terminal.releases {
			require.Equal(t, testScope, call.scope)
			require.Equal(t, f.run, call.runID)
			require.Equal(t, f.item, call.itemID)
			require.Equal(t, originalToken, call.token, "initializer releases only the original terminal reservation")
		}
	}
	assertAdmission()
	current, err := p.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, f.run, current.LatestRunID)
	var count int64
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, r2).Count(&count).Error)
	require.Zero(t, count)
	require.Empty(t, quotaState.ExptID2RunTime, "R2 entry has released R1's slot; Allow for R2 has not run")
	require.Zero(t, raw.Exists(ctx, originalReservation).Val(), "superseded cleanup deleted only R1's original reservation")
	assertOriginalReleases()
	t.Logf("R2_PRE_CREATE r1=%d r2=%d owner=%s latest=%d r2_rows=%d quota=%v old_token=%s", f.run, r2, owner, current.LatestRunID, count, quotaState.ExptID2RunTime, originalToken)
	if replayBeforeCreate {
		before := f.snapshot()
		callsBefore := p.resources.Load()
		releasesBefore := guard.releases()
		terminalBefore := append([]retryQuotaRelease(nil), guard.terminal.releases...)
		replayErr := complete(originalCID)
		require.True(t, errno.NeedMQRetry(replayErr), "R2 owns the actual previous tick before Create; old cleanup must retry")
		require.Equal(t, callsBefore, p.resources.Load(), "busy old completion cannot reach shared quota callbacks")
		require.Equal(t, before, f.snapshot(), "busy replay preserves both run/source SQL")
		require.Equal(t, releasesBefore, guard.releases())
		require.Equal(t, terminalBefore, guard.terminal.releases)
		if pendingSetup {
			oldRun, readErr := p.manager.runLogRepo.Get(ctx, f.expt, f.run)
			require.NoError(t, readErr)
			require.Equal(t, int64(entity.ExptStatus_Failed), oldRun.Status, "R15 already exercised Pending compensation before R2 admission")
		}
		assertAdmission()
		assertOriginalReleases()
	}

	unblock()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("R2 entry did not finish")
	}
	require.NoError(t, startErr)
	require.False(t, joined)
	require.Equal(t, r2, newRun)
	current, err = p.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, r2, current.LatestRunID)
	logged, err = p.manager.runLogRepo.Get(ctx, f.expt, r2)
	require.NoError(t, err)
	require.Equal(t, int32(r2Mode), logged.Mode)
	require.Equal(t, int64(entity.ExptStatus_Pending), logged.Status)
	require.Empty(t, quotaState.ExptID2RunTime)
	// The quota portion of prepareRun, after its current experiment read and
	// after real Create/Latest. Run/RetryItems calls this same AllowExptRun.
	require.NoError(t, quotaService.AllowExptRun(ctx, f.expt, f.space, p.event.Session))
	require.Contains(t, quotaState.ExptID2RunTime, f.expt)

	current.EvalSet = &entity.EvaluationSet{ID: f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 81}}
	itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	items := []*entity.EvaluationSetItem{{ItemID: f.item, ItemVersionID: &f.version, Turns: []*entity.Turn{{ID: 0}}}}
	event := &entity.ExptScheduleEvent{ExptID: f.expt, ExptRunID: r2, SpaceID: f.space, ExptType: entity.ExptType_Offline, ExptRunMode: r2Mode, Session: p.event.Session}
	if r2Mode == entity.EvaluationModeRetryItems {
		itemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, nil)
		event.ExecEvalSetItemIDs = []int64{f.item}
		exec := &ExptRetryItemsExec{manager: p.manager, exptItemResultRepo: p.manager.itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo, exptStatsRepo: p.manager.statsRepo, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo, idem: p.manager.idem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo}
		require.NoError(t, exec.ExptStart(ctx, event, current))
	} else {
		itemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, gptr.Of(int64(1)), nil, nil, nil)
		exec := &ExptRetryAllExec{manager: p.manager, exptItemResultRepo: p.manager.itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo, exptStatsRepo: p.manager.statsRepo, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo, idem: p.manager.idem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo}
		require.NoError(t, exec.ExptStart(ctx, event, current))
	}
	require.NotEmpty(t, guard.terminal.releases, "the real initializer releases R1 terminal reservation")
	assertOriginalReleases()
	dispatch := exptrepo.NewExptItemDispatchRepo(exptmysql.NewExptItemDispatchDAO(f.provider))
	runtime, err := dispatch.LoadDispatchRuntime(ctx, f.space, f.expt, r2, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{f.item}, runtime.CandidateItemIDs)
	require.Zero(t, raw.Exists(ctx, originalReservation).Val(), "old reservation is cleared before the R2 grant")
	r2Reservation := fmt.Sprintf("r2-fixture-reservation:%d:%d", r2, f.item)
	// The OSS fixture grants the local ledger key from the actual dispatch
	// observation after initialization, then commits the real SQL claim.
	require.NoError(t, raw.Set(ctx, r2Reservation, runtime.ItemRequestIDs[f.item], time.Minute).Err())
	claims, err := dispatch.ClaimQuotaReserved(ctx, f.space, f.expt, r2, runtime.CandidateItemIDs, runtime.ItemRequestIDs)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	r2Source, err := f.port.GetCurrentItemRunSource(ctx, f.space, f.expt, r2, f.item)
	require.NoError(t, err)
	require.NotNil(t, r2Source)
	require.Equal(t, r2, r2Source.Current.RunID)
	require.Equal(t, entity.ItemRunState_Queueing, r2Source.Current.Status)
	reserved, err := dispatch.MGetDispatchObservations(ctx, f.space, f.expt, r2, []int64{f.item})
	require.NoError(t, err)
	require.Len(t, reserved, 1)
	require.Equal(t, entity.QuotaReservationStateReserved, reserved[0].QuotaReservationState)
	require.Equal(t, r2Source.Current.RequestID, reserved[0].RequestID)
	require.Equal(t, claims[0].RequestID, r2Source.Current.RequestID)
	require.NotEqual(t, originalToken, r2Source.Current.RequestID)

	before := f.snapshot()
	quotaBefore := quotaState.ExptID2RunTime[f.expt]
	callsBefore, notificationsBefore := p.resources.Load(), p.notifications.Load()
	releasesBefore := guard.releases()
	terminalReleasesBefore := append([]retryQuotaRelease(nil), guard.terminal.releases...)
	lateCID := originalCID + ":after-latest"
	seen, err = p.manager.idem.Exist(ctx, "CompleteExpt:"+lateCID)
	require.NoError(t, err)
	require.False(t, seen, "the final replay is not hidden by a previous Done")
	require.NoError(t, complete(lateCID), "Latest=R2 makes the delayed original run non-current")
	if pendingSetup && !replayBeforeCreate {
		requireHistoricalRunSnapshot(t, before, f.snapshot(), f.run, entity.ExptStatus_Terminated, [5]int64{0, 0, 1, 0, 0}, "")
	} else {
		require.Equal(t, before, f.snapshot(), "R2 reservation/source and both runs' SQL must remain byte-equivalent")
	}
	require.Equal(t, map[int64]int64{f.expt: quotaBefore}, quotaState.ExptID2RunTime, "the actual R2 slot is preserved")
	require.Equal(t, callsBefore, p.resources.Load())
	require.Equal(t, notificationsBefore, p.notifications.Load())
	require.Equal(t, releasesBefore, guard.releases(), "no R2 RequestID substitutes for the delayed R1 token")
	require.Equal(t, terminalReleasesBefore, guard.terminal.releases)
	require.Zero(t, raw.Exists(ctx, originalReservation).Val())
	require.Equal(t, int64(1), raw.Exists(ctx, r2Reservation).Val(), "actual R2 reservation key remains present")
	require.Equal(t, claims[0].RequestID, raw.Get(ctx, r2Reservation).Val())
	assertAdmission()
	t.Logf("R2_CONFIRMED latest=%d quota=%v reservation=%d request=%q old_request=%q replay_before_create=%v pending_setup=%v SQL_UNCHANGED", r2, quotaState.ExptID2RunTime, reserved[0].QuotaReservationState, r2Source.Current.RequestID, originalToken, replayBeforeCreate, pendingSetup)
}

// Normalize only independently asserted R1 history fields; all other SQL stays protected.
func requireHistoricalRunSnapshot(t *testing.T, before, after map[string]string, run int64, status entity.ExptStatus, counts [5]int64, message string) {
	t.Helper()
	var oldRows, newRows []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(before["expt_run_log"]), &oldRows))
	require.NoError(t, json.Unmarshal([]byte(after["expt_run_log"]), &newRows))
	require.Len(t, newRows, len(oldRows))
	found := false
	for i, old := range oldRows {
		if string(old["expt_run_id"]) != fmt.Sprint(run) {
			continue
		}
		found = true
		require.Equal(t, fmt.Sprint(run), string(newRows[i]["expt_run_id"]))
		require.Equal(t, fmt.Sprint(int64(status)), string(newRows[i]["status"]))
		newRows[i]["status"] = old["status"]
		for index, field := range []string{"pending_cnt", "processing_cnt", "success_cnt", "fail_cnt", "terminated_cnt"} {
			require.Equal(t, fmt.Sprint(counts[index]), string(newRows[i][field]), field)
			newRows[i][field] = old[field]
		}
		var oldTime, newTime time.Time
		require.NoError(t, json.Unmarshal(old["updated_at"], &oldTime))
		require.NoError(t, json.Unmarshal(newRows[i]["updated_at"], &newTime))
		require.False(t, newTime.Before(oldTime), "historical updated_at cannot move backwards")
		newRows[i]["updated_at"] = old["updated_at"]
		if message != "" {
			var actual []byte
			require.NoError(t, json.Unmarshal(newRows[i]["status_message"], &actual))
			require.Equal(t, message, string(actual))
			newRows[i]["status_message"] = old["status_message"]
		}
	}
	require.True(t, found, "R1 historical row must exist in the SQL snapshot")
	oldJSON, err := json.Marshal(oldRows)
	require.NoError(t, err)
	newJSON, err := json.Marshal(newRows)
	require.NoError(t, err)
	oldSnapshot, newSnapshot := map[string]string{}, map[string]string{}
	for key, value := range before {
		oldSnapshot[key] = value
	}
	for key, value := range after {
		newSnapshot[key] = value
	}
	oldSnapshot["expt_run_log"], newSnapshot["expt_run_log"] = string(oldJSON), string(newJSON)
	require.Equal(t, oldSnapshot, newSnapshot, "R1 other fields, R2 and all canonical/shared SQL must remain byte-equivalent")
}

func TestNewRunResetReleasesOriginalTerminalManualQuotaBeforeMigration(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	token := "aa6fe7ea-7db6-4d16-8f5b-2d1344556677"
	f.token(token)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	guard := &fakeGuard{}
	p.manager.centralGuard = guard
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Terminated)).Error)
	released, err := p.manager.mutex.Unlock(p.manager.makeExptMutexLockKey(f.expt))
	require.NoError(t, err)
	require.True(t, released)
	metric := metricsmocks.NewMockExptMetric(gomock.NewController(t))
	metric.EXPECT().EmitExptExecRun(f.space, int64(entity.EvaluationModeRetryAll))
	p.manager.mtr = metric
	require.NoError(t, p.manager.LogRun(context.Background(), f.expt, f.run+100, entity.EvaluationModeRetryAll, f.space, []int64{f.item}, p.event.Session))
	calls := guard.releases()
	require.Len(t, calls, 1)
	require.Equal(t, f.run, calls[0].RunID)
	require.Equal(t, token, calls[0].RequestID)
}

func TestNewRunReleasesResidualTerminalQuota(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	f.token("aa6fe7ea-7db6-4d16-8f5b-2d1344556677")
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope, "status": int32(entity.ExptStatus_Terminated), "end_at": time.Now()}).Error)
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Terminated)).Error)
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Terminal)).Error)
	}
	guard := &fakeGuard{}
	p.manager.centralGuard = guard
	released, err := p.manager.mutex.Unlock(p.manager.makeExptMutexLockKey(f.expt))
	require.NoError(t, err)
	require.True(t, released)
	metric := metricsmocks.NewMockExptMetric(gomock.NewController(t))
	metric.EXPECT().EmitExptExecRun(f.space, int64(entity.EvaluationModeRetryAll))
	p.manager.mtr = metric
	require.NoError(t, p.manager.LogRun(context.Background(), f.expt, f.run+100, entity.EvaluationModeRetryAll, f.space, []int64{f.item}, p.event.Session))
	require.Len(t, guard.releases(), 1)
	require.Equal(t, "aa6fe7ea-7db6-4d16-8f5b-2d1344556677", guard.releases()[0].RequestID)
}

func TestRunCancelSandboxUsesCapturedOriginalRecordIDs(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	original := f.base + 17000
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("target_result_id", original).Error)
	target := svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
	target.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), f.space, []int64{original}, int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false).Do(func(ctx context.Context, _ int64, ids []int64, _ int32, _ string, _ bool, _ ...bool) {
		require.NoError(t, ctx.Err())
		require.Equal(t, []int64{original}, ids)
	})
	p.manager.evalTargetService = target
	_, err := p.manager.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, p.event.Session, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	var log model.ExptTurnResultRunLog
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&log).Error)
	require.Equal(t, original, log.TargetResultID)
}

func TestRunCancelFailedItemStillCleansOriginalAsyncRecord(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	original := f.base + 18000
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("target_result_id", original).Error)
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Fail)).Error)
	}
	target := svcmocks.NewMockIEvalTargetService(gomock.NewController(t))
	target.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), f.space, []int64{original}, int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false)
	p.manager.evalTargetService = target
	_, err := p.manager.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, p.event.Session, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.NoAggrCalculate())
	require.NoError(t, err)
}

func TestNewRunActualRetryItemsResetReleasesHistoricalSourceToken(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	historical := f.run - 100
	next := f.run + 100
	token := "aa6fe7ea-7db6-4d16-8f5b-2d1344556677"
	raw, err := json.Marshal(&entity.ManualRetryContext{RunID: historical, RequestID: token, Mode: entity.EvaluationModeRetryItems, Operator: "old", AcceptedAtMS: 1})
	require.NoError(t, err)
	ext, err := json.Marshal(map[string]string{"other": "keep", entity.ManualRetryExtKey: string(raw)})
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Updates(map[string]any{"expt_run_id": historical, "status": int32(entity.ItemRunState_Fail), "ext": ext}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("expt_run_id", historical).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"expt_run_id": historical, "status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("expt_run_id", historical).Error)
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: historical, SpaceID: f.space, ExptID: f.expt, ExptRunID: historical, Status: gptr.Of(int64(entity.ExptStatus_Success)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: next, SpaceID: f.space, ExptID: f.expt, ExptRunID: next, Status: gptr.Of(int64(entity.ExptStatus_Pending)), Mode: gptr.Of(int32(entity.EvaluationModeRetryItems))}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"latest_run_id": next, "scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	ctrl := gomock.NewController(t)
	guard := confmocks.NewMockICentralReservationGuard(ctrl)
	guard.EXPECT().ReleaseTerminalReservation(gomock.Any(), testScope, historical, f.item, token).Return(component.TerminalReservationReleased, nil)
	p.manager.centralGuard = guard
	items := svcmocks.NewMockEvaluationSetItemService(ctrl)
	items.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, ItemVersionID: &f.version, Turns: []*entity.Turn{{ID: 0}}}}, nil)
	event := *p.event
	event.ExptRunID = next
	event.ExptRunMode = entity.EvaluationModeRetryItems
	expt := *p.expt
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 81}}
	reset := &ExptRetryItemsExec{manager: p.manager, exptStatsRepo: p.manager.statsRepo, exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), evaluationSetItemService: items, idgenerator: p.manager.idgenerator}
	require.NoError(t, reset.resetEvalItems(context.Background(), &event, &expt, []int64{f.item}))
	var item model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&item).Error)
	parsed, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
	require.NoError(t, err)
	require.Equal(t, map[string]string{"other": "keep"}, parsed)
	require.Equal(t, next, item.ExptRunID)
}

func TestNewRunResetQuotaMismatchAndErrorDoNotMigrate(t *testing.T) {
	for _, kind := range []string{"mismatch", "error", "absent"} {
		t.Run(kind, func(t *testing.T) {
			p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
			f := p.f
			token := "aa6fe7ea-7db6-4d16-8f5b-2d1344556677"
			f.token(token)
			next := f.run + 100
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"latest_run_id": next, "scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
			guard := confmocks.NewMockICentralReservationGuard(gomock.NewController(t))
			result := component.TerminalReservationMismatch
			var failure error
			if kind == "error" {
				failure = errors.New("quota unavailable")
			}
			if kind == "absent" {
				result = component.TerminalReservationAbsent
			}
			guard.EXPECT().ReleaseTerminalReservation(gomock.Any(), testScope, f.run, f.item, token).Return(result, failure)
			p.manager.centralGuard = guard
			before := f.snapshot()
			err := p.manager.ResetItemsForNewRun(context.Background(), f.space, f.expt, next, []int64{f.item}, map[string]any{"expt_run_id": next, "status": int32(entity.ItemRunState_Queueing)})
			if kind != "absent" {
				require.True(t, errno.NeedMQRetry(err))
				require.Equal(t, before, f.snapshot())
			} else {
				require.NoError(t, err)
				var item model.ExptItemResult
				require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&item).Error)
				require.Equal(t, next, item.ExptRunID)
				ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
				require.NoError(t, err)
				require.Equal(t, map[string]string{"other": "keep"}, ext)
			}
		})
	}
}
