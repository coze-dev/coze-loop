// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"compress/gzip"
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
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
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"

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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("1").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan, claim *rpc.Claim) string {
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan, claim *rpc.Claim) string {
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationDelete, "1", true).Return(nil)
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().DeleteAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					StorageDuration: 3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan, claim *rpc.Claim) string {
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationDelete, "1", true).
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationDelete, "1", true).Return(nil)
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionAnnotationDelete, "1", true).Return(nil)
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, spans []*span.InputSpan, claim *rpc.Claim) string {
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
	authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("1").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         false,
					StorageDuration:  3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("1").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: false,
					IsEnough:         true,
					StorageDuration:  3,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("1").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("invalid").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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
				authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123").AnyTimes()
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("").AnyTimes()
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

	// Test nil spans.
	result := app.unpackTenant(context.Background(), nil)
	assert.Nil(t, result)

	// Test normal scenario.
	tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("tenant1")
	result = app.unpackTenant(context.Background(), []*loop_span.Span{{SpanID: "test"}})
	assert.Len(t, result, 1)
	assert.Len(t, result["tenant1"], 1)
}

func TestOpenAPIApplication_OtelIngestTraces(t *testing.T) {
	t.Run("successful otel ingest", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// Set expectations.
		authMock.EXPECT().CheckIngestPermission(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
			AccountAvailable: true,
			IsEnough:         true,
			StorageDuration:  3,
		}, nil).AnyTimes()
		tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("tenant1").AnyTimes()
		traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		app := &OpenAPIApplication{
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

		// Create test request.
		req := &openapi.OtelIngestTracesRequest{
			WorkspaceID:     "123",
			ContentType:     "application/json",
			ContentEncoding: "",
			Body:            []byte(`{"resourceSpans":[]}`),
		}

		resp, err := app.OtelIngestTraces(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("invalid request", func(t *testing.T) {
		app := &OpenAPIApplication{}

		// Nil request should return an error.
		resp, err := app.OtelIngestTraces(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)

		// Empty body should trigger validation error.
		resp, err = app.OtelIngestTraces(context.Background(), &openapi.OtelIngestTracesRequest{
			Body: []byte{},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid content type", func(t *testing.T) {
		app := &OpenAPIApplication{}

		resp, err := app.OtelIngestTraces(context.Background(), &openapi.OtelIngestTracesRequest{
			ContentType: "invalid/type",
			Body:        []byte("test"),
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestOpenAPIApplication_OtelIngestTraces_InvalidCases(t *testing.T) {
	t.Run("invalid request", func(t *testing.T) {
		app := &OpenAPIApplication{}

		// Nil request should return an error.
		resp, err := app.OtelIngestTraces(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)

		// Empty body should trigger validation error.
		resp, err = app.OtelIngestTraces(context.Background(), &openapi.OtelIngestTracesRequest{
			Body: []byte{},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("invalid content type", func(t *testing.T) {
		app := &OpenAPIApplication{}

		resp, err := app.OtelIngestTraces(context.Background(), &openapi.OtelIngestTracesRequest{
			ContentType: "invalid/type",
			Body:        []byte("test"),
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// 补充SearchTraceOApi测试
func TestOpenAPIApplication_SearchTraceOApi(t *testing.T) {
	t.Run("invalid request validation", func(t *testing.T) {
		app := &OpenAPIApplication{}

		// nil请求
		resp, err := app.SearchTraceOApi(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestOpenAPIApplication_SearchTraceOApi_InvalidCases(t *testing.T) {
	t.Run("invalid request validation", func(t *testing.T) {
		app := &OpenAPIApplication{}

		// nil请求
		resp, err := app.SearchTraceOApi(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestOpenAPIApplication_SearchTraceOApi_Success(t *testing.T) {
	t.Run("successful search", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// Set expectations.
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")
		tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1", "tenant2"})
		traceServiceMock.EXPECT().SearchTraceOApi(gomock.Any(), gomock.Any()).Return(&service.SearchTraceOApiResp{
			Spans: []*loop_span.Span{{SpanID: "test"}},
		}, nil)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			benefit:      benefitMock,
			tenant:       tenantMock,
			workspace:    workspaceMock,
			rateLimiter:  rateLimiter,
			traceConfig:  traceConfigMock,
			metrics:      metricsMock,
			collector:    collectorMock,
		}

		now := time.Now().UnixMilli()
		startTime := now - 3600000 // 1 hour ago
		endTime := now             // current time
		req := &openapi.SearchTraceOApiRequest{
			WorkspaceID:  123,
			TraceID:      ptr.Of("trace123"),
			StartTime:    startTime,
			EndTime:      endTime,
			Limit:        10,
			PlatformType: ptr.Of("platform"),
		}

		resp, err := app.SearchTraceOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// 设置期望
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			benefit:      benefitMock,
			tenant:       tenantMock,
			workspace:    workspaceMock,
			rateLimiter:  rateLimiter,
			traceConfig:  traceConfigMock,
			metrics:      metricsMock,
			collector:    collectorMock,
		}

		now := time.Now().UnixMilli()
		startTime := now - 3600000 // 1 hour ago
		endTime := now             // current time
		req := &openapi.SearchTraceOApiRequest{
			WorkspaceID:  123,
			TraceID:      ptr.Of("trace123"),
			StartTime:    startTime,
			EndTime:      endTime,
			Limit:        10,
			PlatformType: ptr.Of("platform"),
		}

		resp, err := app.SearchTraceOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestOpenAPIApplication_validateSearchTraceOApiReq(t *testing.T) {
	t.Parallel()
	app := &OpenAPIApplication{}
	ctx := context.Background()

	// nil request
	assert.Error(t, app.validateSearchTraceOApiReq(ctx, nil))

	now := time.Now().UnixMilli()
	validStart := now - int64(time.Hour/time.Millisecond)
	validReq := &openapi.SearchTraceOApiRequest{
		WorkspaceID:  1,
		TraceID:      ptr.Of("trace-id"),
		StartTime:    validStart,
		EndTime:      now,
		Limit:        10,
		PlatformType: ptr.Of("platform"),
	}

	// missing trace and log id
	missingIDs := *validReq
	missingIDs.TraceID = nil
	assert.Error(t, app.validateSearchTraceOApiReq(ctx, &missingIDs))

	// limit out of range (positive overflow)
	tooLargeLimit := *validReq
	tooLargeLimit.Limit = MaxListSpansLimit + 1
	assert.Error(t, app.validateSearchTraceOApiReq(ctx, &tooLargeLimit))

	// negative limit
	negativeLimit := *validReq
	negativeLimit.Limit = -1
	assert.Error(t, app.validateSearchTraceOApiReq(ctx, &negativeLimit))

	// invalid time range (zero values)
	invalidTime := *validReq
	invalidTime.StartTime = 0
	invalidTime.EndTime = 0
	assert.Error(t, app.validateSearchTraceOApiReq(ctx, &invalidTime))

	// valid request should pass
	assert.NoError(t, app.validateSearchTraceOApiReq(ctx, validReq))

	// start time later than end time
	invalidRange := *validReq
	invalidRange.StartTime = now + 1000
	assert.Error(t, app.validateSearchTraceOApiReq(ctx, &invalidRange))
}

func TestOpenAPIApplication_buildSearchTraceOApiReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
	workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	app := &OpenAPIApplication{
		tenant:    tenantMock,
		workspace: workspaceMock,
	}

	ctx := context.Background()
	now := time.Now().UnixMilli()
	start := now - int64(time.Hour/time.Millisecond)

	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(1)).Return("third-1")
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), loop_span.PlatformType("platform")).Return([]string{"tenant-a"})

	withPlatformReq := &openapi.SearchTraceOApiRequest{
		WorkspaceID:  1,
		TraceID:      ptr.Of("trace-id"),
		Logid:        ptr.Of("log-id"),
		StartTime:    start,
		EndTime:      now,
		Limit:        50,
		PlatformType: ptr.Of("platform"),
		SpanIds:      []string{"span-1", "span-2"},
	}

	res, err := app.buildSearchTraceOApiReq(ctx, withPlatformReq)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), res.WorkspaceID)
	assert.Equal(t, "third-1", res.ThirdPartyWorkspaceID)
	assert.Equal(t, loop_span.PlatformType("platform"), res.PlatformType)
	assert.True(t, res.WithDetail)
	assert.Equal(t, withPlatformReq.SpanIds, res.SpanIDs)
	assert.Equal(t, withPlatformReq.GetTraceID(), res.TraceID)
	assert.Equal(t, withPlatformReq.GetLogid(), res.LogID)
	assert.Equal(t, withPlatformReq.GetLimit(), res.Limit)

	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(2)).Return("third-2")
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{"tenant-b"})

	defaultPlatformReq := &openapi.SearchTraceOApiRequest{
		WorkspaceID: 2,
		TraceID:     ptr.Of("trace-id-2"),
		StartTime:   start,
		EndTime:     now,
		Limit:       5,
	}

	res2, err := app.buildSearchTraceOApiReq(ctx, defaultPlatformReq)
	assert.NoError(t, err)
	assert.Equal(t, loop_span.PlatformCozeLoop, res2.PlatformType)
	assert.Empty(t, res2.SpanIDs)

	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(3)).Return("third-3")
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), loop_span.PlatformCozeLoop).Return([]string{})

	_, err = app.buildSearchTraceOApiReq(ctx, &openapi.SearchTraceOApiRequest{
		WorkspaceID: 3,
		TraceID:     ptr.Of("trace-id-3"),
		StartTime:   start,
		EndTime:     now,
		Limit:       1,
	})
	assert.Error(t, err)
}

// 补充validateSearchTraceTreeOApiReq的单元测试
func TestOpenAPIApplication_validateSearchTraceTreeOApiReq(t *testing.T) {
	app := &OpenAPIApplication{}

	// 测试nil请求
	err := app.validateSearchTraceTreeOApiReq(context.Background(), nil)
	assert.Error(t, err)

	// 测试空trace_id
	err = app.validateSearchTraceTreeOApiReq(context.Background(), &openapi.SearchTraceTreeOApiRequest{
		TraceID: ptr.Of(""),
	})
	assert.Error(t, err)

	// 测试超过最大限制
	err = app.validateSearchTraceTreeOApiReq(context.Background(), &openapi.SearchTraceTreeOApiRequest{
		TraceID: ptr.Of("test-trace-id"),
		Limit:   MaxTraceTreeLength + 1,
	})
	assert.Error(t, err)

	// 测试负限制
	err = app.validateSearchTraceTreeOApiReq(context.Background(), &openapi.SearchTraceTreeOApiRequest{
		TraceID: ptr.Of("test-trace-id"),
		Limit:   -1,
	})
	assert.Error(t, err)

	// 测试正常情况
	startTime := time.Now().UnixMilli()
	endTime := time.Now().Add(1 * time.Hour).UnixMilli() // 结束时间晚于开始时间
	err = app.validateSearchTraceTreeOApiReq(context.Background(), &openapi.SearchTraceTreeOApiRequest{
		TraceID:   ptr.Of("test-trace-id"),
		Limit:     10,
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	assert.NoError(t, err)

	// 测试日期验证错误 - 开始时间大于结束时间
	startTime = time.Now().UnixMilli()
	endTime = time.Now().Add(-1 * time.Hour).UnixMilli() // 结束时间早于开始时间
	err = app.validateSearchTraceTreeOApiReq(context.Background(), &openapi.SearchTraceTreeOApiRequest{
		TraceID:   ptr.Of("test-trace-id"),
		Limit:     10,
		StartTime: &startTime,
		EndTime:   &endTime,
	})
	assert.Error(t, err) // 开始时间大于结束时间会返回错误
}

// 补充buildSearchTraceTreeOApiReq的单元测试
func TestOpenAPIApplication_buildSearchTraceTreeOApiReq(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
	workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
	app := &OpenAPIApplication{
		tenant:    tenantMock,
		workspace: workspaceMock,
	}

	// 测试正常情况
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1", "tenant2"})
	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")

	req := &openapi.SearchTraceTreeOApiRequest{
		WorkspaceID:  ptr.Of(int64(123)),
		TraceID:      ptr.Of("test-trace-id"),
		StartTime:    ptr.Of(time.Now().Add(-1 * time.Hour).UnixMilli()),
		EndTime:      ptr.Of(time.Now().UnixMilli()),
		Limit:        10,
		PlatformType: ptr.Of(common.PlatformType("platform")),
		Filters: &filter.FilterFields{
			FilterFields: []*filter.FilterField{
				{
					FieldName: ptr.Of("key1"),
					QueryType: ptr.Of(filter.QueryTypeEq),
					Values:    []string{"value1"},
				},
			},
		},
	}

	result, err := app.buildSearchTraceTreeOApiReq(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(123), result.WorkspaceID)
	assert.Equal(t, "third-party-123", result.ThirdPartyWorkspaceID)
	assert.Equal(t, "test-trace-id", result.TraceID)
	assert.Equal(t, int32(10), result.Limit)
	assert.False(t, result.WithDetail)
	assert.Len(t, result.Tenants, 2)

	// Test case without providing a platform type.
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"})
	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")

	req2 := &openapi.SearchTraceTreeOApiRequest{
		WorkspaceID: ptr.Of(int64(123)),
		TraceID:     ptr.Of("test-trace-id"),
		Limit:       10,
	}

	result2, err := app.buildSearchTraceTreeOApiReq(context.Background(), req2)
	assert.NoError(t, err)
	assert.NotNil(t, result2)
	assert.Equal(t, loop_span.PlatformCozeLoop, result2.PlatformType)

	// Test case when no tenants are returned.
	tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{})
	workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")

	result3, err := app.buildSearchTraceTreeOApiReq(context.Background(), req)
	assert.Error(t, err)
	assert.Nil(t, result3)
}

// Add comprehensive unit tests for SearchTraceTreeOApi.
func TestOpenAPIApplication_SearchTraceTreeOApi(t *testing.T) {
	t.Run("successful search trace tree", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// Set expectations.
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")
		tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1", "tenant2"})
		traceServiceMock.EXPECT().SearchTraceOApi(gomock.Any(), gomock.Any()).Return(&service.SearchTraceOApiResp{
			Spans: []*loop_span.Span{{SpanID: "test"}},
		}, nil)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			benefit:      benefitMock,
			tenant:       tenantMock,
			workspace:    workspaceMock,
			rateLimiter:  rateLimiter,
			traceConfig:  traceConfigMock,
			metrics:      metricsMock,
			collector:    collectorMock,
		}

		now := time.Now().UnixMilli()
		startTime := now - 3600000 // 1 hour ago
		endTime := now             // current time
		req := &openapi.SearchTraceTreeOApiRequest{
			WorkspaceID:  ptr.Of(int64(123)),
			TraceID:      ptr.Of("trace123"),
			StartTime:    &startTime,
			EndTime:      &endTime,
			Limit:        10,
			PlatformType: ptr.Of(common.PlatformType("platform")),
		}

		resp, err := app.SearchTraceTreeOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotNil(t, resp.Data)
		assert.NotNil(t, resp.Data.TracesAdvanceInfo)
		assert.NotNil(t, resp.Data.TracesAdvanceInfo.Tokens)
	})

	t.Run("invalid request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// Set metrics and collector mocks to avoid panics when testing a nil request.
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// Set expectations for the calls triggered inside the deferred function.
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			metrics:   metricsMock,
			collector: collectorMock,
		}

		// A nil request should return before the deferred function executes to prevent panics.
		resp, err := app.SearchTraceTreeOApi(context.Background(), nil)
		assert.Error(t, err)
		assert.Nil(t, resp)

		// An empty trace_id should trigger validation while still executing the deferred function.
		resp, err = app.SearchTraceTreeOApi(context.Background(), &openapi.SearchTraceTreeOApiRequest{
			TraceID: ptr.Of(""),
			Limit:   10, // Limit is a required field.
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("permission denied", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(assert.AnError)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			auth:      authMock,
			metrics:   metricsMock,
			collector: collectorMock,
		}

		now := time.Now().UnixMilli()
		start := now - 3600000
		end := now
		req := &openapi.SearchTraceTreeOApiRequest{
			WorkspaceID:  ptr.Of(int64(123)),
			TraceID:      ptr.Of("trace123"),
			StartTime:    &start,
			EndTime:      &end,
			PlatformType: ptr.Of(common.PlatformType("platform")),
		}

		resp, err := app.SearchTraceTreeOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			auth:        authMock,
			rateLimiter: rateLimiter,
			traceConfig: traceConfigMock,
			metrics:     metricsMock,
			collector:   collectorMock,
		}

		now := time.Now().UnixMilli()
		start := now - 3600000
		end := now
		req := &openapi.SearchTraceTreeOApiRequest{
			WorkspaceID:  ptr.Of(int64(123)),
			TraceID:      ptr.Of("trace123"),
			StartTime:    &start,
			EndTime:      &end,
			PlatformType: ptr.Of(common.PlatformType("platform")),
		}

		resp, err := app.SearchTraceTreeOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("build request failed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{}) // Empty tenants should trigger an error.
		workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			auth:        authMock,
			tenant:      tenantMock,
			workspace:   workspaceMock,
			rateLimiter: rateLimiter,
			traceConfig: traceConfigMock,
			metrics:     metricsMock,
			collector:   collectorMock,
		}

		now := time.Now().UnixMilli()
		start := now - 3600000
		end := now
		req := &openapi.SearchTraceTreeOApiRequest{
			WorkspaceID:  ptr.Of(int64(123)),
			TraceID:      ptr.Of("trace123"),
			StartTime:    &start,
			EndTime:      &end,
			PlatformType: ptr.Of(common.PlatformType("platform")),
		}

		resp, err := app.SearchTraceTreeOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("trace service failed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// Set expectations.
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("third-party-123")
		tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1", "tenant2"})
		traceServiceMock.EXPECT().SearchTraceOApi(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			benefit:      benefitMock,
			tenant:       tenantMock,
			workspace:    workspaceMock,
			rateLimiter:  rateLimiter,
			traceConfig:  traceConfigMock,
			metrics:      metricsMock,
			collector:    collectorMock,
		}

		now := time.Now().UnixMilli()
		start := now - 3600000
		end := now
		req := &openapi.SearchTraceTreeOApiRequest{
			WorkspaceID:  ptr.Of(int64(123)),
			TraceID:      ptr.Of("trace123"),
			StartTime:    &start,
			EndTime:      &end,
			PlatformType: ptr.Of(common.PlatformType("platform")),
		}

		resp, err := app.SearchTraceTreeOApi(context.Background(), req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// Add unit tests for ListSpansOApi.
func TestOpenAPIApplication_ListSpansOApi(t *testing.T) {
	t.Run("successful list spans", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// 设置期望
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", "platform").Return(nil)
		rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiter).AnyTimes()
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil)
		tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"})
		workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), int64(123)).Return("123")
		traceServiceMock.EXPECT().ListSpansOApi(gomock.Any(), gomock.Any()).Return(&service.ListSpansOApiResp{
			Spans:         []*loop_span.Span{{SpanID: "test"}},
			NextPageToken: "next_token",
			HasMore:       true,
		}, nil)
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			benefit:      benefitMock,
			tenant:       tenantMock,
			workspace:    workspaceMock,
			rateLimiter:  rateLimiter,
			traceConfig:  traceConfigMock,
			metrics:      metricsMock,
			collector:    collectorMock,
		}

		now := time.Now().UnixMilli()
		startTime := now - 3600000 // 1 hour ago
		endTime := now             // current time
		pageSize := int32(20)
		req := &openapi.ListSpansOApiRequest{
			WorkspaceID:  123,
			StartTime:    startTime,
			EndTime:      endTime,
			PageSize:     &pageSize,
			PageToken:    ptr.Of("token"),
			PlatformType: ptr.Of("platform"),
			SpanListType: ptr.Of(common.SpanListTypeRootSpan),
			OrderBys:     []*common.OrderBy{{Field: ptr.Of("start_time")}},
		}

		resp, err := app.ListSpansOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.True(t, resp.Data.HasMore)
		assert.Equal(t, "next_token", resp.Data.NextPageToken)
	})
}

// 补充ListTracesOApi测试
func TestOpenAPIApplication_ListTracesOApi(t *testing.T) {
	t.Run("successful list traces", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceServiceMock := servicemocks.NewMockITraceService(ctrl)
		authMock := rpcmocks.NewMockIAuthProvider(ctrl)
		authMock.EXPECT().GetClaim(gomock.Any()).Return(nil).AnyTimes()
		benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
		rateLimiter := limitermocks.NewMockIRateLimiter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// 设置期望
		authMock.EXPECT().CheckQueryPermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiter).AnyTimes()
		rateLimiter.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil).AnyTimes()
		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), gomock.Any()).Return(10, nil).AnyTimes()
		tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"}).AnyTimes()
		workspaceMock.EXPECT().GetThirdPartyQueryWorkSpaceID(gomock.Any(), gomock.Any()).Return("123").AnyTimes()
		traceServiceMock.EXPECT().GetTracesAdvanceInfo(gomock.Any(), gomock.Any()).Return(&service.GetTracesAdvanceInfoResp{
			Infos: []*loop_span.TraceAdvanceInfo{{TraceId: "trace123"}},
		}, nil).AnyTimes()
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			traceService: traceServiceMock,
			auth:         authMock,
			benefit:      benefitMock,
			tenant:       tenantMock,
			workspace:    workspaceMock,
			rateLimiter:  rateLimiter,
			traceConfig:  traceConfigMock,
			metrics:      metricsMock,
			collector:    collectorMock,
		}

		// 使用当前时间，避免日期验证错误
		now := time.Now().UnixMilli()
		startTime := now - 3600000 // 1 hour ago
		endTime := now             // current time
		_ = startTime
		_ = endTime
		_ = now

		req := &openapi.ListTracesOApiRequest{
			WorkspaceID:  123,
			TraceIds:     []string{"trace123", "trace456"},
			StartTime:    startTime,
			EndTime:      endTime,
			PlatformType: ptr.Of("platform"),
		}

		resp, err := app.ListTracesOApi(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Data.Traces, 1)
	})

	t.Run("invalid request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// 创建最小化的app，避免nil指针
		metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
		collectorMock := collectormocks.NewMockICollectorProvider(ctrl)

		// 设置期望 - 这些会在defer函数中被调用
		metricsMock.EXPECT().EmitTraceOapi(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
		collectorMock.EXPECT().CollectTraceOpenAPIEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		app := &OpenAPIApplication{
			metrics:   metricsMock,
			collector: collectorMock,
		}

		// nil请求 - 避免nil指针，提供一个空请求
		resp, err := app.ListTracesOApi(context.Background(), &openapi.ListTracesOApiRequest{})
		assert.Error(t, err)
		assert.Nil(t, resp)

		// 无效workspace id
		resp, err = app.ListTracesOApi(context.Background(), &openapi.ListTracesOApiRequest{
			WorkspaceID: 0,
			TraceIds:    []string{"trace123"},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)

		// 空trace ids
		resp, err = app.ListTracesOApi(context.Background(), &openapi.ListTracesOApiRequest{
			WorkspaceID: 123,
			TraceIds:    []string{},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)

		// 空trace id
		resp, err = app.ListTracesOApi(context.Background(), &openapi.ListTracesOApiRequest{
			WorkspaceID: 123,
			TraceIds:    []string{""},
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// Add unit tests for AllowByKey.
func TestOpenAPIApplication_AllowByKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	t.Run("allow by key - success", func(t *testing.T) {
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)

		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "test_key").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "test_key", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)

		app := &OpenAPIApplication{
			traceConfig: traceConfigMock,
			rateLimiter: rateLimiterMock,
		}

		result := app.AllowByKey(context.Background(), "test_key")
		assert.True(t, result)
	})

	t.Run("allow by key - rate limited", func(t *testing.T) {
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)

		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "test_key").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "test_key", 1, gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)

		app := &OpenAPIApplication{
			traceConfig: traceConfigMock,
			rateLimiter: rateLimiterMock,
		}

		result := app.AllowByKey(context.Background(), "test_key")
		assert.False(t, result)
	})

	t.Run("allow by key - config error", func(t *testing.T) {
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)

		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "test_key").Return(0, assert.AnError)

		app := &OpenAPIApplication{
			traceConfig: traceConfigMock,
			rateLimiter: rateLimiterMock,
		}

		result := app.AllowByKey(context.Background(), "test_key")
		assert.True(t, result) // Defaults to allowing requests when an error occurs.
	})

	t.Run("allow by key - rate limiter error", func(t *testing.T) {
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		rateLimiterMock := limitermocks.NewMockIRateLimiter(ctrl)

		traceConfigMock.EXPECT().GetQueryMaxQPS(gomock.Any(), "test_key").Return(10, nil)
		rateLimiterMock.EXPECT().AllowN(gomock.Any(), "test_key", 1, gomock.Any()).Return(nil, assert.AnError)

		app := &OpenAPIApplication{
			traceConfig: traceConfigMock,
			rateLimiter: rateLimiterMock,
		}

		result := app.AllowByKey(context.Background(), "test_key")
		assert.True(t, result) // Defaults to allowing requests when an error occurs.
	})
}

// 补充辅助函数测试
func TestUnmarshalOtelSpan(t *testing.T) {
	t.Run("protobuf content type", func(t *testing.T) {
		// 创建protobuf数据
		req := &coltracepb.ExportTraceServiceRequest{
			ResourceSpans: []*tracepb.ResourceSpans{{}},
		}
		data, err := proto.Marshal(req)
		assert.NoError(t, err)

		result, err := unmarshalOtelSpan(data, "application/x-protobuf")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("json content type", func(t *testing.T) {
		jsonData := []byte(`{"resourceSpans":[]}`)
		result, err := unmarshalOtelSpan(jsonData, "application/json")
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("unsupported content type", func(t *testing.T) {
		result, err := unmarshalOtelSpan([]byte("test"), "text/plain")
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("invalid json", func(t *testing.T) {
		result, err := unmarshalOtelSpan([]byte("invalid json"), "application/json")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestUngzip(t *testing.T) {
	t.Run("no gzip encoding", func(t *testing.T) {
		data := []byte("test data")
		result, err := ungzip("", data)
		assert.NoError(t, err)
		assert.Equal(t, data, result)
	})

	t.Run("gzip encoding", func(t *testing.T) {
		original := []byte("test data to compress")
		var compressed bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressed)
		_, err := gzipWriter.Write(original)
		assert.NoError(t, err)
		err = gzipWriter.Close()
		assert.NoError(t, err)
		result, err := ungzip("gzip", compressed.Bytes())
		assert.NoError(t, err)
		assert.Equal(t, original, result)
	})

	t.Run("invalid gzip data", func(t *testing.T) {
		result, err := ungzip("gzip", []byte("invalid gzip data"))
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
