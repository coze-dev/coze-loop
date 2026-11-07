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
	require.Equal(t, int32(1), p.resources.Load())
	require.Zero(t, p.notifications.Load())
	var row model.Experiment
	require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&row).Error)
	require.Equal(t, int32(entity.ExptStatus_Terminated), row.Status)
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
		name           string
		r1Mode, r2Mode entity.ExptRunMode
		persist        bool
	}{
		{"retry_items_admission_before_create", entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems, false},
		{"ordinary_admission_before_create", entity.EvaluationModeRetryItems, entity.EvaluationModeRetryAll, false},
		{"persisted_retry_items", entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems, true},
		{"persisted_ordinary", entity.EvaluationModeSubmit, entity.EvaluationModeRetryAll, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPendingRunFixture(t, tc.r1Mode)
			f := p.f
			ctx := context.Background()
			// Capture and commit actual original sources through the standard SQL boundary.
			require.NoError(t, p.manager.withTerminationRunLock(ctx, f.expt, f.run, func(lockCtx context.Context) error {
				data, err := p.manager.commitCompleteData(lockCtx, p.expt, f.run, &entity.CompleteExptOption{Status: entity.ExptStatus_Terminated})
				if err == nil {
					require.Equal(t, entity.ExptRunFinishFinished, data.result.Disposition)
					require.Equal(t, entity.ExptStatus_Terminated, data.result.Experiment.Status)
				}
				return err
			}))
			require.Eventually(t, func() bool {
				_, exists, err := p.factory().(lifecycleLockValueReader).GetLifecycleLockValue(ctx, fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run))
				return err == nil && !exists
			}, 2*time.Second, 10*time.Millisecond)
			require.Zero(t, p.resources.Load())
			require.Zero(t, p.notifications.Load())
			lockKey := p.manager.makeExptMutexLockKey(f.expt)
			var released bool
			var err error
			if tc.r1Mode == entity.EvaluationModeRetryItems {
				released, err = p.manager.mutex.UnlockWithValue(ctx, lockKey, fmt.Sprint(f.run))
			} else {
				released, err = p.manager.mutex.Unlock(lockKey)
			}
			require.NoError(t, err)
			require.True(t, released)
			r2 := f.run + 100
			replacement := p.factory()
			owner := legacyStaticHolderValue
			var locked bool
			if tc.r2Mode == entity.EvaluationModeRetryItems {
				owner = fmt.Sprint(r2)
				locked, _, err = replacement.BackoffLockWithValue(ctx, lockKey, owner, time.Minute, time.Millisecond)
			} else {
				locked, err = replacement.WithHolder(owner).LockBackoff(ctx, lockKey, time.Minute, time.Millisecond)
			}
			require.NoError(t, err)
			require.True(t, locked)
			persist := func() {
				require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: r2, SpaceID: f.space, ExptID: f.expt, ExptRunID: r2, Mode: gptr.Of(int32(tc.r2Mode)), Status: gptr.Of(int64(entity.ExptStatus_Pending))}).Error)
				require.NoError(t, p.manager.exptRepo.Update(ctx, &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: r2}))
			}
			if tc.persist {
				persist()
			} else {
				current, readErr := p.manager.exptRepo.GetByID(ctx, f.expt, f.space)
				require.NoError(t, readErr)
				require.Equal(t, f.run, current.LatestRunID)
				var count int64
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, r2).Count(&count).Error)
				require.Zero(t, count, "R2 has only admission, not Create or LatestRunID")
			}
			quotaState := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: time.Now().Add(-time.Minute).Unix()}}
			quota := repomocks.NewMockQuotaRepo(gomock.NewController(t))
			quota.EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
				p.resources.Add(1)
				next, _, err := update(quotaState)
				if next != nil {
					quotaState = next
				}
				return err
			}).AnyTimes()
			p.manager.quotaRepo = quota
			before := f.snapshot()
			err = p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session, entity.WithStatus(entity.ExptStatus_Terminated), entity.NoAggrCalculate())
			t.Logf("R2_WINDOW persist=%v original=%d replacement=%d quota_calls=%d notifications=%d quota=%v", tc.persist, f.run, r2, p.resources.Load(), p.notifications.Load(), quotaState.ExptID2RunTime)
			require.Zero(t, p.resources.Load(), "R1 cannot release shared quota under replacement admission")
			require.Contains(t, quotaState.ExptID2RunTime, f.expt)
			require.Zero(t, p.notifications.Load())
			require.Equal(t, before, f.snapshot())
			actual, exists, readErr := p.factory().(lifecycleLockValueReader).GetLifecycleLockValue(ctx, lockKey)
			require.NoError(t, readErr)
			require.True(t, exists)
			require.Equal(t, owner, actual)
			if tc.persist {
				require.NoError(t, err)
			} else {
				require.True(t, errno.NeedMQRetry(err))
				persist()
				require.NoError(t, p.manager.CompleteExpt(ctx, f.expt, &f.run, f.space, p.event.Session, entity.WithStatus(entity.ExptStatus_Terminated)))
				require.Zero(t, p.resources.Load())
				require.Contains(t, quotaState.ExptID2RunTime, f.expt)
			}
		})
	}
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
