// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

func TestRetryFailure_PreRecordErrorKeepsPartialEvaluatorResults(t *testing.T) {
	for _, tt := range []struct {
		name                                                                   string
		multi, noTarget, targetCalled, missingEvaluator, async, skippedFailure bool
	}{
		{name: "single set benefit failure"},
		{name: "multiple set alias benefit failure", multi: true},
		{name: "multiple set missing pending evaluator", multi: true, missingEvaluator: true},
		{name: "multiple set skipped record failure", multi: true, skippedFailure: true},
		{name: "no target benefit failure", noTarget: true},
		{name: "target rerun invalidates old scores", targetCalled: true},
		{name: "pending async record is preserved", async: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			ctx := ctxcache.Init(context.Background())
			expt := buildRetryFailureSingleSetExpt(3, 401, 402)
			targetID := int64(201)
			if tt.noTarget {
				expt.TargetVersionID = 0
				targetID = 0
			}
			if tt.targetCalled {
				targetID = 202
			}
			reused := &entity.EvaluatorRecord{ID: 301, EvaluatorVersionID: 401, Status: entity.EvaluatorRunStatusSuccess}
			if tt.async {
				reused.Status = entity.EvaluatorRunStatusAsyncInvoking
			}
			target := &entity.EvalTargetRecord{ID: targetID, Status: gptr.Of(entity.EvalTargetRunStatusSuccess), EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}}}
			turn := &entity.Turn{ID: 20, ItemID: 10}
			etec := &entity.ExptTurnEvalCtx{Turn: turn, ExptTurnRunResult: &entity.ExptTurnRunResult{TargetResult: target, EvaluatorResults: []*entity.EvaluatorRecord{reused}}, ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				Expt: expt, Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 103, EvalSetItemID: 10, SpaceID: 3, ExptRunMode: entity.EvaluationModeFailRetry, Session: &entity.Session{UserID: "u"}},
				EvalSetItem:         &entity.EvaluationSetItem{ItemID: 10, Turns: []*entity.Turn{turn}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}},
				ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{20: {ID: 21, ExptID: 1, ExptRunID: 103, SpaceID: 3, ItemID: 10, TurnID: 20, TargetResultID: targetID}}},
			}}
			if tt.multi {
				expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				reused.Alias = "judge-a"
				etec.ItemConfig = &entity.ExptItemConfig{EvaluatorConfs: []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 401, Alias: "judge-a"}, {EvaluatorVersionID: 402, Alias: "judge-b"}}}
			}
			if tt.skippedFailure {
				etec.ItemConfig.EvaluatorConfs[1].FilterMode = 1
				etec.ItemConfig.EvaluatorConfs[1].Filter = &entity.ExptItemFilter{QueryAndOr: "and", FilterFields: []*entity.ExptItemFilterField{{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: []string{"999"}}}}
			}
			if tt.missingEvaluator {
				expt.Evaluators = expt.Evaluators[:1]
			}
			if tt.targetCalled {
				etec.Event.WithCtxTargetCalled(ctx)
			}
			benefit := benefitmocks.NewMockIBenefitService(ctrl)
			failure := errors.New("quota unavailable before pending evaluator record")
			if !tt.missingEvaluator && !tt.skippedFailure {
				benefit.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, failure)
			}
			evaluatorSvc := svcmocks.NewMockEvaluatorService(ctrl)
			if tt.skippedFailure {
				evaluatorSvc.EXPECT().CreateSkippedEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil, failure)
			}
			if tt.async {
				evaluatorSvc.EXPECT().ArmEvaluatorResume(gomock.Any(), int64(301)).Return(nil)
			}
			turnEval := &DefaultExptTurnEvaluationImpl{benefitService: benefit, evaluatorService: evaluatorSvc}
			results, evalErr := turnEval.CallEvaluators(ctx, etec, target)
			require.Error(t, evalErr)
			if !tt.missingEvaluator {
				assert.ErrorIs(t, evalErr, failure)
			}
			if tt.targetCalled {
				assert.Empty(t, results)
			} else {
				require.Len(t, results, 1)
				assert.Same(t, reused, results[0])
			}
			turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			config := configmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetErrCtrl(gomock.Any()).Return(entity.DefaultExptErrCtrl())
			turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
				require.Len(t, rows, 1)
				assert.Equal(t, targetID, rows[0].TargetResultID)
				assert.Equal(t, entity.TurnRunState_Fail, rows[0].Status)
				if tt.targetCalled {
					assert.Empty(t, rows[0].EvaluatorResultIds.Registered)
				} else {
					require.Len(t, rows[0].EvaluatorResultIds.Registered, 1)
					assert.Equal(t, int64(301), rows[0].EvaluatorResultIds.Registered[0].RecordID)
				}
				return nil
			})
			executor := &ExptItemEvalCtxExecutor{TurnResultRepo: turnRepo, Configer: config, evaluatorService: evaluatorSvc}
			require.NoError(t, executor.storeTurnRunResult(ctx, etec, &entity.ExptTurnRunResult{TargetResult: target, EvaluatorResults: results, EvalErr: evalErr}))
		})
	}
}
