// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"compress/gzip"
	"context"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	limitermocks "github.com/coze-dev/coze-loop/backend/infra/limiter/mocks"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/base"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	traced "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/trace"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/openapi"
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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/otel"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
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
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("1")
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), gomock.Any()).Return(100, nil).AnyTimes()
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
					Spans: []*span.InputSpan{},
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
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
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
			want:    openapi.NewDeleteAnnotationResponse(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
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
				ctx:   context.Background(),
				event: &entity.AnnotationEvent{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			err = o.Send(tt.args.ctx, tt.args.event)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestOpenAPIApplication_OtelIngestTraces(t *testing.T) {
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
		req *openapi.OtelIngestTracesRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.OtelIngestTracesResponse
		wantErr      bool
	}{
		{
			name: "success with JSON format data",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         true,
					StorageDuration:  3,
					WhichIsEnough:    -1,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant")
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want: &openapi.OtelIngestTracesResponse{
				Body:        createValidProtoBufResponse(),
				ContentType: ptr.Of("application/x-protobuf"),
			},
			wantErr: false,
		},
		{
			name: "success with ProtoBuf format data",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         true,
					StorageDuration:  3,
					WhichIsEnough:    -1,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant")
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidProtoBufTraceData(),
					ContentType:     "application/x-protobuf",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want: &openapi.OtelIngestTracesResponse{
				Body:        createValidProtoBufResponse(),
				ContentType: ptr.Of("application/x-protobuf"),
			},
			wantErr: false,
		},
		{
			name: "success with gzip compressed data",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         true,
					StorageDuration:  3,
					WhichIsEnough:    -1,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant")
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createGzipData(createValidJSONTraceData()),
					ContentType:     "application/json",
					ContentEncoding: "gzip",
					WorkspaceID:     "123",
				},
			},
			want: &openapi.OtelIngestTracesResponse{
				Body:        createValidProtoBufResponse(),
				ContentType: ptr.Of("application/x-protobuf"),
			},
			wantErr: false,
		},

		{
			name: "success with default benefit when benefit service returns nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(nil, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant")
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want: &openapi.OtelIngestTracesResponse{
				Body:        createValidProtoBufResponse(),
				ContentType: ptr.Of("application/x-protobuf"),
			},
			wantErr: false,
		},
		{
			name: "fail with empty request",
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
		{
			name: "fail with empty body",
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            []byte{},
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with invalid content type",
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/xml",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with corrupted gzip data",
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            []byte("corrupted gzip data"),
					ContentType:     "application/json",
					ContentEncoding: "gzip",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with invalid JSON data",
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            []byte("invalid json"),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with invalid ProtoBuf data",
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            []byte("invalid protobuf"),
					ContentType:     "application/x-protobuf",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with invalid workspace ID",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "invalid").Return(nil)
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "invalid",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with permission check error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(assert.AnError)
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with insufficient capacity",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         false,
					StorageDuration:  3,
					WhichIsEnough:    -1,
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with account not available",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: false,
					IsEnough:         true,
					StorageDuration:  3,
					WhichIsEnough:    -1,
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "fail with trace service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(assert.AnError)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckIngestPermission(gomock.Any(), "123").Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				benefitMock.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{
					AccountAvailable: true,
					IsEnough:         true,
					StorageDuration:  3,
					WhichIsEnough:    -1,
				}, nil)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("test-tenant")
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
				req: &openapi.OtelIngestTracesRequest{
					Body:            createValidJSONTraceData(),
					ContentType:     "application/json",
					ContentEncoding: "",
					WorkspaceID:     "123",
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			got, err := o.OtelIngestTraces(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.NotNil(t, got)
				assert.NotNil(t, got.Body)
				assert.NotNil(t, got.ContentType)
				assert.Equal(t, "application/x-protobuf", *got.ContentType)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

// Test helper functions

// Test helper functions

// createValidJSONTraceData creates valid JSON format trace data for testing
func createValidJSONTraceData() []byte {
	req := &otel.ExportTraceServiceRequest{
		ResourceSpans: []*otel.ResourceSpans{
			{
				Resource: &otel.Resource{
					Attributes: []*otel.KeyValue{
						{
							Key: "service.name",
							Value: &otel.AnyValue{
								Value: &otel.AnyValue_StringValue{StringValue: "test-service"},
							},
						},
					},
				},
				ScopeSpans: []*otel.ScopeSpans{
					{
						Scope: &otel.InstrumentationScope{
							Name:    "test-scope",
							Version: "1.0.0",
						},
						Spans: []*otel.Span{
							{
								TraceId:           "1234567890abcdef1234567890abcdef",
								SpanId:            "1234567890abcdef",
								Name:              "test-span",
								StartTimeUnixNano: "1755076800000000000",
								EndTimeUnixNano:   "1640995201000000000",
								Attributes: []*otel.KeyValue{
									{
										Key: otel.OtelAttributeWorkSpaceID,
										Value: &otel.AnyValue{
											Value: &otel.AnyValue_StringValue{StringValue: "123"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, _ := sonic.Marshal(req)
	return data
}

// createValidProtoBufTraceData creates valid ProtoBuf format trace data for testing
func createValidProtoBufTraceData() []byte {
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key: "service.name",
							Value: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"},
							},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "test-scope",
							Version: "1.0.0",
						},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("1234567890abcdef"),
								SpanId:            []byte("12345678"),
								Name:              "test-span",
								StartTimeUnixNano: 1755076800000000000,
								EndTimeUnixNano:   1640995201000000000,
								Attributes: []*commonpb.KeyValue{
									{
										Key: otel.OtelAttributeWorkSpaceID,
										Value: &commonpb.AnyValue{
											Value: &commonpb.AnyValue_StringValue{StringValue: "123"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, _ := proto.Marshal(req)
	return data
}

// createGzipData compresses data using gzip for testing
func createGzipData(data []byte) []byte {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write(data)
	_ = writer.Close()
	return buf.Bytes()
}

// createValidProtoBufResponse creates a valid protobuf response for testing
func createValidProtoBufResponse() []byte {
	resp := &coltracepb.ExportTraceServiceResponse{
		PartialSuccess: &coltracepb.ExportTracePartialSuccess{
			RejectedSpans: 0,
			ErrorMessage:  "",
		},
	}
	data, _ := proto.Marshal(resp)
	return data
}

func TestOpenAPIApplication_ListSpansOApi(t *testing.T) {
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
		req *openapi.ListSpansOApiRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.ListSpansOApiResponse
		wantErr      bool
	}{
		{
			name: "list spans successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().ListSpansOApi(gomock.Any(), gomock.Any()).Return(&service.ListSpansOApiResp{
					Spans:         []*loop_span.Span{},
					NextPageToken: "next-token",
					HasMore:       true,
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"})
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpansOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
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
				req: &openapi.ListSpansOApiRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					PageSize:    ptr.Of(int32(10)),
				},
			},
			want: &openapi.ListSpansOApiResponse{
				Data: &openapi.ListSpansOApiData{
					Spans:         []*span.OutputSpan{},
					NextPageToken: "next-token",
					HasMore:       true,
				},
			},
			wantErr: false,
		},
		{
			name: "request is nil",
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
		{
			name: "page size exceeds limit",
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
				metricsMock.EXPECT().EmitListSpansOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListSpansOApiRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					PageSize:    ptr.Of(int32(1001)), // exceeds MaxListSpansLimit (1000)
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission check failure",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpansOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListSpansOApiRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				metricsMock.EXPECT().EmitListSpansOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListSpansOApiRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service layer error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().ListSpansOApi(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"})
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				metricsMock.EXPECT().EmitListSpansOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListSpansOApiRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
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
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			got, err := o.ListSpansOApi(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.Data.NextPageToken, got.Data.NextPageToken)
				assert.Equal(t, tt.want.Data.HasMore, got.Data.HasMore)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func TestOpenAPIApplication_SearchTraceOApi(t *testing.T) {
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
		req *openapi.SearchTraceOApiRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.SearchTraceOApiResponse
		wantErr      bool
	}{
		{
			name: "search trace by trace id successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().SearchTraceOApi(gomock.Any(), gomock.Any()).Return(&service.SearchTraceOApiResp{
					Spans: []*loop_span.Span{},
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				metricsMock.EXPECT().EmitSearchTraceOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.SearchTraceOApiRequest{
					WorkspaceID: 123,
					TraceID:     ptr.Of("test-trace-id"),
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					Limit:       100,
				},
			},
			want: &openapi.SearchTraceOApiResponse{
				Data: &openapi.SearchTraceOApiData{
					Spans: []*span.OutputSpan{},
				},
			},
			wantErr: false,
		},
		{
			name: "search trace by log id successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().SearchTraceOApi(gomock.Any(), gomock.Any()).Return(&service.SearchTraceOApiResp{
					Spans: []*loop_span.Span{},
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				metricsMock.EXPECT().EmitSearchTraceOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.SearchTraceOApiRequest{
					WorkspaceID: 123,
					Logid:       ptr.Of("test-log-id"),
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					Limit:       100,
				},
			},
			want: &openapi.SearchTraceOApiResponse{
				Data: &openapi.SearchTraceOApiData{
					Spans: []*span.OutputSpan{},
				},
			},
			wantErr: false,
		},
		{
			name: "request is nil",
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
		{
			name: "missing trace id and log id",
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
				metricsMock.EXPECT().EmitSearchTraceOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.SearchTraceOApiRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					Limit:       100,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "limit exceeds maximum",
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
				metricsMock.EXPECT().EmitSearchTraceOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.SearchTraceOApiRequest{
					WorkspaceID: 123,
					TraceID:     ptr.Of("test-trace-id"),
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					Limit:       1001, // exceeds MaxListSpansLimit (1000)
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission check failure",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitSearchTraceOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.SearchTraceOApiRequest{
					WorkspaceID: 123,
					TraceID:     ptr.Of("test-trace-id"),
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					Limit:       100,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service layer error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().SearchTraceOApi(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetOAPIQueryTenants(gomock.Any(), gomock.Any()).Return([]string{"tenant1"}).AnyTimes()
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				metricsMock.EXPECT().EmitSearchTraceOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.SearchTraceOApiRequest{
					WorkspaceID: 123,
					TraceID:     ptr.Of("test-trace-id"),
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					Limit:       100,
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
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			got, err := o.SearchTraceOApi(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.NotNil(t, got)
				assert.NotNil(t, got.Data.Spans)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

// TestOpenAPIApplication_ListTracesOApi tests the ListTracesOApi method
func TestOpenAPIApplication_ListTracesOApi(t *testing.T) {
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
		req *openapi.ListTracesOApiRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *openapi.ListTracesOApiResponse
		wantErr      bool
	}{
		{
			name: "list traces successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().GetTracesAdvanceInfo(gomock.Any(), gomock.Any()).Return(&service.GetTracesAdvanceInfoResp{
					Infos: []*loop_span.TraceAdvanceInfo{
						{
							TraceId:    "trace-1",
							InputCost:  100,
							OutputCost: 200,
						},
						{
							TraceId:    "trace-2",
							InputCost:  150,
							OutputCost: 250,
						},
					},
				}, nil)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 123,
					TraceIds:    []string{"trace-1", "trace-2"},
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want: &openapi.ListTracesOApiResponse{
				Data: &openapi.ListTracesData{
					Traces: []*traced.Trace{
						{
							TraceID: ptr.Of("trace-1"),
						},
						{
							TraceID: ptr.Of("trace-2"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid workspace ID",
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
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(0),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 0, // invalid workspace ID
					TraceIds:    []string{"trace-1"},
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty trace IDs",
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
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 123,
					TraceIds:    []string{}, // empty trace IDs
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "empty trace ID in list",
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
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 123,
					TraceIds:    []string{"trace-1", ""}, // empty trace ID in list
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission check failure",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(assert.AnError)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(limitermocks.NewMockIRateLimiter(ctrl)).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 123,
					TraceIds:    []string{"trace-1"},
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 123,
					TraceIds:    []string{"trace-1"},
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service layer error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				traceServiceMock.EXPECT().GetTracesAdvanceInfo(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckQueryPermission(gomock.Any(), "123", gomock.Any()).Return(nil)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetQueryWorkSpaceID(gomock.Any(), int64(123)).Return(int64(123))
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListTracesOapi(
					int64(123),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).AnyTimes()
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
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
				req: &openapi.ListTracesOApiRequest{
					WorkspaceID: 123,
					TraceIds:    []string{"trace-1"},
					StartTime:   time.Now().Add(-1 * time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
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
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			got, err := o.ListTracesOApi(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.NotNil(t, got)
				assert.NotNil(t, got.Data)
				assert.Equal(t, len(tt.want.Data.Traces), len(got.Data.Traces))
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

// TestOpenAPIApplication_unpackSpace tests the unpackSpace method
func TestOpenAPIApplication_unpackSpace(t *testing.T) {
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
		ctx   context.Context
		spans []*span.InputSpan
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         map[string][]*span.InputSpan
	}{
		{
			name: "nil spans",
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
				ctx:   context.Background(),
				spans: nil,
			},
			want: nil,
		},
		{
			name: "empty workspace ID skipped",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("")
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
				spans: []*span.InputSpan{
					{SpanID: "span1"},
				},
			},
			want: map[string][]*span.InputSpan{},
		},
		{
			name: "spans grouped by workspace ID",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("workspace1").Times(2)
				workspaceMock.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any()).Return("workspace2").Times(1)
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
				spans: []*span.InputSpan{
					{SpanID: "span1"},
					{SpanID: "span2"},
					{SpanID: "span3"},
				},
			},
			want: map[string][]*span.InputSpan{
				"workspace1": {
					{SpanID: "span1", WorkspaceID: "workspace1"},
					{SpanID: "span2", WorkspaceID: "workspace1"},
				},
				"workspace2": {
					{SpanID: "span3", WorkspaceID: "workspace2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			got := o.(*OpenAPIApplication).unpackSpace(tt.args.ctx, tt.args.spans)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, len(tt.want), len(got))
				for workspaceID, expectedSpans := range tt.want {
					actualSpans, exists := got[workspaceID]
					assert.True(t, exists)
					assert.Equal(t, len(expectedSpans), len(actualSpans))
				}
			}
		})
	}
}

// TestOpenAPIApplication_AllowBySpace tests the AllowBySpace method
func TestOpenAPIApplication_AllowBySpace(t *testing.T) {
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
		ctx         context.Context
		workspaceID int64
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         bool
	}{
		{
			name: "rate limit allowed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), "query_trace:qps:space_id:123", 1, gomock.Any()).Return(&limiter.Result{Allowed: true}, nil)
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
				ctx:         context.Background(),
				workspaceID: 123,
			},
			want: true,
		},
		{
			name: "rate limit exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), "query_trace:qps:space_id:123", 1, gomock.Any()).Return(&limiter.Result{Allowed: false}, nil)
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
				ctx:         context.Background(),
				workspaceID: 123,
			},
			want: false,
		},
		{
			name: "config error returns true",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(0, assert.AnError)
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
				ctx:         context.Background(),
				workspaceID: 123,
			},
			want: true,
		},
		{
			name: "rate limiter error returns true",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), "query_trace:qps:space_id:123", 1, gomock.Any()).Return(nil, assert.AnError)
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
				ctx:         context.Background(),
				workspaceID: 123,
			},
			want: true,
		},
		{
			name: "nil result returns true",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				traceServiceMock := servicemocks.NewMockITraceService(ctrl)
				authMock := rpcmocks.NewMockIAuthProvider(ctrl)
				benefitMock := benefitmocks.NewMockIBenefitService(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				workspaceMock := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				rateLimiterMock := limitermocks.NewMockIRateLimiterFactory(ctrl)
				rateLimiterFactoryMock := limitermocks.NewMockIRateLimiter(ctrl)
				rateLimiterMock.EXPECT().NewRateLimiter().Return(rateLimiterFactoryMock).AnyTimes()
				traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
				metricsMock := metricsmocks.NewMockITraceMetrics(ctrl)
				traceConfigMock.EXPECT().GetQueryMaxQPSBySpace(gomock.Any(), int64(123)).Return(100, nil)
				rateLimiterFactoryMock.EXPECT().AllowN(gomock.Any(), "query_trace:qps:space_id:123", 1, gomock.Any()).Return(nil, nil)
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
				ctx:         context.Background(),
				workspaceID: 123,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			o, err := NewOpenAPIApplication(fields.traceService, fields.auth, fields.benefit, fields.tenant, fields.workspace, fields.rateLimiter, fields.traceConfig, fields.metrics)
			assert.NoError(t, err)
			got := o.(*OpenAPIApplication).AllowBySpace(tt.args.ctx, tt.args.workspaceID)
			assert.Equal(t, tt.want, got)
		})
	}
}
