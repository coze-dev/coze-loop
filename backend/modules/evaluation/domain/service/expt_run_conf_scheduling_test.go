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

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// 本文件覆盖 UpdateRunConf 里新增的两个中心调度参数。
//
// 三件事必须钉住，都是"接口返回成功但实际没改对"这一类问题：
//  1. priority_level 走显式列名写入，且**不会**顺带碰到 scheduler_mode / scheduler_scope
//     （那两列被冻结的理由见 mysql/expt.go 的 schedulingFrozenColumns）
//  2. 向量走 eval_conf 的 read-modify-write，不能覆盖掉 EvalConf 的其它字段
//  3. 改了向量就必须给在飞预占改价，且**先写库再改账本**、改价失败要上抛

const (
	runConfExptID  = int64(7676064108126400001)
	runConfSpaceID = int64(7533128632407949313)
	runConfRunID   = int64(7676064108126404609)
)

func enforceExptForRunConf() *entity.Experiment {
	return &entity.Experiment{
		ID:               runConfExptID,
		SpaceID:          runConfSpaceID,
		Status:           entity.ExptStatus_Processing,
		LatestRunID:      runConfRunID,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
		SchedulerScope:   "fornax_cn",
		EvalConf: &entity.EvaluationConfiguration{
			ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: 999}},
			ItemConcurNum: gptr.Of(3),
			Ext:           map[string]string{"k": "v"},
			ExpectedQuotaConsumption: &entity.ExpectedQuotaConsumption{Resources: []*entity.ExpectedResourceConsumption{
				{Category: "sandbox", ResourceKey: "default", Amount: 10},
			}},
		},
	}
}

func newVector() *entity.ExpectedQuotaConsumption {
	return &entity.ExpectedQuotaConsumption{Resources: []*entity.ExpectedResourceConsumption{
		{Category: "sandbox", ResourceKey: "default", Amount: 25},
	}}
}

func TestUpdateRunConf_PriorityGoesToItsOwnColumn(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mgr := &ExptMangerImpl{exptRepo: mockRepo}

	mockRepo.EXPECT().GetByID(gomock.Any(), runConfExptID, runConfSpaceID).Return(enforceExptForRunConf(), nil)
	mockRepo.EXPECT().UpdateFields(gomock.Any(), runConfExptID, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, ufields map[string]any) error {
			assert.Equal(t, int32(80), ufields["priority_level"])
			// 冻结列一个都不能出现在 map 里：出现即意味着这条路径也能把 enforce 打回 legacy，
			// 那会让旧 daemon 恢复自主派发、两个派发驱动同时绕过账本。
			_, hasMode := ufields["scheduler_mode"]
			_, hasScope := ufields["scheduler_scope"]
			assert.False(t, hasMode, "scheduler_mode 不得被这条路径写入")
			assert.False(t, hasScope, "scheduler_scope 不得被这条路径写入")
			return nil
		})

	require.NoError(t, mgr.UpdateRunConf(context.Background(), &entity.UpdateRunConfParam{
		ExptID:        runConfExptID,
		SpaceID:       runConfSpaceID,
		PriorityLevel: gptr.Of(int32(80)),
	}))
}

func TestUpdateRunConf_VectorIsRMWAndTriggersReprice(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	guard := &fakeGuard{}
	mgr := &ExptMangerImpl{exptRepo: mockRepo, centralGuard: guard}

	mockRepo.EXPECT().GetByID(gomock.Any(), runConfExptID, runConfSpaceID).Return(enforceExptForRunConf(), nil)
	mockRepo.EXPECT().UpdateFields(gomock.Any(), runConfExptID, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, ufields map[string]any) error {
			raw, ok := ufields["eval_conf"].(*[]byte)
			require.True(t, ok)
			var got entity.EvaluationConfiguration
			require.NoError(t, json.Unmarshal(*raw, &got))
			require.NotNil(t, got.ExpectedQuotaConsumption)
			require.Len(t, got.ExpectedQuotaConsumption.Resources, 1)
			assert.Equal(t, int64(25), got.ExpectedQuotaConsumption.Resources[0].Amount)
			// RMW 红线：向量是 EvalConf 的一个字段，用裸结构覆盖会清空同列的其它配置。
			assert.Equal(t, int64(999), got.ConnectorConf.TargetConf.TargetVersionID)
			assert.Equal(t, "v", got.Ext["k"])
			assert.Equal(t, 3, gptr.Indirect(got.ItemConcurNum))
			return nil
		})

	require.NoError(t, mgr.UpdateRunConf(context.Background(), &entity.UpdateRunConfParam{
		ExptID:                   runConfExptID,
		SpaceID:                  runConfSpaceID,
		ExpectedQuotaConsumption: newVector(),
	}))

	// 改价必须发生，且带的是**新**向量与该实验的 LatestRunID。
	// 只断言"调过一次"会放过"调了但传的是旧向量"——那种错误完全静默。
	calls := guard.reprices()
	require.Len(t, calls, 1)
	assert.Equal(t, "fornax_cn", calls[0].Scope)
	assert.Equal(t, runConfExptID, calls[0].ExptID)
	assert.Equal(t, runConfRunID, calls[0].RunID)
	require.NotNil(t, calls[0].Consumption)
	assert.Equal(t, int64(25), calls[0].Consumption.Resources[0].Amount)
}

func TestUpdateRunConf_RepriceFailureIsPropagated(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	guard := &fakeGuard{repriceErr: errors.New("ledger down")}
	mgr := &ExptMangerImpl{exptRepo: mockRepo, centralGuard: guard}

	mockRepo.EXPECT().GetByID(gomock.Any(), runConfExptID, runConfSpaceID).Return(enforceExptForRunConf(), nil)
	mockRepo.EXPECT().UpdateFields(gomock.Any(), runConfExptID, gomock.Any()).Return(nil)

	// 向量已落库、账本还是旧价。返回成功会让调用方以为改好了，而额度会长期算错 ——
	// 这与沙箱名额同步（失败只告警）不同，那里的后果只是暂时吃不满。
	err := mgr.UpdateRunConf(context.Background(), &entity.UpdateRunConfParam{
		ExptID:                   runConfExptID,
		SpaceID:                  runConfSpaceID,
		ExpectedQuotaConsumption: newVector(),
	})
	require.Error(t, err)
}

func TestUpdateRunConf_NoRepriceWhenVectorUntouched(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	guard := &fakeGuard{}
	mgr := &ExptMangerImpl{exptRepo: mockRepo, centralGuard: guard}

	mockRepo.EXPECT().GetByID(gomock.Any(), runConfExptID, runConfSpaceID).Return(enforceExptForRunConf(), nil)
	mockRepo.EXPECT().UpdateFields(gomock.Any(), runConfExptID, gomock.Any()).Return(nil)

	// 只改并发度不该碰账本：无谓的改价会把在飞预占重写一遍，多一次可失败的写。
	require.NoError(t, mgr.UpdateRunConf(context.Background(), &entity.UpdateRunConfParam{
		ExptID:        runConfExptID,
		SpaceID:       runConfSpaceID,
		ItemConcurNum: gptr.Of(10),
	}))
	assert.Empty(t, guard.reprices())
}

func TestUpdateRunConf_RejectsSchedulingParamsOnLegacyExpt(t *testing.T) {
	for name, param := range map[string]*entity.UpdateRunConfParam{
		"priority": {ExptID: runConfExptID, SpaceID: runConfSpaceID, PriorityLevel: gptr.Of(int32(80))},
		"quota":    {ExptID: runConfExptID, SpaceID: runConfSpaceID, ExpectedQuotaConsumption: newVector()},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			guard := &fakeGuard{}
			mgr := &ExptMangerImpl{exptRepo: mockRepo, centralGuard: guard}

			legacy := enforceExptForRunConf()
			legacy.ExptDispatchMode = entity.ExptDispatchModeLegacy
			legacy.SchedulerScope = ""
			mockRepo.EXPECT().GetByID(gomock.Any(), runConfExptID, runConfSpaceID).Return(legacy, nil)
			// 一次写都不该发生：legacy 实验既不参与优先级排序也没有账本，
			// 照写只会得到一个没人读的值，而调用方收到成功。
			mockRepo.EXPECT().UpdateFields(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

			require.Error(t, mgr.UpdateRunConf(context.Background(), param))
			assert.Empty(t, guard.reprices())
		})
	}
}
