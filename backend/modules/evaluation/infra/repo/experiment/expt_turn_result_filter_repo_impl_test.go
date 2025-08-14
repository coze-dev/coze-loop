// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
	ckmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/mocks"
	diffmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/model"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestExptTurnResultFilterRepoImpl_Save(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultFilterDAO := ckmocks.NewMockIExptTurnResultFilterDAO(ctrl)
	mockExptTurnResultFilterKeyMappingDAO := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	repo := NewExptTurnResultFilterRepo(mockExptTurnResultFilterDAO, mockExptTurnResultFilterKeyMappingDAO)

	filterEntities := []*entity.ExptTurnResultFilterEntity{
		{SpaceID: 1, ExptID: 100},
		{SpaceID: 2, ExptID: 200},
	}

	tests := []struct {
		name      string
		mockSetup func()
		input     []*entity.ExptTurnResultFilterEntity
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				models := make([]*model.ExptTurnResultFilter, 0, len(filterEntities))
				for _, filterEntity := range filterEntities {
					models = append(models, convertor.ExptTurnResultFilterEntity2PO(filterEntity))
				}
				mockExptTurnResultFilterDAO.EXPECT().Save(gomock.Any(), models).Return(nil)
			},
			input:   filterEntities,
			wantErr: false,
		},
		{
			name: "fail_dao_save",
			mockSetup: func() {
				models := make([]*model.ExptTurnResultFilter, 0, len(filterEntities))
				for _, filterEntity := range filterEntities {
					models = append(models, convertor.ExptTurnResultFilterEntity2PO(filterEntity))
				}
				mockExptTurnResultFilterDAO.EXPECT().Save(gomock.Any(), models).Return(assert.AnError)
			},
			input:   filterEntities,
			wantErr: true,
		},
		{
			name: "empty_input",
			mockSetup: func() {
				mockExptTurnResultFilterDAO.EXPECT().Save(gomock.Any(), []*model.ExptTurnResultFilter{}).Return(nil)
			},
			input:   []*entity.ExptTurnResultFilterEntity{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.Save(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultFilterRepoImpl_QueryItemIDStates(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultFilterDAO := ckmocks.NewMockIExptTurnResultFilterDAO(ctrl)
	mockExptTurnResultFilterKeyMappingDAO := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	repo := NewExptTurnResultFilterRepo(mockExptTurnResultFilterDAO, mockExptTurnResultFilterKeyMappingDAO)

	// 准备一个示例 filter，实际使用时请根据需求修改
	exampleFilter := &entity.ExptTurnResultFilterAccelerator{
		SpaceID:     1,
		ExptID:      1,
		CreatedDate: time.Time{},
		EvaluatorScoreCorrected: &entity.FieldFilter{
			Key:    "key1",
			Op:     "=",
			Values: []any{"1"},
		},
		ItemIDs: []*entity.FieldFilter{
			{
				Key:    "item_id",
				Op:     "=",
				Values: []any{"2"},
			},
		},
		ItemRunStatus: []*entity.FieldFilter{
			{
				Key:    "item_status",
				Op:     "=",
				Values: []any{"1"},
			},
		},
		TurnRunStatus: nil,
		MapCond: &entity.ExptTurnResultFilterMapCond{
			EvalTargetDataFilters: []*entity.FieldFilter{
				{
					Key:    "actual_output",
					Op:     "=",
					Values: []any{"1"},
				},
			},
			EvaluatorScoreFilters: []*entity.FieldFilter{
				{
					Key:    "key1",
					Op:     "=",
					Values: []any{"1"},
				},
			},
		},
		ItemSnapshotCond: &entity.ItemSnapshotFilter{
			BoolMapFilters: []*entity.FieldFilter{
				{
					Key:    "key1",
					Op:     "=",
					Values: []any{"1"},
				},
			},
			FloatMapFilters: []*entity.FieldFilter{
				{
					Key:    "key2",
					Op:     "=",
					Values: []any{"1"},
				},
			},
			IntMapFilters: []*entity.FieldFilter{
				{
					Key:    "key3",
					Op:     "=",
					Values: []any{"1"},
				},
			},
			StringMapFilters: []*entity.FieldFilter{
				{
					Key:    "key4",
					Op:     "=",
					Values: []any{"1"},
				},
			},
		},
		KeywordSearch: &entity.KeywordFilter{
			Keyword: ptr.Of("1"),
			ItemSnapshotFilter: &entity.ItemSnapshotFilter{
				StringMapFilters: []*entity.FieldFilter{
					{
						Key:    "key4",
						Op:     "=",
						Values: []any{"1"},
					},
				},
			},
		},
		EvalSetSyncCkDate: "",
	}

	tests := []struct {
		name      string
		mockSetup func()
		input     *entity.ExptTurnResultFilterAccelerator
		wantIDs   []int64
		wantCount int64
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptTurnResultFilterDAO.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).Return(map[string]int32{"1": 1}, int64(1), nil)
			},
			input:     exampleFilter,
			wantIDs:   []int64{1},
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			gotIDs, gotCount, err := repo.QueryItemIDStates(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.wantIDs), len(gotIDs))
				assert.Equal(t, tt.wantCount, gotCount)
			}
		})
	}
}

func TestExptTurnResultFilterRepoImpl_GetExptTurnResultFilterKeyMappings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultFilterDAO := ckmocks.NewMockIExptTurnResultFilterDAO(ctrl)
	mockExptTurnResultFilterKeyMappingDAO := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	repo := NewExptTurnResultFilterRepo(mockExptTurnResultFilterDAO, mockExptTurnResultFilterKeyMappingDAO)

	// 假设输入参数和返回值类型，根据实际情况修改
	mockSpaceID := int64(1)
	mockExptID := int64(1)
	mockResult := []*mysqlmodel.ExptTurnResultFilterKeyMapping{{}}

	tests := []struct {
		name      string
		mockSetup func()
		input     string
		want      []*mysqlmodel.ExptTurnResultFilterKeyMapping
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptTurnResultFilterKeyMappingDAO.EXPECT().GetByExptID(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockResult, nil)
			},
			want:    mockResult,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetExptTurnResultFilterKeyMappings(context.Background(), mockSpaceID, mockExptID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.want), len(got))
			}
		})
	}
}

// InsertExptTurnResultFilterKeyMappings 单测
func TestExptTurnResultFilterRepoImpl_InsertExptTurnResultFilterKeyMappings(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultFilterDAO := ckmocks.NewMockIExptTurnResultFilterDAO(ctrl)
	mockExptTurnResultFilterKeyMappingDAO := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	repo := NewExptTurnResultFilterRepo(mockExptTurnResultFilterDAO, mockExptTurnResultFilterKeyMappingDAO)

	// 假设输入参数，根据实际情况修改
	inputEntities := []*entity.ExptTurnResultFilterKeyMapping{{}}

	tests := []struct {
		name      string
		mockSetup func()
		input     []*entity.ExptTurnResultFilterKeyMapping
		wantErr   bool
	}{
		{
			name: "success",
			mockSetup: func() {
				mockExptTurnResultFilterKeyMappingDAO.EXPECT().Insert(gomock.Any(), gomock.Any()).Return(nil)
			},
			input:   inputEntities,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.InsertExptTurnResultFilterKeyMappings(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptTurnResultFilterRepoImpl_GetByExptIDItemIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptTurnResultFilterDAO := ckmocks.NewMockIExptTurnResultFilterDAO(ctrl)
	mockExptTurnResultFilterKeyMappingDAO := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	repo := NewExptTurnResultFilterRepo(mockExptTurnResultFilterDAO, mockExptTurnResultFilterKeyMappingDAO)

	ctx := context.Background()
	spaceIDStr := "1"
	exptIDStr := "1"
	spaceIDInt := int64(1)
	exptIDInt := int64(1)
	createdDate := "2023-01-01"

	tests := []struct {
		name      string
		itemIDs   []string
		mockSetup func()
		want      []*entity.ExptTurnResultFilterEntity
		wantErr   bool
	}{
		{
			name:    "success",
			itemIDs: []string{"1", "2"},
			mockSetup: func() {
				models := []*diffmodel.ExptTurnResultFilter{
					{
						SpaceID:                 spaceIDStr,
						ExptID:                  exptIDStr,
						ItemID:                  "1",
						ItemIdx:                 0,
						TurnID:                  "1",
						Status:                  0,
						ActualOutput:            ptr.Of("1"),
						EvaluatorScoreKey1:      ptr.Of(float64(1)),
						EvaluatorScoreKey2:      ptr.Of(float64(1)),
						EvaluatorScoreKey3:      ptr.Of(float64(1)),
						EvaluatorScoreKey4:      ptr.Of(float64(1)),
						EvaluatorScoreKey5:      ptr.Of(float64(1)),
						EvaluatorScoreKey6:      ptr.Of(float64(1)),
						EvaluatorScoreKey7:      ptr.Of(float64(1)),
						EvaluatorScoreKey8:      ptr.Of(float64(1)),
						EvaluatorScoreKey9:      ptr.Of(float64(1)),
						EvaluatorScoreKey10:     ptr.Of(float64(1)),
						EvaluatorScoreCorrected: 0,
						EvalSetVersionID:        "1",
						CreatedDate:             time.Time{},
					},
				}

				mockExptTurnResultFilterDAO.EXPECT().GetByExptIDItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(models, nil)
			},
			want: []*entity.ExptTurnResultFilterEntity{
				{
					SpaceID:                 spaceIDInt,
					ExptID:                  exptIDInt,
					ItemID:                  int64(1),
					ItemIdx:                 0,
					TurnID:                  1,
					Status:                  0,
					EvalTargetData:          nil,
					EvaluatorScore:          nil,
					CreatedDate:             time.Time{},
					EvaluatorScoreCorrected: false,
					EvalSetVersionID:        0,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := repo.GetByExptIDItemIDs(ctx, spaceIDStr, exptIDStr, createdDate, tt.itemIDs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.want), len(got))
				assert.Equal(t, tt.want[0].ItemID, got[0].ItemID)
				assert.Equal(t, tt.want[0].TurnID, got[0].TurnID)

			}
		})
	}
}
