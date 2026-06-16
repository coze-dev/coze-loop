// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/assert"
)

func newMultiSetExpt(id, primarySetID int64) *entity.Experiment {
	return &entity.Experiment{
		ID:                id,
		EvalSetID:         primarySetID,
		EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig,
		EvalConf: &entity.EvaluationConfiguration{
			EvalSetConfigs: []*entity.EvalSetConfig{
				{EvalSetID: 10, EvalSetVersionID: 110},
				{EvalSetID: 20, EvalSetVersionID: 220}, // primary when primarySetID==20
			},
		},
	}
}

// TestEnrichEvalSetDetails_ListPath List 路径 (withSetDetail=false): 填 id/count + total, 不拉 EvaluationSet 详情。
func TestEnrichEvalSetDetails_ListPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
	mgr := &ExptMangerImpl{itemRefRepo: mockRefRepo}

	expt := newMultiSetExpt(1001, 20)
	spaceID := int64(7)

	mockRefRepo.EXPECT().CountByEvalSetGrouped(gomock.Any(), spaceID, []int64{1001}).
		Return(map[int64][]*entity.ExptEvalSetItemCount{
			1001: {
				{ExptID: 1001, EvalSetID: 10, EvalSetVersionID: 110, ItemCount: 12},
				{ExptID: 1001, EvalSetID: 20, EvalSetVersionID: 220, ItemCount: 30},
			},
		}, nil)

	err := mgr.enrichEvalSetDetails(context.Background(), []*entity.Experiment{expt}, spaceID, false, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), expt.TotalItemCount)
	assert.Len(t, expt.EvalSetDetails, 2)
	assert.Equal(t, int32(12), expt.EvalSetDetails[0].ItemCount)
	assert.False(t, expt.EvalSetDetails[0].IsPrimary)
	assert.True(t, expt.EvalSetDetails[1].IsPrimary) // set 20 == primary
	assert.Nil(t, expt.EvalSetDetails[0].EvalSet)    // List 不拉详情
}

// TestEnrichEvalSetDetails_GetPath Get 路径 (withSetDetail=true): 额外按版本批拉 EvaluationSet 详情。
func TestEnrichEvalSetDetails_GetPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
	mockSetVerSvc := svcMocks.NewMockEvaluationSetVersionService(ctrl)
	mgr := &ExptMangerImpl{
		itemRefRepo:                 mockRefRepo,
		evaluationSetVersionService: mockSetVerSvc,
	}

	expt := newMultiSetExpt(1002, 20)
	spaceID := int64(7)

	mockRefRepo.EXPECT().CountByEvalSetGrouped(gomock.Any(), spaceID, []int64{1002}).
		Return(map[int64][]*entity.ExptEvalSetItemCount{}, nil) // 首跑前无行

	mockSetVerSvc.EXPECT().
		BatchGetEvaluationSetVersions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.BatchGetEvaluationSetVersionsResult{
			{Version: &entity.EvaluationSetVersion{ID: 110}, EvaluationSet: &entity.EvaluationSet{ID: 10, Name: "set-10"}},
			{Version: &entity.EvaluationSetVersion{ID: 220}, EvaluationSet: &entity.EvaluationSet{ID: 20, Name: "set-20"}},
		}, nil)

	err := mgr.enrichEvalSetDetails(context.Background(), []*entity.Experiment{expt}, spaceID, true, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), expt.TotalItemCount) // 首跑前
	assert.Len(t, expt.EvalSetDetails, 2)
	assert.Equal(t, int32(0), expt.EvalSetDetails[0].ItemCount)
	assert.NotNil(t, expt.EvalSetDetails[0].EvalSet)
	assert.Equal(t, "set-10", expt.EvalSetDetails[0].EvalSet.Name)
	assert.NotNil(t, expt.EvalSetDetails[1].EvalSet)
	assert.Equal(t, "set-20", expt.EvalSetDetails[1].EvalSet.Name)
}

// TestEnrichEvalSetDetails_SingleSetSkipped 老实验 (SingleSet) 跳过, 不查 ref repo, 不填新字段。
func TestEnrichEvalSetDetails_SingleSetSkipped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl) // 不应被调用
	mgr := &ExptMangerImpl{itemRefRepo: mockRefRepo}

	expt := &entity.Experiment{
		ID:                1003,
		EvalSetSourceType: entity.ExptEvalSetSourceType_SingleSet,
		EvalSetID:         7,
	}

	err := mgr.enrichEvalSetDetails(context.Background(), []*entity.Experiment{expt}, 7, true, nil)
	assert.NoError(t, err)
	assert.Empty(t, expt.EvalSetDetails)
	assert.Equal(t, int64(0), expt.TotalItemCount)
}

// TestEnrichEvalSetDetails_CountError 计数失败时不阻断主读路径, 降级为未填充 item_count。
func TestEnrichEvalSetDetails_CountError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRefRepo := repoMocks.NewMockIExptItemRefRepo(ctrl)
	mgr := &ExptMangerImpl{itemRefRepo: mockRefRepo}

	expt := newMultiSetExpt(1004, 10)
	mockRefRepo.EXPECT().CountByEvalSetGrouped(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	err := mgr.enrichEvalSetDetails(context.Background(), []*entity.Experiment{expt}, 7, false, nil)
	assert.NoError(t, err) // 计数失败不报错
	assert.Len(t, expt.EvalSetDetails, 2)
	assert.Equal(t, int32(0), expt.EvalSetDetails[0].ItemCount)
	assert.Equal(t, int64(0), expt.TotalItemCount)
}

// TestEnrichEvalSetDetails_NilRepo itemRefRepo 为 nil 时不 panic、不阻断 (计数缺省, 仍按 set 列出骨架)。
func TestEnrichEvalSetDetails_NilRepo(t *testing.T) {
	mgr := &ExptMangerImpl{itemRefRepo: nil}
	expt := newMultiSetExpt(1005, 10)
	err := mgr.enrichEvalSetDetails(context.Background(), []*entity.Experiment{expt}, 7, false, nil)
	assert.NoError(t, err)
	assert.Len(t, expt.EvalSetDetails, 2)
	assert.Equal(t, int32(0), expt.EvalSetDetails[0].ItemCount)
}
