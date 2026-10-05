// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestItemHookExecutionWaitingPropagatesThroughItemAndConsumer(t *testing.T) {
	for _, callback := range []bool{false, true} {
		t.Run(fmt.Sprint(callback), func(t *testing.T) {
			f := newExecutionHookFixture(t, true, false, false)
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			config := configmocks.NewMockIConfiger(ctrl)
			config.EXPECT().BuildEvalExt(gomock.Any(), int64(1), gomock.Any()).Return(nil)
			items.EXPECT().GetItemRunLog(gomock.Any(), int64(2), int64(3), int64(4), int64(1)).Return(&entity.ExptItemResultRunLog{}, nil)
			items.EXPECT().BatchGet(gomock.Any(), int64(1), int64(2), []int64{4}).Return(nil, nil)
			if callback {
				f.etec.Event.AsyncReportTrigger = true
				f.etec.GetExistTurnResultRunLog(1).TargetResultID = 100
				f.targets.EXPECT().GetRecordByID(gomock.Any(), int64(1), int64(100)).DoAndReturn(func(context.Context, int64, int64) (*entity.EvalTargetRecord, error) {
					f.gate.waiting.Store(true)
					return executionTarget(), nil
				})
			} else {
				f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					f.gate.waiting.Store(true)
					return executionTarget(), nil
				})
			}
			f.progress.persist = func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				assert.Equal(t, int64(100), rows[0].TargetResultID)
				assert.NotEqual(t, entity.TurnRunState_Fail, rows[0].Status)
				assert.NotEqual(t, entity.TurnRunState_Success, rows[0].Status)
				assert.Empty(t, rows[0].ErrMsg)
				return nil
			}
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: items, TurnResultRepo: turns, Configer: config, Metric: f.metric, evalTargetService: f.targets, evaluatorService: f.evaluators, benefitService: f.turnEval.benefitService}
			pub := &itemHookPublisher{}
			consumer := &ExptItemEventEvalServiceImpl{publisher: pub}
			require.NoError(t, consumer.HandleEventErr(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error {
				return executor.Eval(ctx, f.etec.ExptItemEvalCtx)
			})(f.ctx, f.etec.Event))
			require.Len(t, pub.events, 1)
			expected := *f.etec.Event
			expected.HookControlContinuation = true
			assert.Equal(t, &expected, pub.events[0])
			assert.Equal(t, 2, pub.events[0].RetryTimes)
			assert.Equal(t, callback, pub.events[0].AsyncReportTrigger)
			assert.False(t, f.etec.Event.CtxForceNoRetry(f.ctx))
		})
	}
}

func TestItemHookExecutionTargetRechecksAfterPreparation(t *testing.T) {
	for _, async := range []bool{false, true} {
		f := newExecutionHookFixture(t, true, false, false)
		if async {
			f.etec.Expt.Target.EvalTargetVersion.CustomRPCServer = &entity.CustomRPCServer{IsAsync: gptr.Of(true)}
		}
		b := benefitmocks.NewMockIBenefitService(gomock.NewController(t))
		b.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
			f.gate.closed.Store(true)
			return nil, nil
		})
		f.turnEval.benefitService = b
		_, err := f.turnEval.CallTarget(f.ctx, f.etec)
		var control itemHookControlError
		assert.ErrorAs(t, err, &control)
	}
}

func TestItemHookExecutionInterruptedRefsArmOldAndNewAsync(t *testing.T) {
	f := newExecutionHookFixture(t, true, true, true)
	old := &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 11}, Registered: []*entity.RegisteredEvalResult{{VersionID: 402, Alias: "old", RecordID: 12}}, Inline: []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 13}}}
	f.etec.GetExistTurnResultRunLog(1).EvaluatorResultIds = old
	f.etec.ExptTurnRunResult.EvaluatorResults = []*entity.EvaluatorRecord{{ID: 12, EvaluatorVersionID: 402, Alias: "old", Status: entity.EvaluatorRunStatusAsyncInvoking}}
	records := []*entity.EvaluatorRecord{{ID: 21, EvaluatorVersionID: 401, Status: entity.EvaluatorRunStatusSuccess}, {ID: 23, InlineKey: "quality", SourceType: entity.EvaluatorRecordSourceTypeInline}, {ID: 24, EvaluatorVersionID: 402, Alias: "new", Status: entity.EvaluatorRunStatusAsyncInvoking}}
	turns := repomocks.NewMockIExptTurnResultRepo(gomock.NewController(t))
	saved := false
	f.progress.persist = func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		saved = true
		refs := rows[0].EvaluatorResultIds
		assert.Equal(t, map[int64]int64{401: 21}, refs.EvalVerIDToResID)
		assert.ElementsMatch(t, []*entity.RegisteredEvalResult{{VersionID: 401, RecordID: 21}, {VersionID: 402, Alias: "old", RecordID: 12}, {VersionID: 402, Alias: "new", RecordID: 24}}, refs.Registered)
		assert.Equal(t, []*entity.InlineEvalResult{{InlineKey: "quality", RecordID: 23}}, refs.Inline)
		return nil
	}
	for _, id := range []int64{12, 24} {
		f.evaluators.EXPECT().ArmEvaluatorResume(gomock.Any(), id).DoAndReturn(func(context.Context, int64) error { assert.True(t, saved); return nil })
	}
	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turns, evaluatorService: f.evaluators}
	require.NoError(t, executor.storeTurnRunResult(f.ctx, f.etec, &entity.ExptTurnRunResult{EvaluatorResults: records, EvalErr: itemHookControlError{wait: true}}))
	assert.Equal(t, int64(11), old.EvalVerIDToResID[401])
	assert.Equal(t, int64(13), old.Inline[0].RecordID)
}

func TestItemHookExecutionMixedErrorDoesNotBecomeControl(t *testing.T) {
	for _, failure := range []error{errors.New("real failure"), fmt.Errorf("outer: %w", errors.Join(itemHookControlError{}, errors.New("real failure")))} {
		f := newExecutionHookFixture(t, true, false, false)
		ctrl := gomock.NewController(t)
		config := configmocks.NewMockIConfiger(ctrl)
		config.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl())
		turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
		f.progress.persist = func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
			assert.Equal(t, entity.TurnRunState_Fail, rows[0].Status)
			return nil
		}
		result := &entity.ExptTurnRunResult{EvalErr: errors.Join(itemHookControlError{wait: true}, failure)}
		executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turns, Configer: config}
		require.NoError(t, executor.storeTurnRunResult(f.ctx, f.etec, result))
		var control itemHookControlError
		assert.False(t, errors.As(result.EvalErr, &control))
		assert.Contains(t, result.EvalErr.Error(), "real failure")
	}
}

func TestItemHookExecutionCancelledQueuedWorkerIsControl(t *testing.T) {
	for _, alias := range []bool{false, true} {
		f := newExecutionHookFixture(t, true, alias, false)
		ctx, cancel := context.WithCancel(f.ctx)
		defer cancel()
		f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
			cancel()
			return &entity.EvaluatorRecord{ID: 1401, EvaluatorVersionID: req.EvaluatorVersionID, Alias: req.Alias, Status: entity.EvaluatorRunStatusSuccess}, nil
		})
		records, err := f.turnEval.CallEvaluators(ctx, f.etec, executionTarget())
		require.Len(t, records, 1)
		var control itemHookControlError
		assert.ErrorAs(t, err, &control)
	}
}

func TestItemHookExecutionInterruptedRerunIsNotTerminal(t *testing.T) {
	for _, oldStatus := range []entity.TurnRunState{entity.TurnRunState_Success, entity.TurnRunState_Fail, entity.TurnRunState_Terminal} {
		f := newExecutionHookFixture(t, true, false, false)
		old := f.etec.GetExistTurnResultRunLog(1)
		old.Status, old.ErrMsg, old.TargetResultID = oldStatus, "previous attempt", 99
		turns := repomocks.NewMockIExptTurnResultRepo(gomock.NewController(t))
		f.progress.persist = func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
			assert.Equal(t, int64(100), rows[0].TargetResultID)
			assert.Equal(t, entity.TurnRunState_Processing, rows[0].Status)
			assert.Empty(t, rows[0].ErrMsg)
			return nil
		}
		executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turns}
		require.NoError(t, executor.storeTurnRunResult(f.ctx, f.etec, &entity.ExptTurnRunResult{TargetResult: executionTarget(), EvalErr: itemHookControlError{wait: true}}))
		assert.Equal(t, oldStatus, old.Status)
	}
}

func TestItemHookExecutionControlSaveFailureIsRetriableControl(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, true)
	turns := repomocks.NewMockIExptTurnResultRepo(gomock.NewController(t))
	f.progress.persist = func(context.Context, []*entity.ExptTurnResultRunLog) error { return errors.New("private DB error") }
	executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turns, evaluatorService: f.evaluators}
	result := &entity.ExptTurnRunResult{EvalErr: itemHookControlError{}, EvaluatorResults: []*entity.EvaluatorRecord{{ID: 99, Status: entity.EvaluatorRunStatusAsyncInvoking}}}
	err := executor.storeTurnRunResult(f.ctx, f.etec, result)
	var control itemHookControlError
	require.ErrorAs(t, err, &control)
	assert.True(t, control.wait)
	assert.NotContains(t, err.Error(), "private")
}

func TestItemHookExecutionAliasOrdinaryFailureKeepsLegacyError(t *testing.T) {
	f := newExecutionHookFixture(t, true, true, false)
	failure := errors.New("ordinary provider failure")
	f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).Return(nil, failure).Times(1)
	_, err := f.turnEval.CallEvaluators(f.ctx, f.etec, executionTarget())
	assert.Same(t, failure, err)
}
