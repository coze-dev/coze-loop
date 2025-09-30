// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/collector"
	collectormocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/collector/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace"
	workspacemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"

	"go.uber.org/mock/gomock"
)

func TestOpenAPIApplication_IngestTraces(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
		collector    collector.ICollectorProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.IngestTracesRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.IngestTracesResponse
		wantErr      bool
	}{
		{
			name: "ingest traces successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         true,
					StorageDuration:  3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("t")
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("1").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				collectorMock := collectormocks.NewMockICollectorProvider(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(100, nil).AnyTimes()
				traceConfigMock.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(nil, nil).AnyTimes()
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
					collector:    collectorMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.IngestTracesRequest{
					Spans: []*span.InputSpan{
						{
							WorkspaceID: "1",
						},
					},
				},
			},
			want:    openapi.NewIngestTracesResponse(),
			wantErr: false,
		},
		{
			name: "ingest traces with no spans provided",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				collectorMock := collectormocks.NewMockICollectorProvider(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
					collector:    collectorMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.IngestTracesRequest{
					Spans: []*span.InputSpan{},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o := &OpenAPIApplication{
				traceService: fields.traceService,
				auth:         fields.auth,
				benefit:      fields.benefit,
				tenant:       fields.tenant,
				workspace:    fields.workspace,
				rateLimiter:  fields.rateLimiter.NewRateLimiter(),
				traceConfig:  fields.traceConfig,
				metrics:      fields.metrics,
				collector:    fields.collector,
			}
			got, err := o.IngestTraces(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAPIApplication_CreateAnnotation(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
		collector    collector.ICollectorProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.CreateAnnotationRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.CreateAnnotationResponse
		wantErr      bool
	}{
		{
			name: "create annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan) string {
					if len(spans) > 0 {
						switch spans[0].SpanID {
						case "span1":
						case "span2":
						case "span3":
							return "workspace2"
						}
					}
					return ""
				}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				collectorMock := collectormocks.NewMockICollectorProvider(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
					collector:    collectorMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeString)),
					AnnotationValue:     "test",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewCreateAnnotationResponse(),
			wantErr: false,
		},
		{
			name: "create annotation with invalid value type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan) string {
					if len(spans) > 0 {
						switch spans[0].SpanID {
						case "span1":
						case "span2":
						case "span3":
							return "workspace2"
						}
					}
					return ""
				}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				collectorMock := collectormocks.NewMockICollectorProvider(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
					collector:    collectorMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeLong)),
					AnnotationValue:     "invalid",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o := &OpenAPIApplication{
				traceService: fields.traceService,
				auth:         fields.auth,
				benefit:      fields.benefit,
				tenant:       fields.tenant,
				workspace:    fields.workspace,
				rateLimiter:  fields.rateLimiter.NewRateLimiter(),
				traceConfig:  fields.traceConfig,
				metrics:      fields.metrics,
				collector:    fields.collector,
			}
			got, err := o.CreateAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAPIApplication_DeleteAnnotation(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
		collector    collector.ICollectorProvider
	}
	type args struct {
		ctx context.Context
		req *openapi.DeleteAnnotationRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.DeleteAnnotationResponse
		wantErr      bool
	}{
		{
			name: "delete annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().DeleteAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan) string {
					if len(spans) > 0 {
						switch spans[0].SpanID {
						case "span1":
						case "span2":
						case "span3":
							return "workspace2"
						}
					}
					return ""
				}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				collectorMock := collectormocks.NewMockICollectorProvider(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
					collector:    collectorMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeleteAnnotationRequest{
					WorkspaceID: 1,
					Base:        &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewDeleteAnnotationResponse(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o := &OpenAPIApplication{
				traceService: fields.traceService,
				auth:         fields.auth,
				benefit:      fields.benefit,
				tenant:       fields.tenant,
				workspace:    fields.workspace,
				rateLimiter:  fields.rateLimiter.NewRateLimiter(),
				traceConfig:  fields.traceConfig,
				metrics:      fields.metrics,
				collector:    fields.collector,
			}
			got, err := o.DeleteAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOpenAPIApplication_Send(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
		collector    collector.ICollectorProvider
	}
	type args struct {
		ctx   context.Context
		event *entity.AnnotationEvent
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "send event successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().Send(gomock.Any(), gomock.Any()).Return(nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan) string {
					if len(spans) > 0 {
						switch spans[0].SpanID {
						case "span1":
						case "span2":
						case "span3":
							return "workspace2"
						}
					}
					return ""
				}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				collectorMock := collectormocks.NewMockICollectorProvider(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
					collector:    collectorMock,
				}
			},
			args: args{
				ctx:   context.Background(),
				event: &entity.AnnotationEvent{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o := &OpenAPIApplication{
				traceService: fields.traceService,
				auth:         fields.auth,
				benefit:      fields.benefit,
				tenant:       fields.tenant,
				workspace:    fields.workspace,
				rateLimiter:  fields.rateLimiter.NewRateLimiter(),
				traceConfig:  fields.traceConfig,
				metrics:      fields.metrics,
				collector:    fields.collector,
			}
			err := o.Send(tt.args.ctx, tt.args.event)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestNewOpenAPIApplication(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceServiceMock := servicemocks.NewMockITraceService(ctrl)
	authMock := rpcmocks.NewMockIAuthProvider(ctrl)
	benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
	tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
	workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	rateLimiterFactoryMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
	rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
	traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
	metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
	collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

	rateLimiterFactoryMock.EXPECT().NewRateLimiter().Return(rateLimiterMock)

	app, err := NewOpenAPIApplication(
		traceServiceMock,
		authMock,
		benefitMock,
		tenantMock,
		workspaceMock,
		rateLimiterFactoryMock,
		traceConfigMock,
		metricsMock,
		collectorMock,
	)

	assert.NoError(t, err)
	assert.NotNil(t, app)

	// 验证返回的实例类型
	openAPIApp, ok := app.(*OpenAPIApplication)
	assert.True(t, ok)
	assert.NotNil(t, openAPIApp.traceService)
	assert.NotNil(t, openAPIApp.auth)
	assert.NotNil(t, openAPIApp.benefit)
	assert.NotNil(t, openAPIApp.tenant)
	assert.NotNil(t, openAPIApp.workspace)
	assert.NotNil(t, openAPIApp.rateLimiter)
	assert.NotNil(t, openAPIApp.traceConfig)
	assert.NotNil(t, openAPIApp.metrics)
	assert.NotNil(t, openAPIApp.collector)
}
