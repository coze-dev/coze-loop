// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/gorm_gen/model"
	ckmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/mocks"
	diffmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck/model"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
				// 使用 Do 方法来验证参数，忽略 UpdatedAt 字段的差异
				mockExptTurnResultFilterDAO.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, models []*model.ExptTurnResultFilter) error {
						assert.Equal(t, 2, len(models))
						assert.Equal(t, "1", models[0].SpaceID)
						assert.Equal(t, "100", models[0].ExptID)
						assert.Equal(t, "2", models[1].SpaceID)
						assert.Equal(t, "200", models[1].ExptID)
						return nil
					},
				)
			},
			input:   filterEntities,
			wantErr: false,
		},
		{
			name: "fail_dao_save",
			mockSetup: func() {
				mockExptTurnResultFilterDAO.EXPECT().Save(gomock.Any(), gomock.Any()).Return(assert.AnError)
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

// itemSnapshotCondsEntityToCK：多评测集 per-set 条件透传到 DAO 层的转换。
func TestItemSnapshotCondsEntityToCK(t *testing.T) {
	t.Run("空输入返回 nil", func(t *testing.T) {
		assert.Nil(t, itemSnapshotCondsEntityToCK(nil))
		assert.Nil(t, itemSnapshotCondsEntityToCK([]*entity.ItemSnapshotVersionCond{}))
	})

	t.Run("跳过 nil 项与 Cond 为 nil 的项", func(t *testing.T) {
		got := itemSnapshotCondsEntityToCK([]*entity.ItemSnapshotVersionCond{
			nil,
			{EvalSetID: 1, EvalSetVersionID: 2},
		})
		assert.Empty(t, got)
	})

	t.Run("id 转字符串，四类 map 条件全部透传", func(t *testing.T) {
		got := itemSnapshotCondsEntityToCK([]*entity.ItemSnapshotVersionCond{
			{
				EvalSetID: 10, EvalSetVersionID: 100, SyncCkDate: "2026-08-30",
				Cond: &entity.ItemSnapshotFilter{
					StringMapFilters: []*entity.FieldFilter{{Key: "string_key_0", Op: "LIKE", Values: []any{"v"}}},
					IntMapFilters:    []*entity.FieldFilter{{Key: "int_key_0", Op: "=", Values: []any{1}}},
					FloatMapFilters:  []*entity.FieldFilter{{Key: "float_key_0", Op: ">", Values: []any{1.5}}},
					BoolMapFilters:   []*entity.FieldFilter{{Key: "bool_key_0", Op: "=", Values: []any{true}}},
				},
			},
			{
				EvalSetID: 20, EvalSetVersionID: 200, SyncCkDate: "2026-08-31",
				Cond: &entity.ItemSnapshotFilter{
					StringMapFilters: []*entity.FieldFilter{{Key: "string_key_3", Op: "=", Values: []any{"w"}}},
				},
			},
		})
		require.Len(t, got, 2)

		assert.Equal(t, "10", got[0].EvalSetID)
		assert.Equal(t, "100", got[0].EvalSetVersionID)
		assert.Equal(t, "2026-08-30", got[0].SyncCkDate)
		require.NotNil(t, got[0].Cond)
		assert.Equal(t, "string_key_0", got[0].Cond.StringMapFilters[0].Key)
		assert.Equal(t, "int_key_0", got[0].Cond.IntMapFilters[0].Key)
		assert.Equal(t, "float_key_0", got[0].Cond.FloatMapFilters[0].Key)
		assert.Equal(t, "bool_key_0", got[0].Cond.BoolMapFilters[0].Key)

		assert.Equal(t, "20", got[1].EvalSetID)
		assert.Equal(t, "200", got[1].EvalSetVersionID)
		assert.Equal(t, "string_key_3", got[1].Cond.StringMapFilters[0].Key)
	})
}

// QueryItemIDStates 透传 per-set 条件与 unmatched 标记到 DAO 层。
func TestExptTurnResultFilterRepoImpl_QueryItemIDStates_PerSetPassthrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dao := ckmocks.NewMockIExptTurnResultFilterDAO(ctrl)
	r := NewExptTurnResultFilterRepo(dao, mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl))

	dao.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cond *ck.ExptTurnResultFilterQueryCond) (map[string]int32, int64, error) {
			require.Len(t, cond.ItemSnapshotCondBySet, 2)
			assert.Equal(t, "100", cond.ItemSnapshotCondBySet[0].EvalSetVersionID)
			assert.Equal(t, "200", cond.ItemSnapshotCondBySet[1].EvalSetVersionID)
			assert.True(t, cond.ItemSnapshotCondUnmatched)
			return map[string]int32{"1": 2}, 1, nil
		}).Times(1)

	states, total, err := r.QueryItemIDStates(context.Background(), &entity.ExptTurnResultFilterAccelerator{
		SpaceID: 1, ExptID: 2,
		ItemSnapshotCondBySet: []*entity.ItemSnapshotVersionCond{
			{EvalSetID: 10, EvalSetVersionID: 100, SyncCkDate: "d1", Cond: &entity.ItemSnapshotFilter{
				StringMapFilters: []*entity.FieldFilter{{Key: "string_key_0", Op: "=", Values: []any{"v"}}},
			}},
			{EvalSetID: 20, EvalSetVersionID: 200, SyncCkDate: "d2", Cond: &entity.ItemSnapshotFilter{
				StringMapFilters: []*entity.FieldFilter{{Key: "string_key_3", Op: "=", Values: []any{"v"}}},
			}},
		},
		ItemSnapshotCondUnmatched: true,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, map[int64]entity.ItemRunState{1: entity.ItemRunState(2)}, states)
}
