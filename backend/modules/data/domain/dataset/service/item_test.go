// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	idgenmock "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	common_entity "github.com/coze-dev/coze-loop/backend/modules/data/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestDatasetServiceImpl_LoadItemData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock repo
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)

	// 定义测试数据
	ctx := context.Background()
	items := []*entity.Item{
		{ID: 1, DataProperties: &entity.ItemDataProperties{Storage: common_entity.ProviderS3}},
		{ID: 2},
	}

	// mock MGetItemData 方法
	mockRepo.EXPECT().MGetItemData(ctx, gomock.Any(), gomock.Any()).Return(nil)

	// 创建服务实例
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	// 调用方法
	err := service.LoadItemData(ctx, items...)
	if err != nil {
		t.Errorf("LoadItemData returned error: %v", err)
	}
}

func TestArchiveAndCreateItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock 对象
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
	}

	// 定义测试数据
	ctx := context.Background()
	ds := &DatasetWithSchema{
		Dataset: &entity.Dataset{
			ID: 1,
		},
		Schema: &entity.DatasetSchema{
			ID: 1,
		},
	}
	oldID := int64(1)
	item := &entity.Item{
		ID: 2,
	}

	// mock MGetItemData
	mockRepo.EXPECT().MSetItemData(ctx, []*entity.Item{item}, gomock.Any()).Return(0, nil)

	// mock MGetDatasetOperations
	mockRepo.EXPECT().MGetDatasetOperations(ctx, ds.ID, gomock.Any()).Return(nil, nil)

	// mock AddDatasetOperation
	mockRepo.EXPECT().AddDatasetOperation(ctx, ds.ID, gomock.Any()).Return(nil)

	// mock DelDatasetOperation
	mockRepo.EXPECT().DelDatasetOperation(ctx, ds.ID, gomock.Any(), gomock.Any()).Return(nil)

	mockRepo.EXPECT().GetSchema(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
	mockRepo.EXPECT().PatchDataset(context.Background(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().MCreateItems(context.Background(), gomock.Any(), gomock.Any()).Return(int64(0), nil)
	mockRepo.EXPECT().ArchiveItems(context.Background(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// mock GetDatasetItemStorage
	mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
		Providers: []*conf.DatasetItemProviderConfig{
			{
				Provider: common_entity.ProviderS3,
				MaxSize:  65536,
			},
		},
	})

	// mock Transaction
	mockProvider.EXPECT().Transaction(ctx, gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
		return fc(nil)
	})

	// 调用被测试方法
	err := service.ArchiveAndCreateItem(ctx, ds, oldID, item)
	if err != nil {
		t.Errorf("ArchiveAndCreateItem returned error: %v", err)
	}
}

func TestDatasetServiceImpl_UpdateItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)

	service := &DatasetServiceImpl{
		repo:          mockRepo,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		txDB:          mockProvider,
	}

	// 定义测试用例
	tests := []struct {
		name        string
		ds          *DatasetWithSchema
		item        *entity.Item
		mockRepo    func()
		expectedErr bool
	}{
		{
			name: "成功更新 Item",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID: 1,
				},
				Schema: &entity.DatasetSchema{},
			},
			item: &entity.Item{
				ID: 1,
			},
			mockRepo: func() {
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
				mockRepo.EXPECT().PatchDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().UpdateItem(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})
			},
			expectedErr: false,
		},
		{
			name: "加锁失败",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID: 1,
				},
				Schema: &entity.DatasetSchema{},
			},
			item: &entity.Item{
				ID: 1,
			},
			mockRepo: func() {
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("err"))
			},
			expectedErr: true,
		},
		{
			name: "事务失败",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID: 1,
				},
				Schema: &entity.DatasetSchema{},
			},
			item: &entity.Item{
				ID: 1,
			},
			mockRepo: func() {
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fmt.Errorf("tx err")
				})
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})
			},
			expectedErr: true,
		},
		{
			name: "更新 Item 失败",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID: 1,
				},
				Schema: &entity.DatasetSchema{},
			},
			item: &entity.Item{
				ID: 1,
			},
			mockRepo: func() {
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
				mockRepo.EXPECT().PatchDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().UpdateItem(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("err"))
				mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})
			},
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.UpdateItem(context.Background(), tt.ds, tt.item)
			if (err != nil) != tt.expectedErr {
				t.Errorf("UpdateItem() error = %v, expectedErr %v", err, tt.expectedErr)
			}
		})
	}
}

func TestBatchDeleteItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
		txDB: mockProvider,
	}

	// 定义测试数据
	ctx := context.Background()
	ds := &DatasetWithSchema{
		Dataset: &entity.Dataset{
			ID:             1,
			SpaceID:        1,
			NextVersionNum: 1,
		},
		Schema: &entity.DatasetSchema{},
	}
	items := []*entity.Item{
		{ID: 1, AddVN: 1},
		{ID: 2, AddVN: 2},
	}
	// mock Transaction
	mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
		return fc(nil)
	})

	// mock IncrItemCount
	expectedCount := int64(10)
	mockRepo.EXPECT().IncrItemCount(ctx, ds.ID, -int64(len(items))).Return(expectedCount, nil)

	mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
	mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
	mockRepo.EXPECT().PatchDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().ArchiveItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().DeleteItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	// 调用方法
	err := service.BatchDeleteItems(ctx, ds, items...)
	if err != nil {
		t.Errorf("BatchDeleteItems returned error: %v", err)
	}
}

func TestDatasetServiceImpl_ClearDataset(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
		txDB: mockProvider,
	}

	// 定义测试数据
	ctx := context.Background()
	ds := &DatasetWithSchema{
		Dataset: &entity.Dataset{
			ID: 1,
		},
		Schema: &entity.DatasetSchema{},
	}

	// 设置 mock 行为
	mockRepo.EXPECT().MGetDatasetOperations(ctx, ds.ID, gomock.Any()).Return(nil, nil)
	mockRepo.EXPECT().AddDatasetOperation(ctx, ds.ID, gomock.Any()).Return(nil)
	mockRepo.EXPECT().DelDatasetOperation(ctx, ds.ID, gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().SetItemCount(ctx, ds.ID, gomock.Any()).Return(nil)
	mockRepo.EXPECT().PatchDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockRepo.EXPECT().ClearDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ItemIdentity{}, nil)
	mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
		return fc(nil)
	})

	// 调用被测试方法
	err := service.ClearDataset(ctx, ds)
	if err != nil {
		t.Errorf("ClearDataset() error = %v", err)
	}
}

func TestDatasetServiceImpl_GetItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock repo
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)

	// 定义测试数据
	ctx := context.Background()
	spaceID := int64(1)
	datasetID := int64(1)
	itemID := int64(1)

	// 定义 mock 返回的 items
	mockItems := []*entity.Item{
		{
			ID:        itemID,
			SpaceID:   spaceID,
			DatasetID: datasetID,
		},
	}

	// mock ListItems 方法
	mockRepo.EXPECT().ListItems(ctx, gomock.Any()).Return(mockItems, &pagination.PageResult{}, nil)

	// 创建服务实例
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	// 调用方法
	result, err := service.GetItem(ctx, spaceID, datasetID, itemID)
	if err != nil {
		t.Errorf("GetItem returned error: %v", err)
	}

	// 验证结果
	if result == nil {
		t.Error("Expected item to be non-nil")
	} else if result.ID != itemID {
		t.Errorf("Expected item ID %d, got %d", itemID, result.ID)
	}
}

func TestDatasetServiceImpl_BatchGetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建 mock repo
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)

	// 定义测试数据
	ctx := context.Background()
	spaceID := int64(1)
	datasetID := int64(2)
	itemIDs := []int64{1, 2, 3}

	// 定义期望的返回值
	expectedItems := []*entity.Item{
		{ID: 1},
		{ID: 2},
		{ID: 3},
	}
	pageResult := &pagination.PageResult{}

	// 设置 mock 行为
	mockRepo.EXPECT().ListItems(ctx, gomock.Any()).Return(expectedItems, pageResult, nil)

	// 创建服务实例
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	// 调用被测试函数
	items, err := service.BatchGetItems(ctx, spaceID, datasetID, itemIDs)

	// 断言结果
	assert.NoError(t, err)
	assert.Equal(t, expectedItems, items)
}

func TestDatasetServiceImpl_BatchCreateItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	mockIIDGenerator := idgenmock.NewMockIIDGenerator(ctrl)
	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
		idgen:         mockIIDGenerator,
	}

	// 定义测试数据
	ctx := context.Background()
	ds := &DatasetWithSchema{
		Dataset: &entity.Dataset{
			ID:      1,
			SpaceID: 1,
			Spec: &entity.DatasetSpec{
				MaxItemCount: 10,
			},
		},
		Schema: &entity.DatasetSchema{
			ID:      1,
			SpaceID: 1,
		},
	}
	iitems := []*IndexedItem{
		{
			Item: &entity.Item{
				SpaceID:   1,
				DatasetID: 1,
			},
		},
	}
	opt := &MAddItemOpt{}
	added := []*IndexedItem{
		{
			Item: &entity.Item{
				ID:        1,
				SpaceID:   1,
				DatasetID: 1,
			},
		},
	}

	// 定义测试用例
	tests := []struct {
		name      string
		mockRepo  func()
		wantAdded []*IndexedItem
		wantErr   bool
	}{
		{
			name: "成功批量创建 Items",
			mockRepo: func() {
				mockIIDGenerator.EXPECT().GenMultiIDs(ctx, len(iitems)).Return([]int64{added[0].ID}, nil)
				mockRepo.EXPECT().ListItems(gomock.Any(), gomock.Any()).Return(nil, &pagination.PageResult{}, nil)
				mockRepo.EXPECT().IncrItemCount(ctx, ds.Dataset.ID, gomock.Any()).Return(int64(len(iitems)), nil).MaxTimes(2)
				mockRepo.EXPECT().MGetDatasetOperations(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockRepo.EXPECT().AddDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DelDatasetOperation(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})
				mockProvider.EXPECT().Transaction(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantAdded: added,
			wantErr:   false,
		},
		{
			name: "批量创建 Items 失败",
			mockRepo: func() {
				mockIIDGenerator.EXPECT().GenMultiIDs(ctx, len(iitems)).Return([]int64{added[0].ID}, nil)
				mockRepo.EXPECT().IncrItemCount(ctx, ds.Dataset.ID, int64(len(iitems))).Return(int64(0), fmt.Errorf("批量创建 Items 失败"))
			},
			wantAdded: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			result, err := service.BatchCreateItems(ctx, ds, iitems, opt)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchCreateItems() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				assert.Equal(t, len(tt.wantAdded), len(result))
			}
		})
	}
}

func TestDatasetServiceImpl_touchDatasetForWriteItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	// 定义测试用例
	tests := []struct {
		name        string
		ds          *DatasetWithSchema
		opts        []repo.Option
		mockRepo    func()
		expectedErr error
	}{
		{
			name: "正常场景",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					SpaceID: 1,
					ID:      1,
				},
				Schema: &entity.DatasetSchema{
					ID: 1,
				},
			},
			opts: []repo.Option{},
			mockRepo: func() {
				mockRepo.EXPECT().GetSchema(context.Background(), int64(1), int64(1), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
				mockRepo.EXPECT().PatchDataset(context.Background(), gomock.Any(), &entity.Dataset{SpaceID: 1, ID: 1}).Return(nil)
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockRepo()
			err := service.touchDatasetForWriteItem(context.Background(), tt.ds, tt.opts)
			if (err != nil) != (tt.expectedErr != nil) {
				t.Errorf("touchDatasetForWriteItem() error = %v, expectedErr %v", err, tt.expectedErr)
				return
			}
			if err != nil && err.Error() != tt.expectedErr.Error() {
				t.Errorf("touchDatasetForWriteItem() error message = %v, expectedErrMsg %v", err.Error(), tt.expectedErr.Error())
			}
		})
	}
}

func TestDatasetServiceImpl_saveItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockProvider := mocks.NewMockProvider(ctrl)
	mockIConfig := confmocks.NewMockIConfig(ctrl)
	service := &DatasetServiceImpl{
		repo:          mockRepo,
		txDB:          mockProvider,
		storageConfig: mockIConfig.GetDatasetItemStorage,
	}

	tests := []struct {
		name      string
		ds        *DatasetWithSchema
		items     []*entity.Item
		mockSetup func()
		wantCount int64
		wantErr   bool
	}{
		{
			name: "Successful save items with RDS storage",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID:        1,
					SpaceID:   1000,
					UpdatedBy: "user1",
				},
				Schema: &entity.DatasetSchema{
					ID:            1,
					UpdateVersion: 1,
				},
			},
			items: []*entity.Item{
				{
					ID:        1,
					SpaceID:   1000,
					DatasetID: 1,
					Data: []*entity.FieldData{
						{
							Key:     "field1",
							Content: "test content",
						},
					},
				},
			},
			mockSetup: func() {
				// Mock storage config
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})

				// Mock touchDatasetForWriteItem
				mockRepo.EXPECT().PatchDataset(
					gomock.Any(),
					&entity.Dataset{
						LastOperation: entity.DatasetOpTypeWriteItem,
						UpdatedBy:     "user1",
					},
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				mockRepo.EXPECT().GetSchema(
					gomock.Any(),
					int64(1000),
					int64(1),
					gomock.Any(),
				).Return(&entity.DatasetSchema{
					ID:            1,
					UpdateVersion: 1,
				}, nil)

				// Mock MCreateItems
				mockRepo.EXPECT().MCreateItems(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(int64(1), nil)

				// Mock Transaction
				mockProvider.EXPECT().Transaction(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "Save items with S3 storage",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID:        2,
					SpaceID:   2000,
					UpdatedBy: "user1",
				},
				Schema: &entity.DatasetSchema{
					ID:            2,
					UpdateVersion: 1,
				},
			},
			items: []*entity.Item{
				{
					ID:        2,
					SpaceID:   2000,
					DatasetID: 2,
					Data: []*entity.FieldData{
						{
							Key:     "field1",
							Content: "large content",
						},
					},
				},
			},
			mockSetup: func() {
				// Mock storage config for S3
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderS3,
							MaxSize:  1048576,
						},
					},
				})

				// Mock MSetItemData for S3 storage
				mockRepo.EXPECT().MSetItemData(
					gomock.Any(),
					gomock.Any(),
					common_entity.ProviderS3,
				).Return(1, nil)

				// Mock touchDatasetForWriteItem
				mockRepo.EXPECT().PatchDataset(
					gomock.Any(),
					&entity.Dataset{
						LastOperation: entity.DatasetOpTypeWriteItem,
						UpdatedBy:     "user1",
					},
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				mockRepo.EXPECT().GetSchema(
					gomock.Any(),
					int64(2000),
					int64(2),
					gomock.Any(),
				).Return(&entity.DatasetSchema{
					ID:            2,
					UpdateVersion: 1,
				}, nil)

				// Mock MCreateItems
				mockRepo.EXPECT().MCreateItems(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(int64(1), nil)

				// Mock Transaction
				mockProvider.EXPECT().Transaction(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "Save items fails - MCreateItems error",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID:        3,
					SpaceID:   3000,
					UpdatedBy: "user1",
				},
				Schema: &entity.DatasetSchema{
					ID:            3,
					UpdateVersion: 1,
				},
			},
			items: []*entity.Item{
				{
					ID:        3,
					SpaceID:   3000,
					DatasetID: 3,
					Data: []*entity.FieldData{
						{
							Key:     "field1",
							Content: "test content",
						},
					},
				},
			},
			mockSetup: func() {
				// Mock storage config
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})

				// Mock touchDatasetForWriteItem
				mockRepo.EXPECT().PatchDataset(
					gomock.Any(),
					&entity.Dataset{
						LastOperation: entity.DatasetOpTypeWriteItem,
						UpdatedBy:     "user1",
					},
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				mockRepo.EXPECT().GetSchema(
					gomock.Any(),
					int64(3000),
					int64(3),
					gomock.Any(),
				).Return(&entity.DatasetSchema{
					ID:            3,
					UpdateVersion: 1,
				}, nil)

				// Mock MCreateItems with error
				mockRepo.EXPECT().MCreateItems(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(int64(0), fmt.Errorf("database error"))

				// Mock Transaction
				mockProvider.EXPECT().Transaction(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
			},
			wantCount: 0,
			wantErr:   true,
		},
		{
			name: "Save items fails - touchDatasetForWriteItem error",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID:        4,
					SpaceID:   4000,
					UpdatedBy: "user1",
				},
				Schema: &entity.DatasetSchema{
					ID:            4,
					UpdateVersion: 1,
				},
			},
			items: []*entity.Item{
				{
					ID:        4,
					SpaceID:   4000,
					DatasetID: 4,
					Data: []*entity.FieldData{
						{
							Key:     "field1",
							Content: "test content",
						},
					},
				},
			},
			mockSetup: func() {
				// Mock storage config
				mockIConfig.EXPECT().GetDatasetItemStorage().Return(&conf.DatasetItemStorage{
					Providers: []*conf.DatasetItemProviderConfig{
						{
							Provider: common_entity.ProviderRDS,
							MaxSize:  65536,
						},
					},
				})

				// Mock touchDatasetForWriteItem with error
				mockRepo.EXPECT().PatchDataset(
					gomock.Any(),
					&entity.Dataset{
						LastOperation: entity.DatasetOpTypeWriteItem,
						UpdatedBy:     "user1",
					},
					gomock.Any(),
					gomock.Any(),
				).Return(fmt.Errorf("update dataset error"))

				// Mock Transaction
				mockProvider.EXPECT().Transaction(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, fc func(*gorm.DB) error, opts ...db.Option) error {
					return fc(nil)
				})
			},
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			count, err := service.saveItems(context.Background(), tt.ds, tt.items)
			if (err != nil) != tt.wantErr {
				t.Errorf("saveItems() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if count != tt.wantCount {
				t.Errorf("saveItems() count = %v, want %v", count, tt.wantCount)
			}
		})
	}
}

func TestDatasetServiceImpl_acquireItemCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	service := &DatasetServiceImpl{
		repo: mockRepo,
	}

	tests := []struct {
		name      string
		ds        *DatasetWithSchema
		want      int64
		partial   bool
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "successful acquire - no limit exceeded",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID: 1,
					Spec: &entity.DatasetSpec{
						MaxItemCount: 100,
					},
				},
			},
			want:    10,
			partial: false,
			mockSetup: func() {
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), int64(1), int64(10)).Return(int64(10), nil)
			},
			wantErr: false,
		},
		{
			name: "limit exceeded - partial not allowed",
			ds: &DatasetWithSchema{
				Dataset: &entity.Dataset{
					ID: 1,
					Spec: &entity.DatasetSpec{
						MaxItemCount: 100,
					},
				},
			},
			want:    10,
			partial: false,
			mockSetup: func() {
				// First call returns total count exceeding limit
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(105), nil)
				// Second call to decrease all requested items
				mockRepo.EXPECT().IncrItemCount(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(95), nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			got, err := service.acquireItemCount(context.Background(), tt.ds, tt.want, tt.partial)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.partial {
				// For partial adds, we should get either the requested amount or the remaining capacity
				assert.True(t, got <= tt.want)
			} else {
				// For non-partial adds, we should get either all or nothing
				assert.True(t, got == tt.want || got == 0)
			}
		})
	}
}

func TestDatasetServiceImpl_ValidateDatasetItems(t *testing.T) {
	item := &entity.Item{
		Data: []*entity.FieldData{
			{Key: "field1", Content: "test content"},
		},
	}

	intFieldSchema := &entity.FieldSchema{
		Key:         "field1",
		Name:        "field1",
		ContentType: entity.ContentTypeText,
		SchemaKey:   entity.SchemaKeyInteger,
	}
	strFieldSchema := &entity.FieldSchema{
		Key:         "field1",
		Name:        "field1",
		ContentType: entity.ContentTypeText,
		SchemaKey:   entity.SchemaKeyString,
	}

	tests := []struct {
		name         string
		param        *ValidateDatasetItemsParam
		want         *ValidateDatasetItemsResult
		mockSetup    func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI)
		wantErr      assert.ErrorAssertionFunc
		assertResult func(t *testing.T, got *ValidateDatasetItemsResult)
	}{
		{
			name: "empty items",
			param: &ValidateDatasetItemsParam{
				DatasetID:       1,
				DatasetCategory: entity.DatasetCategoryGeneral,
				DatasetFields:   []*entity.FieldSchema{strFieldSchema},
				Items:           []*entity.Item{},
			},
			mockSetup:    func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI) {},
			wantErr:      assert.NoError,
			assertResult: func(t *testing.T, got *ValidateDatasetItemsResult) {},
		},
		{
			name: "new dataset without fields",
			param: &ValidateDatasetItemsParam{
				DatasetID:       0,
				DatasetCategory: entity.DatasetCategoryGeneral,
				DatasetFields:   []*entity.FieldSchema{},
				Items:           []*entity.Item{item},
			},
			mockSetup: func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI) {},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "dataset_fields is required")
			},
			assertResult: func(t *testing.T, got *ValidateDatasetItemsResult) {},
		},
		{
			name: "new dataset with fields",
			param: &ValidateDatasetItemsParam{
				DatasetID:       0,
				DatasetCategory: entity.DatasetCategoryGeneral,
				DatasetFields:   []*entity.FieldSchema{strFieldSchema},
				Items:           []*entity.Item{item},
			},
			mockSetup: func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI) {},
			wantErr:   assert.NoError,
			assertResult: func(t *testing.T, got *ValidateDatasetItemsResult) {
				assert.Len(t, got.ValidItemIndices, 1)
				assert.Len(t, got.ErrorGroups, 0)
			},
		},
		{
			name: "schema mismatch",
			param: &ValidateDatasetItemsParam{
				DatasetID:       1,
				DatasetCategory: entity.DatasetCategoryGeneral,
				DatasetFields:   []*entity.FieldSchema{intFieldSchema}, // 覆盖已有 schema
				Items:           []*entity.Item{item},
			},
			mockSetup: func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI) {
				dataset := &entity.Dataset{
					ID:       1,
					Spec:     &entity.DatasetSpec{MaxItemCount: 100},
					Features: &entity.DatasetFeatures{},
				}
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).Return(dataset, nil)
			},
			wantErr: assert.NoError,
			assertResult: func(t *testing.T, got *ValidateDatasetItemsResult) {
				assert.Len(t, got.ValidItemIndices, 0)
				assert.Len(t, got.ErrorGroups, 1)
				eg := got.ErrorGroups[0]
				assert.Equal(t, entity.ItemErrorType_MismatchSchema, *eg.Type)
				assert.Equal(t, int32(1), *eg.ErrorCount)
			},
		},
		{
			name: "capacity exceeded",
			param: &ValidateDatasetItemsParam{
				DatasetID: 1,
				Items:     []*entity.Item{item},
			},
			mockSetup: func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI) {
				dataset := &entity.Dataset{
					ID:       1,
					Spec:     &entity.DatasetSpec{MaxItemCount: 100},
					Features: &entity.DatasetFeatures{},
				}
				schema := &entity.DatasetSchema{
					ID:            1,
					UpdateVersion: 1,
					Fields:        []*entity.FieldSchema{strFieldSchema},
				}
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).Return(dataset, nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any()).Return(schema, nil)
				mockRepo.EXPECT().GetItemCount(gomock.Any(), gomock.Any()).Return(int64(100), nil)
			},
			wantErr: assert.NoError,
			assertResult: func(t *testing.T, got *ValidateDatasetItemsResult) {
				assert.Len(t, got.ValidItemIndices, 0)
				assert.Len(t, got.ErrorGroups, 1)
				eg := got.ErrorGroups[0]
				assert.Equal(t, entity.ItemErrorType_ExceedDatasetCapacity, *eg.Type)
				assert.Equal(t, int32(1), *eg.ErrorCount)
			},
		},
		{
			name: "ignore current item count",
			param: &ValidateDatasetItemsParam{
				DatasetID:              1,
				IgnoreCurrentItemCount: true,
				Items:                  []*entity.Item{item, item}, // add 2 items
			},
			mockSetup: func(t *testing.T, mockRepo *mock_repo.MockIDatasetAPI) {
				dataset := &entity.Dataset{
					ID:       1,
					Spec:     &entity.DatasetSpec{MaxItemCount: 1},
					Features: &entity.DatasetFeatures{},
				}
				schema := &entity.DatasetSchema{
					ID:            1,
					UpdateVersion: 1,
					Fields:        []*entity.FieldSchema{strFieldSchema},
				}
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).Return(dataset, nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any()).Return(schema, nil)
			},
			wantErr: assert.NoError,
			assertResult: func(t *testing.T, got *ValidateDatasetItemsResult) {
				// 1 item added, 1 item failed due to capacity
				assert.Len(t, got.ValidItemIndices, 1)
				assert.Len(t, got.ErrorGroups, 1)
				eg := got.ErrorGroups[0]
				assert.Equal(t, entity.ItemErrorType_ExceedDatasetCapacity, *eg.Type)
				assert.Equal(t, int32(1), *eg.ErrorCount)
				assert.Equal(t, int32(1), *eg.Details[0].Index)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
			service := &DatasetServiceImpl{
				repo: mockRepo,
				featConfig: func() *conf.DatasetFeature {
					return &conf.DatasetFeature{
						Feature: &entity.DatasetFeatures{},
					}
				},
				specConfig: func() *conf.DatasetSpec {
					return &conf.DatasetSpec{
						Spec: &entity.DatasetSpec{
							MaxItemCount: 100,
						},
					}
				},
			}
			tt.mockSetup(t, mockRepo)
			ctx := context.Background()
			got, err := service.ValidateDatasetItems(ctx, tt.param)
			if !tt.wantErr(t, err) {
				return
			}
			tt.assertResult(t, got)
		})
	}
}
