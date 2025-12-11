// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/infra/redis/mocks"
	annotationpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	kitexdataset "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	mqmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	taskentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	taskRepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	taskRepomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

const defaultUserID = "user-1"

type taskRepoMock struct {
	*taskRepomocks.MockITaskRepo
}

func newTaskRepoMock(ctrl *gomock.Controller) *taskRepoMock {
	return &taskRepoMock{MockITaskRepo: taskRepomocks.NewMockITaskRepo(ctrl)}
}

func (m *taskRepoMock) ListNonFinalTask(context.Context, string) ([]int64, error) {
	panic("unexpected call to ListNonFinalTaskBySpaceID in taskRepoMock")
}

func (m *taskRepoMock) AddNonFinalTask(context.Context, string, int64) error {
	panic("unexpected call to AddNonFinalTask in taskRepoMock")
}

func (m *taskRepoMock) RemoveNonFinalTask(context.Context, string, int64) error {
	panic("unexpected call to RemoveNonFinalTask in taskRepoMock")
}

func (m *taskRepoMock) GetTaskByCache(context.Context, int64) (*taskentity.ObservabilityTask, error) {
	panic("unexpected call to GetTaskByCache in taskRepoMock")
}

func (m *taskRepoMock) SetTask(context.Context, *taskentity.ObservabilityTask) error {
	panic("unexpected call to SetTask in taskRepoMock")
}

var _ taskRepo.ITaskRepo = (*taskRepoMock)(nil)

func TestTraceServiceImpl_GetTracesAdvanceInfo(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				persistCmdMock := mocks.NewMockPersistentCmdable(ctrl)
				return fields{
					traceRepo:      repoMock,
					traceConfig:    confMock,
					buildHelper:    buildHelper,
					metrics:        metricsMock,
					tenantProvider: tenantProviderMock,
					persistCmd:     persistCmdMock,
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
						loop_span.PlatformDefault: {
							loop_span.SpanListTypeAllSpan: {"field1", "field2"},
						},
						loop_span.PlatformCozeLoop: {
							loop_span.SpanListTypeAllSpan: {},
						},
					},
					AvailableFields: map[string]*config.FieldMeta{
						"field1": {FieldType: "string"},
						"field2": {FieldType: "int"},
					},
				}, nil)
				confMock.EXPECT().GetKeySpanTypes(gomock.Any()).Return(map[string][]string{
					string(loop_span.PlatformDefault):  {},
					string(loop_span.PlatformCozeLoop): {},
				})
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
				KeySpanTypeList: []string{},
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
			got, err := r.GetTracesMetaInfo(tt.args.ctx, tt.args.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
		{
			name: "create annotation on root span when span_id is empty",
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

				// Mock ListSpans call with ParentID filter for root span
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param *repo.ListSpansParam) (*repo.ListSpansResult, error) {
						return &repo.ListSpansResult{
							Spans: loop_span.SpanList{
								{
									TraceID:     "test-trace-id",
									SpanID:      "root-span-id",
									ParentID:    "0",
									WorkspaceID: "1",
									SystemTagsString: map[string]string{
										loop_span.SpanFieldTenant: "spans",
									},
								},
							},
						}, nil
					},
				)
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
					SpanID:        "", // Empty span_id to trigger root span search
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
			name: "create annotation when span_id is empty but no root span found",
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

				// Mock ListSpans call with ParentID filter but return no spans
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
				// Expect annotation to be sent via producer when no span found
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
					WorkspaceID:   1,
					SpanID:        "", // Empty span_id to trigger root span search
					TraceID:       "test-trace-id",
					AnnotationKey: "test-key",
					AnnotationVal: loop_span.AnnotationValue{StringValue: "test-value"},
					Caller:        "test-caller",
					QueryDays:     1,
				},
			},
			wantErr: false,
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
			name: "delete annotation on root span when span_id is empty",
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

				// Mock ListSpans call with ParentID filter for root span
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, param *repo.ListSpansParam) (*repo.ListSpansResult, error) {
						return &repo.ListSpansResult{
							Spans: loop_span.SpanList{
								{
									TraceID:     "test-trace-id",
									SpanID:      "root-span-id",
									ParentID:    "0",
									WorkspaceID: "1",
									SystemTagsString: map[string]string{
										loop_span.SpanFieldTenant: "spans",
									},
								},
							},
						}, nil
					},
				)
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
					SpanID:        "", // Empty span_id to trigger root span search
					TraceID:       "test-trace-id",
					AnnotationKey: "test-key",
					Caller:        "test-caller",
					QueryDays:     1,
				},
			},
			wantErr: false,
		},
		{
			name: "delete annotation when span_id is empty but no root span found",
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

				// Mock ListSpans call with ParentID filter but return no spans
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
				// Expect annotation to be sent via producer when no span found
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
				req: &DeleteAnnotationReq{
					WorkspaceID:   1,
					SpanID:        "", // Empty span_id to trigger root span search
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
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
					WorkSpaceID:        "123",
					Tenants:            []string{"tenant1"},
					TraceID:            "trace-123",
					LogID:              "",
					SpanIDs:            nil,
					StartAt:            1640995200000,
					EndAt:              1640995800000,
					Limit:              100,
					NotQueryAnnotation: false,
					Filters:            nil,
					OmitColumns:        []string{"input", "output"},
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

func TestTraceFilterProcessorBuilderImpl_BuildListSpansOApiProcessors(t *testing.T) {
	tests := []struct {
		name                            string
		listSpansOApiProcessorFactories []span_processor.Factory
		want                            int
		wantErr                         bool
	}{
		{
			name:                            "build processors successfully with empty factories",
			listSpansOApiProcessorFactories: []span_processor.Factory{},
			want:                            0,
			wantErr:                         false,
		},
		{
			name: "build processors successfully with multiple factories",
			listSpansOApiProcessorFactories: []span_processor.Factory{
				span_processor.NewCheckProcessorFactory(),
				span_processor.NewCheckProcessorFactory(),
			},
			want:    2,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
			builder := NewTraceFilterProcessorBuilder(
				filterFactoryMock,
				nil,
				nil,
				nil,
				nil,
				nil,
				tt.listSpansOApiProcessorFactories,
			)

			got, err := builder.BuildListSpansOApiProcessors(context.Background(), span_processor.Settings{
				WorkspaceId:    123,
				QueryStartTime: 1640995200000,
				QueryEndTime:   1640995800000,
			})

			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.want, len(got))
			}
		})
	}
}

func TestTraceServiceImpl_ChangeEvaluatorScore(t *testing.T) {
	type fields struct {
		traceRepo      repo.ITraceRepo
		tenantProvider tenant.ITenantProvider
		evalSvc        rpc.IEvaluatorRPCAdapter
		after          func(t *testing.T, resp *ChangeEvaluatorScoreResp)
	}
	type args struct {
		ctx context.Context
		req *ChangeEvaluatorScoreRequest
	}

	buildSpan := func(req *ChangeEvaluatorScoreRequest) *loop_span.Span {
		now := time.Now()
		return &loop_span.Span{
			SpanID:          req.SpanID,
			TraceID:         "trace-" + req.SpanID,
			WorkspaceID:     strconv.FormatInt(req.WorkspaceID, 10),
			StartTime:       now.UnixMicro(),
			LogicDeleteTime: now.Add(24 * time.Hour).UnixMicro(),
			SystemTagsString: map[string]string{
				loop_span.SpanFieldTenant: "tenant",
			},
		}
	}
	buildAnnotation := func(req *ChangeEvaluatorScoreRequest, span *loop_span.Span) *loop_span.Annotation {
		now := time.Now()
		return &loop_span.Annotation{
			ID:             req.AnnotationID,
			SpanID:         span.SpanID,
			TraceID:        span.TraceID,
			StartTime:      time.UnixMicro(span.StartTime),
			WorkspaceID:    span.WorkspaceID,
			AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
			Metadata:       loop_span.AutoEvaluateMetadata{EvaluatorRecordID: 100},
			Reasoning:      "origin reason",
			Value:          loop_span.NewDoubleValue(1.1),
			CreatedAt:      now.Add(-time.Hour),
			CreatedBy:      "origin",
			UpdatedAt:      now.Add(-time.Minute),
			UpdatedBy:      "origin",
		}
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields
		args         args
		wantErr      bool
		wantResp     bool
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)

				span := buildSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)

				annotation := buildAnnotation(req, span)
				traceRepoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(annotation, nil)

				var capturedUpsert *repo.InsertAnnotationParam
				traceRepoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *repo.InsertAnnotationParam) error {
					capturedUpsert = param
					return nil
				})

				var capturedUpdate *rpc.UpdateEvaluatorRecordParam
				evalMock.EXPECT().UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.UpdateEvaluatorRecordParam) error {
					capturedUpdate = param
					return nil
				})

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
					evalSvc:        evalMock,
					after: func(t *testing.T, resp *ChangeEvaluatorScoreResp) {
						assert.NotNil(t, resp)
						if assert.NotNil(t, capturedUpsert) && assert.NotEmpty(t, capturedUpsert.Span.Annotations) {
							updated := capturedUpsert.Span.Annotations[0]
							assert.Len(t, updated.Corrections, 2)
							assert.InDelta(t, req.Correction.GetScore(), updated.Value.FloatValue, 1e-9)
							assert.Equal(t, defaultUserID, updated.UpdatedBy)
							assert.Equal(t, "tenant", capturedUpsert.Tenant)
						}
						if assert.NotNil(t, capturedUpdate) {
							assert.Equal(t, strconv.FormatInt(req.WorkspaceID, 10), capturedUpdate.WorkspaceID)
							assert.InDelta(t, req.Correction.GetScore(), capturedUpdate.Score, 1e-9)
							assert.Equal(t, req.Correction.GetExplain(), capturedUpdate.Reasoning)
							assert.Equal(t, defaultUserID, capturedUpdate.UpdatedBy)
						}
					},
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 2.5
					explain := "new reason"
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					correction.SetExplain(&explain)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  123,
						AnnotationID: "anno-1",
						SpanID:       "span-1",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: true,
		},
		{
			name: "upsert failed returns nil resp",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				span := buildSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				annotation := buildAnnotation(req, span)
				traceRepoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(annotation, nil)
				evalMock.EXPECT().UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil)

				var capturedUpsert *repo.InsertAnnotationParam
				traceRepoMock.EXPECT().InsertAnnotations(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *repo.InsertAnnotationParam) error {
					capturedUpsert = param
					return fmt.Errorf("upsert error")
				})

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
					evalSvc:        evalMock,
					after: func(t *testing.T, _ *ChangeEvaluatorScoreResp) {
						if assert.NotNil(t, capturedUpsert) && assert.NotEmpty(t, capturedUpsert.Span.Annotations) {
							assert.Len(t, capturedUpsert.Span.Annotations[0].Corrections, 2)
						}
					},
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 3.3
					explain := "another"
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					correction.SetExplain(&explain)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  222,
						AnnotationID: "anno-2",
						SpanID:       "span-2",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  false,
		},
		{
			name: "get tenants error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return(nil, fmt.Errorf("tenant err"))
				return fields{
					tenantProvider: tenantMock,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 1.1
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  1,
						AnnotationID: "anno-3",
						SpanID:       "span-3",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
		{
			name: "list span error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("list error"))

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 2.2
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  3,
						AnnotationID: "anno-4",
						SpanID:       "span-4",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
		{
			name: "span not found",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 4.4
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  4,
						AnnotationID: "anno-5",
						SpanID:       "span-5",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
		{
			name: "get annotation error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				span := buildSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				traceRepoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("annotation error"))

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 5.5
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  5,
						AnnotationID: "anno-6",
						SpanID:       "span-6",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
		{
			name: "annotation not found",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				span := buildSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				traceRepoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(nil, nil)

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 6.6
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  6,
						AnnotationID: "anno-7",
						SpanID:       "span-7",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
		{
			name: "user id missing",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				span := buildSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				annotation := buildAnnotation(req, span)
				traceRepoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(annotation, nil)

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 7.7
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  7,
						AnnotationID: "anno-8",
						SpanID:       "span-8",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
		{
			name: "correct evaluator records error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ChangeEvaluatorScoreRequest) fields {
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				span := buildSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				annotation := buildAnnotation(req, span)
				traceRepoMock.EXPECT().GetAnnotation(gomock.Any(), gomock.Any()).Return(annotation, nil)
				evalMock.EXPECT().UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(fmt.Errorf("rpc error"))

				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
					evalSvc:        evalMock,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: defaultUserID}),
				req: func() *ChangeEvaluatorScoreRequest {
					score := 8.8
					correction := annotationpb.NewCorrection()
					correction.SetScore(&score)
					return &ChangeEvaluatorScoreRequest{
						WorkspaceID:  8,
						AnnotationID: "anno-9",
						SpanID:       "span-9",
						StartTime:    time.Now().UnixMilli(),
						PlatformType: loop_span.PlatformCozeLoop,
						Correction:   correction,
					}
				}(),
			},
			wantResp: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl, tt.args.req)
			svc := &TraceServiceImpl{
				traceRepo:      f.traceRepo,
				tenantProvider: f.tenantProvider,
				evalSvc:        f.evalSvc,
			}
			resp, err := svc.ChangeEvaluatorScore(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantResp {
				assert.NotNil(t, resp)
			} else {
				assert.Nil(t, resp)
			}
			if f.after != nil {
				f.after(t, resp)
			}
		})
	}
}

func TestTraceFilterProcessorBuilderImpl_BuildIngestTraceProcessors_ErrorHandling(t *testing.T) {
	tests := []struct {
		name                          string
		ingestTraceProcessorFactories []span_processor.Factory
		want                          int
		wantErr                       bool
	}{
		{
			name:                          "build ingest processors successfully with empty factories",
			ingestTraceProcessorFactories: []span_processor.Factory{},
			want:                          0,
			wantErr:                       false,
		},
		{
			name: "build ingest processors successfully with multiple factories",
			ingestTraceProcessorFactories: []span_processor.Factory{
				span_processor.NewCheckProcessorFactory(),
			},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)

			builder := NewTraceFilterProcessorBuilder(
				filterFactoryMock,
				nil,
				nil,
				nil,
				tt.ingestTraceProcessorFactories,
				nil,
				nil,
			)

			got, err := builder.BuildIngestTraceProcessors(context.Background(), span_processor.Settings{})

			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.want, len(got))
			}
		})
	}
}

func TestTraceServiceImpl_correctEvaluatorRecords(t *testing.T) {
	type testCase struct {
		name       string
		annotation *loop_span.Annotation
		mockSetup  func(mock *rpcmocks.MockIEvaluatorRPCAdapter, captured **rpc.UpdateEvaluatorRecordParam)
		after      func(t *testing.T, captured *rpc.UpdateEvaluatorRecordParam)
		wantErr    bool
	}

	newAnnotation := func() *loop_span.Annotation {
		return &loop_span.Annotation{
			AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
			Metadata:       loop_span.AutoEvaluateMetadata{EvaluatorRecordID: 100},
			WorkspaceID:    "123",
			Corrections: []loop_span.AnnotationCorrection{
				{
					Reasoning: "reason",
					Value:     loop_span.NewDoubleValue(9.9),
					UpdatedBy: defaultUserID,
				},
			},
		}
	}

	tests := []testCase{
		{
			name:    "annotation nil",
			wantErr: true,
		},
		{
			name: "metadata nil",
			annotation: &loop_span.Annotation{
				AnnotationType: loop_span.AnnotationTypeManualFeedback,
				WorkspaceID:    "1",
				Corrections: []loop_span.AnnotationCorrection{
					{Value: loop_span.NewDoubleValue(1)},
				},
			},
			wantErr: true,
		},
		{
			name: "corrections empty",
			annotation: &loop_span.Annotation{
				AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
				Metadata:       loop_span.AutoEvaluateMetadata{EvaluatorRecordID: 1},
				WorkspaceID:    "1",
				Corrections:    nil,
			},
			wantErr: true,
		},
		{
			name:       "update evaluator error",
			annotation: newAnnotation(),
			mockSetup: func(mock *rpcmocks.MockIEvaluatorRPCAdapter, _ **rpc.UpdateEvaluatorRecordParam) {
				mock.EXPECT().UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(fmt.Errorf("rpc error"))
			},
			wantErr: true,
		},
		{
			name:       "success",
			annotation: newAnnotation(),
			mockSetup: func(mock *rpcmocks.MockIEvaluatorRPCAdapter, captured **rpc.UpdateEvaluatorRecordParam) {
				mock.EXPECT().UpdateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.UpdateEvaluatorRecordParam) error {
					*captured = param
					return nil
				})
			},
			after: func(t *testing.T, captured *rpc.UpdateEvaluatorRecordParam) {
				if assert.NotNil(t, captured) {
					assert.Equal(t, "123", captured.WorkspaceID)
					assert.InDelta(t, 9.9, captured.Score, 1e-9)
					assert.Equal(t, "reason", captured.Reasoning)
					assert.Equal(t, defaultUserID, captured.UpdatedBy)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)
			var captured *rpc.UpdateEvaluatorRecordParam
			if tt.mockSetup != nil {
				tt.mockSetup(evalMock, &captured)
			}
			svc := &TraceServiceImpl{}
			err := svc.correctEvaluatorRecords(context.Background(), evalMock, tt.annotation)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.after != nil {
				tt.after(t, captured)
			}
		})
	}
}

func TestTraceFilterProcessorBuilderImpl_BuildSearchTraceOApiProcessors_ErrorHandling(t *testing.T) {
	tests := []struct {
		name                              string
		searchTraceOApiProcessorFactories []span_processor.Factory
		want                              int
		wantErr                           bool
	}{
		{
			name:                              "build search trace oapi processors successfully with empty factories",
			searchTraceOApiProcessorFactories: []span_processor.Factory{},
			want:                              0,
			wantErr:                           false,
		},
		{
			name: "build search trace oapi processors successfully with multiple factories",
			searchTraceOApiProcessorFactories: []span_processor.Factory{
				span_processor.NewCheckProcessorFactory(),
			},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			filterFactoryMock := filtermocks.NewMockPlatformFilterFactory(ctrl)
			builder := NewTraceFilterProcessorBuilder(
				filterFactoryMock,
				nil,
				nil,
				nil,
				nil,
				tt.searchTraceOApiProcessorFactories,
				nil,
			)

			got, err := builder.BuildSearchTraceOApiProcessors(context.Background(), span_processor.Settings{
				WorkspaceId:    123,
				QueryStartTime: 1640995200000,
				QueryEndTime:   1640995800000,
			})

			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, tt.want, len(got))
			}
		})
	}
}

func TestTraceServiceImpl_ListAnnotationEvaluators(t *testing.T) {
	type fields struct {
		taskRepo taskRepo.ITaskRepo
		evalSvc  rpc.IEvaluatorRPCAdapter
		after    func(t *testing.T, resp *ListAnnotationEvaluatorsResp)
	}
	type args struct {
		ctx context.Context
		req *ListAnnotationEvaluatorsRequest
	}

	name := "evaluator"

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller, req *ListAnnotationEvaluatorsRequest) fields
		args         args
		wantErr      bool
	}{
		{
			name: "name provided success",
			fieldsGetter: func(ctrl *gomock.Controller, req *ListAnnotationEvaluatorsRequest) fields {
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)
				var capturedParam *rpc.ListEvaluatorsParam
				evalMock.EXPECT().ListEvaluators(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.ListEvaluatorsParam) ([]*rpc.Evaluator, error) {
					capturedParam = param
					return []*rpc.Evaluator{{EvaluatorVersionID: 11, EvaluatorName: "ev", EvaluatorVersion: "v1"}}, nil
				})
				return fields{
					evalSvc: evalMock,
					after: func(t *testing.T, resp *ListAnnotationEvaluatorsResp) {
						assert.NotNil(t, capturedParam)
						if assert.NotNil(t, resp) {
							assert.Len(t, resp.Evaluators, 1)
							assert.Equal(t, int64(11), resp.Evaluators[0].EvaluatorVersionID)
						}
					},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationEvaluatorsRequest{
					WorkspaceID: 10,
					Name:        &name,
				},
			},
		},
		{
			name: "name provided error",
			fieldsGetter: func(ctrl *gomock.Controller, _ *ListAnnotationEvaluatorsRequest) fields {
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)
				evalMock.EXPECT().ListEvaluators(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("rpc error"))
				return fields{evalSvc: evalMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationEvaluatorsRequest{WorkspaceID: 10, Name: &name},
			},
			wantErr: true,
		},
		{
			name: "name nil success",
			fieldsGetter: func(ctrl *gomock.Controller, req *ListAnnotationEvaluatorsRequest) fields {
				taskRepoMock := newTaskRepoMock(ctrl)
				returnTasks := []*taskentity.ObservabilityTask{
					{TaskConfig: &taskentity.TaskConfig{AutoEvaluateConfigs: []*taskentity.AutoEvaluateConfig{{EvaluatorVersionID: 101}}}},
				}
				taskRepoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(returnTasks, int64(1), nil)
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)
				var capturedParam *rpc.BatchGetEvaluatorVersionsParam
				evalMock.EXPECT().BatchGetEvaluatorVersions(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, param *rpc.BatchGetEvaluatorVersionsParam) ([]*rpc.Evaluator, map[int64]*rpc.Evaluator, error) {
					capturedParam = param
					return []*rpc.Evaluator{{EvaluatorVersionID: 101, EvaluatorName: "alpha", EvaluatorVersion: "v1"}}, nil, nil
				})
				return fields{
					taskRepo: taskRepoMock,
					evalSvc:  evalMock,
					after: func(t *testing.T, resp *ListAnnotationEvaluatorsResp) {
						assert.NotNil(t, capturedParam)
						if assert.NotNil(t, capturedParam) {
							assert.Contains(t, capturedParam.EvaluatorVersionIds, int64(101))
						}
						if assert.NotNil(t, resp) {
							assert.Len(t, resp.Evaluators, 1)
							assert.Equal(t, int64(101), resp.Evaluators[0].EvaluatorVersionID)
						}
					},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationEvaluatorsRequest{WorkspaceID: 20},
			},
		},
		{
			name: "name nil list tasks error",
			fieldsGetter: func(ctrl *gomock.Controller, _ *ListAnnotationEvaluatorsRequest) fields {
				taskRepoMock := newTaskRepoMock(ctrl)
				taskRepoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return(nil, int64(0), fmt.Errorf("list error"))
				return fields{taskRepo: taskRepoMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationEvaluatorsRequest{WorkspaceID: 30},
			},
			wantErr: true,
		},
		{
			name: "name nil tasks empty",
			fieldsGetter: func(ctrl *gomock.Controller, _ *ListAnnotationEvaluatorsRequest) fields {
				taskRepoMock := newTaskRepoMock(ctrl)
				taskRepoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*taskentity.ObservabilityTask{}, int64(0), nil)
				return fields{
					taskRepo: taskRepoMock,
					after: func(t *testing.T, resp *ListAnnotationEvaluatorsResp) {
						if assert.NotNil(t, resp) {
							assert.Empty(t, resp.Evaluators)
						}
					},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationEvaluatorsRequest{WorkspaceID: 40},
			},
		},
		{
			name: "name nil batch get error",
			fieldsGetter: func(ctrl *gomock.Controller, _ *ListAnnotationEvaluatorsRequest) fields {
				taskRepoMock := newTaskRepoMock(ctrl)
				taskRepoMock.EXPECT().ListTasks(gomock.Any(), gomock.Any()).Return([]*taskentity.ObservabilityTask{
					{TaskConfig: &taskentity.TaskConfig{AutoEvaluateConfigs: []*taskentity.AutoEvaluateConfig{{EvaluatorVersionID: 202}}}},
				}, int64(1), nil)
				evalMock := rpcmocks.NewMockIEvaluatorRPCAdapter(ctrl)
				evalMock.EXPECT().BatchGetEvaluatorVersions(gomock.Any(), gomock.Any()).Return(nil, nil, fmt.Errorf("batch error"))
				return fields{taskRepo: taskRepoMock, evalSvc: evalMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListAnnotationEvaluatorsRequest{WorkspaceID: 50},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl, tt.args.req)
			svc := &TraceServiceImpl{
				taskRepo: f.taskRepo,
				evalSvc:  f.evalSvc,
			}
			resp, err := svc.ListAnnotationEvaluators(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if f.after != nil {
				f.after(t, resp)
			}
		})
	}
}

func TestTraceServiceImpl_ExtractSpanInfo(t *testing.T) {
	type fields struct {
		traceRepo      repo.ITraceRepo
		tenantProvider tenant.ITenantProvider
		after          func(t *testing.T, resp *ExtractSpanInfoResp)
	}
	type args struct {
		ctx context.Context
		req *ExtractSpanInfoRequest
	}

	makeSpan := func(req *ExtractSpanInfoRequest) *loop_span.Span {
		now := time.Now()
		return &loop_span.Span{
			SpanID:          req.SpanIds[0],
			TraceID:         req.TraceID,
			WorkspaceID:     strconv.FormatInt(req.WorkspaceID, 10),
			StartTime:       now.UnixMicro(),
			LogicDeleteTime: now.Add(24 * time.Hour).UnixMicro(),
			Input:           "hello world",
			SystemTagsString: map[string]string{
				loop_span.SpanFieldTenant: "tenant",
			},
		}
	}

	fieldKey := "input"
	fieldMapping := entity.FieldMapping{
		FieldSchema: entity.FieldSchema{
			Key:         ptr.Of(fieldKey),
			Name:        "Input",
			ContentType: entity.ContentType_Text,
		},
		TraceFieldKey:      "Input",
		TraceFieldJsonpath: "",
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller, req *ExtractSpanInfoRequest) fields
		args         args
		wantErr      bool
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller, req *ExtractSpanInfoRequest) fields {
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				span := makeSpan(req)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				return fields{
					traceRepo:      traceRepoMock,
					tenantProvider: tenantMock,
					after: func(t *testing.T, resp *ExtractSpanInfoResp) {
						if assert.NotNil(t, resp) {
							assert.Len(t, resp.SpanInfos, 1)
							assert.Equal(t, span.SpanID, resp.SpanInfos[0].SpanID)
							assert.Len(t, resp.SpanInfos[0].FieldList, 1)
							assert.Equal(t, fieldKey, resp.SpanInfos[0].FieldList[0].GetKey())
							assert.Equal(t, "hello world", resp.SpanInfos[0].FieldList[0].Content.GetText())
						}
					},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &ExtractSpanInfoRequest{
					WorkspaceID:   100,
					TraceID:       "trace-1",
					SpanIds:       []string{"span-1"},
					StartTime:     time.Now().Add(-time.Minute).UnixMilli(),
					EndTime:       time.Now().UnixMilli(),
					PlatformType:  loop_span.PlatformCozeLoop,
					FieldMappings: []entity.FieldMapping{fieldMapping},
				},
			},
		},
		{
			name: "tenant error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ExtractSpanInfoRequest) fields {
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return(nil, fmt.Errorf("tenant error"))
				return fields{tenantProvider: tenantMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ExtractSpanInfoRequest{WorkspaceID: 1, TraceID: "trace", SpanIds: []string{"span"}, PlatformType: loop_span.PlatformCozeLoop},
			},
			wantErr: true,
		},
		{
			name: "list spans error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ExtractSpanInfoRequest) fields {
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("list error"))
				return fields{traceRepo: traceRepoMock, tenantProvider: tenantMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ExtractSpanInfoRequest{WorkspaceID: 2, TraceID: "trace", SpanIds: []string{"span"}, PlatformType: loop_span.PlatformCozeLoop, FieldMappings: []entity.FieldMapping{fieldMapping}},
			},
			wantErr: true,
		},
		{
			name: "no spans",
			fieldsGetter: func(ctrl *gomock.Controller, req *ExtractSpanInfoRequest) fields {
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
				return fields{traceRepo: traceRepoMock, tenantProvider: tenantMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ExtractSpanInfoRequest{WorkspaceID: 3, TraceID: "trace", SpanIds: []string{"span"}, PlatformType: loop_span.PlatformCozeLoop, FieldMappings: []entity.FieldMapping{fieldMapping}},
			},
			wantErr: true,
		},
		{
			name: "build extract info error",
			fieldsGetter: func(ctrl *gomock.Controller, req *ExtractSpanInfoRequest) fields {
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant"}, nil)
				traceRepoMock := repomocks.NewMockITraceRepo(ctrl)
				span := makeSpan(req)
				span.Input = "invalid-json"
				traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
				return fields{traceRepo: traceRepoMock, tenantProvider: tenantMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ExtractSpanInfoRequest{
					WorkspaceID:  4,
					TraceID:      "trace",
					SpanIds:      []string{"span"},
					PlatformType: loop_span.PlatformCozeLoop,
					FieldMappings: []entity.FieldMapping{
						{
							FieldSchema: entity.FieldSchema{
								Key:         ptr.Of(fieldKey),
								Name:        "Input",
								ContentType: entity.ContentType_MultiPart,
							},
							TraceFieldKey:      "Input",
							TraceFieldJsonpath: "",
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl, tt.args.req)
			svc := &TraceServiceImpl{
				traceRepo:      f.traceRepo,
				tenantProvider: f.tenantProvider,
			}
			resp, err := svc.ExtractSpanInfo(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if f.after != nil {
				f.after(t, resp)
			}
		})
	}
}

func Test_buildContent(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		content := kitexdataset.NewContent()
		ct := common.ContentTypeText
		text := "hello"
		content.SetContentType(&ct)
		content.SetText(&text)
		data, err := json.Marshal(content)
		assert.NoError(t, err)
		result := buildContent(string(data))
		assert.Equal(t, ct, result.GetContentType())
		assert.Equal(t, text, result.GetText())
	})

	t.Run("fallback for invalid json", func(t *testing.T) {
		value := "plain"
		result := buildContent(value)
		assert.Equal(t, common.ContentTypeText, result.GetContentType())
		assert.Equal(t, value, result.GetText())
	})
}

func TestTraceServiceImpl_batchGetPreSpanFromCk(t *testing.T) {
	type fields struct {
		traceRepo repo.ITraceRepo
	}
	type args struct {
		ctx       context.Context
		spanIDs   []string
		tenants   []string
		startTime int64
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         []*loop_span.Span
		wantErr      bool
	}{
		{
			name: "batch get pre span - normal case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: []*loop_span.Span{
					{SpanID: "span1", TraceID: "trace1"},
					{SpanID: "span2", TraceID: "trace1"},
				}}, nil)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx:       context.Background(),
				spanIDs:   []string{"span1", "span2"},
				tenants:   []string{"tenant1"},
				startTime: time.Now().UnixMilli(),
			},
			want: []*loop_span.Span{
				{SpanID: "span1", TraceID: "trace1"},
				{SpanID: "span2", TraceID: "trace1"},
			},
			wantErr: false,
		},
		{
			name: "batch get pre span - empty span IDs",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{traceRepo: repomocks.NewMockITraceRepo(ctrl)}
			},
			args: args{
				ctx:       context.Background(),
				spanIDs:   []string{},
				tenants:   []string{"tenant1"},
				startTime: time.Now().UnixMilli(),
			},
			want:    []*loop_span.Span{},
			wantErr: false,
		},
		{
			name: "batch get pre span - single span",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: []*loop_span.Span{
					{SpanID: "span1", TraceID: "trace1"},
				}}, nil)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx:       context.Background(),
				spanIDs:   []string{"span1"},
				tenants:   []string{"tenant1"},
				startTime: time.Now().UnixMilli(),
			},
			want: []*loop_span.Span{
				{SpanID: "span1", TraceID: "trace1"},
			},
			wantErr: false,
		},
		{
			name: "batch get pre span - exactly 100 spans",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				// Should be called once since we have exactly 100 spans
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: []*loop_span.Span{
					{SpanID: "span1", TraceID: "trace1"},
				}}, nil)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx:       context.Background(),
				spanIDs:   make([]string, 100), // Exactly 100 spans
				tenants:   []string{"tenant1"},
				startTime: time.Now().UnixMilli(),
			},
			want: []*loop_span.Span{
				{SpanID: "span1", TraceID: "trace1"},
			},
			wantErr: false,
		},
		{
			name: "batch get pre span - more than 100 spans",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				// Should be called twice since we have 150 spans
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: []*loop_span.Span{
					{SpanID: "span1", TraceID: "trace1"},
				}}, nil).Times(2)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx:       context.Background(),
				spanIDs:   make([]string, 150), // More than 100 spans
				tenants:   []string{"tenant1"},
				startTime: time.Now().UnixMilli(),
			},
			want: []*loop_span.Span{
				{SpanID: "span1", TraceID: "trace1"},
				{SpanID: "span1", TraceID: "trace1"},
			},
			wantErr: false,
		},
		{
			name: "batch get pre span - db error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("db error"))
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx:       context.Background(),
				spanIDs:   []string{"span1"},
				tenants:   []string{"tenant1"},
				startTime: time.Now().UnixMilli(),
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
			r := &TraceServiceImpl{
				traceRepo: fields.traceRepo,
			}
			got, err := r.batchGetPreSpan(tt.args.ctx, tt.args.spanIDs, tt.args.tenants, tt.args.startTime)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.Equal(t, len(tt.want), len(got))
			}
		})
	}
}

func TestTraceServiceImpl_ListPreSpan_Comprehensive(t *testing.T) {
	type fields struct {
		traceRepo          repo.ITraceRepo
		traceConfig        config.ITraceConfig
		traceProducer      mq.ITraceProducer
		annotationProducer mq.IAnnotationProducer
		metrics            metrics.ITraceMetrics
		buildHelper        TraceFilterProcessorBuilder
		tenantProvider     tenant.ITenantProvider
		evalSvc            rpc.IEvaluatorRPCAdapter
		taskRepo           taskRepo.ITaskRepo
		persistCmd         redis.PersistentCmdable
	}
	type args struct {
		ctx context.Context
		req *ListPreSpanReq
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *ListPreSpanResp
		wantErr      bool
	}{
		{
			name: "list pre span successfully - full flow",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				// Mock GetPreSpanIDs
				repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return(
					[]string{"span1", "span2", "current_span"},
					[]string{"resp1", "resp2", "resp3"},
					nil,
				)
				// Mock ListSpans for batchGetPreSpan
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(
					&repo.ListSpansResult{
						Spans: []*loop_span.Span{
							{
								SpanID:  "span1",
								TraceID: "trace1",
								SystemTagsString: map[string]string{
									keyResponseID:         "resp1",
									keyPreviousResponseID: "prev1",
								},
								WorkspaceID: "1",
							},
							{
								SpanID:  "span2",
								TraceID: "trace1",
								SystemTagsString: map[string]string{
									keyResponseID:         "resp2",
									keyPreviousResponseID: "prev2",
								},
								WorkspaceID: "1",
							},
							{
								SpanID:  "current_span",
								TraceID: "trace1",
								SystemTagsString: map[string]string{
									keyResponseID:         "resp3",
									keyPreviousResponseID: "prev_resp_id",
								},
								WorkspaceID: "1",
							},
						},
					}, nil)

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
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					StartTime:          time.Now().UnixMilli(),
					TraceID:            "trace1",
					SpanID:             "current_span",
					PreviousResponseID: "prev_resp_id",
					PlatformType:       loop_span.PlatformCozeLoop,
				},
			},
			want: &ListPreSpanResp{
				Spans: loop_span.SpanList{
					{
						SpanID:  "span1",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyResponseID:         "resp1",
							keyPreviousResponseID: "prev1",
						},
						WorkspaceID: "1",
					},
					{
						SpanID:  "span2",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyResponseID:         "resp2",
							keyPreviousResponseID: "prev2",
						},
						WorkspaceID: "1",
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
				tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("tenant error")).AnyTimes()

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
				req: &ListPreSpanReq{
					WorkspaceID:  1,
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
		{
			name: "get pre span IDs failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return(nil, nil, fmt.Errorf("redis error"))

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
				req: &ListPreSpanReq{
					WorkspaceID:  1,
					PlatformType: loop_span.PlatformCozeLoop,
				},
			},
			wantErr: true,
		},
		{
			name: "batch get pre span failed",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().GetPreSpanIDs(gomock.Any(), gomock.Any()).Return(
					[]string{"span1"},
					[]string{"resp1"},
					nil,
				)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("db error"))

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
				req: &ListPreSpanReq{
					WorkspaceID:  1,
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
				fields.tenantProvider,
				fields.evalSvc,
				fields.taskRepo,
				fields.persistCmd,
			)
			_, err := r.ListPreSpan(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceServiceImpl_checkGetPreSpanAuth_Comprehensive(t *testing.T) {
	type fields struct {
		traceRepo repo.ITraceRepo
	}
	type args struct {
		ctx                context.Context
		req                *ListPreSpanReq
		tenants            []string
		preAndCurrentSpans []*loop_span.Span
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
		errMsg       string
	}{
		{
			name: "auth check success - current span in workspace",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{traceRepo: repomocks.NewMockITraceRepo(ctrl)}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span1",
					TraceID:            "trace1",
					PreviousResponseID: "prev1",
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span1",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "prev1",
						},
						WorkspaceID: "1", // Same workspace
					},
				},
			},
			wantErr: false,
		},
		{
			name: "auth check success - database verification with result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{
						{SpanID: "found_span", TraceID: "trace1", WorkspaceID: "1"},
					},
				}, nil)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span2",
					TraceID:            "trace1",
					PreviousResponseID: "prev2",
					StartTime:          time.Now().UnixMilli(),
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span2",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "prev2",
						},
						WorkspaceID: "999", // Different workspace
					},
				},
			},
			wantErr: false,
		},
		{
			name: "auth check failed - current span not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{traceRepo: repomocks.NewMockITraceRepo(ctrl)}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "missing_span",
					TraceID:            "trace1",
					PreviousResponseID: "prev1",
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span1",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "prev1",
						},
						WorkspaceID: "1",
					},
				},
			},
			wantErr: true,
			errMsg:  "current span not found",
		},
		{
			name: "auth check failed - previous response ID mismatch",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{traceRepo: repomocks.NewMockITraceRepo(ctrl)}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span1",
					TraceID:            "trace1",
					PreviousResponseID: "expected_prev",
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span1",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "actual_prev", // Mismatch
						},
						WorkspaceID: "1",
					},
				},
			},
			wantErr: true,
			errMsg:  "req previous_response_id is not current span's",
		},
		{
			name: "auth check failed - database verification with no result",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
					Spans: loop_span.SpanList{}, // No spans found
				}, nil)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span2",
					TraceID:            "trace1",
					PreviousResponseID: "prev2",
					StartTime:          time.Now().UnixMilli(),
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span2",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "prev2",
						},
						WorkspaceID: "999", // Different workspace
					},
				},
			},
			wantErr: true,
			errMsg:  "no span in this workspace",
		},
		{
			name: "auth check failed - database verification error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("db error"))
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span2",
					TraceID:            "trace1",
					PreviousResponseID: "prev2",
					StartTime:          time.Now().UnixMilli(),
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span2",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "prev2",
						},
						WorkspaceID: "999", // Different workspace
					},
				},
			},
			wantErr: true,
			errMsg:  "db error",
		},
		{
			name: "auth check - empty preAndCurrentSpans",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span1",
					TraceID:            "trace1",
					PreviousResponseID: "prev1",
					StartTime:          time.Now().UnixMilli(),
				},
				tenants:            []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{}, // Empty
			},
			wantErr: true,
			errMsg:  "current span not found",
		},
		{
			name: "auth check - nil preAndCurrentSpans",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockITraceRepo(ctrl)
				return fields{traceRepo: repoMock}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span1",
					TraceID:            "trace1",
					PreviousResponseID: "prev1",
					StartTime:          time.Now().UnixMilli(),
				},
				tenants:            []string{"tenant1"},
				preAndCurrentSpans: nil, // Nil
			},
			wantErr: true,
			errMsg:  "current span not found",
		},
		{
			name: "auth check - workspace ID conversion edge case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{traceRepo: repomocks.NewMockITraceRepo(ctrl)}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        0, // Zero workspace ID
					SpanID:             "span1",
					TraceID:            "trace1",
					PreviousResponseID: "prev1",
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SpanID:  "span1",
						TraceID: "trace1",
						SystemTagsString: map[string]string{
							keyPreviousResponseID: "prev1",
						},
						WorkspaceID: "0", // String "0"
					},
				},
			},
			wantErr: false,
		},
		{
			name: "auth check - current span with missing previous response ID",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{traceRepo: repomocks.NewMockITraceRepo(ctrl)}
			},
			args: args{
				ctx: context.Background(),
				req: &ListPreSpanReq{
					WorkspaceID:        1,
					SpanID:             "span1",
					TraceID:            "trace1",
					PreviousResponseID: "prev1",
				},
				tenants: []string{"tenant1"},
				preAndCurrentSpans: []*loop_span.Span{
					{
						SystemTagsString: map[string]string{
							// Missing keyPreviousResponseID
						},
						SpanID:      "span1",  // blankx
						TraceID:     "trace1", // blankx
						WorkspaceID: "1",
					},
				},
			},
			wantErr: true,
			errMsg:  "req previous_response_id is not current span's",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			r := &TraceServiceImpl{
				traceRepo: fields.traceRepo,
			}
			err := r.checkGetPreSpanAuth(tt.args.ctx, tt.args.req, tt.args.tenants, tt.args.preAndCurrentSpans)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
