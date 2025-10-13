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
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
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
		{
			name: "create annotation with bool value successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeBool)),
					AnnotationValue:     "true",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewCreateAnnotationResponse(),
			wantErr: false,
		},
		{
			name: "create annotation with invalid bool value",
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
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeBool)),
					AnnotationValue:     "invalid_bool",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create annotation with double value successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeDouble)),
					AnnotationValue:     "3.14",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewCreateAnnotationResponse(),
			wantErr: false,
		},
		{
			name: "create annotation with invalid double value",
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
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeDouble)),
					AnnotationValue:     "invalid_double",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create annotation with category value successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeCategory)),
					AnnotationValue:     "category_value",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewCreateAnnotationResponse(),
			wantErr: false,
		},
		{
			name: "create annotation with permission denied",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).
					Return(assert.AnError)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "create annotation with benefit check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "create annotation with trace service failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().DeleteAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
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
		{
			name: "delete annotation with permission denied",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).
					Return(assert.AnError)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeleteAnnotationRequest{
					WorkspaceID: 1,
					Base:        &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "delete annotation with benefit check failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeleteAnnotationRequest{
					WorkspaceID: 1,
					Base:        &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "delete annotation with trace service failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationCreate, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().DeleteAnnotation(gomock.Any(), gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeleteAnnotationRequest{
					WorkspaceID: 1,
					Base:        &base.Base{Caller: "test"},
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

// 补充IngestTraces的边界测试场景
func TestOpenAPIApplication_IngestTraces_AdditionalScenarios(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
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
			name: "permission check fails",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("1").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "benefit check fails - insufficient capacity",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         false,
					StorageDuration:  3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("1").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "benefit check fails - account not available",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: false,
					IsEnough:         true,
					StorageDuration:  3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("1").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid workspace id format",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("invalid").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "nil request",
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
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: nil,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			}
			got, err := o.IngestTraces(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

// 补充CreateAnnotation的更多测试场景
func TestOpenAPIApplication_CreateAnnotation_AdditionalScenarios(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
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
			name: "create annotation with double value type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeDouble)),
					AnnotationValue:     "3.14",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewCreateAnnotationResponse(),
			wantErr: false,
		},
		{
			name: "create annotation with bool value type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeBool)),
					AnnotationValue:     "true",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    openapi.NewCreateAnnotationResponse(),
			wantErr: false,
		},
		{
			name: "create annotation with invalid double value",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeDouble)),
					AnnotationValue:     "invalid",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "create annotation with invalid bool value",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.CreateAnnotationRequest{
					WorkspaceID:         1,
					AnnotationValueType: ptr.Of(annotation.ValueType(loop_span.AnnotationValueTypeBool)),
					AnnotationValue:     "invalid",
					Base:                &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "benefit check fails",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
		{
			name: "trace service fails",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().CreateAnnotation(gomock.Any(), gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
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
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			}
			got, err := o.CreateAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

// 补充DeleteAnnotation的更多测试场景
func TestOpenAPIApplication_DeleteAnnotation_AdditionalScenarios(t *testing.T) {
	type fields struct {
		traceService service.ITraceService
		auth         rpc.IAuthProvider
		benefit      benefit.IBenefitService
		tenant       tenant.ITenantProvider
		workspace    workspace.IWorkSpaceProvider
		rateLimiter  limiter.IRateLimiterFactory
		traceConfig  config.ITraceConfig
		metrics      metrics.ITraceMetrics
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
			name: "benefit check fails",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeleteAnnotationRequest{
					WorkspaceID: 1,
					Base:        &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "trace service fails",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().DeleteAnnotation(gomock.Any(), gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("").AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				return fields{
					traceService: traceServiceMock,
					auth:         authMock,
					benefit:      benefitMock,
					tenant:       tenantMock,
					workspace:    workspaceMock,
					rateLimiter:  rateLimiterMock,
					traceConfig:  traceConfigMock,
					metrics:      metricsMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &openapi.DeleteAnnotationRequest{
					WorkspaceID: 1,
					Base:        &base.Base{Caller: "test"},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			}
			got, err := o.DeleteAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

// 测试validate和build函数
func TestOpenAPIApplication_validateIngestTracesReq(t *testing.T) {
	app := &OpenAPIApplication{}

	// 测试nil请求
	err := app.validateIngestTracesReq(context.Background(), nil)
	assert.Error(t, err)

	// 测试空spans
	err = app.validateIngestTracesReq(context.Background(), &openapi.IngestTracesRequest{
		Spans: []*span.InputSpan{},
	})
	assert.Error(t, err)

	// 测试不同workspace id的spans
	err = app.validateIngestTracesReq(context.Background(), &openapi.IngestTracesRequest{
		Spans: []*span.InputSpan{
			{WorkspaceID: "1"},
			{WorkspaceID: "2"},
		},
	})
	assert.Error(t, err)

	// 测试正常情况
	err = app.validateIngestTracesReq(context.Background(), &openapi.IngestTracesRequest{
		Spans: []*span.InputSpan{
			{WorkspaceID: "1"},
			{WorkspaceID: "1"},
		},
	})
	assert.NoError(t, err)
}

func TestOpenAPIApplication_validateIngestTracesReqByTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
	app := &OpenAPIApplication{
		traceConfig: traceConfigMock,
	}

	// 测试nil请求
	traceConfigMock.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(nil, nil)
	err := app.validateIngestTracesReqByTenant(context.Background(), "tenant", nil)
	assert.Error(t, err)

	// 测试超过最大span长度
	traceConfigMock.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(map[string]*config.IngestConfig{
		"tenant": {MaxSpanLength: 1},
	}, nil)
	err = app.validateIngestTracesReqByTenant(context.Background(), "tenant", &openapi.IngestTracesRequest{
		Spans: []*span.InputSpan{{}, {}},
	})
	assert.Error(t, err)

	// 测试正常情况
	traceConfigMock.EXPECT().GetTraceIngestTenantProducerCfg(gomock.Any()).Return(nil, nil)
	err = app.validateIngestTracesReqByTenant(context.Background(), "tenant", &openapi.IngestTracesRequest{
		Spans: []*span.InputSpan{{}},
	})
	assert.NoError(t, err)
}

func TestOpenAPIApplication_unpackTenant(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
	app := &OpenAPIApplication{
		tenant: tenantMock,
	}

	// 测试nil spans
	result := app.unpackTenant(context.Background(), nil)
	assert.Nil(t, result)

	// 测试正常情况
	tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("tenant1")
	result = app.unpackTenant(context.Background(), []*loop_span.Span{{SpanID: "test"}})
	assert.Len(t, result, 1)
	assert.Len(t, result["tenant1"], 1)
}
