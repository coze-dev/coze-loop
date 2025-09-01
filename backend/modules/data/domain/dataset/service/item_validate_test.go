// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
)

func TestDatasetServiceImpl_ValidateDatasetItems(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	svc := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name        string
		param       *ValidateDatasetItemsParam
		mockSetup   func()
		expected    *ValidateDatasetItemsResult
		expectedErr error
	}{
		{
			name: "空items列表",
			param: &ValidateDatasetItemsParam{
				Items: []*entity.Item{},
			},
			mockSetup: func() {},
			expected:  &ValidateDatasetItemsResult{},
		},
		{
			name: "无DatasetID且缺少必要参数",
			param: &ValidateDatasetItemsParam{
				SpaceID:   123,
				DatasetID: 0,
				Items:     []*entity.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {},
			expectedErr: errno.BadReqErrorf("dataset_id is required"),
		},
		{
			name: "无DatasetID且缺少字段定义",
			param: &ValidateDatasetItemsParam{
				SpaceID:         123,
				DatasetID:       0,
				DatasetCategory: entity.DatasetCategoryQA,
				Items:           []*entity.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {},
			expectedErr: errno.BadReqErrorf("dataset_fields is required"),
		},
		{
			name: "有DatasetID但获取失败",
			param: &ValidateDatasetItemsParam{
				SpaceID:   123,
				DatasetID: 456,
				Items:     []*entity.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), int64(123), int64(456)).Return(nil, errors.New("db error"))
			},
			expectedErr: errors.New("get dataset"),
		},
		{
			name: "有DatasetID且获取成功但需要获取schema失败",
			param: &ValidateDatasetItemsParam{
				SpaceID:   123,
				DatasetID: 456,
				Items:     []*entity.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), int64(123), int64(456)).Return(&entity.Dataset{
					ID:       456,
					SchemaID: 789,
				}, nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), int64(123), int64(789)).Return(nil, errors.New("schema error"))
			},
			expectedErr: errors.New("get schema"),
		},
		{
			name: "容量校验失败",
			param: &ValidateDatasetItemsParam{
				SpaceID:                123,
				DatasetID:              456,
				Items:                  []*entity.Item{{Data: gptr.Of("test")}},
				IgnoreCurrentItemCount: false,
			},
			mockSetup: func() {
				dataset := &entity.Dataset{
					ID:       456,
					SchemaID: 789,
				}
				svc.buildNewDataset(dataset)
				
				mockRepo.EXPECT().GetDataset(gomock.Any(), int64(123), int64(456)).Return(dataset, nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), int64(123), int64(789)).Return(&entity.DatasetSchema{
					Fields: []*entity.FieldSchema{},
				}, nil)
				mockRepo.EXPECT().GetItemCount(gomock.Any(), int64(456)).Return(int64(1000), nil)
			},
			expected: &ValidateDatasetItemsResult{
				ValidItemIndices: []int32{},
				ErrorGroups: []*entity.ItemErrorGroup{
					{
						Type:       gptr.Of(entity.ItemErrorType_ExceedDatasetCapacity),
						Summary:    gptr.Of("capacity=1000, current=1000, to_add=1"),
						ErrorCount: gptr.Of(int32(1)),
						Details: []*entity.ItemErrorDetail{
							{Index: gptr.Of(int32(0))},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := svc.ValidateDatasetItems(context.Background(), tt.param)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				if tt.expected != nil {
					assert.Equal(t, tt.expected.ValidItemIndices, result.ValidItemIndices)
					assert.Equal(t, len(tt.expected.ErrorGroups), len(result.ErrorGroups))
				}
			}
		})
	}
}

func TestDatasetServiceImpl_buildDatasetForValidate(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	svc := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name        string
		param       *ValidateDatasetItemsParam
		mockSetup   func()
		expectedErr error
	}{
		{
			name: "无DatasetID创建新dataset成功",
			param: &ValidateDatasetItemsParam{
				SpaceID:         123,
				DatasetID:       0,
				DatasetCategory: entity.DatasetCategoryQA,
				DatasetFields:   []*entity.FieldSchema{{Name: "test"}},
			},
			mockSetup: func() {},
		},
		{
			name: "有DatasetID且有自定义字段",
			param: &ValidateDatasetItemsParam{
				SpaceID:       123,
				DatasetID:     456,
				DatasetFields: []*entity.FieldSchema{{Name: "test"}},
			},
			mockSetup: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), int64(123), int64(456)).Return(&entity.Dataset{
					ID: 456,
				}, nil)
			},
		},
		{
			name: "有DatasetID使用原有schema",
			param: &ValidateDatasetItemsParam{
				SpaceID:   123,
				DatasetID: 456,
			},
			mockSetup: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), int64(123), int64(456)).Return(&entity.Dataset{
					ID:       456,
					SchemaID: 789,
				}, nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), int64(123), int64(789)).Return(&entity.DatasetSchema{
					Fields: []*entity.FieldSchema{{Name: "original"}},
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := svc.buildDatasetForValidate(context.Background(), tt.param)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Dataset)
				assert.NotNil(t, result.Schema)
			}
		})
	}
}