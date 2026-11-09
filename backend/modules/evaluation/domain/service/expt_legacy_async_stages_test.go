// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestLegacyContinuationRunsMissingEvaluatorWithoutRepeatingExisting(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "existing_success", true: "existing_invoking"}[pending], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := ctxcache.Init(context.Background())

			expt := buildRetryFailureSingleSetExpt(3, 401, 402)
			expt.ID = 1
			expt.LatestRunID = 103
			expt.Status = entity.ExptStatus_Processing
			expt.ExptType = entity.ExptType_Offline
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
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			manager := svcmocks.NewMockIExptManager(ctrl)
			source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 1, RunID: 103, ItemID: 10, ItemVersionID: 11, RequestID: "current-Q", Mode: entity.EvaluationModeRetryItems, Status: entity.ItemRunState_Processing}, Turns: map[int64]*entity.ExptTurnResultRunLog{20: {SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10, ItemVersionID: 11, TurnID: 20, Status: entity.TurnRunState_Processing, TargetResultID: 201, EvaluatorResultIds: &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{401: 301}}}}}
			items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(103), int64(10)).Return(&source.Current, nil)
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(3), int64(1), int64(103), int64(10)).Return(source, nil)
			port.EXPECT().CompleteCurrentItemRun(gomock.Any(), source, gomock.Any()).Return(true, nil)
			expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).AnyTimes()
			manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(103), int64(3), gomock.Any()).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 103, Status: int64(entity.ExptStatus_Processing), Mode: int32(entity.EvaluationModeSubmit)}, nil).AnyTimes()
			manager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(expt, nil).AnyTimes()
			targetReader := svcmocks.NewMockIEvalTargetService(ctrl)
			targetReader.EXPECT().GetRecordByID(gomock.Any(), int64(3), int64(201)).Return(&entity.EvalTargetRecord{ID: 201, SpaceID: 3, ExperimentRunID: 103, ItemID: 10, ItemVersionID: 11, TurnID: 20, TargetVersionID: expt.TargetVersionID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil).AnyTimes()
			metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			factory, _ := finishRealLockerFactory(t)
			svc := &ExptItemEventEvalServiceImpl{manager: manager, experimentRepo: expts, exptItemResultRepo: &legacyWakeSourceRepo{IExptItemResultRepo: items, IExptCurrentResultRepo: port}, exptTurnResultRepo: repomocks.NewMockIExptTurnResultRepo(ctrl), evaTargetService: targetReader, evaluatorRecordService: records, itemLocker: factory, metric: metric}
			var results []*entity.EvaluatorRecord
			svc.endpoints = RecordEvalChain(svc.HandleEventErr, svc.HandleEventCheck, svc.HandleCentralAdmission, svc.HandleEventLock, svc.HandleCurrentSource)(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				require.Equal(t, "current-Q", event.RequestID)
				require.NotNil(t, event.ResumeSource)
				require.True(t, executionSource(ctx).LegacyContinuation)
				etec.Event = event
				var err error
				results, err = engine.CallEvaluators(ctx, etec, target)
				return err
			})
			err := svc.Eval(ctx, &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 103, EvalSetItemID: 10, ExptRunMode: entity.EvaluationModeSubmit, AsyncReportTrigger: true, Session: &entity.Session{UserID: "u"}})
			require.NoError(t, err)
			require.Len(t, results, 2)
			require.Equal(t, int64(301), results[0].ID)
			require.Equal(t, int64(302), results[1].ID)
			outcome := (&entity.ExptTurnRunResult{}).SetTargetResult(target).SetEvaluatorResults(results)
			require.Equal(t, pending, outcome.AbortWithEvaluatorResults(ctx, etec.Event), "partial invoking stages cannot finish the item")
		})
	}
}
