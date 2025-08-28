// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestTraceServiceImpl_GetTracesAdvanceInfo(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *GetTracesAdvanceInfoReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *GetTracesAdvanceInfoResp
		wantErr      bool
	}{
		{
			name: "get traces advance info successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(loop_span.SpanList{{
					TraceID: "123",
					SpanID:  "234",
				}}, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTracesAdvanceInfoReq{
					WorkspaceID:  1,
					PlatformType: loop_span.PlatformCozeLoop,
					Traces: []*TraceQueryParam{{
						TraceID:   "123",
						StartTime: 0,
						EndTime:   0,
					}},
				},
			},
			want: &GetTracesAdvanceInfoResp{
				Infos: []*loop_span.TraceAdvanceInfo{{
					TraceId:    "123",
					InputCost:  0,
					OutputCost: 0,
				}},
			},
		},
		{
			name: "get traces advance info successfully with processor",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(loop_span.SpanList{{
					TraceID:     "123",
					SpanID:      "234",
					WorkspaceID: "123",
				}}, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock,
					nil,
					nil,
					[]span_processor.Factory{span_processor.NewCheckProcessorFactory()},
					nil,
					nil,
					nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTracesAdvanceInfoReq{
					WorkspaceID:  123,
					PlatformType: loop_span.PlatformCozeLoop,
					Traces: []*TraceQueryParam{{
						TraceID:   "123",
						StartTime: 0,
						EndTime:   0,
					}},
				},
			},
			want: &GetTracesAdvanceInfoResp{
				Infos: []*loop_span.TraceAdvanceInfo{{
					TraceId:    "123",
					InputCost:  0,
					OutputCost: 0,
				}},
			},
		},
		{
			name: "get traces advance info failed due to repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					metrics:        metricsMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTracesAdvanceInfoReq{
					WorkspaceID:  1,
					PlatformType: loop_span.PlatformCozeLoop,
					Traces: []*TraceQueryParam{{
						TraceID:   "123",
						StartTime: 0,
						EndTime:   0,
					}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			got, err := r.GetTracesAdvanceInfo(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTraceServiceImpl_IngestTraces(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *IngestTracesReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "ingest traces successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				producerMock := mqmocks.NewMockITraceProducer(ctrl)
				producerMock.EXPECT().IngestSpans(gomock.Any(), gomock.Any()).Return(nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceProducer:  producerMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &IngestTracesReq{
					TTL: loop_span.TTL3d,
					Spans: loop_span.SpanList{{
						TraceID:     "123",
						SpanID:      "234",
						WorkspaceID: "1",
					}},
				},
			},
			wantErr: false,
		},
		{
			name: "ingest traces failed due to producer error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				producerMock := mqmocks.NewMockITraceProducer(ctrl)
				producerMock.EXPECT().IngestSpans(gomock.Any(), gomock.Any()).Return(fmt.Errorf("producer error"))
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceProducer:  producerMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &IngestTracesReq{
					TTL: loop_span.TTL3d,
					Spans: loop_span.SpanList{{
						TraceID:     "123",
						SpanID:      "234",
						WorkspaceID: "1",
					}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			err := r.IngestTraces(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceServiceImpl_GetTracesMetaInfo(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *GetTracesMetaInfoReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *GetTracesMetaInfoResp
		wantErr      bool
	}{
		{
			name: "get traces meta info successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetTraceFieldMetaInfo(gomock.Any()).Return(&config.TraceFieldMetaInfoCfg{
					FieldMetas: map[loop_span.PlatformType]map[loop_span.SpanListType][]string{
						loop_span.PlatformCozeLoop: {
							loop_span.SpanListTypeAllSpan: {"field1", "field2"},
						},
					},
					AvailableFields: map[string]*config.FieldMeta{
						"field1": {FieldType: "string"},
						"field2": {FieldType: "int"},
					},
				}, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTracesMetaInfoReq{
					WorkspaceID:  1,
					PlatformType: loop_span.PlatformCozeLoop,
					SpanListType: loop_span.SpanListTypeAllSpan,
				},
			},
			want: &GetTracesMetaInfoResp{
				FilesMetas: map[string]*config.FieldMeta{
					"field1": {FieldType: "string"},
					"field2": {FieldType: "int"},
				},
			},
		},
		{
			name: "get traces meta info failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetTraceFieldMetaInfo(gomock.Any()).Return(nil, fmt.Errorf("config error"))
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTracesMetaInfoReq{
					WorkspaceID:  1,
					PlatformType: loop_span.PlatformCozeLoop,
					SpanListType: loop_span.SpanListTypeAllSpan,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			got, err := r.GetTracesMetaInfo(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTraceServiceImpl_ListAnnotations(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *ListAnnotationsReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *ListAnnotationsResp
		wantErr      bool
	}{
		{
			name: "list annotations successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListAnnotations(gomock.Any(), gomock.Any()).Return(loop_span.AnnotationList{{
					ID:      "anno-123",
					TraceID: "123",
					SpanID:  "234",
				}}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationsReq{
					WorkspaceID:  1,
					TraceID:      "123",
					SpanID:       "234",
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			want: &ListAnnotationsResp{
				Annotations: loop_span.AnnotationList{{
					ID:      "anno-123",
					TraceID: "123",
					SpanID:  "234",
				}},
			},
		},
		{
			name: "list annotations failed due to repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListAnnotations(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationsReq{
					WorkspaceID:  1,
					TraceID:      "123",
					SpanID:       "234",
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
		{
			name: "list annotations failed due to config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("config error")).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationsReq{
					WorkspaceID:  1,
					TraceID:      "123",
					SpanID:       "234",
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			got, err := r.ListAnnotations(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceServiceImpl_UpdateManualAnnotation(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *UpdateManualAnnotationReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "update manual annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(
					&loop_span.Annotation{
						TraceID: "test-trace-id",
						SpanID:  "test-span-id",
					}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &UpdateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					AnnotationID: "829c8de8be8aea88af058cac0a5578e5184f3f6c9b21d08ccfafca0d27f49de4",
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "update manual annotation failed because of invalid id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &UpdateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					AnnotationID: "829c8de8be8aea88af058cac0a5578e5184f3f6c9b21d08ccfafca0d27f49",
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "get tenants failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("config error")).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repomocks.NewMockITraceRepo(ctrl),
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &UpdateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation:   &loop_span.Annotation{StartTime: time.Now()},
				},
			},
			wantErr: true,
		},
		{
			name: "get span failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &UpdateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			err := r.UpdateManualAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceServiceImpl_CreateManualAnnotation(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *CreateManualAnnotationReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *CreateManualAnnotationResp
		wantErr      bool
	}{
		{
			name: "create manual annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "get tenants failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("config error")).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repomocks.NewMockITraceRepo(ctrl),
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation:   &loop_span.Annotation{StartTime: time.Now()},
				},
			},
			wantErr: true,
		},
		{
			name: "get span failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "span not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{}, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "insert annotation failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(errorx.WrapByCode(fmt.Errorf("insert error"), obErrorx.CommercialCommonRPCErrorCodeCode))
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					traceProducer:      mqmocks.NewMockITraceProducer(ctrl),
					annotationProducer: mqmocks.NewMockIAnnotationProducer(ctrl),
					metrics:            metricmocks.NewMockITraceMetrics(ctrl),
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Annotation: &loop_span.Annotation{
						SpanID:      "test-span-id",
						TraceID:     "test-trace-id",
						WorkspaceID: "1",
						StartTime:   time.Now(),
						Key:         "test-key",
						Value:       loop_span.AnnotationValue{StringValue: "test-value"},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			got, err := r.CreateManualAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.NotNil(t, got)
			}
		})
	}
}

func TestTraceServiceImpl_ListSpans(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *ListSpansReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *ListSpansResp
		wantErr      bool
	}{
		{
			name: "list spans successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{{
						TraceID: "123",
						SpanID:  "234",
					}},
					PageToken: "",
					HasMore:   false,
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{
					{
						FieldName: loop_span.SpanFieldSpaceId,
						FieldType: loop_span.FieldTypeString,
						Values:    []string{"123"},
						QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
					},
				}, false, nil)
				filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeAllSpan,
				},
			},
			want: &ListSpansResp{
				Spans: loop_span.SpanList{{
					TraceID: "123",
					SpanID:  "234",
				}},
			},
		},
		{
			name: "list spans successfully with specific filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{{
						TraceID: "123",
						SpanID:  "234",
					}},
					PageToken: "",
					HasMore:   false,
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeAllSpan,
					Filters: &loop_span.FilterFields{
						QueryAndOr: nil,
						FilterFields: []*loop_span.FilterField{
							{
								FieldName: "status",
								FieldType: loop_span.FieldTypeString,
								Values:    []string{"success"},
								QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
							},
							{
								FieldName: "status",
								FieldType: loop_span.FieldTypeString,
								Values:    []string{"success", "error"},
								QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
							},
							{
								FieldName: "status",
								FieldType: loop_span.FieldTypeString,
								Values:    []string{"error"},
								QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
							},
							{
								FieldName: loop_span.SpanFieldStartTimeFirstResp,
								FieldType: loop_span.FieldTypeLong,
								Values:    []string{"1234"},
								QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
							},
						},
					},
				},
			},
			want: &ListSpansResp{
				Spans: loop_span.SpanList{{
					TraceID: "123",
					SpanID:  "234",
				}},
			},
		},
		{
			name: "list spans successfully with root span",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{{
						TraceID: "123",
						SpanID:  "234",
					}},
					PageToken: "",
					HasMore:   false,
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildRootSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeRootSpan,
				},
			},
			want: &ListSpansResp{
				Spans: loop_span.SpanList{{
					TraceID: "123",
					SpanID:  "234",
				}},
			},
		},
		{
			name: "list spans successfully with llm span",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{{
						TraceID: "123",
						SpanID:  "234",
					}},
					PageToken: "",
					HasMore:   false,
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildLLMSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeLLMSpan,
				},
			},
			want: &ListSpansResp{
				Spans: loop_span.SpanList{{
					TraceID: "123",
					SpanID:  "234",
				}},
			},
		},
		{
			name: "list spans successfully with processor",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{{
						TraceID:     "123",
						SpanID:      "234",
						WorkspaceID: "123",
					}},
					PageToken: "",
					HasMore:   false,
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock,
					nil,
					[]span_processor.Factory{
						span_processor.NewCheckProcessorFactory(),
					},
					nil,
					nil,
					nil,
					nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeAllSpan,
					WorkspaceID:  123,
				},
			},
			want: &ListSpansResp{
				Spans: loop_span.SpanList{{
					TraceID:     "123",
					SpanID:      "234",
					WorkspaceID: "123",
				}},
			},
		},
		{
			name: "list spans successfully with processor failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{{
						TraceID:     "123",
						SpanID:      "234",
						WorkspaceID: "1234",
					}},
					PageToken: "",
					HasMore:   false,
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock,
					nil,
					[]span_processor.Factory{
						span_processor.NewCheckProcessorFactory(),
					},
					nil,
					nil,
					nil,
					nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeAllSpan,
					WorkspaceID:  123,
				},
			},
			wantErr: true,
		},
		{
			name: "list spans failed due to invalid platform type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("bad")).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: "abc",
					Limit:        10,
					SpanListType: loop_span.SpanListTypeAllSpan,
				},
			},
			wantErr: true,
		},
		{
			name: "list spans failed due to repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed"))
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterMock := filtermocks.NewMockFilter(ctrl)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{}}, false, nil)
				filterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return(nil, nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				filterFactoryMock.EXPECT().GetFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					metrics:        metricsMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansReq{
					PlatformType: loop_span.PlatformCozeLoop,
					Limit:        10,
					SpanListType: loop_span.SpanListTypeAllSpan,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			got, err := r.ListSpans(tt.args.ctx, tt.args.req)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTraceServiceImpl_CreateAnnotation(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *CreateAnnotationReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "create annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				annoProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeManualFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(nil, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					annotationProducer: annoProducerMock,
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateAnnotationReq{
					WorkspaceID:   1,
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					AnnotationKey: "test-key",
					AnnotationVal: loop_span.AnnotationValue{StringValue: "test-value"},
					Caller:        "test-caller",
					QueryDays:     1,
				},
			},
			wantErr: false,
		},
		{
			name: "get caller config failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(nil, fmt.Errorf("config error"))
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateAnnotationReq{
					Caller: "test-caller",
				},
			},
			wantErr: true,
		},
		{
			name: "get span failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeCozeFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateAnnotationReq{
					WorkspaceID: 1,
					SpanID:      "test-span-id",
					TraceID:     "test-trace-id",
					Caller:      "test-caller",
				},
			},
			wantErr: true,
		},
		{
			name: "span not found, send to mq",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				annoProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeManualFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
				annoProducerMock.EXPECT().SendAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					annotationProducer: annoProducerMock,
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateAnnotationReq{
					WorkspaceID: 1,
					SpanID:      "test-span-id",
					TraceID:     "test-trace-id",
					Caller:      "test-caller",
				},
			},
			wantErr: false,
		},
		{
			name: "insert annotation failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				annoProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeManualFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(nil, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(fmt.Errorf("insert error"))
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					annotationProducer: annoProducerMock,
					buildHelper:        buildHelper,
					tenantProvider:     tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &CreateAnnotationReq{
					WorkspaceID:   1,
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					AnnotationKey: "test-key",
					AnnotationVal: loop_span.AnnotationValue{StringValue: "test-value"},
					Caller:        "test-caller",
					QueryDays:     1,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			err := r.CreateAnnotation(tt.args.ctx, tt.args.req)
			t.Log(err)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceServiceImpl_DeleteAnnotation(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *DeleteAnnotationReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "delete annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeManualFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteAnnotationReq{
					WorkspaceID:   1,
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					AnnotationKey: "test-key",
					Caller:        "test-caller",
					QueryDays:     1,
				},
			},
			wantErr: false,
		},
		{
			name: "get caller config failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(nil, fmt.Errorf("config error"))
				return fields{
					traceConfig: confMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteAnnotationReq{
					Caller: "test-caller",
				},
			},
			wantErr: true,
		},
		{
			name: "get span failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeManualFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				return fields{
					traceRepo:   repoMock,
					traceConfig: confMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteAnnotationReq{
					WorkspaceID: 1,
					SpanID:      "test-span-id",
					TraceID:     "test-trace-id",
					Caller:      "test-caller",
				},
			},
			wantErr: true,
		},
		{
			name: "span not found, send to mq",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				annoProducerMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationCorrectionTypeManual),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
				annoProducerMock.EXPECT().SendAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					annotationProducer: annoProducerMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteAnnotationReq{
					WorkspaceID: 1,
					SpanID:      "test-span-id",
					TraceID:     "test-trace-id",
					Caller:      "test-caller",
				},
			},
			wantErr: false,
		},
		{
			name: "insert annotation failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"test-caller": {
							Tenants:        []string{"spans"},
							AnnotationType: string(loop_span.AnnotationTypeManualFeedback),
						},
					},
				}, nil)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(fmt.Errorf("insert error"))
				return fields{
					traceRepo:   repoMock,
					traceConfig: confMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteAnnotationReq{
					WorkspaceID:   1,
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					AnnotationKey: "test-key",
					Caller:        "test-caller",
					QueryDays:     1,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			err := r.DeleteAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceServiceImpl_DeleteManualAnnotation(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *DeleteManualAnnotationReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "delete manual annotation successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(nil)
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteManualAnnotationReq{
					PlatformType:  loop_span.PlatformCozeLoop,
					AnnotationID:  "829c8de8be8aea88af058cac0a5578e5184f3f6c9b21d08ccfafca0d27f49de4",
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					WorkspaceID:   1,
					StartTime:     time.Now().UnixMilli(),
					AnnotationKey: "test-key",
				},
			},
			wantErr: false,
		},
		{
			name: "get tenants failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("config error")).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				return fields{
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteManualAnnotationReq{
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
		{
			name: "get span failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteManualAnnotationReq{
					AnnotationID: "123",
					TraceID:      "test-trace-id",
					WorkspaceID:  1,
					SpanID:       "test-span-id",
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
		{
			name: "span not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteManualAnnotationReq{
					AnnotationID: "123",
					TraceID:      "test-trace-id",
					WorkspaceID:  1,
					SpanID:       "test-span-id",
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
		{
			name: "insert annotation failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(fmt.Errorf("insert error"))
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteManualAnnotationReq{
					PlatformType:  loop_span.PlatformCozeLoop,
					AnnotationID:  "829c8de8be8aea88af058cac0a5578e5184f3f6c9b21d08ccfafca0d27f49de4",
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					WorkspaceID:   1,
					StartTime:     time.Now().UnixMilli(),
					AnnotationKey: "test-key",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid annotation id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{
							TraceID:     "test-trace-id",
							SpanID:      "test-span-id",
							WorkspaceID: "1",
							SystemTagsString: map[string]string{
								loop_span.SpanFieldTenant: "spans",
							},
						},
					},
				}, nil)
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &DeleteManualAnnotationReq{
					PlatformType:  loop_span.PlatformCozeLoop,
					AnnotationID:  "invalid-id",
					SpanID:        "test-span-id",
					TraceID:       "test-trace-id",
					WorkspaceID:   1,
					StartTime:     time.Now().UnixMilli(),
					AnnotationKey: "test-key",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r, _ := NewTraceServiceImpl(
				fields.traceRepo,
				fields.traceConfig,
				fields.traceProducer,
				fields.annotationProducer,
				fields.metrics,
				fields.buildHelper,
				fields.tenantProvider)
			err := r.DeleteManualAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceServiceImpl_GetTrace(t *testing.T) {
	type fields struct {
		traceRepo      repo.ITraceRepo
		traceConfig    config.ITraceConfig
		traceProducer  mq.ITraceProducer
		metrics        metrics.ITraceMetrics
		buildHelper    TraceFilterProcessorBuilder
		tenantProvider tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *GetTraceReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *GetTraceResp
		wantErr      bool
	}{
		{
			name: "get trace successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(loop_span.SpanList{
					{
						TraceID: "123",
						SpanID:  "234",
					},
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTraceReq{
					PlatformType: loop_span.PlatformCozeLoop,
					TraceID:      "123",
				},
			},
			want: &GetTraceResp{
				TraceId: "123",
				Spans: loop_span.SpanList{
					{
						TraceID: "123",
						SpanID:  "234",
					},
				},
			},
		},
		{
			name: "get trace successfully with processor",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(loop_span.SpanList{
					{
						TraceID:     "123",
						SpanID:      "234",
						WorkspaceID: "123",
					},
				}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock,
					[]span_processor.Factory{span_processor.NewCheckProcessorFactory()},
					nil,
					nil,
					nil,
					nil,
					nil)
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTraceReq{
					PlatformType: loop_span.PlatformCozeLoop,
					TraceID:      "123",
					WorkspaceID:  123,
				},
			},
			want: &GetTraceResp{
				TraceId: "123",
				Spans: loop_span.SpanList{
					{
						TraceID:     "123",
						SpanID:      "234",
						WorkspaceID: "123",
					},
				},
			},
		},
		{
			name: "get failed due to invalid platform type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("bad")).AnyTimes()
				return fields{
					traceConfig:    confMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTraceReq{
					PlatformType: "abc",
					TraceID:      "123",
				},
			},
			wantErr: true,
		},
		{
			name: "get failed due to repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("failed"))
				confMock := confmocks.NewMockITraceConfig(ctrl)
				tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"spans"}, nil).AnyTimes()
				metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
				metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any()).Return()
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &GetTraceReq{
					PlatformType: loop_span.PlatformCozeLoop,
					TraceID:      "123",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceServiceImpl{
				traceRepo:      fields.traceRepo,
				traceConfig:    fields.traceConfig,
				traceProducer:  fields.traceProducer,
				metrics:        fields.metrics,
				buildHelper:    fields.buildHelper,
				tenantProvider: fields.tenantProvider,
			}
			got, err := r.GetTrace(tt.args.ctx, tt.args.req)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestTraceServiceImpl_Send(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		annotationProducer mq.IAnnotationProducer
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
			name: "span not found, return nil & retry",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{}, nil)
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"caller1": {
							AnnotationType: "test",
							Tenants:        []string{"spans"},
						},
					},
				}, nil)
				annoMock := mqmocks.NewMockIAnnotationProducer(ctrl)
				annoMock.EXPECT().SendAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceRepo:          repoMock,
					traceConfig:        confMock,
					annotationProducer: annoMock,
				}
			},
			args: args{
				ctx: context.Background(),
				event: &entity.AnnotationEvent{
					Annotation: &loop_span.Annotation{
						SpanID:      "span1",
						TraceID:     "trace1",
						WorkspaceID: "workspace1",
					},
					Caller:     "caller1",
					RetryTimes: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "insert error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{},
					},
				}, nil)
				repoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).Return(fmt.Errorf("insert error"))
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetAnnotationSourceCfg(gomock.Any()).Return(&config.AnnotationSourceConfig{
					SourceCfg: map[string]config.AnnotationConfig{
						"caller1": {
							AnnotationType: "test",
							Tenants:        []string{"spans"},
						},
					},
				}, nil)
				return fields{
					traceRepo:   repoMock,
					traceConfig: confMock,
				}
			},
			args: args{
				ctx: context.Background(),
				event: &entity.AnnotationEvent{
					Annotation: &loop_span.Annotation{
						SpanID:         "span1",
						TraceID:        "trace1",
						WorkspaceID:    "workspace1",
						AnnotationType: "123",
						Key:            "12",
					},
					Caller:     "caller1",
					RetryTimes: 2,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			s := &TraceServiceImpl{
				traceRepo:          fields.traceRepo,
				traceConfig:        fields.traceConfig,
				annotationProducer: fields.annotationProducer,
			}
			err := s.Send(tt.args.ctx, tt.args.event)
			assert.Equal(t, err != nil, tt.wantErr)
		})
	}
}

func TestTraceServiceImpl_SearchTraceOApi(t *testing.T) {
	type fields struct {
		traceRepo   repo.ITraceRepo
		buildHelper TraceFilterProcessorBuilder
	}
	type args struct {
		ctx context.Context
		req *SearchTraceOApiReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *SearchTraceOApiResp
		wantErr      bool
	}{
		{
			name: "search trace successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), &repo.GetTraceParam{
					Tenants:            []string{"tenant1"},
					TraceID:            "trace-123",
					LogID:              "",
					StartAt:            1640995200000,
					EndAt:              1640995800000,
					Limit:              100,
					NotQueryAnnotation: false,
				}).Return(loop_span.SpanList{
					{
						TraceID:   "trace-123",
						SpanID:    "span-456",
						StartTime: 1640995200000000,
					},
				}, nil)

				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

				return fields{
					traceRepo:   repoMock,
					buildHelper: buildHelper,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &SearchTraceOApiReq{
					WorkspaceID:  123,
					Tenants:      []string{"tenant1"},
					TraceID:      "trace-123",
					StartTime:    1640995200000,
					EndTime:      1640995800000,
					Limit:        100,
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			want: &SearchTraceOApiResp{
				Spans: loop_span.SpanList{
					{
						TraceID:   "trace-123",
						SpanID:    "span-456",
						StartTime: 1640995200000000,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "search trace failed due to repo error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("repo error"))

				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

				return fields{
					traceRepo:   repoMock,
					buildHelper: buildHelper,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &SearchTraceOApiReq{
					WorkspaceID:  123,
					Tenants:      []string{"tenant1"},
					TraceID:      "trace-123",
					StartTime:    1640995200000,
					EndTime:      1640995800000,
					Limit:        100,
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceServiceImpl{
				traceRepo:   fields.traceRepo,
				buildHelper: fields.buildHelper,
			}
			got, err := r.SearchTraceOApi(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTraceServiceImpl_ListSpansOApi(t *testing.T) {
	type fields struct {
		traceRepo   repo.ITraceRepo
		buildHelper TraceFilterProcessorBuilder
	}
	type args struct {
		ctx context.Context
		req *ListSpansOApiReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *ListSpansOApiResp
		wantErr      bool
	}{
		{
			name: "list spans failed due to invalid filter",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
				buildHelper := NewTraceFilterProcessorBuilder(filterFactoryMock, nil, nil, nil, nil, nil, nil)

				return fields{
					buildHelper: buildHelper,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListSpansOApiReq{
					WorkspaceID: 123,
					Tenants:     []string{"tenant1"},
					StartTime:   1640995200000,
					EndTime:     1640995800000,
					Filters: &loop_span.FilterFields{
						FilterFields: []*loop_span.FilterField{
							{
								FieldName: "status",
								FieldType: loop_span.FieldTypeString,
								Values:    []string{"invalid"},
								QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
							},
						},
					},
					Limit:        100,
					PlatformType: loop_span.PlatformCozeLoop,
					SpanListType: loop_span.SpanListTypeAllSpan,
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceServiceImpl{
				traceRepo:   fields.traceRepo,
				buildHelper: fields.buildHelper,
			}
			got, err := r.ListSpansOApi(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
