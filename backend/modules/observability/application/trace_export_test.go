// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	confmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	tenantmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	svcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
)

func TestTraceApplication_ExportTracesToDataset(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTraceService := svcmock.NewMockITraceService(ctrl)
	mockTraceExportService := svcmock.NewMockITraceExportService(ctrl)
	mockAuthService := rpcmock.NewMockIAuthProvider(ctrl)
	mockTraceConfig := confmock.NewMockITraceConfig(ctrl)
	mockTenant := tenantmock.NewMockITenantProvider(ctrl)

	app := &TraceApplication{
		traceService:       mockTraceService,
		traceExportService: mockTraceExportService,
		authSvc:            mockAuthService,
		traceConfig:        mockTraceConfig,
		tenant:             mockTenant,
	}

	tests := []struct {
		name        string
		req         *trace.ExportTracesToDatasetRequest
		mockSetup   func()
		expectedErr error
		expected    *trace.ExportTracesToDatasetResponse
	}{
		{
			name: "成功案例",
			req: &trace.ExportTracesToDatasetRequest{
				WorkspaceID:  123,
				StartTime:    gptr.Of(time.Now().Add(-time.Hour).Unix()),
				EndTime:      gptr.Of(time.Now().Unix()),
				PlatformType: "coze-loop",
			},
			mockSetup: func() {
				mockTraceConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), "coze-loop").Return(30)
				mockAuthService.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), "123").Return(nil)
				mockTraceExportService.EXPECT().ExportTracesToDataset(gomock.Any(), gomock.Any()).Return(&service.ExportTracesToDatasetResponse{
					TaskID: "task-123",
				}, nil)
			},
			expected: &trace.ExportTracesToDatasetResponse{
				TaskID: gptr.Of("task-123"),
			},
		},
		{
			name: "鉴权失败",
			req: &trace.ExportTracesToDatasetRequest{
				WorkspaceID:  123,
				StartTime:    gptr.Of(time.Now().Add(-time.Hour).Unix()),
				EndTime:      gptr.Of(time.Now().Unix()),
				PlatformType: "coze-loop",
			},
			mockSetup: func() {
				mockTraceConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), "coze-loop").Return(30)
				mockAuthService.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), "123").Return(errors.New("auth failed"))
			},
			expectedErr: errors.New("auth failed"),
		},
		{
			name: "服务层导出失败",
			req: &trace.ExportTracesToDatasetRequest{
				WorkspaceID:  123,
				StartTime:    gptr.Of(time.Now().Add(-time.Hour).Unix()),
				EndTime:      gptr.Of(time.Now().Unix()),
				PlatformType: "coze-loop",
			},
			mockSetup: func() {
				mockTraceConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), "coze-loop").Return(30)
				mockAuthService.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), "123").Return(nil)
				mockTraceExportService.EXPECT().ExportTracesToDataset(gomock.Any(), gomock.Any()).Return(nil, errors.New("export failed"))
			},
			expectedErr: errors.New("export failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := app.ExportTracesToDataset(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				if tt.expected != nil && tt.expected.TaskID != nil {
					assert.Equal(t, *tt.expected.TaskID, *result.TaskID)
				}
			}
		})
	}
}

func TestTraceApplication_PreviewExportTracesToDataset(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockTraceService := svcmock.NewMockITraceService(ctrl)
	mockTraceExportService := svcmock.NewMockITraceExportService(ctrl)
	mockAuthService := rpcmock.NewMockIAuthProvider(ctrl)
	mockTraceConfig := confmock.NewMockITraceConfig(ctrl)
	mockTenant := tenantmock.NewMockITenantProvider(ctrl)

	app := &TraceApplication{
		traceService:       mockTraceService,
		traceExportService: mockTraceExportService,
		authSvc:            mockAuthService,
		traceConfig:        mockTraceConfig,
		tenant:             mockTenant,
	}

	tests := []struct {
		name        string
		req         *trace.PreviewExportTracesToDatasetRequest
		mockSetup   func()
		expectedErr error
		expected    *trace.PreviewExportTracesToDatasetResponse
	}{
		{
			name: "成功案例",
			req: &trace.PreviewExportTracesToDatasetRequest{
				WorkspaceID:  123,
				StartTime:    gptr.Of(time.Now().Add(-time.Hour).Unix()),
				EndTime:      gptr.Of(time.Now().Unix()),
				PlatformType: "coze-loop",
			},
			mockSetup: func() {
				mockTraceConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), "coze-loop").Return(30)
				mockAuthService.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), "123").Return(nil)
				mockTraceExportService.EXPECT().PreviewExportTracesToDataset(gomock.Any(), gomock.Any()).Return(&service.PreviewExportTracesToDatasetResponse{
					Items:  []service.DatasetItem{},
					Errors: []service.ItemErrorGroup{},
				}, nil)
			},
			expected: &trace.PreviewExportTracesToDatasetResponse{
				Items:  []*trace.DatasetItem{},
				Errors: []*trace.ItemErrorGroup{},
			},
		},
		{
			name: "鉴权失败",
			req: &trace.PreviewExportTracesToDatasetRequest{
				WorkspaceID:  123,
				StartTime:    gptr.Of(time.Now().Add(-time.Hour).Unix()),
				EndTime:      gptr.Of(time.Now().Unix()),
				PlatformType: "coze-loop",
			},
			mockSetup: func() {
				mockTraceConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), "coze-loop").Return(30)
				mockAuthService.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), "123").Return(errors.New("auth failed"))
			},
			expectedErr: errors.New("auth failed"),
		},
		{
			name: "服务层预览失败",
			req: &trace.PreviewExportTracesToDatasetRequest{
				WorkspaceID:  123,
				StartTime:    gptr.Of(time.Now().Add(-time.Hour).Unix()),
				EndTime:      gptr.Of(time.Now().Unix()),
				PlatformType: "coze-loop",
			},
			mockSetup: func() {
				mockTraceConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), "coze-loop").Return(30)
				mockAuthService.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), "123").Return(nil)
				mockTraceExportService.EXPECT().PreviewExportTracesToDataset(gomock.Any(), gomock.Any()).Return(nil, errors.New("preview failed"))
			},
			expectedErr: errors.New("preview failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.mockSetup()

			result, err := app.PreviewExportTracesToDataset(context.Background(), tt.req)

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.NotNil(t, result.Items)
				assert.NotNil(t, result.Errors)
			}
		})
	}
}