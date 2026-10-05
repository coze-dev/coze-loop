// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

type executionHookGate struct {
	closed, waiting atomic.Bool
	reads           atomic.Int32
	onClosed        func()
}

func (g *executionHookGate) CanDispatch(_ context.Context, key entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	g.reads.Add(1)
	if key != (entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}) {
		return entity.HookAdmissionDecision{}, errors.New("wrong run binding")
	}
	if g.waiting.Load() {
		return entity.HookAdmissionDecision{}, errors.New("private dependency failure")
	}
	if g.closed.Load() {
		if g.onClosed != nil {
			g.onClosed()
		}
		return entity.HookAdmissionDecision{Gate: entity.HookGateClosed}, nil
	}
	return entity.HookAdmissionDecision{Gate: entity.HookGateReady}, nil
}

type executionHookFixture struct {
	ctx        context.Context
	gate       *executionHookGate
	etec       *entity.ExptTurnEvalCtx
	turnEval   *DefaultExptTurnEvaluationImpl
	targets    *svcmocks.MockIEvalTargetService
	evaluators *svcmocks.MockEvaluatorService
	async      *repomocks.MockIEvalAsyncRepo
	metric     *metricmocks.MockExptMetric
	progress   *executionHookProgress
}

type executionHookProgress struct {
	read    func(context.Context, entity.HookTurnProgressKey) (*entity.ExptTurnResultRunLog, error)
	write   func(context.Context, entity.HookTurnProgressInput) (*entity.ExptTurnResultRunLog, error)
	persist func(context.Context, []*entity.ExptTurnResultRunLog) error
}

func (p *executionHookProgress) ReadTurnProgress(ctx context.Context, k entity.HookTurnProgressKey) (*entity.ExptTurnResultRunLog, error) {
	if p.read == nil {
		return nil, entity.ErrHookStoreMissing
	}
	return p.read(ctx, k)
}
func (p *executionHookProgress) WriteTurnProgress(ctx context.Context, in entity.HookTurnProgressInput) (*entity.ExptTurnResultRunLog, error) {
	if p.write != nil {
		return p.write(ctx, in)
	}
	if p.persist != nil {
		if err := p.persist(ctx, []*entity.ExptTurnResultRunLog{in.Progress}); err != nil {
			return nil, err
		}
		return in.Progress, nil
	}
	return nil, entity.ErrHookStoreMissing
}

func newExecutionHookFixture(t *testing.T, managed, alias, async bool) *executionHookFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	f := &executionHookFixture{gate: &executionHookGate{}, ctx: ctxcache.Init(context.Background()), progress: &executionHookProgress{}}
	if managed {
		admission := newItemHookFixture(t, true, false)
		admission.ports.held = true
		svc := admission.svc.(*ExptItemEventEvalServiceImpl)
		svc.hookAdmission.gate = f.gate
		svc.hookAdmission.progress = f.progress
		err := svc.handleHookItemAdmission(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error { f.ctx = ctx; return nil })(context.WithValue(f.ctx, itemHookManagedKey{}, true), admission.event)
		require.NoError(t, err)
		require.Len(t, admission.ports.admitted, 1)
	}
	f.etec = newRunConfEtec(nil, nil)
	f.etec.Event = &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, RetryTimes: 2, MaxRetryTimes: 3, Session: &entity.Session{UserID: "u"}}
	f.etec.Expt.ID, f.etec.Expt.SpaceID, f.etec.Expt.TargetVersionID = 2, 1, 1
	f.etec.Expt.Target.EvalTargetType = entity.EvalTargetTypeCustomRPCServer
	f.etec.Expt.EvalConf.ConnectorConf.EvaluatorsConf = buildRetryFailureSingleSetExpt(1, 401, 402).EvalConf.ConnectorConf.EvaluatorsConf
	f.etec.Expt.Evaluators = buildRetryFailureSingleSetExpt(1, 401, 402).Evaluators
	if async {
		for _, ev := range f.etec.Expt.Evaluators {
			id := ev.GetEvaluatorVersionID()
			ev.EvaluatorType = entity.EvaluatorTypeCustomRPC
			ev.CustomRPCEvaluatorVersion = &entity.CustomRPCEvaluatorVersion{ID: id, IsAsync: true}
		}
	}
	if alias {
		f.etec.Expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		f.etec.ItemConfig = &entity.ExptItemConfig{EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: "a"}, {EvaluatorVersionID: 402, Alias: "b"}}}
	}
	f.etec.EvalSetItem.ItemID = 4
	f.etec.EvalSetItem.BaseInfo = &entity.BaseInfo{}
	f.etec.EvalSetItem.Turns = []*entity.Turn{f.etec.Turn}
	f.etec.ExptTurnRunResult = &entity.ExptTurnRunResult{}
	f.etec.ExistItemEvalResult = &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{1: {ID: 9, SpaceID: 1, ExptID: 2, ExptRunID: 3, ItemID: 4, TurnID: 1}}}
	f.progress.read = func(_ context.Context, k entity.HookTurnProgressKey) (*entity.ExptTurnResultRunLog, error) {
		row := f.etec.GetExistTurnResultRunLog(k.TurnID)
		if entity.HookTurnProgressIdentity(row) != k {
			return nil, entity.ErrHookStoreMissing
		}
		copy := *row
		return &copy, nil
	}
	f.metric = metricmocks.NewMockExptMetric(ctrl)
	f.metric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any()).AnyTimes()
	f.metric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	f.metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any()).AnyTimes()
	f.metric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any()).AnyTimes()
	f.targets = svcmocks.NewMockIEvalTargetService(ctrl)
	f.evaluators = svcmocks.NewMockEvaluatorService(ctrl)
	f.evaluators.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).AnyTimes()
	f.async = repomocks.NewMockIEvalAsyncRepo(ctrl)
	benefit := benefitmocks.NewMockIBenefitService(ctrl)
	benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	f.turnEval = &DefaultExptTurnEvaluationImpl{metric: f.metric, evalTargetService: f.targets, evaluatorService: f.evaluators, benefitService: benefit, evalAsyncRepo: f.async}
	return f
}

func executionTarget() *entity.EvalTargetRecord {
	return &entity.EvalTargetRecord{ID: 100, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}}
}

func TestItemHookExecutionBeforeTarget(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "async"}[async], func(t *testing.T) {
			f := newExecutionHookFixture(t, true, false, false)
			f.gate.closed.Store(true)
			var calls int
			if async {
				f.etec.Expt.Target.EvalTargetVersion.CustomRPCServer = &entity.CustomRPCServer{IsAsync: gptr.Of(true)}
				f.targets.EXPECT().AsyncExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
					calls++
					return executionTarget(), "callee", nil
				}).AnyTimes()
				f.async.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			} else {
				f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					calls++
					return executionTarget(), nil
				}).AnyTimes()
			}
			_, err := f.turnEval.CallTarget(f.ctx, f.etec)
			var control itemHookControlError
			assert.ErrorAs(t, err, &control)
			assert.Zero(t, calls)
		})
	}
}

func TestItemHookExecutionAfterTarget(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
		f.gate.closed.Store(true)
		return executionTarget(), nil
	})
	var calls atomic.Int32
	f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		calls.Add(1)
		return &entity.EvaluatorRecord{ID: req.EvaluatorVersionID, EvaluatorVersionID: req.EvaluatorVersionID}, nil
	}).AnyTimes()
	result := f.turnEval.Eval(f.ctx, f.etec)
	var control itemHookControlError
	assert.ErrorAs(t, result.EvalErr, &control)
	require.NotNil(t, result.TargetResult)
	assert.Equal(t, int64(100), result.TargetResult.ID)
	assert.Zero(t, calls.Load())
}

func TestItemHookExecutionQueuedEvaluators(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		alias, async, managed bool
	}{{"sync", false, false, true}, {"async", false, true, true}, {"alias-sync", true, false, true}, {"alias-async", true, true, true}, {"legacy", false, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			f := newExecutionHookFixture(t, tc.managed, tc.alias, tc.async)
			var calls atomic.Int32
			run := func(id int64, alias string) *entity.EvaluatorRecord {
				calls.Add(1)
				f.gate.closed.Store(true)
				status := entity.EvaluatorRunStatusSuccess
				if tc.async {
					status = entity.EvaluatorRunStatusAsyncInvoking
				}
				return &entity.EvaluatorRecord{ID: id + 1000, EvaluatorVersionID: id, Alias: alias, Status: status}
			}
			if tc.async {
				f.evaluators.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
					return run(req.EvaluatorVersionID, req.Alias), nil
				}).AnyTimes()
			} else {
				f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
					return run(req.EvaluatorVersionID, req.Alias), nil
				}).AnyTimes()
			}
			records, err := f.turnEval.CallEvaluators(f.ctx, f.etec, executionTarget())
			if tc.managed {
				var control itemHookControlError
				assert.ErrorAs(t, err, &control)
				assert.Equal(t, int32(1), calls.Load())
				require.Len(t, records, 1)
				assert.Equal(t, int64(1401), records[0].ID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int32(2), calls.Load())
				assert.Zero(t, f.gate.reads.Load())
			}
		})
	}
}

func TestItemHookExecutionControlStorageAndCompletion(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	config := configmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl()).AnyTimes()
	config.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.RetryConf{RetryTimes: 3}).AnyTimes()
	original := &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}, Registered: []*entity.RegisteredEvalResult{{VersionID: 402, Alias: "old", RecordID: 12}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 13}}}
	f.etec.GetExistTurnResultRunLog(1).EvaluatorResultIds = original
	f.etec.GetExistTurnResultRunLog(1).TargetResultID = 100
	f.progress.persist = func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		assert.Equal(t, int64(100), rows[0].TargetResultID)
		assert.NotEqual(t, entity.TurnRunState_Fail, rows[0].Status)
		assert.NotEqual(t, entity.TurnRunState_Success, rows[0].Status)
		assert.Empty(t, rows[0].ErrMsg)
		assert.Equal(t, map[int64]int64{401: 11}, rows[0].EvaluatorResultIds.EvalVerIDToResID)
		assert.Equal(t, original.Inline, rows[0].EvaluatorResultIds.Inline)
		assert.Contains(t, rows[0].EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "old", RecordID: 12})
		assert.Contains(t, rows[0].EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: 402, Alias: "new", RecordID: 14})
		return nil
	}
	f.evaluators.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(14)).Return(nil)
	result := &entity.ExptTurnRunResult{TargetResult: executionTarget(), EvaluatorResults: []*entity.EvaluatorRecord{{ID: 14, EvaluatorVersionID: 402, Alias: "new", Status: entity.EvaluatorRunStatusAsyncInvoking}}, EvalErr: itemHookControlError{wait: true}}
	exec := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, Configer: config, evaluatorService: f.evaluators}
	require.NoError(t, exec.storeTurnRunResult(f.ctx, f.etec, result))
	var control itemHookControlError
	assert.ErrorAs(t, result.EvalErr, &control)
	assert.ErrorAs(t, exec.CompleteItemRun(f.ctx, f.etec.ExptItemEvalCtx, result.EvalErr), &control)
	assert.Equal(t, 2, f.etec.Event.RetryTimes)
	assert.False(t, f.etec.Event.CtxForceNoRetry(f.ctx))
	assert.Len(t, original.Registered, 1)
}

func TestItemHookExecutionAliasMixedFailure(t *testing.T) {
	f := newExecutionHookFixture(t, true, true, false)
	f.etec.Expt.EvalConf.ConnectorConf.EvaluatorsConf = buildRetryFailureSingleSetExpt(1, 401, 402, 403).EvalConf.ConnectorConf.EvaluatorsConf
	f.etec.Expt.EvalConf.ConnectorConf.EvaluatorsConf.EvaluatorConcurNum = gptr.Of(2)
	f.etec.Expt.Evaluators = buildRetryFailureSingleSetExpt(1, 401, 402, 403).Evaluators
	f.etec.ItemConfig.EvaluatorConfs = append(f.etec.ItemConfig.EvaluatorConfs, &entity.ItemEvaluatorConf{EvaluatorVersionID: 403, Alias: "c"})
	started, stopped := make(chan struct{}), make(chan struct{})
	var once sync.Once
	f.gate.onClosed = func() { once.Do(func() { close(stopped) }) }
	failure := errors.New("real evaluator failure")
	f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		switch req.EvaluatorVersionID {
		case 401:
			<-started
			f.gate.closed.Store(true)
			return &entity.EvaluatorRecord{ID: 1401, EvaluatorVersionID: 401, Alias: "a", Status: entity.EvaluatorRunStatusSuccess}, nil
		case 402:
			close(started)
			<-stopped
			return nil, failure
		default:
			once.Do(func() { close(stopped) })
			return nil, errors.New("queued evaluator was dispatched")
		}
	}).AnyTimes()
	_, err := f.turnEval.CallEvaluators(f.ctx, f.etec, executionTarget())
	assert.ErrorIs(t, err, failure, "a first control error must not mask an in-flight failure")
}
