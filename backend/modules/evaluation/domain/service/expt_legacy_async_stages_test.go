// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"github.com/bytedance/gg/gptr"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestLegacyContinuationRunsMissingEvaluatorWithoutRepeatingExisting(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing_success", true: "existing_invoking"}[pending], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := ctxcache.Init(context.Background())
			ctx = context.WithValue(ctx, currentItemSourceKey{}, &currentItemExecution{LegacyContinuation: true})
			expt := buildRetryFailureSingleSetExpt(3, 401, 402)
			reused := &entity.EvaluatorRecord{ID: 301, EvaluatorVersionID: 401, TargetRecordID: 201, Status: entity.EvaluatorRunStatusSuccess}
			if pending {
				reused.Status = entity.EvaluatorRunStatusAsyncInvoking
			}
			target := &entity.EvalTargetRecord{ID: 201, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}}
			turn := &entity.Turn{ID: 20, ItemID: 10}
			etec := &entity.ExptTurnEvalCtx{Turn: turn, ExptTurnRunResult: &entity.ExptTurnRunResult{TargetResult: target, EvaluatorResults: []*entity.EvaluatorRecord{reused}}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{Expt: expt, Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 103, EvalSetItemID: 10, SpaceID: 3, ExptRunMode: entity.EvaluationModeRetryItems, RequestID: "current-Q", ResumeSource: &entity.ExptResumeSource{Kind: "target", TurnID: 20, RecordID: 201, InvokeID: "201"}, AsyncReportTrigger: true, Session: &entity.Session{UserID: "u"}}, EvalSetItem: &entity.EvaluationSetItem{ItemID: 10, Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}}}
			evaluator := svcmocks.NewMockEvaluatorService(ctrl)
			benefit := benefitmocks.NewMockIBenefitService(ctrl)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			records := svcmocks.NewMockEvaluatorRecordService(ctrl)
			benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
			evaluator.EXPECT().ShouldInterceptEvaluator(gomock.Any(), gomock.Any()).Return(nil, false, nil).Times(1)
			evaluator.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				require.Equal(t, int64(402), req.EvaluatorVersionID)
				return &entity.EvaluatorRecord{ID: 302, EvaluatorVersionID: 402, TargetRecordID: 201, Status: entity.EvaluatorRunStatusSuccess}, nil
			}).Times(1)
			metric.EXPECT().EmitTurnExecEvaluatorResult(int64(3), false).Times(1)
			if pending {
				records.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(301), false).Return(reused, nil).Times(1)
			}
			engine := &DefaultExptTurnEvaluationImpl{metric: metric, evaluatorService: evaluator, benefitService: benefit, evaluatorRecordService: records}
			results, err := engine.CallEvaluators(ctx, etec, target)
			require.NoError(t, err)
			require.Len(t, results, 2)
			require.Equal(t, int64(301), results[0].ID)
			require.Equal(t, int64(302), results[1].ID)
			outcome := (&entity.ExptTurnRunResult{}).SetTargetResult(target).SetEvaluatorResults(results)
			require.Equal(t, pending, outcome.AbortWithEvaluatorResults(ctx, etec.Event), "partial invoking stages cannot finish the item")
		})
	}
}
