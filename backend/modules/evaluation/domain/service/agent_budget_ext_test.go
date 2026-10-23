// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAgentBudgetCallTargetFrozenExt(t *testing.T) {
	ctrl := gomock.NewController(t)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	benefits := benefitmocks.NewMockIBenefitService(ctrl)
	svc := &DefaultExptTurnEvaluationImpl{metric: metric, evalTargetService: target, benefitService: benefits}
	itemConfig := &entity.ExptItemConfig{EvalTargetConf: &entity.ItemTargetConf{
		TargetVersionID: 1,
		RunConf:         &entity.ItemRunConf{MaxRunMinutes: 0},
	}}
	raw, err := json.Marshal(itemConfig)
	require.NoError(t, err)
	var frozen entity.ExptItemConfig
	require.NoError(t, json.Unmarshal(raw, &frozen))
	etec := newRunConfEtec(&frozen, &entity.RunModeConfig{MaxRunMinutes: 20})
	etec.Expt.TargetVersionID = 1
	etec.ExptTurnRunResult = &entity.ExptTurnRunResult{}
	benefits.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
	metric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
	target.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _ int64, _ *entity.ExecuteTargetCtx, in *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
			var item, experiment map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(in.Ext[consts.TargetExecuteExtRunConfKey]), &item))
			require.NoError(t, json.Unmarshal([]byte(in.Ext[consts.TargetExecuteExtRunModeConfigKey]), &experiment))
			require.NotContains(t, item, "max_run_minutes")
			require.JSONEq(t, "20", string(experiment["max_run_minutes"]))
			return &entity.EvalTargetRecord{ID: 1}, nil
		})
	record, err := svc.CallTarget(context.Background(), etec)
	require.NoError(t, err)
	require.Equal(t, int64(1), record.ID)
}

func TestAgentBudgetExptStartDoesNotSeedZero(t *testing.T) {
	ctrl := gomock.NewController(t)
	exec, items, refs := newExptStartMultiSetTestExec(ctrl)
	exec.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	items.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(
		[]*entity.EvaluationSetItem{{ItemID: 1, Turns: []*entity.Turn{{ID: 11}}}},
		gptr.Of(int64(1)), nil, nil, nil)
	refs.EXPECT().BatchCreate(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, values []*entity.ExptItemRef) error {
			require.Len(t, values, 1)
			converter := store.NewExptItemRefConvertor()
			saved := converter.PO2DO(converter.DO2PO(values[0]))
			require.Nil(t, saved.ItemConfig.EvalTargetConf.RunConf)
			return nil
		})
	expt := &entity.Experiment{
		ID: 1, SpaceID: 3, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf: &entity.EvaluationConfiguration{
			RunModeConfig: &entity.RunModeConfig{MaxRunMinutes: 0},
			EvalSetConfigs: []*entity.EvalSetConfig{{
				EvalSetID: 10, EvalSetVersionID: 100,
				TargetConfs: []*entity.ExptTargetConf{{TargetVersionID: 1}},
			}},
		},
	}
	require.NoError(t, exec.ExptStart(context.Background(), &entity.ExptScheduleEvent{
		ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "u1"},
	}, expt))
}
