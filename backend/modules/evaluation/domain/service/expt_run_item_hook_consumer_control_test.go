// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// The existing generic consumer fixture is explicitly after-only, not a frozen Run.
func (p *itemHookPorts) ApplyHookConsumerControl(_ context.Context, in entity.HookConsumerControlInput) (entity.HookConsumerControlResult, error) {
	if p.run == nil || p.run.State.Key != in.Key || p.run.State.Before.Status != entity.HookOperationDisabled || p.run.State.After.Status == entity.HookOperationDisabled {
		return entity.HookConsumerControlResult{Handled: true}, entity.ErrHookStoreCorrupt
	}
	return entity.HookConsumerControlResult{}, nil
}

func consumerControlFixture(t *testing.T, state entity.ItemRunState) (*finalizationManagerFixture, entity.HookExecutionManifest, *ExptItemEventEvalServiceImpl, *entity.ExptItemEvalEvent) {
	t.Helper()
	f, manifests := activeTerminationFixture(t, state)
	m := manifests[0]
	m.Frozen.ItemVersionID = 7
	digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), []entity.HookPlanItem{m.Frozen})
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("plan_hash", digest.Hash).Error)
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumns(map[string]any{"item_version_id": 7, "execution_manifest": raw}).Error)
	for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log", "expt_item_result", "expt_turn_result"} {
		require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID).UpdateColumn("item_version_id", 7).Error)
	}
	if state == entity.ItemRunState_Processing {
		require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 0, "processing_cnt": 1}).Error)
	}
	items, turns := lazyTurnRepos(f)
	metrics := mm.NewMockExptMetric(gomock.NewController(t))
	metrics.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	svc := &ExptItemEventEvalServiceImpl{
		manager: f.manager, mutex: lock.NewRedisLocker(f.redis), exptItemResultRepo: items, exptTurnResultRepo: turns,
		exptStatsRepo: exptinfra.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.p)),
		dispatchRepo:  exptinfra.NewExptItemDispatchRepo(exptmysql.NewExptItemDispatchDAO(f.p)),
		metric:        metrics, publisher: &itemHookPublisher{},
		hookAdmission: &itemHookAdmission{source: exptinfra.NewHookItemSourceRepo(f.p), progress: exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil })},
	}
	event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, EvalSetItemID: m.Frozen.ItemID, ExptRunMode: entity.EvaluationModeSubmit, RetryTimes: 1, MaxRetryTimes: 1}
	return f, m, svc, event
}

func consumerControlSnapshot(t *testing.T, f *finalizationManagerFixture) string {
	t.Helper()
	out := map[string]any{}
	for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log", "expt_item_result", "expt_turn_result", "expt_stats"} {
		var rows []map[string]any
		require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(&rows).Error)
		out[table] = rows
	}
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	return string(raw)
}

func consumerControlCancel(t *testing.T, f *finalizationManagerFixture, newLatest bool) {
	t.Helper()
	require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
	if newLatest {
		run := finalizationTestIDs.Add(1)
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("latest_run_id", run).Error)
		for _, table := range []string{"expt_item_result", "expt_turn_result"} {
			require.NoError(t, f.sql.Table(table).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"expt_run_id": run, "status": 0}).Error)
		}
		require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumns(map[string]any{"pending_cnt": 4, "processing_cnt": 3, "success_cnt": 2, "fail_cnt": 1, "terminated_cnt": 0}).Error)
	}
}

func TestHookConsumerOuterErrorAfterUnlockCannotReviveMySQL(t *testing.T) {
	for _, kind := range []string{"yield", "fail"} {
		for _, newer := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/newLatest=%t", kind, newer), func(t *testing.T) {
				f, _, svc, event := consumerControlFixture(t, entity.ItemRunState_Processing)
				if kind == "yield" {
					event.RetryTimes = 0
					event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
				}
				config := cm.NewMockIConfiger(gomock.NewController(t))
				svc.configer = config
				var sealed string
				config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).DoAndReturn(func(context.Context, int64, error) *entity.RetryConf {
					require.Zero(t, f.redis.Exists(context.Background(), fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, event.EvalSetItemID)).Val(), "the real inner lock must be released before the outer decision")
					consumerControlCancel(t, f, newer)
					sealed = consumerControlSnapshot(t, f)
					return &entity.RetryConf{RetryTimes: 1}
				})
				endpoint := svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("execution failed") }))
				require.NoError(t, endpoint(context.Background(), event))
				require.Equal(t, sealed, consumerControlSnapshot(t, f), "outer error handling cannot invalidate the cancellation proof or new Latest")
			})
		}
	}
}

type consumerAllocationError struct {
	idgen.IIDGenerator
	before func()
}

func (g consumerAllocationError) GenMultiIDs(context.Context, int) ([]int64, error) {
	if g.before != nil {
		g.before()
	}
	return nil, errors.New("ID service temporarily unavailable")
}

func TestHookConsumerIDErrorStaysControlMySQL(t *testing.T) {
	f, m, svc, event := consumerControlFixture(t, entity.ItemRunState_Queueing)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	pre.idgen = consumerAllocationError{}
	config := cm.NewMockIConfiger(gomock.NewController(t))
	svc.configer = config
	generic := 0
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).DoAndReturn(func(context.Context, int64, error) *entity.RetryConf { generic++; return &entity.RetryConf{} }).AnyTimes()
	before := consumerControlSnapshot(t, f)
	require.NoError(t, svc.HandleEventErr(svc.HandleEventLock(func(callCtx context.Context, _ *entity.ExptItemEvalEvent) error { return pre.PreEval(callCtx, eiec) }))(ctx, event))
	require.Zero(t, generic, "ID dependency errors must not consume business retry/fallback")
	require.Equal(t, before, consumerControlSnapshot(t, f))
	assertLazyTurnCount(t, f, 0)
	require.Len(t, svc.publisher.(*itemHookPublisher).events, 1)
	require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
	assertLazyTurnFinalized(t, f, m)
}

type consumerReservationGuard struct {
	component.ICentralReservationGuard
	before    func()
	confirmed bool
}

func (g *consumerReservationGuard) ConfirmRunning(context.Context, string, int64, int64) (bool, error) {
	if g.before != nil {
		g.before()
	}
	return g.confirmed, nil
}
func (*consumerReservationGuard) Release(context.Context, string, int64, int64, string) error {
	return nil
}

func TestHookConsumerReservationLostLeaseCannotProjectMySQL(t *testing.T) {
	for _, start := range []bool{true, false} {
		t.Run(fmt.Sprintf("start=%t", start), func(t *testing.T) {
			f, _, svc, event := consumerControlFixture(t, entity.ItemRunState_Processing)
			var sealed string
			svc.centralGuard = &consumerReservationGuard{confirmed: start, before: func() {
				ok, err := svc.mutex.Unlock(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, event.EvalSetItemID))
				require.NoError(t, err)
				require.True(t, ok)
				consumerControlCancel(t, f, true)
				sealed = consumerControlSnapshot(t, f)
			}}
			ctx := ctxcache.Init(context.Background())
			event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: "local"})
			err := svc.HandleEventLock(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil }))(ctx, event)
			if err != nil {
				require.True(t, itemHookControlOnly(err))
			}
			require.Equal(t, sealed, consumerControlSnapshot(t, f), "failed original-Run CAS cannot grant projection rights")
		})
	}
}

func TestHookConsumerActiveUnretriableLazyFailureMySQL(t *testing.T) {
	f, m, svc, event := consumerControlFixture(t, entity.ItemRunState_Queueing)
	_, _, _ = admitLazyTurn(t, f, m)
	config := cm.NewMockIConfiger(gomock.NewController(t))
	svc.configer = config
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{})
	require.NoError(t, svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
		return errors.New("dataset fetch failed before execution")
	}))(context.Background(), event))
	var row model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&row, m.ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Fail), row.Status)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(row.ResultState))
	require.Equal(t, int64(7), row.ItemVersionID)
	assertLazyTurnCount(t, f, 0)
}

func TestHookConsumerRequeueCASAndProjectionShareBoundaryMySQL(t *testing.T) {
	f, _, svc, event := consumerControlFixture(t, entity.ItemRunState_Processing)
	svc.centralGuard = &consumerReservationGuard{confirmed: false}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: "local"})
	entered, release := make(chan struct{}), make(chan struct{})
	var once, released sync.Once
	unblock := func() { released.Do(func() { close(release) }) }
	defer unblock()
	require.NoError(t, f.sql.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("consumer-requeue-after-cas", func(tx *gorm.DB) {
		fields, ok := tx.Statement.Dest.(map[string]any)
		if tx.Statement.Table == "expt_item_result_run_log" && ok && fields["status"] == int32(entity.ItemRunState_Queueing) {
			once.Do(func() { close(entered); <-release })
		}
	}))
	done := make(chan error, 1)
	go func() {
		done <- svc.HandleEventLock(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil }))(ctx, event)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("no requeue CAS")
	}
	_, err := svc.mutex.Unlock(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, event.EvalSetItemID))
	require.NoError(t, err)
	cancelled := make(chan error, 1)
	go func() {
		cancelled <- f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil)
	}()
	blocked := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		err := f.sql.Raw("SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON w.BLOCKING_ENGINE_LOCK_ID=l.ENGINE_LOCK_ID WHERE l.OBJECT_SCHEMA=DATABASE() AND l.OBJECT_NAME='experiment' AND l.LOCK_DATA=?", fmt.Sprint(f.expt)).Scan(&n).Error
		if err == nil && n > 0 {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	unblock()
	require.NoError(t, <-done)
	require.NoError(t, <-cancelled)
	require.NoError(t, f.sql.Callback().Update().Remove("consumer-requeue-after-cas"))
	require.True(t, blocked, "cancellation must wait for the same transaction that owns CAS and projection")
	var projection model.ExptItemResult
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&projection).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), projection.Status)
}

func TestHookConsumerActiveStateChangesAndReplayMySQL(t *testing.T) {
	for _, action := range []entity.HookConsumerAction{entity.HookConsumerYield, entity.HookConsumerStartReserved, entity.HookConsumerReservationAbsent} {
		t.Run(string(action), func(t *testing.T) {
			state := entity.ItemRunState_Processing
			if action == entity.HookConsumerStartReserved {
				state = entity.ItemRunState_Queueing
			}
			f, m, svc, event := consumerControlFixture(t, state)
			if action == entity.HookConsumerStartReserved {
				_, _, _ = admitLazyTurn(t, f, m)
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("quota_reservation_state", int32(entity.QuotaReservationStateReserved)).Error)
			}
			event.RetryTimes = 0
			config := cm.NewMockIConfiger(gomock.NewController(t))
			svc.configer = config
			config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{RetryTimes: 1}).AnyTimes()
			invoke := func() error {
				if action == entity.HookConsumerYield {
					event.Ext = map[string]string{entity.RetryYieldExtKey: "true"}
					return svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("retry me") }))(context.Background(), event)
				}
				svc.centralGuard = &consumerReservationGuard{confirmed: action == entity.HookConsumerStartReserved}
				ctx := ctxcache.Init(context.Background())
				event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: "local"})
				return svc.HandleEventLock(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil }))(ctx, event)
			}
			require.NoError(t, invoke())
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
			var projection model.ExptItemResult
			require.NoError(t, f.sql.First(&projection, m.ItemResultID).Error)
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			want := entity.ItemRunState_Queueing
			if action == entity.HookConsumerStartReserved {
				want = entity.ItemRunState_Processing
				require.Equal(t, int32(1), stats.ProcessingCnt)
				require.Zero(t, stats.PendingCnt)
			} else {
				require.Equal(t, int32(1), stats.PendingCnt)
				require.Zero(t, stats.ProcessingCnt)
			}
			require.Equal(t, int32(want), item.Status)
			require.Equal(t, int32(want), projection.Status)
			require.Equal(t, int64(7), item.ItemVersionID)
			if action == entity.HookConsumerYield {
				require.Equal(t, int32(1), item.RetryTimes)
			}
			before := consumerControlSnapshot(t, f)
			require.NoError(t, invoke())
			require.Equal(t, before, consumerControlSnapshot(t, f), "duplicate delivery cannot double-adjust counts")
		})
	}
}

func consumerControlArchive(t *testing.T, f *finalizationManagerFixture, itemID int64) {
	t.Helper()
	service, err := (&ExptResultServiceImpl{}).WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	source, err := f.deps.Repository.ReadFinalizationSource(context.Background(), f.key)
	require.NoError(t, err)
	_, err = service.RecordItemRunLogs(context.Background(), f.expt, f.key.RunID, itemID, f.space, source.Experiment)
	require.NoError(t, err)
}

func TestHookConsumerActiveFailurePreservesCompletedTurnsMySQL(t *testing.T) {
	f, m, svc, event := consumerControlFixture(t, entity.ItemRunState_Processing)
	m = lazyTurnManifest(t, f, m, 2)
	m.Turns[1].RunLogID = finalizationTestIDs.Add(1)
	raw, err := json.Marshal(m)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", raw).Error)
	finished := model.ExptTurnResultRunLog{ID: m.Turns[1].RunLogID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: 7, TurnID: m.Turns[1].TurnID, Status: int32(entity.TurnRunState_Success), LogID: "preserved-success"}
	require.NoError(t, f.sql.Create(&finished).Error)
	var before model.ExptTurnResultRunLog
	require.NoError(t, f.sql.First(&before, finished.ID).Error)
	config := cm.NewMockIConfiger(gomock.NewController(t))
	svc.configer = config
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{})
	require.NoError(t, svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
		return errors.New("terminal execution failure")
	}))(context.Background(), event))
	var after model.ExptTurnResultRunLog
	require.NoError(t, f.sql.First(&after, finished.ID).Error)
	require.Equal(t, before, after)
	consumerControlArchive(t, f, m.Frozen.ItemID)
	require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
	require.Equal(t, entity.ExptStatus_Failed, finalizationRead(t, f).State.Status)
}

func TestHookConsumerLazyFailureArchiveAndCancelMySQL(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		for _, cancelRun := range []bool{false, true} {
			t.Run(fmt.Sprintf("admitted=%t/cancel=%t", admitted, cancelRun), func(t *testing.T) {
				f, m, svc, event := consumerControlFixture(t, entity.ItemRunState_Queueing)
				if admitted {
					_, _, _ = admitLazyTurn(t, f, m)
				}
				config := cm.NewMockIConfiger(gomock.NewController(t))
				svc.configer = config
				config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{})
				require.NoError(t, svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error {
					return errors.New("generic pre-execution platform error")
				}))(context.Background(), event))
				var ledger model.ExptLifecycleRunItem
				require.NoError(t, f.sql.First(&ledger, m.Frozen.ID).Error)
				require.Equal(t, admitted, ledger.AdmittedAt != nil)
				var manifest map[string]any
				require.NoError(t, json.Unmarshal(gptr.Indirect(ledger.ExecutionManifest), &manifest))
				require.Equal(t, true, manifest["no_execution_failure"])
				assertLazyTurnCount(t, f, 0)
				if cancelRun {
					require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
				} else {
					consumerControlArchive(t, f, m.Frozen.ItemID)
					require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
				}
				var turn model.ExptTurnResult
				require.NoError(t, f.sql.First(&turn, m.Turns[0].ResultID).Error)
				require.Equal(t, int32(entity.TurnRunState_Fail), turn.Status)
				require.Nil(t, turn.WeightedScore)
				require.Zero(t, turn.TargetResultID)
				var run model.ExptRunLog
				require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
				require.Equal(t, int32(1), run.FailCnt)
				require.Zero(t, run.TerminatedCnt)
				require.True(t, finalizationRead(t, f).State.After.Activated)
				assertLazyTurnCount(t, f, 0)
			})
		}
	}
}

func TestHookConsumerProjectionFailureRollsBackMySQL(t *testing.T) {
	f, m, svc, _ := consumerControlFixture(t, entity.ItemRunState_Processing)
	before := consumerControlSnapshot(t, f)
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("consumer-stats-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_stats" {
			tx.AddError(errors.New("stats unavailable"))
		}
	}))
	writer := svc.hookAdmission.progress.(repo.IHookConsumerControlRepo)
	_, err := writer.ApplyHookConsumerControl(context.Background(), entity.HookConsumerControlInput{Key: f.key, ItemID: m.Frozen.ItemID, Action: entity.HookConsumerYield, RetryTimes: 1})
	require.NoError(t, f.sql.Callback().Update().Remove("consumer-stats-failure"))
	require.Error(t, err)
	require.Equal(t, before, consumerControlSnapshot(t, f))
	result, err := writer.ApplyHookConsumerControl(context.Background(), entity.HookConsumerControlInput{Key: f.key, ItemID: m.Frozen.ItemID, Action: entity.HookConsumerYield, RetryTimes: 1})
	require.NoError(t, err)
	require.True(t, result.Changed)
}

type consumerControlDecisionBarrier struct {
	repo.IHookTurnProgressRepo
	writer repo.IHookConsumerControlRepo
	before func()
}

func (b consumerControlDecisionBarrier) ApplyHookConsumerControl(ctx context.Context, in entity.HookConsumerControlInput) (entity.HookConsumerControlResult, error) {
	b.before()
	return b.writer.ApplyHookConsumerControl(ctx, in)
}

func TestHookConsumerCancellationBetweenSourceAndWriteMySQL(t *testing.T) {
	for _, action := range []entity.HookConsumerAction{entity.HookConsumerYield, entity.HookConsumerFail, entity.HookConsumerStartReserved, entity.HookConsumerReservationAbsent} {
		for _, newer := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/newLatest=%t", action, newer), func(t *testing.T) {
				f, _, svc, event := consumerControlFixture(t, entity.ItemRunState_Processing)
				event.RetryTimes = 0
				var sealed string
				progress := svc.hookAdmission.progress
				svc.hookAdmission.progress = consumerControlDecisionBarrier{IHookTurnProgressRepo: progress, writer: progress.(repo.IHookConsumerControlRepo), before: func() { consumerControlCancel(t, f, newer); sealed = consumerControlSnapshot(t, f) }}
				result, err := svc.applyHookConsumerControl(context.Background(), event, action, errors.New("old execution error"))
				require.NoError(t, err)
				require.True(t, result.Handled)
				require.False(t, result.Changed)
				require.False(t, result.Proceed)
				require.Equal(t, sealed, consumerControlSnapshot(t, f))
			})
		}
	}
}

func TestHookConsumerFailedStartCASHasNoProjectionRightsMySQL(t *testing.T) {
	f, m, svc, event := consumerControlFixture(t, entity.ItemRunState_Queueing)
	_, _, _ = admitLazyTurn(t, f, m)
	svc.centralGuard = &consumerReservationGuard{confirmed: true}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: "local"})
	before := consumerControlSnapshot(t, f)
	next := false
	err := svc.HandleEventLock(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { next = true; return nil }))(ctx, event)
	require.True(t, itemHookControlOnly(err))
	require.False(t, next)
	require.Equal(t, before, consumerControlSnapshot(t, f))
}

func TestHookConsumerIDErrorAfterCancellationStaysControlMySQL(t *testing.T) {
	f, m, svc, event := consumerControlFixture(t, entity.ItemRunState_Queueing)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	var sealed string
	pre.idgen = consumerAllocationError{before: func() {
		_, err := svc.mutex.Unlock(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, event.EvalSetItemID))
		require.NoError(t, err)
		consumerControlCancel(t, f, false)
		sealed = consumerControlSnapshot(t, f)
	}}
	config := cm.NewMockIConfiger(gomock.NewController(t))
	svc.configer = config
	generic := 0
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).DoAndReturn(func(context.Context, int64, error) *entity.RetryConf { generic++; return &entity.RetryConf{} }).AnyTimes()
	require.NoError(t, svc.HandleEventErr(svc.HandleEventLock(func(callCtx context.Context, _ *entity.ExptItemEvalEvent) error { return pre.PreEval(callCtx, eiec) }))(ctx, event))
	require.Zero(t, generic)
	require.Equal(t, sealed, consumerControlSnapshot(t, f))
	assertLazyTurnCount(t, f, 0)
}

// The external index boundary reads committed SQL, as UpsertExptTurnResultFilter does.
type consumerFilterIndex struct {
	ExptResultService
	sql       *gorm.DB
	key       entity.HookRunKey
	itemID    int64
	indexed   int32
	calls     int
	fail      bool
	lockError error
}

func (i *consumerFilterIndex) UpsertExptTurnResultFilter(ctx context.Context, space, expt int64, items []int64) error {
	i.calls++
	if space != i.key.WorkspaceID || expt != i.key.ExperimentID || len(items) != 1 || items[0] != i.itemID {
		return errors.New("wrong index refresh scope")
	}
	var item model.ExptItemResult
	i.lockError = i.sql.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var id int64
		if err := tx.Raw("SELECT id FROM experiment WHERE id=? AND space_id=? FOR UPDATE NOWAIT", expt, space).Scan(&id).Error; err != nil {
			return err
		}
		if err := tx.Raw("SELECT expt_run_id FROM expt_lifecycle_run WHERE space_id=? AND expt_id=? AND expt_run_id=? FOR UPDATE NOWAIT", space, expt, i.key.RunID).Scan(&id).Error; err != nil {
			return err
		}
		if err := tx.Raw("SELECT id FROM expt_item_result_run_log WHERE space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? FOR UPDATE NOWAIT", space, expt, i.key.RunID, i.itemID).Scan(&id).Error; err != nil {
			return err
		}
		return tx.Where("space_id=? AND expt_id=? AND item_id=?", space, expt, i.itemID).First(&item).Error
	})
	if i.lockError != nil {
		return i.lockError
	}
	if i.fail {
		return errors.New("index temporarily unavailable")
	}
	i.indexed = item.Status
	return nil
}

func consumerFilterFixture(t *testing.T, start bool) (*finalizationManagerFixture, entity.HookExecutionManifest, *ExptItemEventEvalServiceImpl, *entity.ExptItemEvalEvent, *consumerFilterIndex) {
	t.Helper()
	state := entity.ItemRunState_Processing
	if start {
		state = entity.ItemRunState_Queueing
	}
	f, m, svc, event := consumerControlFixture(t, state)
	if start {
		_, _, _ = admitLazyTurn(t, f, m)
		require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("quota_reservation_state", int32(entity.QuotaReservationStateReserved)).Error)
	}
	svc.hookAdmission.gate = exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil })
	index := &consumerFilterIndex{sql: f.sql, key: f.key, itemID: m.Frozen.ItemID, indexed: int32(state)}
	svc.resultSvc = index
	return f, m, svc, event, index
}

func invokeConsumerFilter(svc *ExptItemEventEvalServiceImpl, event *entity.ExptItemEvalEvent, start bool) (bool, error) {
	svc.centralGuard = &consumerReservationGuard{confirmed: start}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: "local"})
	next := false
	err := svc.HandleEventLock(svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { next = true; return nil }))(ctx, event)
	return next, err
}

func TestHookConsumerFilterRefreshAfterCommitMySQL(t *testing.T) {
	for _, start := range []bool{true, false} {
		for _, failure := range []bool{false, true} {
			t.Run(fmt.Sprintf("start=%t/indexError=%t", start, failure), func(t *testing.T) {
				f, m, svc, event, index := consumerFilterFixture(t, start)
				index.fail = failure
				old := index.indexed
				next, err := invokeConsumerFilter(svc, event, start)
				require.NoError(t, err, "index failure is best-effort and cannot fail execution")
				require.Equal(t, start, next)
				var item model.ExptItemResult
				require.NoError(t, f.sql.First(&item, m.ItemResultID).Error)
				want := entity.ItemRunState_Queueing
				if start {
					want = entity.ItemRunState_Processing
				}
				require.Equal(t, int32(want), item.Status)
				require.Equal(t, 1, index.calls, "effective reservation state change must refresh filtering")
				require.NoError(t, index.lockError, "external refresh must not run under parent/item DB locks")
				if failure {
					require.Equal(t, old, index.indexed)
				} else {
					require.Equal(t, int32(want), index.indexed, "filtering must see the newly committed status")
				}
				before := consumerControlSnapshot(t, f)
				_, err = invokeConsumerFilter(svc, event, start)
				require.NoError(t, err)
				require.Equal(t, 1, index.calls, "replay is not a new effective state change")
				require.Equal(t, before, consumerControlSnapshot(t, f))
			})
		}
	}
}

func TestHookConsumerFilterNoopMetadataAndRollbackMySQL(t *testing.T) {
	for _, kind := range []string{"failed_cas", "metadata_reset", "rollback", "projection_already_current"} {
		t.Run(kind, func(t *testing.T) {
			f, m, svc, event, index := consumerFilterFixture(t, true)
			start := true
			switch kind {
			case "failed_cas":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("quota_reservation_state", int32(entity.QuotaReservationStateNone)).Error)
			case "metadata_reset":
				start = false
			case "rollback":
				require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("consumer-filter-rollback", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_stats" {
						tx.AddError(errors.New("rollback before refresh"))
					}
				}))
			case "projection_already_current":
				require.NoError(t, f.sql.Model(&model.ExptItemResult{}).Where("id=?", m.ItemResultID).UpdateColumn("status", int32(entity.ItemRunState_Processing)).Error)
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("status", int32(entity.TurnRunState_Processing)).Error)
			}
			_, err := invokeConsumerFilter(svc, event, start)
			if kind == "failed_cas" || kind == "rollback" {
				require.True(t, itemHookControlOnly(err))
			} else {
				require.NoError(t, err)
			}
			if kind == "rollback" {
				require.NoError(t, f.sql.Callback().Update().Remove("consumer-filter-rollback"))
			}
			require.Zero(t, index.calls)
		})
	}
}

type consumerFilterAfterCommit struct {
	repo.IHookTurnProgressRepo
	writer repo.IHookConsumerControlRepo
	after  func()
}

func (p consumerFilterAfterCommit) ApplyHookConsumerControl(ctx context.Context, in entity.HookConsumerControlInput) (entity.HookConsumerControlResult, error) {
	out, err := p.writer.ApplyHookConsumerControl(ctx, in)
	if err == nil && out.Changed {
		p.after()
	}
	return out, err
}

func TestHookConsumerFilterCancellationBeforeRefreshMySQL(t *testing.T) {
	for _, afterCommit := range []bool{false, true} {
		for _, newer := range []bool{false, true} {
			t.Run(fmt.Sprintf("afterCommit=%t/newLatest=%t", afterCommit, newer), func(t *testing.T) {
				f, _, svc, event, index := consumerFilterFixture(t, true)
				progress := svc.hookAdmission.progress
				cancel := func() {
					_, err := svc.mutex.Unlock(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, event.EvalSetItemID))
					require.NoError(t, err)
					consumerControlCancel(t, f, newer)
				}
				if afterCommit {
					svc.hookAdmission.progress = consumerFilterAfterCommit{IHookTurnProgressRepo: progress, writer: progress.(repo.IHookConsumerControlRepo), after: cancel}
				} else {
					svc.hookAdmission.progress = consumerControlDecisionBarrier{IHookTurnProgressRepo: progress, writer: progress.(repo.IHookConsumerControlRepo), before: cancel}
				}
				_, err := invokeConsumerFilter(svc, event, true)
				if err != nil {
					require.True(t, itemHookControlOnly(err))
				}
				require.Zero(t, index.calls, "canceled or superseded original Run cannot request an index refresh")
			})
		}
	}
}
