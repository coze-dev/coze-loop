// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestOpenAPIApplication_ListTrajectoryOApi(t *testing.T) {
	t.Run("success with start_time and platform_type provided", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceServiceMock.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(&service.ListTrajectoryResponse{Trajectories: nil}, nil)

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			rateLimiter:  rateLimiterMock,
			traceConfig:  traceConfigMock,
		}

		start := time.Now().Add(-time.Hour).UnixMilli()
		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1", "trace-2"},
			StartTime:    ptr.Of(start),
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Data)
	})

	t.Run("success with platform_type not set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceServiceMock.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(&service.ListTrajectoryResponse{Trajectories: nil}, nil)

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			rateLimiter:  rateLimiterMock,
			traceConfig:  traceConfigMock,
		}

		start := time.Now().Add(-time.Hour).UnixMilli()
		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID: 123,
			TraceIds:    []string{"trace-1"},
			StartTime:   ptr.Of(start),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Data)
	})

	t.Run("success with start_time nil uses benefit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceConfigMock.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Nil()).Return(int64(3))
		benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{StorageDuration: 7}, nil)
		traceServiceMock.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(&service.ListTrajectoryResponse{Trajectories: nil}, nil)

		ctx := session.WithCtxUser(context.Background(), &session.User{ID: "user-1"})
		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			rateLimiter:  rateLimiterMock,
			traceConfig:  traceConfigMock,
			benefit:      benefitMock,
		}

		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1"},
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Data)
	})

	t.Run("success with nil service response", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceServiceMock.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(nil, nil)

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			rateLimiter:  rateLimiterMock,
			traceConfig:  traceConfigMock,
		}

		start := time.Now().Add(-time.Hour).UnixMilli()
		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1"},
			StartTime:    ptr.Of(start),
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Data)
	})

	t.Run("invalid request - nil", func(t *testing.T) {
		app := &OpenAPIApplication{}
		resp, err := app.ListTrajectoryOApi(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid request - workspace_id zero", func(t *testing.T) {
		app := &OpenAPIApplication{}
		resp, err := app.ListTrajectoryOApi(context.Background(), &openapi.ListTrajectoryOApiRequest{
			WorkspaceID: 0,
			TraceIds:    []string{"trace-1"},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid request - empty trace_ids", func(t *testing.T) {
		app := &OpenAPIApplication{}
		resp, err := app.ListTrajectoryOApi(context.Background(), &openapi.ListTrajectoryOApiRequest{
			WorkspaceID: 123,
			TraceIds:    []string{},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid request - empty trace_id string", func(t *testing.T) {
		app := &OpenAPIApplication{}
		resp, err := app.ListTrajectoryOApi(context.Background(), &openapi.ListTrajectoryOApiRequest{
			WorkspaceID: 123,
			TraceIds:    []string{""},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("permission error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(assert.AnError)

		app := &OpenAPIApplication{auth: authMock}

		start := time.Now().Add(-time.Hour).UnixMilli()
		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1"},
			StartTime:    ptr.Of(start),
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("rate limited", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)

		app := &OpenAPIApplication{
			auth:        authMock,
			rateLimiter: rateLimiterMock,
			traceConfig: traceConfigMock,
		}

		start := time.Now().Add(-time.Hour).UnixMilli()
		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1"},
			StartTime:    ptr.Of(start),
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("user missing when start_time nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)

		app := &OpenAPIApplication{
			auth:        authMock,
			rateLimiter: rateLimiterMock,
			traceConfig: traceConfigMock,
		}

		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1"},
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("service error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "open_api").Return(nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "123").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceServiceMock.EXPECT().ListTrajectory(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			rateLimiter:  rateLimiterMock,
			traceConfig:  traceConfigMock,
		}

		start := time.Now().Add(-time.Hour).UnixMilli()
		req := &openapi.ListTrajectoryOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace-1"},
			StartTime:    ptr.Of(start),
			PlatformType: ptr.Of("open_api"),
		}

		resp, err := app.ListTrajectoryOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
