// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
	collectormocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/collector/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	workspacemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestOpenAPIApplication_validateListPreSpanOApiReq(t *testing.T) {
	app := &OpenAPIApplication{}
	ctx := context.Background()

	// nil 请求
	assert.Error(t, app.validateListPreSpanOApiReq(ctx, nil))

	// workspace 无效
	assert.Error(t, app.validateListPreSpanOApiReq(ctx, &openapi.ListPreSpanOApiRequest{WorkspaceID: 0}))

	// traceID 为空
	assert.Error(t, app.validateListPreSpanOApiReq(ctx, &openapi.ListPreSpanOApiRequest{WorkspaceID: 1, TraceID: "", PreviousResponseID: ptr.Of("p"), SpanID: ptr.Of("s")}))

	// previousResponseID 为空
	assert.Error(t, app.validateListPreSpanOApiReq(ctx, &openapi.ListPreSpanOApiRequest{WorkspaceID: 1, TraceID: "t", PreviousResponseID: ptr.Of(""), SpanID: ptr.Of("s")}))

	// spanID 为空
	assert.Error(t, app.validateListPreSpanOApiReq(ctx, &openapi.ListPreSpanOApiRequest{WorkspaceID: 1, TraceID: "t", PreviousResponseID: ptr.Of("p"), SpanID: ptr.Of(""), StartTime: time.Now().UnixMilli()}))

	// StartTime 为 0，触发时间校验错误
	assert.Error(t, app.validateListPreSpanOApiReq(ctx, &openapi.ListPreSpanOApiRequest{WorkspaceID: 1, TraceID: "t", PreviousResponseID: ptr.Of("p"), SpanID: ptr.Of("s"), StartTime: 0}))

	// 正常场景
	now := time.Now().UnixMilli()
	start := now - int64(time.Hour/time.Millisecond)
	req := &openapi.ListPreSpanOApiRequest{WorkspaceID: 1, TraceID: "t", PreviousResponseID: ptr.Of("p"), SpanID: ptr.Of("s"), StartTime: start, PlatformType: ptr.Of("platform")}
	assert.NoError(t, app.validateListPreSpanOApiReq(ctx, req))
	// StartTime 会被修正为不超过边界（此处只验证无错误即可）
}

func TestOpenAPIApplication_buildListPreSpanOApiReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
	app := &OpenAPIApplication{workspace: workspaceMock, tenant: tenantMock}

	// 成功场景
	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(3)).Return("third-1").AnyTimes()
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"t1"})
	req := &openapi.ListPreSpanOApiRequest{
		WorkspaceID:        3,
		StartTime:          time.Now().Add(-time.Hour).UnixMilli(),
		TraceID:            "trace",
		SpanID:             ptr.Of("span"),
		PreviousResponseID: ptr.Of("prev"),
		PlatformType:       ptr.Of("platform"),
	}
	got, err := app.buildListPreSpanOApiReq(context.Background(), req)
	assert.NoError(t, err)
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(3), got.WorkspaceID)
		assert.Equal(t, "third-1", got.ThirdPartyWorkspaceID)
		assert.Equal(t, "trace", got.TraceID)
		assert.Equal(t, "span", got.SpanID)
		assert.Equal(t, "prev", got.PreviousResponseID)
		assert.Equal(t, []string{"t1"}, got.Tenants)
	}

	// 平台类型为 nil 时：仍返回 tenants（依据当前实现），ret.PlatformType 为零值
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"t2"})
	reqNilPlat := &openapi.ListPreSpanOApiRequest{
		WorkspaceID:        3,
		StartTime:          time.Now().Add(-time.Hour).UnixMilli(),
		TraceID:            "trace",
		SpanID:             ptr.Of("span"),
		PreviousResponseID: ptr.Of("prev"),
		PlatformType:       nil,
	}
	got2, err := app.buildListPreSpanOApiReq(context.Background(), reqNilPlat)
	assert.NoError(t, err)
	if assert.NotNil(t, got2) {
		assert.Equal(t, []string{"t2"}, got2.Tenants)
	}

	// tenants 为空返回错误
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{})
	_, err = app.buildListPreSpanOApiReq(context.Background(), &openapi.ListPreSpanOApiRequest{
		WorkspaceID:        3,
		StartTime:          time.Now().Add(-time.Hour).UnixMilli(),
		TraceID:            "t",
		SpanID:             ptr.Of("s"),
		PreviousResponseID: ptr.Of("p"),
		PlatformType:       ptr.Of("platform"),
	})
	assert.Error(t, err)
}

func TestOpenAPIApplication_ListPreSpanOApi(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceSvc := servicemocks.NewMockITraceService(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	tenantProv := tenantmocks.NewMockITenantProvider(ctrl)
	workspaceProv := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
	traceCfg := configmocks.NewMockITraceConfig(ctrl)
	metricsProv := metricsmocks.NewMockITraceMetrics(ctrl)
	collectorProv := collectormocks.NewMockICollectorProvider(ctrl)

	app := &OpenAPIApplication{
		traceService: traceSvc,
		auth:         auth,
		tenant:       tenantProv,
		workspace:    workspaceProv,
		rateLimiter:  rateLimiter,
		traceConfig:  traceCfg,
		metrics:      metricsProv,
		collector:    collectorProv,
	}

	// 公共期望：允许速率
	traceCfg.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil).AnyTimes()
	metricsProv.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	collectorProv.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// 成功场景
	auth.EXPECT().CheckQueryPermission(gomock.Any(), "1", gomock.Any()).Return(nil)
	rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
	workspaceProv.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(1)).Return("third-1")
	tenantProv.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"t1"})
	traceSvc.EXPECT().ListPreSpanOApi(gomock.Any(), gomock.Any()).Return(&service.ListPreSpanOApiResp{Spans: []*loop_span.Span{{SpanID: "s1"}}}, nil)

	req := &openapi.ListPreSpanOApiRequest{
		WorkspaceID:        1,
		TraceID:            "t",
		SpanID:             ptr.Of("s"),
		PreviousResponseID: ptr.Of("p"),
		StartTime:          time.Now().Add(-time.Hour).UnixMilli(),
		PlatformType:       ptr.Of("platform"),
	}
	resp, err := app.ListPreSpanOApi(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if assert.NotNil(t, resp.Spans) {
		assert.Equal(t, "s1", resp.Spans[0].GetSpanID())
	}

	// 速率限制超限
	auth.EXPECT().CheckQueryPermission(gomock.Any(), "1", gomock.Any()).Return(nil)
	rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
	resp, err = app.ListPreSpanOApi(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)

	// 校验失败（无 workspace）
	badReq := &openapi.ListPreSpanOApiRequest{}
	resp, err = app.ListPreSpanOApi(context.Background(), badReq)
	assert.Error(t, err)
	assert.Nil(t, resp)

	// 构建请求失败（tenants 为空）
	auth.EXPECT().CheckQueryPermission(gomock.Any(), "1", gomock.Any()).Return(nil)
	rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
	workspaceProv.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(1)).Return("third-1")
	tenantProv.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{})
	resp, err = app.ListPreSpanOApi(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestOpenAPIApplication_ListPreSpanOApi_NoPermission(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceSvc := servicemocks.NewMockITraceService(ctrl)
	auth := rpcmocks.NewMockIAuthProvider(ctrl)
	tenantProv := tenantmocks.NewMockITenantProvider(ctrl)
	workspaceProv := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
	traceCfg := configmocks.NewMockITraceConfig(ctrl)
	metricsProv := metricsmocks.NewMockITraceMetrics(ctrl)
	collectorProv := collectormocks.NewMockICollectorProvider(ctrl)

	app := &OpenAPIApplication{
		traceService: traceSvc,
		auth:         auth,
		tenant:       tenantProv,
		workspace:    workspaceProv,
		rateLimiter:  rateLimiter,
		traceConfig:  traceCfg,
		metrics:      metricsProv,
		collector:    collectorProv,
	}

	// 公共期望：允许速率
	traceCfg.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil).AnyTimes()
	metricsProv.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	collectorProv.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// 无权限场景
	auth.EXPECT().CheckQueryPermission(gomock.Any(), "1", gomock.Any()).Return(errors.New("permission denied"))
	req := &openapi.ListPreSpanOApiRequest{
		WorkspaceID:        1,
		TraceID:            "t",
		SpanID:             ptr.Of("s"),
		PreviousResponseID: ptr.Of("p"),
		StartTime:          time.Now().Add(-time.Hour).UnixMilli(),
		PlatformType:       ptr.Of("platform"),
	}
	_, err := app.ListPreSpanOApi(context.Background(), req)
	assert.Error(t, err)
}
