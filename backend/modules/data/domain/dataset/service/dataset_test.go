// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestDatasetServiceImpl_CreateDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	service := &DatasetServiceImpl{
		repo:          mockRepo,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		specConfig:    mockIConfig.GetDatasetSpec,
		featConfig:    mockIConfig.GetDatasetFeature,
		retryCfg:      mockIConfig.GetSnapshotRetry,
	}

	// 定义测试用例
	tests := []struct {
		name     string
		dataset  *entity.Dataset
		fields   []*entity.FieldSchema
		mockRepo func()
		wantErr  bool
	}{
		{
			name:    "成功创建数据集",
			dataset: &entity.Dataset{
				// 填充数据集信息
			},
			fields: []*entity.FieldSchema{
				// 填充字段信息
				{
					Name:   "input",
					Key:    "input",
					Status: entity.FieldStatusAvailable,
				},
			},
			mockRepo: func() {
				mockIConfig.EXPECT().GetDatasetFeature().Return(&conf.DatasetFeature{
					Feature: &entity.DatasetFeatures{},
				})
				mockIConfig.EXPECT().GetDatasetSpec().Return(&conf.DatasetSpec{
					Spec: &entity.DatasetSpec{},
				})
				mockRepo.EXPECT().CreateDatasetAndSchema(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "校验失败",
			dataset: &entity.Dataset{
				// 填充数据集信息
			},
			fields: []*entity.FieldSchema{
				// 填充字段信息
			},
			mockRepo: func() {
				mockIConfig.EXPECT().GetDatasetFeature().Return(&conf.DatasetFeature{
					Feature: &entity.DatasetFeatures{},
				})
				mockIConfig.EXPECT().GetDatasetSpec().Return(&conf.DatasetSpec{
					Spec: &entity.DatasetSpec{},
				})
			},
			wantErr: true,
		},
		{
			name:    "创建数据集失败",
			dataset: &entity.Dataset{
				// 填充数据集信息
			},
			fields: []*entity.FieldSchema{
				// 填充字段信息
				{
					Name:   "input",
					Key:    "input",
					Status: entity.FieldStatusAvailable,
				},
			},
			mockRepo: func() {
				mockIConfig.EXPECT().GetDatasetFeature().Return(&conf.DatasetFeature{
					Feature: &entity.DatasetFeatures{},
				})
				mockIConfig.EXPECT().GetDatasetSpec().Return(&conf.DatasetSpec{
					Spec: &entity.DatasetSpec{},
				})
				mockRepo.EXPECT().CreateDatasetAndSchema(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("创建数据集失败")).AnyTimes()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.CreateDataset(context.Background(), tt.dataset, tt.fields)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDataset() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestDatasetServiceImpl_UpdateDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name           string
		param          *UpdateDatasetParam
		mockRepo       func()
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name: "成功更新数据集",
			param: &UpdateDatasetParam{
				SpaceID:     1,
				DatasetID:   1,
				Name:        "New Name",
				Description: gptr.Of("New Description"),
				UpdatedBy:   "user1",
			},
			mockRepo: func() {
				existingDataset := &entity.Dataset{
					SpaceID:     1,
					ID:          1,
					Name:        "Old Name",
					Description: gptr.Of("Old Description"),
				}
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(1)).Return(existingDataset, nil)
				patch := &entity.Dataset{
					Name:        "New Name",
					Description: gptr.Of("New Description"),
					UpdatedBy:   "user1",
				}
				mockRepo.EXPECT().PatchDataset(context.Background(), patch, &entity.Dataset{SpaceID: 1, ID: 1}).Return(nil)
			},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name: "数据集不存在",
			param: &UpdateDatasetParam{
				SpaceID:     1,
				DatasetID:   2,
				Name:        "New Name",
				Description: gptr.Of("New Description"),
				UpdatedBy:   "user1",
			},
			mockRepo: func() {
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(2)).Return(nil, nil)
			},
			expectedErr:    true,
			expectedErrMsg: errno.NotFoundErrorf("dataset 2 not found").Error(),
		},
		{
			name: "获取数据集出错",
			param: &UpdateDatasetParam{
				SpaceID:     1,
				DatasetID:   3,
				Name:        "New Name",
				Description: gptr.Of("New Description"),
				UpdatedBy:   "user1",
			},
			mockRepo: func() {
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(3)).Return(nil, fmt.Errorf("get dataset error"))
			},
			expectedErr:    true,
			expectedErrMsg: "get dataset error",
		},
		{
			name: "更新数据集出错",
			param: &UpdateDatasetParam{
				SpaceID:     1,
				DatasetID:   4,
				Name:        "New Name",
				Description: gptr.Of("New Description"),
				UpdatedBy:   "user1",
			},
			mockRepo: func() {
				existingDataset := &entity.Dataset{
					SpaceID:     1,
					ID:          4,
					Name:        "Old Name",
					Description: gptr.Of("Old Description"),
				}
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(4)).Return(existingDataset, nil)
				patch := &entity.Dataset{
					Name:        "New Name",
					Description: gptr.Of("New Description"),
					UpdatedBy:   "user1",
				}
				mockRepo.EXPECT().PatchDataset(context.Background(), patch, &entity.Dataset{SpaceID: 1, ID: 4}).Return(fmt.Errorf("patch dataset error"))
			},
			expectedErr:    true,
			expectedErrMsg: "patch dataset error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.UpdateDataset(context.Background(), tt.param)
			if (err != nil) != tt.expectedErr {
				t.Errorf("UpdateDataset() error = %v, expectedErr %v", err, tt.expectedErr)
				return
			}
		})
	}
}

func TestDatasetServiceImpl_DeleteDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name           string
		spaceID        int64
		id             int64
		mockRepo       func()
		expectedErr    bool
		expectedErrMsg string
	}{
		{
			name:    "成功删除数据集",
			spaceID: 1,
			id:      1,
			mockRepo: func() {
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(1)).Return(&entity.Dataset{}, nil)
				mockRepo.EXPECT().DeleteDataset(context.Background(), int64(1), int64(1)).Return(nil)
			},
			expectedErr:    false,
			expectedErrMsg: "",
		},
		{
			name:    "获取数据集失败",
			spaceID: 1,
			id:      2,
			mockRepo: func() {
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(2)).Return(nil, fmt.Errorf("获取数据集失败"))
			},
			expectedErr:    true,
			expectedErrMsg: "获取数据集失败",
		},
		{
			name:    "获取数据集空",
			spaceID: 1,
			id:      2,
			mockRepo: func() {
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(2)).Return(nil, nil)
			},
			expectedErr:    true,
			expectedErrMsg: "获取数据集空",
		},
		{
			name:    "删除数据集失败",
			spaceID: 1,
			id:      2,
			mockRepo: func() {
				mockRepo.EXPECT().GetDataset(context.Background(), int64(1), int64(2)).Return(&entity.Dataset{}, nil)
				mockRepo.EXPECT().DeleteDataset(context.Background(), int64(1), int64(2)).Return(fmt.Errorf("删除数据集失败"))
			},
			expectedErr:    true,
			expectedErrMsg: "删除数据集失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.DeleteDataset(context.Background(), tt.spaceID, tt.id)
			if (err != nil) != tt.expectedErr {
				t.Errorf("DeleteDataset() error = %v, expectedErr %v", err, tt.expectedErr)
				return
			}
		})
	}
}

func TestDatasetServiceImpl_GetDatasetWithOpt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	serviceImpl := &DatasetServiceImpl{
		repo: mockRepo,
	}

	// 定义测试数据
	ctx := context.Background()
	spaceID := int64(1)
	id := int64(2)
	opt := &GetOpt{
		WithDeleted: true,
	}
	expectedResult := &DatasetWithSchema{
		Dataset: &entity.Dataset{
			ID:      id,
			SpaceID: spaceID,
		},
		Schema: &entity.DatasetSchema{},
	}

	// 设置 mock 行为
	mockRepo.EXPECT().GetDataset(ctx, spaceID, id, gomock.Any()).Return(expectedResult.Dataset, nil)
	mockRepo.EXPECT().GetSchema(ctx, spaceID, gomock.Any(), gomock.Any()).Return(expectedResult.Schema, nil)

	// 调用被测试函数
	result, err := serviceImpl.GetDatasetWithOpt(ctx, spaceID, id, opt)

	// 断言结果
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestDatasetServiceImpl_BatchGetDatasetWithOpt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	// 定义测试数据
	ctx := context.Background()
	spaceID := int64(1)
	ids := []int64{1, 2, 3}
	opt := &GetOpt{
		WithDeleted: true,
	}

	datasets := []*entity.Dataset{
		{
			ID:      1,
			SpaceID: spaceID,
		},
		{
			ID:      2,
			SpaceID: spaceID,
		},
		{
			ID:      3,
			SpaceID: spaceID,
		},
	}
	schemas := []*entity.DatasetSchema{
		{},
		{},
		{},
	}

	// 模拟返回结果
	expectedResults := []*DatasetWithSchema{
		{
			Dataset: datasets[0],
			Schema:  schemas[0],
		},
		{
			Dataset: datasets[1],
			Schema:  schemas[1],
		},
		{
			Dataset: datasets[2],
			Schema:  schemas[2],
		},
	}

	// 设置 mock 行为
	mockRepo.EXPECT().MGetDatasets(ctx, spaceID, ids, gomock.Any()).Return(datasets, nil)
	mockRepo.EXPECT().MGetSchema(ctx, spaceID, gomock.Any(), gomock.Any()).Return(schemas, nil)

	// 调用被测试函数
	result, err := service.BatchGetDatasetWithOpt(ctx, spaceID, ids, opt)

	// 断言结果
	assert.NoError(t, err)
	assert.Equal(t, expectedResults, result)
}

func TestDatasetServiceImpl_SearchDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	service := &DatasetServiceImpl{
		repo:          mockRepo,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		specConfig:    mockIConfig.GetDatasetSpec,
		featConfig:    mockIConfig.GetDatasetFeature,
		retryCfg:      mockIConfig.GetSnapshotRetry,
	}

	tests := []struct {
		name          string
		req           *SearchDatasetsParam
		mockSetup     func()
		wantResults   *SearchDatasetsResults
		wantErr       bool
		wantErrString string
	}{
		{
			name: "successful search with filters",
			req: &SearchDatasetsParam{
				SpaceID: 1,
				Name:    gptr.Of("test"),
				OrderBy: &OrderBy{Field: gptr.Of("created_at"), IsAsc: gptr.Of(true)},
			},
			mockSetup: func() {
				datasets := []*entity.Dataset{
					{
						ID:        1,
						SpaceID:   1,
						AppID:     100,
						Name:      "test dataset 1",
						Category:  entity.DatasetCategoryGeneral,
						Status:    entity.DatasetStatusAvailable,
						SchemaID:  1000,
						CreatedAt: time.Now(),
					},
				}
				schemas := []*entity.DatasetSchema{
					{
						ID:        1000,
						SpaceID:   1,
						DatasetID: 1,
						Fields: []*entity.FieldSchema{
							{
								Key:         "field1",
								Name:        "Field 1",
								ContentType: entity.ContentTypeText,
							},
						},
					},
				}
				pageResult := &pagination.PageResult{
					Total: 1,
				}

				mockRepo.EXPECT().ListDatasets(gomock.Any(), gomock.Any()).Return(datasets, pageResult, nil)
				mockRepo.EXPECT().MGetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(schemas, nil)
				mockRepo.EXPECT().CountDatasets(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			wantResults: &SearchDatasetsResults{
				Total: 1,
				DatasetWithSchemas: []*DatasetWithSchema{
					{
						Dataset: &entity.Dataset{
							ID:       1,
							SpaceID:  1,
							AppID:    100,
							Name:     "test dataset 1",
							Category: entity.DatasetCategoryGeneral,
							Status:   entity.DatasetStatusAvailable,
							SchemaID: 1000,
						},
						Schema: &entity.DatasetSchema{
							ID:        1000,
							SpaceID:   1,
							DatasetID: 1,
							Fields: []*entity.FieldSchema{
								{
									Key:         "field1",
									Name:        "Field 1",
									ContentType: entity.ContentTypeText,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "error search with filters",
			req: &SearchDatasetsParam{
				SpaceID: 1,
				Name:    gptr.Of("test"),
				OrderBy: &OrderBy{Field: gptr.Of("created_at"), IsAsc: gptr.Of(true)},
			},
			mockSetup: func() {
				datasets := []*entity.Dataset{
					{
						ID:        1,
						SpaceID:   1,
						AppID:     100,
						Name:      "test dataset 1",
						Category:  entity.DatasetCategoryGeneral,
						Status:    entity.DatasetStatusAvailable,
						SchemaID:  1000,
						CreatedAt: time.Now(),
					},
				}
				pageResult := &pagination.PageResult{
					Total: 1,
				}

				mockRepo.EXPECT().ListDatasets(gomock.Any(), gomock.Any()).Return(datasets, pageResult, nil)
				mockRepo.EXPECT().CountDatasets(gomock.Any(), gomock.Any()).Return(int64(1), fmt.Errorf("test error"))
			},
			wantResults: &SearchDatasetsResults{
				Total: 1,
				DatasetWithSchemas: []*DatasetWithSchema{
					{
						Dataset: &entity.Dataset{
							ID:       1,
							SpaceID:  1,
							AppID:    100,
							Name:     "test dataset 1",
							Category: entity.DatasetCategoryGeneral,
							Status:   entity.DatasetStatusAvailable,
							SchemaID: 1000,
						},
						Schema: &entity.DatasetSchema{
							ID:        1000,
							SpaceID:   1,
							DatasetID: 1,
							Fields: []*entity.FieldSchema{
								{
									Key:         "field1",
									Name:        "Field 1",
									ContentType: entity.ContentTypeText,
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error search with filters, mget schema err",
			req: &SearchDatasetsParam{
				SpaceID: 1,
				Name:    gptr.Of("test"),
				OrderBy: &OrderBy{Field: gptr.Of("created_at"), IsAsc: gptr.Of(true)},
			},
			mockSetup: func() {
				datasets := []*entity.Dataset{
					{
						ID:        1,
						SpaceID:   1,
						AppID:     100,
						Name:      "test dataset 1",
						Category:  entity.DatasetCategoryGeneral,
						Status:    entity.DatasetStatusAvailable,
						SchemaID:  1000,
						CreatedAt: time.Now(),
					},
				}
				schemas := []*entity.DatasetSchema{
					{
						ID:        1000,
						SpaceID:   1,
						DatasetID: 1,
						Fields: []*entity.FieldSchema{
							{
								Key:         "field1",
								Name:        "Field 1",
								ContentType: entity.ContentTypeText,
							},
						},
					},
				}
				pageResult := &pagination.PageResult{
					Total: 1,
				}

				mockRepo.EXPECT().ListDatasets(gomock.Any(), gomock.Any()).Return(datasets, pageResult, nil)
				mockRepo.EXPECT().MGetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(schemas, fmt.Errorf("test error"))
				mockRepo.EXPECT().CountDatasets(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			wantResults: &SearchDatasetsResults{
				Total: 1,
				DatasetWithSchemas: []*DatasetWithSchema{
					{
						Dataset: &entity.Dataset{
							ID:       1,
							SpaceID:  1,
							AppID:    100,
							Name:     "test dataset 1",
							Category: entity.DatasetCategoryGeneral,
							Status:   entity.DatasetStatusAvailable,
							SchemaID: 1000,
						},
						Schema: &entity.DatasetSchema{
							ID:        1000,
							SpaceID:   1,
							DatasetID: 1,
							Fields: []*entity.FieldSchema{
								{
									Key:         "field1",
									Name:        "Field 1",
									ContentType: entity.ContentTypeText,
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			gotResults, err := service.SearchDataset(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrString != "" {
					assert.Equal(t, tt.wantErrString, err.Error())
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResults.Total, gotResults.Total)
		})
	}
}

func TestDatasetServiceImpl_buildSearchDatasetParam_DescriptionLike(t *testing.T) {
	service := &DatasetServiceImpl{}

	tests := []struct {
		name                string
		req                 *SearchDatasetsParam
		wantNameLike        string
		wantDescriptionLike string
	}{
		{
			name: "description provided is passed through as DescriptionLike",
			req: &SearchDatasetsParam{
				SpaceID:     1,
				Name:        gptr.Of("foo"),
				Description: gptr.Of("bar"),
				OrderBy:     &OrderBy{},
			},
			wantNameLike:        "foo",
			wantDescriptionLike: "bar",
		},
		{
			name: "description nil yields empty DescriptionLike (no condition)",
			req: &SearchDatasetsParam{
				SpaceID: 1,
				Name:    gptr.Of("foo"),
				OrderBy: &OrderBy{},
			},
			wantNameLike:        "foo",
			wantDescriptionLike: "",
		},
		{
			name: "description empty string yields empty DescriptionLike",
			req: &SearchDatasetsParam{
				SpaceID:     1,
				Description: gptr.Of(""),
				OrderBy:     &OrderBy{},
			},
			wantNameLike:        "",
			wantDescriptionLike: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.buildSearchDatasetParam(tt.req)
			assert.Equal(t, tt.wantNameLike, got.NameLike)
			assert.Equal(t, tt.wantDescriptionLike, got.DescriptionLike)
		})
	}
}

func TestDatasetServiceImpl_GetDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name           string
		spaceID        int64
		id             int64
		mockRepo       func()
		expectedResult *DatasetWithSchema
		expectedErr    error
	}{
		{
			name:    "成功获取数据集",
			spaceID: 1,
			id:      1,
			mockRepo: func() {
				mockDataset := &entity.Dataset{
					ID:       1,
					SpaceID:  1,
					SchemaID: 1,
				}
				mockSchema := &entity.DatasetSchema{
					ID:      1,
					SpaceID: 1,
				}
				mockRepo.EXPECT().GetDataset(gomock.Any(), int64(1), int64(1)).Return(mockDataset, nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), int64(1), int64(1), gomock.Any()).Return(mockSchema, nil)
			},
			expectedResult: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID:      1,
					SpaceID: 1,
				},
				Schema: &entity.DatasetSchema{
					ID:      1,
					SpaceID: 1,
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			result, err := service.GetDataset(context.Background(), tt.spaceID, tt.id)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult.ID, result.ID)
		})
	}
}

func TestDatasetServiceImpl_BatchGetDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name           string
		spaceID        int64
		ids            []int64
		mockRepo       func()
		expectedResult []*DatasetWithSchema
		expectedErr    error
	}{
		{
			name:    "成功批量获取数据集",
			spaceID: 1,
			ids:     []int64{1, 2},
			mockRepo: func() {
				mockDatasets := []*entity.Dataset{
					{
						ID:      1,
						SpaceID: 1,
					},
					{
						ID:      2,
						SpaceID: 1,
					},
				}
				mockSchemas := []*entity.DatasetSchema{
					{
						ID:      1,
						SpaceID: 1,
					},
					{
						ID:      2,
						SpaceID: 1,
					},
				}
				mockRepo.EXPECT().MGetDatasets(context.Background(), int64(1), []int64{1, 2}, gomock.Any()).Return(mockDatasets, nil)
				mockRepo.EXPECT().MGetSchema(context.Background(), int64(1), gomock.Any(), gomock.Any()).Return(mockSchemas, nil)
			},
			expectedResult: []*DatasetWithSchema{
				{
					Dataset: &entity.Dataset{
						ID:      1,
						SpaceID: 1,
					},
					Schema: &entity.DatasetSchema{
						ID:      1,
						SpaceID: 1,
					},
				},
				{
					Dataset: &entity.Dataset{
						ID:      2,
						SpaceID: 1,
					},
					Schema: &entity.DatasetSchema{
						ID:      2,
						SpaceID: 1,
					},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			result, err := service.BatchGetDataset(context.Background(), tt.spaceID, tt.ids)
			if tt.expectedErr != nil {
				assert.EqualError(t, err, tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, len(tt.expectedResult), len(result))
		})
	}
}
