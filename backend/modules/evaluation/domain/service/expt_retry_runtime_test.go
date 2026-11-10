// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRetryRuntimeOverrideUsesCurrentEventOnly(t *testing.T) {
	for _, override := range []string{"", `{"binary_version":"v0.0.250-rc.1"}`} {
		t.Run(override, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			target := svcmocks.NewMockIEvalTargetService(ctrl)
			svc := &DefaultExptTurnEvaluationImpl{metric: metric, evalTargetService: target}
			event := &entity.ExptItemEvalEvent{SpaceID: 1, ExptRunID: 2, Ext: map[string]string{}}
			if override != "" {
				event.Ext[entity.RetryRuntimeParamExtKey] = override
			}
			frozen := `{"model_name":"original-model","effort":"max","binary_version":"original"}`
			conf := &entity.TargetConf{TargetVersionID: 3, IngressConf: &entity.TargetIngressConf{EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "query", FromField: "query"}}}, CustomConf: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam, Value: frozen}}}}}
			etec := &entity.ExptTurnEvalCtx{ExptItemEvalCtx: &entity.ExptItemEvalCtx{Event: event, EvalSetItem: &entity.EvaluationSetItem{ItemID: 4}, Expt: &entity.Experiment{Target: &entity.EvalTarget{ID: 5, EvalTargetVersion: &entity.EvalTargetVersion{ID: 3}}, EvalConf: &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: conf}}}}, Turn: &entity.Turn{ID: 0, FieldDataList: []*entity.FieldData{{Name: "query", Content: &entity.Content{Text: gptr.Of("test")}}}}, Ext: map[string]string{entity.RetryRuntimeParamExtKey: `{"binary_version":"stale-item"}`}}
			metric.EXPECT().EmitTurnExecTargetResult(int64(1), false)
			target.EXPECT().ExecuteTarget(gomock.Any(), int64(1), int64(5), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				expected := frozen
				if override != "" {
					expected = `{"model_name":"original-model","effort":"max","binary_version":"v0.0.250-rc.1"}`
				}
				require.JSONEq(t, expected, input.Ext[consts.TargetExecuteExtRuntimeParamKey])
				require.NotContains(t, input.Ext, entity.RetryRuntimeParamExtKey)
				return &entity.EvalTargetRecord{ID: 9}, nil
			})
			_, err := svc.callTarget(context.Background(), etec, nil, 1)
			require.NoError(t, err)
			require.Equal(t, frozen, conf.IngressConf.CustomConf.FieldConfs[0].Value, "must not mutate experiment configuration")
		})
	}
}

func TestRetryRuntimeOverrideStartsWithoutPriorStageRefs(t *testing.T) {
	ctrl := gomock.NewController(t)
	turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
	ids := idgenmocks.NewMockIIDGenerator(ctrl)
	event := &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, EvalSetItemID: 4, ExptRunMode: entity.EvaluationModeFailRetry, Ext: map[string]string{entity.RetryRuntimeParamExtKey: `{"binary_version":"v0.0.250-rc.1"}`}}
	mode, err := NewRecordEvalMode(event, nil, turnRepo, nil, nil, nil, nil, ids, nil, nil)
	require.NoError(t, err)
	require.IsType(t, &ExptRecordEvalModeRetryIgnoreResult{}, mode)
	ids.EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{9}, nil)
	turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, logs []*entity.ExptTurnResultRunLog) error {
		require.Len(t, logs, 1)
		require.Zero(t, logs[0].TargetResultID)
		require.Nil(t, logs[0].EvaluatorResultIds)
		require.Equal(t, entity.TurnRunState_Processing, logs[0].Status)
		return nil
	})
	ctx := &entity.ExptItemEvalCtx{Event: event, EvalSetItem: &entity.EvaluationSetItem{ItemID: 4, Turns: []*entity.Turn{{ID: 0}}}, ExistItemEvalResult: &entity.ExptItemEvalResult{TurnResultRunLogs: map[int64]*entity.ExptTurnResultRunLog{}}}
	require.NoError(t, mode.PreEval(context.Background(), ctx))
}
