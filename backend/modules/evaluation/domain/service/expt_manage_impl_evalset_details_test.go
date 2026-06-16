// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// fillEvalSetDetails 单测:
// - SingleSet (老实验) 跳过, 不调 repo
// - MultiSetConfig 实验: 按 EvalConf.EvalSetConfigs 拼骨架, IsPrimary 与 EvalSetID 一致, ItemCount 来源 repo, 缺失补 0
// - exptItemRefRepo nil 时直接返回 nil 不阻断
// - repo 错误时返回 error (caller 在 packExperimentResult 里只 warn 不阻断, 这层语义已经测试覆盖)
func TestExptMangerImpl_fillEvalSetDetails(t *testing.T) {
	t.Run("nil exptItemRefRepo returns nil", func(t *testing.T) {
		impl := &ExptMangerImpl{exptItemRefRepo: nil}
		err := impl.fillEvalSetDetails(context.Background(), []*entity.Experiment{
			{ID: 1, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig},
		}, 100)
		assert.NoError(t, err)
	})

	t.Run("only MultiSetConfig expts trigger repo call", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockRepo := repomocks.NewMockIExptItemRefRepo(ctrl)
		// 期望只有 MultiSetConfig expt id=2 进 repo, expt id=1 (SingleSet) 不进
		mockRepo.EXPECT().CountByEvalSetGrouped(gomock.Any(), int64(100), []int64{2}).
			Return(map[int64][]*entity.ExptEvalSetItemCount{
				2: {{ExptID: 2, EvalSetID: 10, EvalSetVersionID: 100, ItemCount: 4}},
			}, nil)

		impl := &ExptMangerImpl{exptItemRefRepo: mockRepo}
		expts := []*entity.Experiment{
			{ID: 1, EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet},
			{
				ID:                2,
				EvalSetID:         10,
				EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
				EvalConf: &entity.EvaluationConfiguration{
					EvalSetConfigs: []*entity.EvalSetConfig{
						{EvalSetID: 10, EvalSetVersionID: 100},
						{EvalSetID: 20, EvalSetVersionID: 200},
					},
				},
			},
		}
		err := impl.fillEvalSetDetails(context.Background(), expts, 100)
		assert.NoError(t, err)
		// SingleSet 不填
		assert.Nil(t, expts[0].EvalSetDetails)
		// MultiSetConfig 填两 set
		assert.Len(t, expts[1].EvalSetDetails, 2)
		// 第一 set: IsPrimary=true (与 EvalSetID 一致), ItemCount=4
		assert.Equal(t, int64(10), expts[1].EvalSetDetails[0].EvalSetID)
		assert.True(t, expts[1].EvalSetDetails[0].IsPrimary)
		assert.Equal(t, int32(4), expts[1].EvalSetDetails[0].ItemCount)
		// 第二 set: IsPrimary=false, 没有 count 数据 → ItemCount=0 默认值
		assert.Equal(t, int64(20), expts[1].EvalSetDetails[1].EvalSetID)
		assert.False(t, expts[1].EvalSetDetails[1].IsPrimary)
		assert.Equal(t, int32(0), expts[1].EvalSetDetails[1].ItemCount)
	})

	t.Run("repo error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockRepo := repomocks.NewMockIExptItemRefRepo(ctrl)
		mockRepo.EXPECT().CountByEvalSetGrouped(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("db down"))

		impl := &ExptMangerImpl{exptItemRefRepo: mockRepo}
		expts := []*entity.Experiment{
			{
				ID: 1, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
				EvalConf: &entity.EvaluationConfiguration{
					EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 100}},
				},
			},
		}
		err := impl.fillEvalSetDetails(context.Background(), expts, 100)
		assert.Error(t, err)
	})

	t.Run("MultiSetConfig without EvalConf.EvalSetConfigs is skipped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockRepo := repomocks.NewMockIExptItemRefRepo(ctrl)
		// 即便是 MultiSetConfig, 没 EvalConf 也只触发 repo 拿 count(空map) 但不写 details
		mockRepo.EXPECT().CountByEvalSetGrouped(gomock.Any(), int64(100), []int64{1}).
			Return(map[int64][]*entity.ExptEvalSetItemCount{}, nil)

		impl := &ExptMangerImpl{exptItemRefRepo: mockRepo}
		expts := []*entity.Experiment{
			{ID: 1, EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalConf: nil},
		}
		err := impl.fillEvalSetDetails(context.Background(), expts, 100)
		assert.NoError(t, err)
		assert.Nil(t, expts[0].EvalSetDetails)
	})

	t.Run("no MultiSetConfig expts skips repo call entirely", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockRepo := repomocks.NewMockIExptItemRefRepo(ctrl)
		// 没有 MultiSetConfig 时不该调 repo
		// mockRepo.EXPECT() 不写, gomock 会自动断言无意外调用

		impl := &ExptMangerImpl{exptItemRefRepo: mockRepo}
		expts := []*entity.Experiment{
			{ID: 1, EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet},
			{ID: 2, EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet},
		}
		err := impl.fillEvalSetDetails(context.Background(), expts, 100)
		assert.NoError(t, err)
		assert.Nil(t, expts[0].EvalSetDetails)
		assert.Nil(t, expts[1].EvalSetDetails)
	})
}
