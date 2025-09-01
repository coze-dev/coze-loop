// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	mock_audit "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/dataset"
	domain_dataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/dataset"
	mock_auth "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	mock_repo "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service"
	mock_dataset "github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/service/mocks"
)

func TestDatasetApplicationImpl_ValidateDatasetItemsNew(t *testing.T) {
	t.Parallel()

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
		name        string
		req         *dataset.ValidateDatasetItemsReq
		mockSetup   func()
		expectedErr error
		expected    *dataset.ValidateDatasetItemsResp
	}{
		{
			name: "成功案例 - 有DatasetID",
			req: &dataset.ValidateDatasetItemsReq{
				WorkspaceID: gptr.Of(int64(123)),
				DatasetID:   gptr.Of(int64(456)),
				Items:       []*dataset.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockDatasetService.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).Return(&service.ValidateDatasetItemsResult{
					ValidItemIndices: []int32{0},
					ErrorGroups:      []*entity.ItemErrorGroup{},
				}, nil)
			},
			expected: &dataset.ValidateDatasetItemsResp{
				ValidItemIndices: []int32{0},
				Errors:           []*domain_dataset.ItemErrorGroup{},
			},
		},
		{
			name: "成功案例 - 无DatasetID",
			req: &dataset.ValidateDatasetItemsReq{
				WorkspaceID: gptr.Of(int64(123)),
				Items:       []*dataset.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
				mockDatasetService.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).Return(&service.ValidateDatasetItemsResult{
					ValidItemIndices: []int32{0},
					ErrorGroups:      []*entity.ItemErrorGroup{},
				}, nil)
			},
			expected: &dataset.ValidateDatasetItemsResp{
				ValidItemIndices: []int32{0},
				Errors:           []*domain_dataset.ItemErrorGroup{},
			},
		},
		{
			name: "鉴权失败 - 有DatasetID",
			req: &dataset.ValidateDatasetItemsReq{
				WorkspaceID: gptr.Of(int64(123)),
				DatasetID:   gptr.Of(int64(456)),
			},
			mockSetup: func() {
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			expectedErr: errors.New("auth failed"),
		},
		{
			name: "鉴权失败 - 无DatasetID",
			req: &dataset.ValidateDatasetItemsReq{
				WorkspaceID: 123,
			},
			mockSetup: func() {
				mockAuth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(errors.New("auth failed"))
			},
			expectedErr: errors.New("auth failed"),
		},
		{
			name: "服务层验证失败",
			req: &dataset.ValidateDatasetItemsReq{
				WorkspaceID: 123,
				DatasetID:   gptr.Of(int64(456)),
				Items:       []*domain_dataset.Item{{Data: gptr.Of("test")}},
			},
			mockSetup: func() {
				mockAuth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				mockDatasetService.EXPECT().ValidateDatasetItems(gomock.Any(), gomock.Any()).Return(nil, errors.New("validation failed"))
			},
			expectedErr: errors.New("validation failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := app.ValidateDatasetItems(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.ValidItemIndices, result.ValidItemIndices)
				assert.Equal(t, len(tt.expected.Errors), len(result.Errors))
			}
		})
	}
}