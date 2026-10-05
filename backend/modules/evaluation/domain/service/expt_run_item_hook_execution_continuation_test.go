// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func prepareHookContinuationRecords(f *executionHookFixture, alias, async bool) {
	for _, ev := range f.etec.Expt.Evaluators {
		ev.SpaceID = 1
	}
	tr := executionTarget()
	tr.SpaceID, tr.TargetID, tr.TargetVersionID, tr.ExperimentRunID, tr.ItemID, tr.TurnID = 1, 1, 1, 3, 4, 1
	f.etec.ExptTurnRunResult.TargetResult = tr
	log := f.etec.GetExistTurnResultRunLog(1)
	log.TargetResultID = tr.ID
	log.EvaluatorResultIds = &entity.EvaluatorResults{}
	for i, id := range []int64{401, 402} {
		a := ""
		if alias {
			a = []string{"a", "b"}[i]
		}
		status := entity.EvaluatorRunStatusSuccess
		if async {
			status = entity.EvaluatorRunStatusAsyncInvoking
		}
		r := &entity.EvaluatorRecord{ID: id + 1000, SpaceID: 1, ExperimentID: 2, ExperimentRunID: 3, ItemID: 4, TurnID: 1, EvaluatorVersionID: id, Alias: a, Status: status}
		f.etec.ExptTurnRunResult.EvaluatorResults = append(f.etec.ExptTurnRunResult.EvaluatorResults, r)
		log.EvaluatorResultIds.Registered = append(log.EvaluatorResultIds.Registered, &entity.RegisteredEvalResult{VersionID: id, Alias: a, RecordID: r.ID})
	}
}

func TestItemHookControlContinuationReusesRetryAllAndItems(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, alias := range []bool{false, true} {
			for _, async := range []bool{false, true} {
				t.Run(fmt.Sprintf("%d/alias=%v/async=%v", mode, alias, async), func(t *testing.T) {
					f := newExecutionHookFixture(t, true, alias, async)
					f.etec.Event.ExptRunMode, f.etec.Event.RetryTimes = mode, 0
					prepareHookContinuationRecords(f, alias, async)
					pub := &itemHookPublisher{}
					require.NoError(t, (&ExptItemEventEvalServiceImpl{publisher: pub}).publishItemHookWait(f.ctx, f.etec.Event))
					f.etec.Event = pub.events[0]
					raw, err := json.Marshal(f.etec.Event)
					require.NoError(t, err)
					assert.Contains(t, string(raw), `"hook_control_continuation":true`)
					var targetCalls, evaluatorCalls atomic.Int32
					f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
						targetCalls.Add(1)
						return f.etec.ExptTurnRunResult.TargetResult, nil
					}).AnyTimes()
					f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
						evaluatorCalls.Add(1)
						return &entity.EvaluatorRecord{ID: r.EvaluatorVersionID + 2000, EvaluatorVersionID: r.EvaluatorVersionID, Alias: r.Alias, Status: entity.EvaluatorRunStatusSuccess}, nil
					}).AnyTimes()
					f.evaluators.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
						evaluatorCalls.Add(1)
						return &entity.EvaluatorRecord{ID: r.EvaluatorVersionID + 2000, EvaluatorVersionID: r.EvaluatorVersionID, Alias: r.Alias, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil
					}).AnyTimes()
					result := f.turnEval.Eval(f.ctx, f.etec)
					require.NoError(t, result.EvalErr)
					assert.Zero(t, targetCalls.Load())
					assert.Zero(t, evaluatorCalls.Load())
					assert.Equal(t, f.etec.ExptTurnRunResult.EvaluatorResults, result.EvaluatorResults)
					assert.Equal(t, async, result.AsyncAbort)
				})
			}
		}
	}
}

func TestItemHookControlContinuationPreservesAsyncTarget(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	f.etec.Event.ExptRunMode, f.etec.Event.RetryTimes = entity.EvaluationModeRetryAll, 0
	prepareHookContinuationRecords(f, false, false)
	f.etec.Expt.Target.EvalTargetVersion.CustomRPCServer = &entity.CustomRPCServer{IsAsync: gptr.Of(true)}
	f.etec.ExptTurnRunResult.TargetResult.Status = gptr.Of(entity.EvalTargetRunStatusAsyncInvoking)
	pub := &itemHookPublisher{}
	require.NoError(t, (&ExptItemEventEvalServiceImpl{publisher: pub}).publishItemHookWait(f.ctx, f.etec.Event))
	f.etec.Event = pub.events[0]
	var calls int
	f.targets.EXPECT().AsyncExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
		calls++
		return f.etec.ExptTurnRunResult.TargetResult, "callee", nil
	}).AnyTimes()
	f.async.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	result := f.turnEval.Eval(f.ctx, f.etec)
	require.NoError(t, result.EvalErr)
	assert.True(t, result.AsyncAbort)
	assert.Zero(t, calls)
}

func TestItemHookControlContinuationRejectsMismatchedRecords(t *testing.T) {
	for _, field := range []string{"target-run", "target-item", "target-turn", "target-version", "target-space", "target-id", "item-version", "eval-run", "eval-experiment", "eval-item", "eval-turn", "eval-version", "eval-space", "eval-alias", "eval-id", "stored-ref", "stored-run", "stored-terminal", "read-failure"} {
		t.Run(field, func(t *testing.T) {
			f := newExecutionHookFixture(t, true, true, false)
			prepareHookContinuationRecords(f, true, false)
			f.etec.Event.ExptRunMode, f.etec.Event.RetryTimes, f.etec.Event.HookControlContinuation = entity.EvaluationModeRetryAll, 0, true
			tr := f.etec.ExptTurnRunResult.TargetResult
			er := f.etec.ExptTurnRunResult.EvaluatorResults[0]
			switch field {
			case "target-run":
				tr.ExperimentRunID++
			case "target-item":
				tr.ItemID++
			case "target-turn":
				tr.TurnID++
			case "target-version":
				tr.TargetVersionID++
			case "target-space":
				tr.SpaceID++
			case "target-id":
				tr.ID++
			case "item-version":
				tr.ItemVersionID++
			case "eval-run":
				er.ExperimentRunID++
			case "eval-experiment":
				er.ExperimentID++
			case "eval-item":
				er.ItemID++
			case "eval-turn":
				er.TurnID++
			case "eval-version":
				er.EvaluatorVersionID++
			case "eval-space":
				er.SpaceID++
			case "eval-alias":
				er.Alias = "wrong"
			case "eval-id":
				er.ID++
			case "stored-ref":
				f.etec.GetExistTurnResultRunLog(1).EvaluatorResultIds.Registered[0].RecordID++
			case "stored-run":
				f.etec.GetExistTurnResultRunLog(1).ExptRunID++
			case "stored-terminal":
				f.etec.GetExistTurnResultRunLog(1).Status = entity.TurnRunState_Terminal
			case "read-failure":
				f.progress.read = func(context.Context, entity.HookTurnProgressKey) (*entity.ExptTurnResultRunLog, error) {
					return nil, errors.New("private database failure")
				}
			}
			result := f.turnEval.Eval(f.ctx, f.etec)
			var control itemHookControlError
			require.ErrorAs(t, result.EvalErr, &control)
			assert.NotContains(t, result.EvalErr.Error(), "private")
		})
	}
}

func TestItemHookControlContinuationHintIsNotAdmission(t *testing.T) {
	f := newItemHookFixture(t, true, false)
	f.event.HookControlContinuation = true
	f.ports.gates = []entity.HookGateState{entity.HookGateClosed}
	assert.Error(t, f.svc.Eval(context.Background(), f.event))
	f.noExecution(t)
	assert.Empty(t, f.ports.admitted)
}

func TestItemHookControlContinuationOrdinaryAndLegacyStillForceIgnore(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		f := newExecutionHookFixture(t, !legacy, false, false)
		prepareHookContinuationRecords(f, false, false)
		f.etec.Event.ExptRunMode, f.etec.Event.RetryTimes = entity.EvaluationModeRetryAll, 0
		f.etec.Event.HookControlContinuation = legacy
		f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(executionTarget(), nil).Times(1)
		f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
			return &entity.EvaluatorRecord{ID: r.EvaluatorVersionID + 2000, EvaluatorVersionID: r.EvaluatorVersionID, Status: entity.EvaluatorRunStatusSuccess}, nil
		}).Times(2)
		require.NoError(t, f.turnEval.Eval(f.ctx, f.etec).EvalErr)
	}
}

func TestItemHookControlContinuationGenuineRetryClearsHint(t *testing.T) {
	f := newItemHookFixture(t, true, false)
	f.event.HookControlContinuation = true
	svc := f.svc.(*ExptItemEventEvalServiceImpl)
	require.NoError(t, svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("real execution failure") })(context.Background(), f.event))
	require.Len(t, f.publisher.events, 1)
	assert.False(t, f.publisher.events[0].HookControlContinuation)
	assert.Equal(t, 3, f.publisher.events[0].RetryTimes)
	assert.True(t, f.event.HookControlContinuation)
	bytes, err := json.Marshal(&entity.ExptItemEvalEvent{})
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "hook_control_continuation")
}

func TestItemHookControlContinuationTargetChangesBeforeEvaluators(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.HookControlContinuation = true
	reads := 0
	f.progress.read = func(context.Context, entity.HookTurnProgressKey) (*entity.ExptTurnResultRunLog, error) {
		reads++
		row := *f.etec.GetExistTurnResultRunLog(1)
		if reads > 1 {
			row.TargetResultID = 999
		}
		return &row, nil
	}
	var evaluatorCalls atomic.Int32
	f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		evaluatorCalls.Add(1)
		return &entity.EvaluatorRecord{ID: r.EvaluatorVersionID + 2000, EvaluatorVersionID: r.EvaluatorVersionID, Status: entity.EvaluatorRunStatusSuccess}, nil
	}).AnyTimes()
	result := f.turnEval.Eval(f.ctx, f.etec)
	var control itemHookControlError
	assert.ErrorAs(t, result.EvalErr, &control)
	assert.Zero(t, evaluatorCalls.Load(), "a concurrently replaced target cannot feed new evaluators")
}

func TestItemHookControlContinuationCanExecuteMissingOrFailedTarget(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.HookControlContinuation = true
	f.etec.ExptTurnRunResult.TargetResult.Status = gptr.Of(entity.EvalTargetRunStatusFail)
	fresh := executionTarget()
	fresh.ID = 101
	f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fresh, nil).Times(1)
	f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
		return &entity.EvaluatorRecord{ID: r.EvaluatorVersionID + 2000, EvaluatorVersionID: r.EvaluatorVersionID, Status: entity.EvaluatorRunStatusSuccess}, nil
	}).Times(2)
	require.NoError(t, f.turnEval.Eval(f.ctx, f.etec).EvalErr)
}

func TestItemHookControlContinuationFreshTargetProofDoesNotSurviveAdmission(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.HookControlContinuation = true
	rememberItemHookTarget(f.ctx, f.etec, f.etec.ExptTurnRunResult.TargetResult)
	b := f.ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	b.targets = new(sync.Map)
	ctx := context.WithValue(f.ctx, itemHookProgressContextKey{}, b)
	f.etec.GetExistTurnResultRunLog(1).TargetResultID = 999
	_, err := f.turnEval.CallEvaluators(ctx, f.etec, f.etec.ExptTurnRunResult.TargetResult)
	var control itemHookControlError
	require.ErrorAs(t, err, &control)
}

func TestItemHookControlContinuationSharedEvaluatorUsesResourceOwner(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.HookControlContinuation = true
	for _, ev := range f.etec.Expt.Evaluators {
		ev.SpaceID = 9
	}
	for _, record := range f.etec.ExptTurnRunResult.EvaluatorResults {
		record.SpaceID = 9
	}
	result := f.turnEval.Eval(f.ctx, f.etec)
	require.NoError(t, result.EvalErr)
	assert.Equal(t, f.etec.ExptTurnRunResult.EvaluatorResults, result.EvaluatorResults)
}

func TestItemHookControlContinuationMissingItemVersionStaysBlocked(t *testing.T) {
	f := newExecutionHookFixture(t, true, false, false)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.HookControlContinuation = true
	f.etec.GetExistTurnResultRunLog(1).ItemVersionID = 7
	result := f.turnEval.Eval(f.ctx, f.etec)
	var control itemHookControlError
	require.ErrorAs(t, result.EvalErr, &control)
	assert.Equal(t, int64(0), f.etec.ExptTurnRunResult.TargetResult.ItemVersionID)
}

func TestItemHookControlContinuationMatchingItemVersionCanResume(t *testing.T) {
	f := newHookMetadataFixture(t, true, 1)
	prepareHookContinuationRecords(f, false, false)
	f.etec.Event.HookControlContinuation = true
	f.etec.GetExistTurnResultRunLog(1).ItemVersionID = 7
	f.etec.ExptTurnRunResult.TargetResult.ItemVersionID = 7
	for _, r := range f.etec.ExptTurnRunResult.EvaluatorResults {
		r.ItemVersionID = 7
	}
	require.NoError(t, f.turnEval.Eval(f.ctx, f.etec).EvalErr)
}

func TestItemHookControlContinuationProgressDependencyRequired(t *testing.T) {
	f := newItemHookFixture(t, true, false)
	_, err := NewHookAwareExptRecordEvalService(f.base, f.ports, f.ports, f.ports, nil)
	require.Error(t, err)
	var missing *executionHookProgress
	_, err = NewHookAwareExptRecordEvalService(f.base, f.ports, f.ports, f.ports, missing)
	require.Error(t, err)
	fixture := newExecutionHookFixture(t, true, false, false)
	err = (&ExptItemEvalCtxExecutor{}).storeTurnRunResult(context.Background(), fixture.etec, &entity.ExptTurnRunResult{EvalErr: itemHookControlError{wait: true}})
	var control itemHookControlError
	require.ErrorAs(t, err, &control)
}
