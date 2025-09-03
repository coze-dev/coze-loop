// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	mock_audit "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	domain_dataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	mock_auth "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service"
	mock_dataset "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

func TestDatasetApplicationImpl_BatchCreateDatasetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.BatchCreateDatasetItemsRequest
		mockAuth     func()
		mockCreate   func()
		expectedResp *dataset.BatchCreateDatasetItemsResponse
		expectedErr  error
	}{
		// 正常场景：成功批量创建数据集条目
		{
			name: "成功批量创建数据集条目",
			req: &dataset.BatchCreateDatasetItemsRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				Items:       []*domain_dataset.DatasetItem{{}}, // 根据实际情况补充
			},
			mockAuth: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockCreate: func() {
				mockRepo.EXPECT().GetItemCount(gomock.Any(), gomock.Any()).Return(int64(0), nil)
				mockDatasetService.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&service.DatasetWithSchema{Dataset: &entity.Dataset{Features: &entity.DatasetFeatures{}, Spec: &entity.DatasetSpec{MaxItemCount: 100}}, Schema: &entity.DatasetSchema{}}, nil)
				mockDatasetService.EXPECT().BatchCreateItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*service.IndexedItem{}, nil)
			},
			expectedResp: &dataset.BatchCreateDatasetItemsResponse{},
			expectedErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()
			tt.mockCreate()

			_, err := app.BatchCreateDatasetItems(context.Background(), tt.req)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

// ... existing code ...

func TestDatasetApplicationImpl_GetDatasetItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.GetDatasetItemRequest
		mockAuth     func()
		mockGetItem  func()
		expectedResp *dataset.GetDatasetItemResponse
		expectedErr  error
	}{
		// 正常场景
		{
			name: "正常获取数据集条目",
			req: &dataset.GetDatasetItemRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				ItemID:      int64(1),
			},
			mockAuth: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockGetItem: func() {
				mockDatasetService.EXPECT().GetItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&entity.Item{
						Data:         []*entity.FieldData{{Key: "key1", Attachments: []*entity.ObjectStorage{{}}, Parts: []*entity.FieldData{{}}}},
						RepeatedData: []*entity.ItemData{{Data: []*entity.FieldData{{}}}},
						DataProperties: &entity.ItemDataProperties{
							Bytes: 100,
							Runes: 100,
						},
					}, nil)
				mockDatasetService.EXPECT().LoadItemData(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedResp: &dataset.GetDatasetItemResponse{
				// 根据实际情况补充
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()
			tt.mockGetItem()

			_, err := app.GetDatasetItem(context.Background(), tt.req)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestDatasetApplicationImpl_BatchGetDatasetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.BatchGetDatasetItemsRequest
		mockAuth     func()
		mockGetItems func()
		expectedResp *dataset.BatchGetDatasetItemsResponse
		expectedErr  error
	}{
		// 正常场景
		{
			name: "正常批量获取数据集条目",
			req: &dataset.BatchGetDatasetItemsRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				ItemIds:     []int64{1, 2, 3},
			},
			mockAuth: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockGetItems: func() {
				mockDatasetService.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&service.DatasetWithSchema{Dataset: &entity.Dataset{Features: &entity.DatasetFeatures{}, Spec: &entity.DatasetSpec{MaxItemCount: 100}}, Schema: &entity.DatasetSchema{}}, nil)
				mockRepo.EXPECT().ListItems(gomock.Any(), gomock.Any()).Return([]*entity.Item{{}}, &pagination.PageResult{}, nil)
				mockDatasetService.EXPECT().LoadItemData(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedResp: &dataset.BatchGetDatasetItemsResponse{
				// 根据实际情况补充
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()
			tt.mockGetItems()

			_, err := app.BatchGetDatasetItems(context.Background(), tt.req)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

func TestDatasetApplicationImpl_DeleteDatasetItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.DeleteDatasetItemRequest
		mockAuth     func()
		mockDelete   func()
		expectedResp *dataset.DeleteDatasetItemResponse
		expectedErr  error
	}{
		// 正常场景
		{
			name: "正常删除数据集条目",
			req: &dataset.DeleteDatasetItemRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				ItemID:      int64(1),
			},
			mockAuth: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockDelete: func() {
				mockDatasetService.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&service.DatasetWithSchema{Dataset: &entity.Dataset{Features: &entity.DatasetFeatures{}, Spec: &entity.DatasetSpec{MaxItemCount: 100}}, Schema: &entity.DatasetSchema{}}, nil)
				mockDatasetService.EXPECT().GetItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Item{}, nil)
				mockDatasetService.EXPECT().BatchDeleteItems(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedResp: &dataset.DeleteDatasetItemResponse{},
			expectedErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()
			tt.mockDelete()

			_, err := app.DeleteDatasetItem(context.Background(), tt.req)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

// ... existing code ...

func TestDatasetApplicationImpl_BatchDeleteDatasetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.BatchDeleteDatasetItemsRequest
		mockAuth     func()
		mockDelete   func()
		expectedResp *dataset.BatchDeleteDatasetItemsResponse
		expectedErr  error
	}{
		// 正常场景
		{
			name: "正常批量删除数据集条目",
			req: &dataset.BatchDeleteDatasetItemsRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				ItemIds:     []int64{1, 2, 3},
			},
			mockAuth: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockDelete: func() {
				mockDatasetService.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&service.DatasetWithSchema{Dataset: &entity.Dataset{Features: &entity.DatasetFeatures{}, Spec: &entity.DatasetSpec{MaxItemCount: 100}}, Schema: &entity.DatasetSchema{}}, nil)
				mockDatasetService.EXPECT().BatchGetItems(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.Item{{}}, nil)
				mockDatasetService.EXPECT().BatchDeleteItems(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedResp: &dataset.BatchDeleteDatasetItemsResponse{},
			expectedErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()
			tt.mockDelete()

			_, err := app.BatchDeleteDatasetItems(context.Background(), tt.req)
			assert.Equal(t, tt.expectedErr, err)
		})
	}
}

// ... existing code ...

func TestDatasetApplicationImpl_ListDatasetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.ListDatasetItemsRequest
		mockAuth     func()
		mockList     func()
		expectedResp *dataset.ListDatasetItemsResponse
		expectedErr  error
	}{
		// 正常场景
		{
			name: "正常列出数据集条目",
			req: &dataset.ListDatasetItemsRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
			},
			mockAuth: func() {
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil)
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockList: func() {
				mockDatasetService.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).Return(&service.DatasetWithSchema{Dataset: &entity.Dataset{Features: &entity.DatasetFeatures{}, Spec: &entity.DatasetSpec{MaxItemCount: 100}}, Schema: &entity.DatasetSchema{}}, nil)
				mockRepo.EXPECT().GetItemCount(gomock.Any(), gomock.Any()).Return(int64(10), nil)
				mockRepo.EXPECT().ListItems(gomock.Any(), gomock.Any()).Return([]*entity.Item{{}}, &pagination.PageResult{}, nil)
				mockDatasetService.EXPECT().LoadItemData(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedResp: &dataset.ListDatasetItemsResponse{},
			expectedErr:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockAuth()
			tt.mockList()

			_, err := app.ListDatasetItems(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ... existing code ...

func TestDatasetApplicationImpl_ListDatasetItemsByVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl) // Assuming auditClient is used, add if necessary

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit, // Assuming auditClient is used
	}

	tests := []struct {
		name          string
		req           *dataset.ListDatasetItemsByVersionRequest
		mockSetup     func()
		expectedResp  *dataset.ListDatasetItemsByVersionResponse
		expectedErr   error
		checkResponse func(t *testing.T, resp *dataset.ListDatasetItemsByVersionResponse)
	}{
		{
			name: "正常场景：成功列出版本下的数据集条目",
			req: &dataset.ListDatasetItemsByVersionRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				VersionID:   int64(1),
				PageToken:   gptr.Of(""),
				PageSize:    gptr.Of(int32(10)),
			},
			mockSetup: func() {
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)                                   // Simplified auth mock
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil) // From authByDatasetID
				mockRepo.EXPECT().GetVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetVersion{ID: 1, SchemaID: 100, SnapshotStatus: entity.SnapshotStatusCompleted}, nil)
				// Mocking for listItemsByVersion internal calls - assuming it calls repo.ListItemsByVersion
				mockRepo.EXPECT().ListItemSnapshots(gomock.Any(), gomock.Any()).Return([]*entity.ItemSnapshot{{Snapshot: &entity.Item{}}}, &pagination.PageResult{}, errors.New("test error"))
				mockRepo.EXPECT().ListItems(gomock.Any(), gomock.Any()).Return([]*entity.Item{{}}, &pagination.PageResult{}, nil)
				mockDatasetService.EXPECT().LoadItemData(gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
				mockDatasetService.EXPECT().GetOrSetItemCountOfVersion(gomock.Any(), gomock.Any()).Return(int64(1), nil)
			},
			expectedResp: &dataset.ListDatasetItemsByVersionResponse{
				Items:         []*domain_dataset.DatasetItem{{ItemID: gptr.Of(int64(1)), Data: []*domain_dataset.FieldData{{Key: gptr.Of("test")}}}}, // Adjust based on actual conversion
				Total:         gptr.Of(int64(1)),
				NextPageToken: gptr.Of("next_token"),
			},
			expectedErr: nil,
			checkResponse: func(t *testing.T, resp *dataset.ListDatasetItemsByVersionResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, int64(1), *resp.Total)
				assert.Len(t, resp.Items, 1)
				// Add more specific checks for item content if necessary
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := app.ListDatasetItemsByVersion(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				// 使用 Contains 而不是 Equals 来处理可能的错误包装
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, resp) // 发生错误时，resp 通常为 nil
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				} else {
					// 对于复杂的响应，可能需要更细致的比较，这里使用 assert.Equal 作为默认比较
					// 如果 expectedResp 为 nil (例如在某些错误情况下)，则跳过比较
					if tt.expectedResp != nil {
						assert.Equal(t, tt.expectedResp.Total, resp.Total)
						assert.Equal(t, tt.expectedResp.NextPageToken, resp.NextPageToken)
						// 比较 Items 可能需要更复杂的逻辑，特别是如果顺序不重要或内容复杂
						// 这里简单比较长度，具体内容比较可以在 checkResponse 中实现
						assert.Len(t, resp.Items, len(tt.expectedResp.Items))
						// 如果需要深度比较 Items，可以取消注释并调整以下代码：
						// for i := range tt.expectedResp.Items {
						//    assert.Equal(t, tt.expectedResp.Items[i], resp.Items[i])
						// }
					}
				}
			}
		})
	}
}

// ... existing code ...

func TestDatasetApplicationImpl_BatchGetDatasetItemsByVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	// mockAudit := mock_audit.NewMockIAuditService(ctrl) // 如果需要审计，取消注释

	app := &DatasetApplicationImpl{
		auth: mockAuth,
		repo: mockRepo,
		svc:  mockDatasetService,
		// auditClient: mockAudit, // 如果需要审计，取消注释
	}

	tests := []struct {
		name          string
		req           *dataset.BatchGetDatasetItemsByVersionRequest
		mockSetup     func()
		expectedResp  *dataset.BatchGetDatasetItemsByVersionResponse
		expectedErr   error
		checkResponse func(t *testing.T, resp *dataset.BatchGetDatasetItemsByVersionResponse)
	}{
		{
			name: "正常场景：成功批量获取版本下的数据集条目",
			req: &dataset.BatchGetDatasetItemsByVersionRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
				VersionID:   int64(1),
				ItemIds:     []int64{10, 20},
			},
			mockSetup: func() {
				// Mock for authByDatasetID
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil) // From authByDatasetID
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				// Mock for GetVersion
				mockRepo.EXPECT().GetVersion(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetVersion{ID: 1, SchemaID: 100, SnapshotStatus: entity.SnapshotStatusCompleted}, nil)
				// Mock for ListItems
				mockRepo.EXPECT().ListItems(gomock.Any(), gomock.Any()).Return([]*entity.Item{{}}, &pagination.PageResult{}, nil)
				// Mock for LoadItemData
				mockDatasetService.EXPECT().LoadItemData(gomock.Any(), gomock.Any()).Return(nil)
				// Mock for GetSchema
				mockRepo.EXPECT().GetSchema(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.DatasetSchema{}, nil)
			},
			expectedResp: &dataset.BatchGetDatasetItemsByVersionResponse{
				Items: []*domain_dataset.DatasetItem{
					{ItemID: gptr.Of(int64(10))}, // 补充期望的 Item 内容
					{ItemID: gptr.Of(int64(20))}, // 补充期望的 Item 内容
				},
			},
			expectedErr: nil,
			checkResponse: func(t *testing.T, resp *dataset.BatchGetDatasetItemsByVersionResponse) {
				assert.NotNil(t, resp)
				// 根据实际情况添加更详细的断言
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := app.BatchGetDatasetItemsByVersion(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				// 使用 Contains 而不是 Equals 来处理可能的错误包装或特定错误类型的差异
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, resp) // 发生错误时，resp 通常为 nil
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				} else if tt.expectedResp != nil {
					// 对于复杂的响应，可能需要更细致的比较，这里使用 assert.Equal 作为默认比较
					// 如果 expectedResp 为 nil (例如在某些错误情况下)，则跳过比较
					assert.Equal(t, tt.expectedResp.Items, resp.Items) // 比较 Items
					// 您可能还需要比较其他字段，例如 Total, NextPageToken 等，如果它们在 BatchGetDatasetItemsByVersionResponse 中存在
				}
			}
		})
	}
}

// ... existing code ...

func TestDatasetApplicationImpl_ClearDatasetItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	// mockAudit := mock_audit.NewMockIAuditService(ctrl) // 如果需要审计，取消注释

	app := &DatasetApplicationImpl{
		auth: mockAuth,
		repo: mockRepo,
		svc:  mockDatasetService,
		// auditClient: mockAudit, // 如果需要审计，取消注释
	}

	tests := []struct {
		name         string
		req          *dataset.ClearDatasetItemRequest
		mockSetup    func()
		expectedResp *dataset.ClearDatasetItemResponse
		expectedErr  error
	}{
		{
			name: "正常场景：成功清空数据集条目",
			req: &dataset.ClearDatasetItemRequest{
				WorkspaceID: gptr.Of(int64(1)),
				DatasetID:   int64(1),
			},
			mockSetup: func() {
				// Mock for authByDatasetID
				mockRepo.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Dataset{}, nil) // From authByDatasetID
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				// Mock for svc.GetDataset
				mockDatasetService.EXPECT().GetDataset(gomock.Any(), gomock.Any(), gomock.Any()).Return(&service.DatasetWithSchema{Dataset: &entity.Dataset{Features: &entity.DatasetFeatures{}, Spec: &entity.DatasetSpec{MaxItemCount: 100}}, Schema: &entity.DatasetSchema{}}, nil)
				// Mock for svc.ClearDataset
				mockDatasetService.EXPECT().ClearDataset(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedResp: &dataset.ClearDatasetItemResponse{},
			expectedErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := app.ClearDatasetItem(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResp, resp)
			}
		})
	}
}

func TestDatasetApplicationImpl_ValidateDatasetItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockAuth := mock_auth.NewMockIAuthProvider(ctrl)
	mockRepo := mock_repo.NewMockIDatasetAPI(ctrl)
	mockDatasetService := mock_dataset.NewMockIDatasetAPI(ctrl)
	mockAudit := mock_audit.NewMockIAuditService(ctrl)

	app := &DatasetApplicationImpl{
		auth:        mockAuth,
		repo:        mockRepo,
		svc:         mockDatasetService,
		auditClient: mockAudit,
	}

	tests := []struct {
		name         string
		req          *dataset.ValidateDatasetItemsReq
		mockSetup    func()
		expectedResp *dataset.ValidateDatasetItemsResp
		expectedErr  assert.ErrorAssertionFunc
	}{
		{
			name: "permission denied",
			req:  &dataset.ValidateDatasetItemsReq{WorkspaceID: gptr.Of(int64(1))},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("permission denied"))
			},
			expectedErr: assert.Error,
		},
		{
			name: "valid",
			req: &dataset.ValidateDatasetItemsReq{
				WorkspaceID: gptr.Of(int64(1)),
				Items:       []*domain_dataset.DatasetItem{{ItemID: gptr.Of(int64(1))}},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockDatasetService.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).
					Return(&service.ValidateDatasetItemsResult{
						ValidItemIndices: []int32{0},
						ErrorGroups: []*entity.ItemErrorGroup{
							{
								Type:    gptr.Of(entity.ItemErrorType_ExceedDatasetCapacity),
								Summary: gptr.Of("capacity=100, current=0, to_add=1"),
							},
						},
					}, nil)
			},
			expectedResp: &dataset.ValidateDatasetItemsResp{
				ValidItemIndices: []int32{0},
				Errors: []*domain_dataset.ItemErrorGroup{
					{
						Type:    gptr.Of(domain_dataset.ItemErrorType_ExceedDatasetCapacity),
						Summary: gptr.Of("capacity=100, current=0, to_add=1"),
						Details: []*domain_dataset.ItemErrorDetail{},
					},
				},
			},
			expectedErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			resp, err := app.ValidateDatasetItems(context.Background(), tt.req)
			tt.expectedErr(t, err)
			assert.Equal(t, tt.expectedResp, resp)
		})
	}
}
