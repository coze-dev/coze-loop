// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmock "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	confmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	svcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestTraceApplication_UpsertTrajectoryConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockSvc.EXPECT().UpsertTrajectoryConfig(gomock.Any(), gomock.Any()).Return(nil)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth}
		resp, err := app.UpsertTrajectoryConfig(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &trace.UpsertTrajectoryConfigRequest{WorkspaceID: 1})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	t.Run("permission error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("denied"))
		app := &TraceApplication{authSvc: mockAuth}
		resp, err := app.UpsertTrajectoryConfig(context.Background(), &trace.UpsertTrajectoryConfigRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
	t.Run("user missing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		app := &TraceApplication{authSvc: mockAuth}
		resp, err := app.UpsertTrajectoryConfig(context.Background(), &trace.UpsertTrajectoryConfigRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
	t.Run("service error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockSvc.EXPECT().UpsertTrajectoryConfig(gomock.Any(), gomock.Any()).Return(assert.AnError)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth}
		resp, err := app.UpsertTrajectoryConfig(session.WithCtxUser(context.Background(), &session.User{ID: "u"}), &trace.UpsertTrajectoryConfigRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestTraceApplication_GetTrajectoryConfig(t *testing.T) {
	t.Run("success with filters", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockSvc.EXPECT().GetTrajectoryConfig(gomock.Any(), gomock.Any()).Return(&service.GetTrajectoryConfigResponse{Filters: nil}, nil)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth}
		resp, err := app.GetTrajectoryConfig(context.Background(), &trace.GetTrajectoryConfigRequest{WorkspaceID: 1})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	t.Run("permission error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("denied"))
		app := &TraceApplication{authSvc: mockAuth}
		resp, err := app.GetTrajectoryConfig(context.Background(), &trace.GetTrajectoryConfigRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
	t.Run("service error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockSvc.EXPECT().GetTrajectoryConfig(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth}
		resp, err := app.GetTrajectoryConfig(context.Background(), &trace.GetTrajectoryConfigRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestTraceApplication_ListTrajectory(t *testing.T) {
	t.Run("success with start_time provided", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mockSvc.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(&service.ListTrajectoryResponse{Trajectories: nil}, nil)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth}
		start := time.Now().Add(-time.Hour).UnixMilli()
		resp, err := app.ListTrajectory(context.Background(), &trace.ListTrajectoryRequest{WorkspaceID: 1, StartTime: ptr.Of(start)})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	t.Run("success with start_time nil uses benefit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockBenefit := benefitmock.NewMockIBenefitService(ctrl)
		mockConf := confmock.NewMockITraceConfig(ctrl)
		// 权限 OK
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		// user from ctx
		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "u"})
		// config 默认值
		mockConf.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(3)).AnyTimes()
		// benefit 返回覆盖 start_time
		mockBenefit.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{StorageDuration: 1, IsEnough: true, AccountAvailable: true}, nil)
		mockSvc.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(&service.ListTrajectoryResponse{Trajectories: nil}, nil)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth, benefit: mockBenefit, traceConfig: mockConf}
		resp, err := app.ListTrajectory(ctx, &trace.ListTrajectoryRequest{WorkspaceID: 1})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	t.Run("permission error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("denied"))
		app := &TraceApplication{authSvc: mockAuth}
		resp, err := app.ListTrajectory(context.Background(), &trace.ListTrajectoryRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
	t.Run("user missing when start_time nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		app := &TraceApplication{authSvc: mockAuth}
		resp, err := app.ListTrajectory(context.Background(), &trace.ListTrajectoryRequest{WorkspaceID: 1})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
	t.Run("service error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvc := svcmock.NewMockITraceService(ctrl)
		mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
		mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		start := time.Now().Add(-time.Hour).UnixMilli()
		mockSvc.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		app := &TraceApplication{traceService: mockSvc, authSvc: mockAuth}
		resp, err := app.ListTrajectory(context.Background(), &trace.ListTrajectoryRequest{WorkspaceID: 1, StartTime: ptr.Of(start)})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
