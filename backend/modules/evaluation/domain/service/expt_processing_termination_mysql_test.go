// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Keep the real item lease over both turns and block inside the synchronous
// target RPC. Cancellation must commit while that RPC is still in flight.
func TestProcessingTerminationDuringSynchronousTurns(t *testing.T) {
	f := newLateWriteFixture(t, "")
	for _, table := range []any{&model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}} {
		require.NoError(t, f.conn.Model(table).Where("space_id=?", f.space).Update("target_result_id", 0).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"item_version_id": 0, "evaluator_result_ids": []byte(`null`)}).Error)
	require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": "f2-scope"}).Error)
	factory, server := finishRealLockerFactory(t)
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any()).AnyTimes()
	benefits := benefitmocks.NewMockIBenefitService(ctrl)
	benefits.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	target.EXPECT().ExecuteTarget(gomock.Any(), f.space, f.base+42, f.base+43, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ int64, _ int64, tc *entity.ExecuteTargetCtx, _ *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
		calls.Add(1)
		if tc.TurnID == 0 {
			close(entered)
			<-release
		}
		return &entity.EvalTargetRecord{ID: f.base + 71, SpaceID: f.space, ExperimentRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: tc.TurnID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{}}, nil
	}).AnyTimes()
	guard := componentmocks.NewMockICentralReservationGuard(ctrl)
	guard.EXPECT().Release(gomock.Any(), "f2-scope", f.run, f.item, "", "current run item terminated").Return(nil).Times(1)
	manager := &ExptMangerImpl{exptRepo: f.experimentRepo(), itemResultRepo: f.itemRepo(), runFinishRepo: experimentrepo.NewExptRunFinishRepo(f.provider), manualRetryLocker: factory, centralGuard: guard}
	executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), TurnResultRepo: f.turnRepo(), Configer: config, Metric: metric, evalTargetService: target, benefitService: benefits}
	event := f.event()
	event.RequestID = ""
	event.ExptRunMode = entity.EvaluationModeSubmit
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, TargetVersionID: f.base + 43, Target: &entity.EvalTarget{ID: f.base + 42, EvalTargetType: entity.EvalTargetTypeCustomRPCServer, EvalTargetVersion: &entity.EvalTargetVersion{ID: f.base + 43}}, EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: f.base + 43}}}}
	// The executor uses the exact source frozen by the consumer admission gate.
	consumer := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), itemLocker: factory}
	go func() {
		done <- consumer.HandleEventLock(consumer.HandleCurrentSource(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error {
			source := executionSource(ctx).Source
			eiec := &entity.ExptItemEvalCtx{Event: event, Expt: expt, EvalSetItem: &entity.EvaluationSetItem{ItemID: f.item, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}, Turns: []*entity.Turn{{ID: 0}, {ID: f.base + 22}}}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: source.Turns}}
			return executor.Eval(ctx, eiec)
		}))(context.Background(), event)
	}()
	select {
	case <-entered:
	case err := <-done:
		t.Fatalf("executor never entered target RPC: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("target RPC barrier not entered")
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
	owner, err := server.Get(key)
	require.NoError(t, err)
	cancelErr := manager.TerminateItems(context.Background(), f.expt, f.run, f.space, []int64{f.item}, &entity.Session{UserID: "cancel"})
	if cancelErr != nil {
		unblock()
		<-done
		require.NoError(t, cancelErr)
		return
	}
	stillOwner, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, owner, stillOwner, "termination must preserve the worker lease")
	require.Equal(t, entity.ItemRunState_Terminal, f.source().Current.Status)
	require.Equal(t, entity.ExptItemResultStateLogged, f.source().Current.ResultState)
	before := f.snapshot()
	unblock()
	require.ErrorIs(t, <-done, errExptNonCurrentSource, "the late first-turn result must be fenced")
	require.Equal(t, int32(1), calls.Load(), "second turn must never invoke target")
	require.Equal(t, before, f.snapshot(), "late Store/Complete cannot revive the cancelled item")
	require.False(t, server.Exists(key), "worker releases its own lease after RPC completion")
	// The same late source also cannot write Success directly.
	stale := &entity.ExptCurrentItemRunSource{Current: f.source().Current}
	stale.Current.Status = entity.ItemRunState_Processing
	stale.Turns = f.source().Turns
	stale.CanonicalTurns = f.source().CanonicalTurns
	applied, err := f.port.CompleteCurrentItemRun(context.Background(), stale, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success})
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, before, f.snapshot())
	// Run the scheduler's real publication chain after the late RPC returns.
	// Terminal publication must never send item-complete(success).
	result := &ExptResultServiceImpl{resultPublishRepo: experimentrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: factory, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil)}
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 81}}
	dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
	dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item}}, nil)
	mode := entitymocks.NewMockExptSchedulerMode(ctrl)
	mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	notices := &stubItemCompletePublisher{}
	scheduler := &ExptSchedulerImpl{ResultSvc: &publicationSchedulerResult{result}, ExptItemResultRepo: f.itemRepo(), itemCompletePublisher: notices, evaluationSetItemService: dataset}
	require.NoError(t, scheduler.recordEvalItemRunLogs(context.Background(), &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}, []*entity.ExptEvalItem{{ItemID: f.item, State: entity.ItemRunState_Terminal}}, mode, expt))
	require.Empty(t, notices.events)
	require.Equal(t, entity.ItemRunState_Terminal, f.source().Current.Status)
	require.Equal(t, entity.ExptItemResultStateResulted, f.source().Current.ResultState)
}

// A lock failure remains an error; only false,nil on a Processing source is
// eligible for the DAO's stricter state check. No unsuccessful path releases quota.
func TestProcessingTerminationLockBoundaries(t *testing.T) {
	for _, kind := range []string{"processing", "queueing", "success", "fail", "terminal", "lock_error", "lost_lease"} {
		t.Run(kind, func(t *testing.T) {
			d := newTermUnitDeps(t)
			m := d.manager()
			status := entity.ItemRunState_Processing
			switch kind {
			case "queueing":
				status = entity.ItemRunState_Queueing
			case "success":
				status = entity.ItemRunState_Success
			case "fail":
				status = entity.ItemRunState_Fail
			case "terminal":
				status = entity.ItemRunState_Terminal
			}
			lockCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var lockErr error
			if kind == "lock_error" {
				lockErr = fmt.Errorf("redis unavailable")
			}
			if kind == "lost_lease" {
				cancel()
			}
			d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(kind == "lost_lease", lockCtx, func() {}, lockErr)
			if kind == "processing" {
				d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
			}
			got, err := m.commitTerminationItems(context.Background(), d.termination, &entity.ExptRunTerminationRequest{Items: []*entity.ExptCurrentItemRunSource{termUnitSource(9, status)}})
			if kind == "processing" {
				require.NoError(t, err)
				require.True(t, got.Applied)
			} else {
				require.True(t, errno.NeedMQRetry(err))
				require.Nil(t, got)
			}
		})
	}
}

func TestProcessingTerminationFrozenSourceAndAtomicBatch(t *testing.T) {
	for _, kind := range []string{"old_zero", "missing_turn_log", "mixed", "canonical_success", "log_success", "canonical_queueing", "log_queueing", "new_request", "target_ref_changed", "mixed_state_changed", "lost_first_lease"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "")
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("item_version_id", 0).Error)
			if kind == "missing_turn_log" {
				require.NoError(t, f.conn.Unscoped().Where("id=?", f.base+501).Delete(&model.ExptTurnResultRunLog{}).Error)
			}
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{"scheduler_mode": "enforce", "scheduler_scope": "f2-scope"}).Error)
			ctrl := gomock.NewController(t)
			locker := lockmocks.NewMockILocker(ctrl)
			guard := componentmocks.NewMockICentralReservationGuard(ctrl)
			ids := []int64{f.item}
			if kind == "mixed" || kind == "mixed_state_changed" || kind == "lost_first_lease" {
				ids = append(ids, f.item+1)
			}
			success := kind == "old_zero" || kind == "missing_turn_log" || kind == "mixed"
			releases := 0
			if success {
				releases = 1
			}
			for _, id := range ids {
				guard.EXPECT().Release(gomock.Any(), "f2-scope", f.run, id, "", "current run item terminated").Return(nil).Times(releases)
			}
			targets := svcmocks.NewMockIEvalTargetService(ctrl)
			if success {
				targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), f.space, []int64{f.base + 41}, gomock.Any(), gomock.Any(), false).Return(nil).Times(1)
			}
			gen := idmocks.NewMockIIDGenerator(ctrl)
			if kind == "missing_turn_log" {
				gen.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{f.base + 800}, nil)
			}
			var before map[string]string
			firstCtx, loseFirst := context.WithCancel(context.Background())
			defer loseFirst()
			locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, key string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
				if key == fmt.Sprintf("expt_run_exec_lock:%d:%d", f.expt, f.run) {
					c, cancel := context.WithCancel(ctx)
					return true, c, cancel, nil
				}
				if len(ids) == 2 && key == fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item) {
					return true, firstCtx, func() {}, nil
				}
				switch kind {
				case "canonical_success", "canonical_queueing":
					status := entity.ItemRunState_Success
					if kind == "canonical_queueing" {
						status = entity.ItemRunState_Queueing
					}
					require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(status)).Error)
				case "log_success", "log_queueing", "mixed_state_changed":
					status := entity.ItemRunState_Success
					if kind == "log_queueing" {
						status = entity.ItemRunState_Queueing
					}
					item := f.item
					if kind == "mixed_state_changed" {
						item++
					}
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, item).Updates(map[string]any{"status": int32(status), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
				case "new_request":
					f.token("rq1")
					require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("item_version_id", f.version).Error)
				case "target_ref_changed":
					require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", f.base+777).Error)
				case "lost_first_lease":
					loseFirst()
				}
				before = f.snapshot()
				return false, ctx, func() { t.Error("busy worker lease must not be cancelled") }, nil
			}).AnyTimes()
			manager := &ExptMangerImpl{exptRepo: f.experimentRepo(), itemResultRepo: f.itemRepo(), runFinishRepo: experimentrepo.NewExptRunFinishRepo(f.provider), manualRetryLocker: func() lock.ILocker { return locker }, centralGuard: guard, evalTargetService: targets, idgenerator: gen}
			err := manager.TerminateItems(context.Background(), f.expt, f.run, f.space, ids, &entity.Session{UserID: "cancel"})
			if success {
				require.NoError(t, err)
				for _, id := range ids {
					s, e := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, id)
					require.NoError(t, e)
					require.Equal(t, entity.ItemRunState_Terminal, s.Current.Status)
					require.Equal(t, entity.ExptItemResultStateLogged, s.Current.ResultState)
				}
				var logs []model.ExptTurnResultRunLog
				require.NoError(t, f.conn.Where("space_id=?", f.space).Find(&logs).Error)
				require.Len(t, logs, 2)
				for _, log := range logs {
					require.Equal(t, int32(entity.TurnRunState_Terminal), log.Status)
				}
			} else {
				if kind == "new_request" {
					require.NoError(t, err)
				} else {
					require.True(t, errno.NeedMQRetry(err), "state/reference/lease loss must retry")
				}
				require.Equal(t, before, f.snapshot(), "rejected mixed cancellation has no SQL or resource effects")
			}
		})
	}
}

func TestProcessingTerminationPublicationWins(t *testing.T) {
	f, result, records, expt := publicationFixture(t)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return([]*entity.EvaluatorRecord{publicationRecord(f, gptr.Of(0.0))}, nil).AnyTimes()
	factory, server := finishRealLockerFactory(t)
	result.resultPublishLocker = factory
	source, err := result.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	publication, err := result.PrepareItemResultPublication(context.Background(), source, expt)
	require.NoError(t, err)
	sent, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	go func() {
		ok, e := result.PublishPreparedItemResult(context.Background(), publication, func(_ context.Context, s *entity.ExptItemResultPublishSource) error {
			if s.ItemLog.Status != int32(entity.ItemRunState_Success) {
				return fmt.Errorf("not a success notification")
			}
			close(sent)
			<-release
			return nil
		})
		if e == nil && !ok {
			e = fmt.Errorf("publication lost after sending success")
		}
		done <- e
	}()
	select {
	case <-sent:
	case err := <-done:
		t.Fatalf("publication did not send: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("publication barrier not entered")
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, f.item)
	owner, err := server.Get(key)
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	guard := componentmocks.NewMockICentralReservationGuard(ctrl)
	guard.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	manager := &ExptMangerImpl{exptRepo: f.experimentRepo(), itemResultRepo: f.itemRepo(), runFinishRepo: experimentrepo.NewExptRunFinishRepo(f.provider), manualRetryLocker: factory, centralGuard: guard}
	before := f.snapshot()
	cancelErr := manager.TerminateItems(context.Background(), f.expt, f.run, f.space, []int64{f.item}, &entity.Session{})
	after := f.snapshot()
	stillOwner, e := server.Get(key)
	unblock()
	require.NoError(t, <-done)
	require.True(t, errno.NeedMQRetry(cancelErr))
	require.Equal(t, before, after)
	require.NoError(t, e)
	require.Equal(t, owner, stillOwner)
	require.Equal(t, entity.ItemRunState_Success, f.source().Current.Status)
	require.Equal(t, entity.ExptItemResultStateResulted, f.source().Current.ResultState)
}
