// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestHookMetadataActualTargetRecords(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, async := range []bool{false, true} {
			t.Run(fmt.Sprintf("managed=%v/async=%v", managed, async), func(t *testing.T) {
				f := newHookMetadataFixture(t, managed, 0)
				ctx, err := bindItemHookTurnIdentity(f.ctx, f.etec)
				require.NoError(t, err)
				ctrl := gomock.NewController(t)
				r := repomocks.NewMockIEvalTargetRepo(ctrl)
				id := idmocks.NewMockIIDGenerator(ctrl)
				m := metricmocks.NewMockEvalTargetMetrics(ctrl)
				op := svcmocks.NewMockISourceEvalTargetOperateService(ctrl)
				c := configmocks.NewMockIConfiger(ctrl)
				c.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				target := &entity.EvalTarget{ID: 10, SpaceID: 9, EvalTargetType: entity.EvalTargetTypeLoopPrompt, EvalTargetVersion: &entity.EvalTargetVersion{ID: 11}}
				r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(11)).Return(target, nil).AnyTimes()
				m.EXPECT().EmitRun(int64(9), gomock.Any(), gomock.Any())
				op.EXPECT().ValidateInput(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).Return(nil)
				if async {
					op.EXPECT().AsyncExecute(gomock.Any(), int64(9), gomock.Any()).Return(int64(99), "callee", map[string]string(nil), nil)
				} else {
					op.EXPECT().Execute(gomock.Any(), int64(9), gomock.Any()).Return(&entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}, entity.EvalTargetRunStatusSuccess, nil)
					id.EXPECT().GenID(gomock.Any()).Return(int64(99), nil)
				}
				r.EXPECT().CreateEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvalTargetRecord, _ *bool) (int64, error) {
					want := int64(0)
					if managed {
						want = 7
					}
					assert.Equal(t, want, rec.ItemVersionID)
					assert.Equal(t, int64(9), rec.SpaceID)
					assert.Equal(t, int64(3), rec.ExperimentRunID)
					assert.Equal(t, int64(0), rec.TurnID)
					return rec.ID, nil
				})
				s := &EvalTargetServiceImpl{evalTargetRepo: r, idgen: id, metric: m, configer: c, typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{entity.EvalTargetTypeLoopPrompt: op}}
				param := &entity.ExecuteTargetCtx{ExptSpaceID: 1, ExperimentID: gptr.Of(int64(2)), ExperimentRunID: gptr.Of(int64(3)), ItemID: 4, TurnID: 0, EnableExtractTrajectory: gptr.Of(false)}
				input := &entity.EvalTargetInputData{Ext: map[string]string{"item_version_id": "999"}}
				if async {
					_, _, err = s.AsyncExecuteTarget(ctx, 9, 10, 11, param, input)
				} else {
					_, err = s.ExecuteTarget(ctx, 9, 10, 11, param, input)
				}
				require.NoError(t, err)
			})
		}
	}
}

func TestHookMetadataActualEvaluatorRecords(t *testing.T) {
	for _, managed := range []bool{false, true} {
		for _, kind := range []string{"sync", "async", "intercept", "skipped", "failed"} {
			t.Run(fmt.Sprintf("managed=%v/%s", managed, kind), func(t *testing.T) {
				f := newHookMetadataFixture(t, managed, 0)
				ctx, err := bindItemHookTurnIdentity(f.ctx, f.etec)
				require.NoError(t, err)
				ctrl := gomock.NewController(t)
				r := repomocks.NewMockIEvaluatorRepo(ctrl)
				rr := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
				id := idmocks.NewMockIIDGenerator(ctrl)
				rate := repomocks.NewMockRateLimiter(ctrl)
				plain := repomocks.NewMockIPlainRateLimiter(ctrl)
				source := svcmocks.NewMockEvaluatorSourceService(ctrl)
				ar := repomocks.NewMockIEvalAsyncRepo(ctrl)
				ev := &entity.Evaluator{ID: 10, SpaceID: 9, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 11}}
				if kind == "async" {
					ev.EvaluatorType = entity.EvaluatorTypeAgent
					ev.AgentEvaluatorVersion = &entity.AgentEvaluatorVersion{ID: 11}
				}
				s := &EvaluatorServiceImpl{evaluatorRepo: r, evaluatorRecordRepo: rr, idgen: id, limiter: rate, plainRateLimiter: plain, evalAsyncRepo: ar, evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{ev.EvaluatorType: source}}
				req := &entity.RunEvaluatorRequest{SpaceID: 1, ExperimentID: 2, ExperimentRunID: 3, ItemID: 4, TurnID: 0, EvaluatorVersionID: 11, Alias: "a", InputData: &entity.EvaluatorInputData{}, Ext: map[string]string{"item_version_id": "999"}, SharedOption: &entity.SharedResourceOption{IsShared: true, SourceSpaceID: gptr.Of(int64(9))}}
				id.EXPECT().GenID(gomock.Any()).Return(int64(99), nil)
				if kind == "sync" || kind == "async" || kind == "intercept" {
					r.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{11}, false, false).Return([]*entity.Evaluator{ev}, nil)
				}
				if kind == "sync" || kind == "async" {
					rate.EXPECT().AllowInvoke(gomock.Any(), gomock.Any()).Return(true).Times(2)
					plain.EXPECT().AllowInvokeWithKeyLimit(gomock.Any(), gomock.Any(), gomock.Any()).Return(true)
				}
				if kind == "sync" {
					source.EXPECT().PreHandle(gomock.Any(), ev).Return(nil)
					source.EXPECT().Run(gomock.Any(), ev, req.InputData, gomock.Any(), int64(9), false).Return(&entity.EvaluatorOutputData{}, entity.EvaluatorRunStatusSuccess, "")
				}
				if kind == "async" {
					ar.EXPECT().SetEvalAsyncCtx(gomock.Any(), "evaluator:99", gomock.Any()).Return(nil)
					source.EXPECT().AsyncRun(gomock.Any(), ev, req.InputData, gomock.Any(), int64(9), int64(99)).Return(map[string]string(nil), "", nil)
				}
				if kind == "intercept" {
					source.EXPECT().ShouldIntercept(gomock.Any(), ev, req.InputData).Return(&entity.EvaluatorOutputData{}, entity.EvaluatorRunStatusSuccess, true)
				}
				rr.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rec *entity.EvaluatorRecord) error {
					want := int64(0)
					if managed {
						want = 7
					}
					assert.Equal(t, want, rec.ItemVersionID)
					owner := int64(1)
					if kind == "sync" || kind == "async" {
						owner = 9
					}
					assert.Equal(t, owner, rec.SpaceID)
					assert.Equal(t, int64(3), rec.ExperimentRunID)
					assert.Equal(t, int64(0), rec.TurnID)
					if kind == "intercept" {
						alias := ""
						if managed {
							alias = "a"
						}
						assert.Equal(t, alias, rec.Alias)
					}
					return nil
				})
				switch kind {
				case "sync":
					_, err = s.RunEvaluator(ctx, req)
				case "async":
					_, err = s.AsyncRunEvaluator(ctx, &entity.AsyncRunEvaluatorRequest{SpaceID: 1, ExperimentID: 2, ExperimentRunID: 3, ItemID: 4, TurnID: 0, EvaluatorVersionID: 11, Alias: "a", InputData: req.InputData, Ext: req.Ext, SharedOption: req.SharedOption})
				case "intercept":
					_, _, err = s.ShouldInterceptEvaluator(ctx, req)
				case "skipped":
					_, err = s.CreateSkippedEvaluatorRecord(ctx, req)
				case "failed":
					_, err = s.CreateEvaluatorRunFailRecord(ctx, req, errors.New("provider failed"))
				}
				require.NoError(t, err)
			})
		}
	}
}

func TestHookMetadataDoesNotBypassEvaluatorAuthorization(t *testing.T) {
	f := newHookMetadataFixture(t, true, 0)
	ctx, err := bindItemHookTurnIdentity(f.ctx, f.etec)
	require.NoError(t, err)
	r := repomocks.NewMockIEvaluatorRepo(gomock.NewController(t))
	r.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), nil, []int64{11}, false, false).Return([]*entity.Evaluator{{ID: 10, SpaceID: 9, EvaluatorType: entity.EvaluatorTypePrompt}}, nil)
	_, err = (&EvaluatorServiceImpl{evaluatorRepo: r}).RunEvaluator(ctx, &entity.RunEvaluatorRequest{SpaceID: 1, ExperimentID: 2, ExperimentRunID: 3, ItemID: 4, TurnID: 0, EvaluatorVersionID: 11})
	require.Error(t, err)
	var control itemHookControlError
	assert.False(t, errors.As(err, &control))
}

func TestHookMetadataCreatorsRejectWrongInvocation(t *testing.T) {
	f := newHookMetadataFixture(t, true, 0)
	ctx, err := bindItemHookTurnIdentity(f.ctx, f.etec)
	require.NoError(t, err)
	req := &entity.RunEvaluatorRequest{SpaceID: 1, ExperimentID: 2, ExperimentRunID: 99, ItemID: 4, TurnID: 0, EvaluatorVersionID: 11}
	s := &EvaluatorServiceImpl{}
	var control itemHookControlError
	_, err = s.RunEvaluator(ctx, req)
	require.ErrorAs(t, err, &control)
	_, _, err = s.ShouldInterceptEvaluator(ctx, req)
	require.ErrorAs(t, err, &control)
	_, err = s.CreateSkippedEvaluatorRecord(ctx, req)
	require.ErrorAs(t, err, &control)
	_, err = s.CreateEvaluatorRunFailRecord(ctx, req, errors.New("bad"))
	require.ErrorAs(t, err, &control)
	_, err = s.AsyncRunEvaluator(ctx, &entity.AsyncRunEvaluatorRequest{SpaceID: 1, ExperimentID: 2, ExperimentRunID: 99, ItemID: 4, TurnID: 0})
	require.ErrorAs(t, err, &control)
	metric := metricmocks.NewMockEvalTargetMetrics(gomock.NewController(t))
	metric.EXPECT().EmitRun(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	target := &EvalTargetServiceImpl{metric: metric}
	param := &entity.ExecuteTargetCtx{ExptSpaceID: 1, ExperimentID: gptr.Of(int64(2)), ExperimentRunID: gptr.Of(int64(99)), ItemID: 4, TurnID: 0}
	_, err = target.ExecuteTarget(ctx, 9, 10, 11, param, &entity.EvalTargetInputData{})
	require.ErrorAs(t, err, &control)
	_, _, err = target.asyncExecuteTarget(ctx, 9, &entity.EvalTarget{ID: 10, EvalTargetVersion: &entity.EvalTargetVersion{ID: 11}}, param, &entity.EvalTargetInputData{})
	require.ErrorAs(t, err, &control)
}
