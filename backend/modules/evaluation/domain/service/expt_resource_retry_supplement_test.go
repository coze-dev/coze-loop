// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/stretchr/testify/require"
)

type supplementBenefit struct{ benefit.IBenefitService }

func (supplementBenefit) CheckAndDeductEvalBenefit(context.Context, *benefit.CheckAndDeductEvalBenefitParams) (*benefit.CheckAndDeductEvalBenefitResult, error) {
	return nil, nil
}

type supplementMetric struct{ resourceMetric }

func (supplementMetric) EmitTurnExecTargetResult(int64, bool) {}

type supplementAsyncStore struct{ repo.IEvalAsyncRepo }

func (supplementAsyncStore) SetEvalAsyncCtx(context.Context, string, *entity.EvalAsyncCtx) error {
	return nil
}

type supplementTarget struct {
	IEvalTargetService
	starts int
}

func (s *supplementTarget) AsyncExecuteTarget(_ context.Context, space, target, version int64, param *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
	s.starts++
	return &entity.EvalTargetRecord{ID: int64(100 + s.starts), SpaceID: space, TargetID: target, TargetVersionID: version, ExperimentRunID: gptr.Indirect(param.ExperimentRunID), ItemID: param.ItemID, TurnID: param.TurnID, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), Ext: input.GetExt()}, "sandbox", nil
}
func supplementExecutionFixture() (*ExptItemEventEvalServiceImpl, *resourceItemStore, *resourceBroker, *supplementTarget, *entity.ExptTurnEvalCtx, context.Context) {
	svc, items, guard, event, ctx := resourceFixture()
	guard.ready = true
	event.ExptRunMode = entity.EvaluationModeSubmit
	broker := &resourceBroker{}
	svc.publisher = broker
	svc.configer = resourceConfig{retry: 3}
	svc.metric = resourceMetric{}
	svc.mutex = cancellationMutex{}
	target := &supplementTarget{}
	turnSvc := &DefaultExptTurnEvaluationImpl{metric: supplementMetric{}, evalTargetService: target, benefitService: supplementBenefit{}, evalAsyncRepo: supplementAsyncStore{}}
	expt := &entity.Experiment{ID: 1, SpaceID: 3, TargetID: 10, TargetVersionID: 20, TargetType: entity.EvalTargetTypeSandboxAgent, ExptDispatchMode: entity.ExptDispatchModeEnforce, Target: &entity.EvalTarget{ID: 10, EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 20, EvalTargetType: entity.EvalTargetTypeSandboxAgent}}, EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: 20, IngressConf: &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}}}}}
	etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event, Expt: expt, EvalSetItem: &entity.EvaluationSetItem{ID: 4}}, Turn: &entity.Turn{ID: 5}, ExptTurnRunResult: &entity.ExptTurnRunResult{}}
	svc.endpoints = svc.HandleCentralReservation(func(ctx context.Context, _ *entity.ExptItemEvalEvent) error {
		record, err := turnSvc.CallTarget(ctx, etec)
		if err == nil {
			etec.ExptTurnRunResult.TargetResult = record
		}
		return err
	})
	return svc, items, broker, target, etec, ctx
}
func (s *resourceItemStore) AuthorizeItemRetry(_ context.Context, _, _, _, _ int64, expected int32, errMsg []byte) (bool, error) {
	if s.writeErr != nil {
		return false, s.writeErr
	}
	if entity.ItemRunState(s.run.Status) != entity.ItemRunState_Processing || s.run.RetryTimes != expected {
		return false, nil
	}
	s.run.RetryTimes = expected + 1
	s.run.ErrMsg = errMsg
	return true, nil
}
func TestSupplementInterruptedInitialClaimDoesNotBlindlyCreate(t *testing.T) {
	svc, items, _, target, etec, ctx := supplementExecutionFixture()
	items.run.Status = int32(entity.ItemRunState_Processing)
	require.ErrorIs(t, svc.Eval(ctx, etec.Event), errCentralItemAdmission)
	require.Equal(t, 0, target.starts)
	require.Equal(t, int32(entity.ItemRunState_Processing), items.run.Status)
}

func TestSupplementUnknownPreviousAttemptDoesNotBlindlyCreate(t *testing.T) {
	svc, items, _, target, etec, ctx := supplementExecutionFixture()
	items.run.Status = int32(entity.ItemRunState_Processing)
	items.run.RetryTimes = 1
	etec.Event.RetryTimes = 1
	require.ErrorIs(t, svc.Eval(ctx, etec.Event), errCentralItemAdmission)
	require.Equal(t, 0, target.starts)
	require.Equal(t, int32(entity.ItemRunState_Processing), items.run.Status)
}

func TestSupplementTerminalWinsLateRetryAndError(t *testing.T) {
	svc, items, broker, _, etec, ctx := supplementExecutionFixture()
	items.run.Status = int32(entity.ItemRunState_Terminal)
	svc.endpoints = svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return errors.New("late retryable error") })
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Nil(t, broker.event)
	require.Equal(t, int32(entity.ItemRunState_Terminal), items.run.Status)
	require.Equal(t, int32(0), items.run.RetryTimes)
	svc.configer = resourceConfig{}
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, int32(entity.ItemRunState_Terminal), items.run.Status)
}
func TestSupplementLegacyTargetBehaviorUnchanged(t *testing.T) {
	svc, _, _, target, etec, _ := supplementExecutionFixture()
	ctx := ctxcache.Init(context.Background())
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, 2, target.starts, "legacy did not gain the new central reuse rule")
}

func TestSupplementRetryPublicationFailureReplaysAuthorizedRetry(t *testing.T) {
	svc, items, broker, _, etec, ctx := supplementExecutionFixture()
	items.run.Status = int32(entity.ItemRunState_Processing)
	broker.err = errors.New("broker temporarily unavailable")
	svc.endpoints = svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error {
		return errors.New("retryable execution failure")
	})
	require.ErrorIs(t, svc.Eval(ctx, etec.Event), broker.err)
	require.Equal(t, int32(1), items.run.RetryTimes, "authorize the next retry durably before publishing")
	require.Equal(t, int32(entity.ItemRunState_Processing), items.run.Status)
	broker.err = nil
	restarted := false
	svc.endpoints = svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { restarted = true; return nil })
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.False(t, restarted, "the stale original message must republish, not restart the old attempt")
	require.Equal(t, 1, broker.event.RetryTimes)
	require.Equal(t, etec.Event.ExptRunID, broker.event.ExptRunID)
	require.Equal(t, int32(1), items.run.RetryTimes)
}

func TestSupplementSameAttemptDoesNotRestartAndAuthorizedRetryProceeds(t *testing.T) {
	svc, items, _, target, etec, ctx := supplementExecutionFixture()
	require.NoError(t, svc.Eval(ctx, etec.Event))
	etec.ExptTurnRunResult.TargetResult.Status = gptr.Of(entity.EvalTargetRunStatusFail)
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, 1, target.starts, "a failed record does not authorize rerunning the same message")
	items.run.RetryTimes = 1
	etec.Event.RetryTimes = 1
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, 2, target.starts, "a genuine next authorized retry must run")
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, 2, target.starts, "duplicate delayed retry must reuse its in-flight record")
	etec.Event.AsyncReportTrigger = true
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, 2, target.starts, "matching callback must not restart the target")
}

func TestSupplementDuplicateItemAfterAsyncReturnStartsOnce(t *testing.T) {
	svc, _, _, target, etec, ctx := supplementExecutionFixture()
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.NoError(t, svc.Eval(ctx, etec.Event))
	require.Equal(t, 1, target.starts)
}
