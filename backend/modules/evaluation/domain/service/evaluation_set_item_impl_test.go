// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestBatchCreateEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建模拟的 DatasetRPCAdapter
	mockAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)

	// 创建 EvaluationSetItemServiceImpl 实例
	service := &EvaluationSetItemServiceImpl{
		datasetRPCAdapter: mockAdapter,
	}

	// 定义测试用例
	testCases := []struct {
		name           string
		param          *entity.BatchCreateEvaluationSetItemsParam
		expectedIDMap  map[int64]int64
		expectedErrors []*entity.ItemErrorGroup
		expectedErr    error
		mockSetup      func()
	}{
		{
			name:  "成功批量创建评估集项",
			param: &entity.BatchCreateEvaluationSetItemsParam{
				// 填充实际的参数值
			},
			expectedIDMap:  map[int64]int64{1: 100, 2: 200},
			expectedErrors: nil,
			expectedErr:    nil,
			mockSetup: func() {
				mockAdapter.EXPECT().BatchCreateDatasetItems(gomock.Any(), gomock.Any()).Return(map[int64]int64{1: 100, 2: 200}, nil, nil, nil)
			},
		},
		{
			name:           "参数为空",
			param:          nil,
			expectedIDMap:  nil,
			expectedErrors: nil,
			expectedErr:    errorx.NewByCode(errno.CommonInternalErrorCode),
			mockSetup:      func() {},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()

			idMap, errors, _, err := service.BatchCreateEvaluationSetItems(context.Background(), tc.param)

			if !equalIDMaps(idMap, tc.expectedIDMap) {
				t.Errorf("期望 IDMap 为 %v, 但得到 %v", tc.expectedIDMap, idMap)
			}

			if !equalErrorGroups(errors, tc.expectedErrors) {
				t.Errorf("期望 Errors 为 %v, 但得到 %v", tc.expectedErrors, errors)
			}

			if (err == nil && tc.expectedErr != nil) || (err != nil && tc.expectedErr == nil) {
				t.Errorf("期望错误为 %v, 但得到 %v", tc.expectedErr, err)
			}
		})
	}
}

func TestEvaluationSetItemServiceImpl_UpdateEvaluationSetItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetItemServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name      string
		spaceID   int64
		setID     int64
		itemID    int64
		turns     []*entity.Turn
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "成功更新评估集项目",
			spaceID: 1,
			setID:   100,
			itemID:  1000,
			turns: []*entity.Turn{
				{
					ID: int64(1),
					FieldDataList: []*entity.FieldData{
						{
							Key:     "field1",
							Name:    "Field 1",
							Content: &entity.Content{Text: gptr.Of("test content")},
						},
					},
				},
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					UpdateDatasetItem(gomock.Any(), int64(1), int64(100), int64(1000), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "更新失败 - RPC错误",
			spaceID: 1,
			setID:   100,
			itemID:  1000,
			turns:   []*entity.Turn{},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					UpdateDatasetItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := service.UpdateEvaluationSetItem(context.Background(), tt.spaceID, tt.setID, tt.itemID, tt.turns)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluationSetItemServiceImpl_BatchDeleteEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetItemServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetItemServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name      string
		spaceID   int64
		setID     int64
		itemIDs   []int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "成功批量删除",
			spaceID: 1,
			setID:   100,
			itemIDs: []int64{1, 2, 3},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchDeleteDatasetItems(gomock.Any(), int64(1), int64(100), []int64{1, 2, 3}).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "删除失败 - 空ID列表",
			spaceID: 1,
			setID:   100,
			itemIDs: []int64{},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchDeleteDatasetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInvalidParamCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := service.BatchDeleteEvaluationSetItems(context.Background(), tt.spaceID, tt.setID, tt.itemIDs)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluationSetItemServiceImpl_ListEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetItemServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetItemServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name          string
		param         *entity.ListEvaluationSetItemsParam
		mockSetup     func()
		wantItems     []*entity.EvaluationSetItem
		wantTotal     *int64
		wantNextToken *string
		wantErr       bool
	}{
		{
			name: "成功列出项目 - 无版本ID",
			param: &entity.ListEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				PageSize:        gptr.Of[int32](10),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ListDatasetItems(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSetItem{
						{ID: 1, ItemKey: "item1"},
					}, gptr.Of[int64](1), gptr.Of("next_token"), nil)
			},
			wantItems: []*entity.EvaluationSetItem{
				{ID: 1, ItemKey: "item1"},
			},
			wantTotal:     gptr.Of[int64](1),
			wantNextToken: gptr.Of("next_token"),
			wantErr:       false,
		},
		{
			name: "成功列出项目 - 有版本ID",
			param: &entity.ListEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				VersionID:       gptr.Of[int64](1),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ListDatasetItemsByVersion(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSetItem{
						{ID: 1, ItemKey: "item1"},
					}, gptr.Of[int64](1), nil, nil)
			},
			wantItems:     []*entity.EvaluationSetItem{{ID: 1, ItemKey: "item1"}},
			wantTotal:     gptr.Of[int64](1),
			wantNextToken: nil,
			wantErr:       false,
		},
		{
			name:      "列出失败 - 参数为空",
			param:     nil,
			mockSetup: func() {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			items, total, nextToken, err := service.ListEvaluationSetItems(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantItems, items)
			assert.Equal(t, tt.wantTotal, total)
			assert.Equal(t, tt.wantNextToken, nextToken)
		})
	}
}

func TestEvaluationSetItemServiceImpl_BatchGetEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetItemServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetItemServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name      string
		param     *entity.BatchGetEvaluationSetItemsParam
		mockSetup func()
		wantItems []*entity.EvaluationSetItem
		wantErr   bool
	}{
		{
			name: "成功批量获取 - 无版本ID",
			param: &entity.BatchGetEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				ItemIDs:         []int64{1, 2},
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchGetDatasetItems(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSetItem{
						{ID: 1, ItemKey: "item1"},
						{ID: 2, ItemKey: "item2"},
					}, nil)
			},
			wantItems: []*entity.EvaluationSetItem{
				{ID: 1, ItemKey: "item1"},
				{ID: 2, ItemKey: "item2"},
			},
			wantErr: false,
		},
		{
			name: "成功批量获取 - 有版本ID",
			param: &entity.BatchGetEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				ItemIDs:         []int64{1},
				VersionID:       gptr.Of[int64](1),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchGetDatasetItemsByVersion(gomock.Any(), gomock.Any()).
					Return([]*entity.EvaluationSetItem{
						{ID: 1, ItemKey: "item1"},
					}, nil)
			},
			wantItems: []*entity.EvaluationSetItem{
				{ID: 1, ItemKey: "item1"},
			},
			wantErr: false,
		},
		{
			name:      "获取失败 - 参数为空",
			param:     nil,
			mockSetup: func() {},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			items, err := service.BatchGetEvaluationSetItems(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantItems, items)
		})
	}
}

func TestEvaluationSetItemServiceImpl_BatchUpdateEvaluationSetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetItemServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetItemServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name           string
		param          *entity.BatchUpdateEvaluationSetItemsParam
		mockSetup      func()
		wantErrors     []*entity.ItemErrorGroup
		wantItemOutputs []*entity.DatasetItemOutput
		wantErr        bool
	}{
		{
			name: "成功批量更新评估集项",
			param: &entity.BatchUpdateEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				Items: []*entity.EvaluationSetItem{
					{
						ID:              1,
						ItemKey:         "item1",
						EvaluationSetID: 100,
						Turns: []*entity.Turn{
							{
								ID: 1,
								FieldDataList: []*entity.FieldData{
									{
										Key:     "field1",
										Name:    "Field 1",
										Content: &entity.Content{Text: gptr.Of("updated content")},
									},
								},
							},
						},
					},
				},
				SkipInvalidItems: gptr.Of(true),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchUpdateDatasetItems(gomock.Any(), gomock.Any()).
					Return(nil, []*entity.DatasetItemOutput{
						{
							ItemIndex: gptr.Of[int32](0),
							ItemKey:   gptr.Of("item1"),
							ItemID:    gptr.Of[int64](1),
							IsNewItem: gptr.Of(false),
						},
					}, nil)
			},
			wantErrors: nil,
			wantItemOutputs: []*entity.DatasetItemOutput{
				{
					ItemIndex: gptr.Of[int32](0),
					ItemKey:   gptr.Of("item1"),
					ItemID:    gptr.Of[int64](1),
					IsNewItem: gptr.Of(false),
				},
			},
			wantErr: false,
		},
		{
			name: "批量更新失败 - 存在错误",
			param: &entity.BatchUpdateEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				Items: []*entity.EvaluationSetItem{
					{
						ID:              1,
						ItemKey:         "invalid_item",
						EvaluationSetID: 100,
					},
				},
				SkipInvalidItems: gptr.Of(false),
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchUpdateDatasetItems(gomock.Any(), gomock.Any()).
					Return([]*entity.ItemErrorGroup{
						{
							Type:       gptr.Of(entity.ItemErrorType_MismatchSchema),
							Summary:    gptr.Of("Schema validation failed"),
							ErrorCount: gptr.Of[int32](1),
							Details: []*entity.ItemErrorDetail{
								{
									Message:   gptr.Of("Field validation error"),
									Index:     gptr.Of[int32](0),
								},
							},
						},
					}, nil, nil)
			},
			wantErrors: []*entity.ItemErrorGroup{
				{
					Type:       gptr.Of(entity.ItemErrorType_MismatchSchema),
					Summary:    gptr.Of("Schema validation failed"),
					ErrorCount: gptr.Of[int32](1),
					Details: []*entity.ItemErrorDetail{
						{
							Message:   gptr.Of("Field validation error"),
							Index:     gptr.Of[int32](0),
						},
					},
				},
			},
			wantItemOutputs: nil,
			wantErr:         false,
		},
		{
			name: "批量更新失败 - RPC错误",
			param: &entity.BatchUpdateEvaluationSetItemsParam{
				SpaceID:         1,
				EvaluationSetID: 100,
				Items: []*entity.EvaluationSetItem{
					{
						ID:              1,
						ItemKey:         "item1",
						EvaluationSetID: 100,
					},
				},
			},
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					BatchUpdateDatasetItems(gomock.Any(), gomock.Any()).
					Return(nil, nil, errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErrors:     nil,
			wantItemOutputs: nil,
			wantErr:        true,
		},
		{
			name:           "批量更新失败 - 参数为空",
			param:          nil,
			mockSetup:      func() {},
			wantErrors:     nil,
			wantItemOutputs: nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			errors, itemOutputs, err := service.BatchUpdateEvaluationSetItems(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantErrors, errors)
			assert.Equal(t, tt.wantItemOutputs, itemOutputs)
		})
	}
}

func TestEvaluationSetItemServiceImpl_ClearEvaluationSetDraftItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	evaluationSetItemServiceOnce = sync.Once{}
	mockDatasetRPCAdapter := mocks.NewMockIDatasetRPCAdapter(ctrl)
	service := NewEvaluationSetItemServiceImpl(mockDatasetRPCAdapter)

	tests := []struct {
		name      string
		spaceID   int64
		setID     int64
		mockSetup func()
		wantErr   bool
	}{
		{
			name:    "成功清除草稿项目",
			spaceID: 1,
			setID:   100,
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ClearEvaluationSetDraftItem(gomock.Any(), int64(1), int64(100)).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "清除失败 - RPC错误",
			spaceID: 1,
			setID:   100,
			mockSetup: func() {
				mockDatasetRPCAdapter.EXPECT().
					ClearEvaluationSetDraftItem(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errorx.NewByCode(errno.CommonInternalErrorCode))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			err := service.ClearEvaluationSetDraftItem(context.Background(), tt.spaceID, tt.setID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// equalIDMaps 辅助函数，用于比较两个 IDMap 是否相等
func equalIDMaps(m1, m2 map[int64]int64) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v := range m1 {
		if m2[k] != v {
			return false
		}
	}
	return true
}

// equalErrorGroups 辅助函数，用于比较两个错误组切片是否相等
func equalErrorGroups(g1, g2 []*entity.ItemErrorGroup) bool {
	if len(g1) != len(g2) {
		return false
	}
	for i := range g1 {
		if g1[i] != g2[i] {
			return false
		}
	}
	return true
}
