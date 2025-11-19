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
	annodto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/annotation"
	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/view"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	tenantmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	domaincommon "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	svcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestTraceApplication_CreateView(t *testing.T) {
	type fields struct {
		repo repo.IViewRepo
		auth rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *trace.CreateViewRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.CreateViewResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().CreateView(gomock.Any(), gomock.Any()).Return(int64(0), nil)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.CreateViewRequest{
					WorkspaceID: 12,
					Filters:     "{}",
					ViewName:    "test",
				},
			},
			want: &trace.CreateViewResponse{
				ID: 0,
			},
			wantErr: false,
		},
		{
			name: "error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().CreateView(gomock.Any(), gomock.Any()).Return(int64(0), assert.AnError)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.CreateViewRequest{
					WorkspaceID: 12,
					Filters:     "{}",
					ViewName:    "test",
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
			tr := &TraceApplication{
				viewRepo: fields.repo,
				authSvc:  fields.auth,
			}
			got, err := tr.CreateView(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_UpdateView(t *testing.T) {
	type fields struct {
		repo repo.IViewRepo
		auth rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *trace.UpdateViewRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.UpdateViewResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckViewPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().GetView(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ObservabilityView{}, nil)
				mockRepo.EXPECT().UpdateView(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.UpdateViewRequest{
					WorkspaceID: 12,
					ViewName:    ptr.Of("1"),
				},
			},
			want:    &trace.UpdateViewResponse{},
			wantErr: false,
		},
		{
			name: "error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckViewPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().GetView(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ObservabilityView{}, nil)
				mockRepo.EXPECT().UpdateView(gomock.Any(), gomock.Any()).Return(assert.AnError)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.UpdateViewRequest{
					WorkspaceID: 12,
					ViewName:    ptr.Of("1"),
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
			tr := &TraceApplication{
				viewRepo: fields.repo,
				authSvc:  fields.auth,
			}
			got, err := tr.UpdateView(tt.args.ctx, tt.args.req)
			t.Log(got, err)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_DeleteView(t *testing.T) {
	type fields struct {
		repo repo.IViewRepo
		auth rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *trace.DeleteViewRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.DeleteViewResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckViewPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DeleteView(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.DeleteViewRequest{
					ID:          1,
					WorkspaceID: 12,
				},
			},
			want:    &trace.DeleteViewResponse{},
			wantErr: false,
		},
		{
			name: "error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckViewPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().DeleteView(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.DeleteViewRequest{
					ID:          1,
					WorkspaceID: 12,
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
			tr := &TraceApplication{
				viewRepo: fields.repo,
				authSvc:  fields.auth,
			}
			got, err := tr.DeleteView(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_ListViews(t *testing.T) {
	type fields struct {
		repo repo.IViewRepo
		auth rpc.IAuthProvider
		conf config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.ListViewsRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ListViewsResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConf := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().ListViews(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*entity.ObservabilityView{}, nil)
				mockConf.EXPECT().GetSystemViews(gomock.Any()).Return([]*config.SystemView{}, nil)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
					conf: mockConf,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.ListViewsRequest{
					WorkspaceID: 12,
				},
			},
			want: &trace.ListViewsResponse{
				Views: make([]*view.View, 0),
			},
			wantErr: false,
		},
		{
			name: "error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockRepo := repomock.NewMockIViewRepo(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConf := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockRepo.EXPECT().ListViews(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				mockConf.EXPECT().GetSystemViews(gomock.Any()).Return([]*config.SystemView{}, nil)
				return fields{
					repo: mockRepo,
					auth: mockAuth,
					conf: mockConf,
				}
			},
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: "123"}),
				req: &trace.ListViewsRequest{
					WorkspaceID: 12,
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
			tr := &TraceApplication{
				viewRepo:    fields.repo,
				authSvc:     fields.auth,
				traceConfig: fields.conf,
			}
			got, err := tr.ListViews(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_ListSpans(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		tagSvc   rpc.ITagRPCAdapter
		evalSvc  rpc.IEvaluatorRPCAdapter
		userSvc  rpc.IUserProvider
		traceCfg config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.ListSpansRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ListSpansResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockEval := rpcmock.NewMockIEvaluatorRPCAdapter(ctrl)
				mockUser := rpcmock.NewMockIUserProvider(ctrl)
				mockTag.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				mockEval.EXPECT().BatchGetEvaluatorVersions(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				mockUser.EXPECT().GetUserInfo(gomock.Any(), gomock.Any()).Return(nil, nil, nil)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&service.ListSpansResp{
					Spans: loop_span.SpanList{
						{
							TraceID:   "1",
							StartTime: 0,
							Annotations: loop_span.AnnotationList{
								{
									AnnotationType: loop_span.AnnotationTypeManualFeedback,
									Value:          loop_span.NewLongValue(1),
									StartTime:      time.UnixMicro(0),
									CreatedAt:      time.UnixMicro(0),
									UpdatedAt:      time.UnixMicro(0),
								},
								{
									AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
									Metadata: loop_span.AutoEvaluateMetadata{
										TaskID:             123,
										EvaluatorRecordID:  123,
										EvaluatorVersionID: 123,
									},
									Value:     loop_span.NewDoubleValue(1),
									StartTime: time.UnixMicro(0),
									CreatedAt: time.UnixMicro(0),
									UpdatedAt: time.UnixMicro(0),
								},
								{
									AnnotationType: loop_span.AnnotationTypeManualFeedback,
									Value:          loop_span.NewStringValue("1.0"),
									StartTime:      time.UnixMicro(0),
									CreatedAt:      time.UnixMicro(0),
									UpdatedAt:      time.UnixMicro(0),
								},
								{
									AnnotationType: loop_span.AnnotationTypeCozeFeedback,
									Value:          loop_span.NewStringValue("like"),
									StartTime:      time.UnixMicro(0),
									CreatedAt:      time.UnixMicro(0),
									UpdatedAt:      time.UnixMicro(0),
								},
								{
									AnnotationType: loop_span.AnnotationTypeManualFeedback,
									Value:          loop_span.NewBoolValue(true),
									StartTime:      time.UnixMicro(0),
									CreatedAt:      time.UnixMicro(0),
									UpdatedAt:      time.UnixMicro(0),
								},
							},
						},
					},
				}, nil)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
					tagSvc:   mockTag,
					evalSvc:  mockEval,
					userSvc:  mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListSpansRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want: &trace.ListSpansResponse{
				Spans: []*span.OutputSpan{
					{
						TraceID:         "1",
						Type:            span.SpanTypeUnknown,
						Status:          span.SpanStatusSuccess,
						LogicDeleteDate: ptr.Of(int64(0)),
						CallType:        ptr.Of(""),
						CustomTags:      map[string]string{},
						SystemTags:      map[string]string{},
						Annotations: []*annodto.Annotation{
							{
								ID:          ptr.Of(""),
								TraceID:     ptr.Of(""),
								SpanID:      ptr.Of(""),
								WorkspaceID: ptr.Of(""),
								Key:         ptr.Of(""),
								Status:      ptr.Of(""),
								Reasoning:   ptr.Of(""),
								Type:        ptr.Of(annodto.AnnotationTypeManualFeedback),
								ValueType:   ptr.Of(annodto.ValueTypeLong),
								Value:       ptr.Of("1"),
								StartTime:   ptr.Of(int64(0)),
								BaseInfo: &commondto.BaseInfo{
									UpdatedAt: ptr.Of(int64(0)),
									CreatedAt: ptr.Of(int64(0)),
								},
								ManualFeedback: &annodto.ManualFeedback{
									TagKeyID: 0,
								},
							},
							{
								ID:          ptr.Of(""),
								TraceID:     ptr.Of(""),
								SpanID:      ptr.Of(""),
								WorkspaceID: ptr.Of(""),
								Key:         ptr.Of(""),
								Status:      ptr.Of(""),
								Reasoning:   ptr.Of(""),
								Type:        ptr.Of(annodto.AnnotationTypeAutoEvaluate),
								ValueType:   ptr.Of(annodto.ValueTypeDouble),
								Value:       ptr.Of("1"),
								AutoEvaluate: &annodto.AutoEvaluate{
									TaskID:             "123",
									RecordID:           123,
									EvaluatorVersionID: 123,
									EvaluatorResult_: &annodto.EvaluatorResult_{
										Score:     ptr.Of(1.0),
										Reasoning: ptr.Of(""),
									},
								},
								StartTime: ptr.Of(int64(0)),
								BaseInfo: &commondto.BaseInfo{
									UpdatedAt: ptr.Of(int64(0)),
									CreatedAt: ptr.Of(int64(0)),
								},
							},
							{
								ID:          ptr.Of(""),
								TraceID:     ptr.Of(""),
								SpanID:      ptr.Of(""),
								WorkspaceID: ptr.Of(""),
								Key:         ptr.Of(""),
								Status:      ptr.Of(""),
								Reasoning:   ptr.Of(""),
								Type:        ptr.Of(annodto.AnnotationTypeManualFeedback),
								ValueType:   ptr.Of(annodto.ValueTypeString),
								Value:       ptr.Of("1.0"),
								StartTime:   ptr.Of(int64(0)),
								BaseInfo: &commondto.BaseInfo{
									UpdatedAt: ptr.Of(int64(0)),
									CreatedAt: ptr.Of(int64(0)),
								},
								ManualFeedback: &annodto.ManualFeedback{
									TagKeyID: 0,
								},
							},
							{
								ID:          ptr.Of(""),
								TraceID:     ptr.Of(""),
								SpanID:      ptr.Of(""),
								WorkspaceID: ptr.Of(""),
								Key:         ptr.Of(""),
								Status:      ptr.Of(""),
								Reasoning:   ptr.Of(""),
								Type:        ptr.Of(annodto.AnnotationTypeCozeFeedback),
								ValueType:   ptr.Of(annodto.ValueTypeString),
								Value:       ptr.Of("赞"),
								StartTime:   ptr.Of(int64(0)),
								BaseInfo: &commondto.BaseInfo{
									UpdatedAt: ptr.Of(int64(0)),
									CreatedAt: ptr.Of(int64(0)),
								},
							},
							{
								ID:          ptr.Of(""),
								TraceID:     ptr.Of(""),
								SpanID:      ptr.Of(""),
								WorkspaceID: ptr.Of(""),
								Key:         ptr.Of(""),
								Status:      ptr.Of(""),
								Reasoning:   ptr.Of(""),
								Type:        ptr.Of(annodto.AnnotationTypeManualFeedback),
								ValueType:   ptr.Of(annodto.ValueTypeBool),
								Value:       ptr.Of("true"),
								StartTime:   ptr.Of(int64(0)),
								BaseInfo: &commondto.BaseInfo{
									UpdatedAt: ptr.Of(int64(0)),
									CreatedAt: ptr.Of(int64(0)),
								},
								ManualFeedback: &annodto.ManualFeedback{
									TagKeyID: 0,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "list spans error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListSpansRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission check error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad"))
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListSpansRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "parameter error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListSpansRequest{
					WorkspaceID: 0,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				traceConfig:  fields.traceCfg,
				tagSvc:       fields.tagSvc,
				evalSvc:      fields.evalSvc,
				userSvc:      fields.userSvc,
			}
			got, err := tr.ListSpans(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_GetTrace(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		traceCfg config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.GetTraceRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.GetTraceResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(&service.GetTraceResp{}, nil)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.GetTraceRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					TraceID:     "123",
				},
			},
			want: &trace.GetTraceResponse{
				Spans: make([]*span.OutputSpan, 0),
				TracesAdvanceInfo: &trace.TraceAdvanceInfo{
					Tokens: &trace.TokenCost{},
				},
			},
			wantErr: false,
		},
		{
			name: "get trace error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.GetTraceRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					TraceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission check error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad"))
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.GetTraceRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					TraceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "parameter error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.GetTraceRequest{
					WorkspaceID: 0,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					TraceID:     "123",
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get trace with span case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(&service.GetTraceResp{}, nil)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.GetTraceRequest{
					WorkspaceID: 12,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					TraceID:     "123",
					SpanIds:     []string{"123"},
				},
			},
			want: &trace.GetTraceResponse{
				Spans: make([]*span.OutputSpan, 0),
				TracesAdvanceInfo: &trace.TraceAdvanceInfo{
					Tokens: &trace.TokenCost{},
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				traceConfig:  fields.traceCfg,
			}
			got, err := tr.GetTrace(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_SearchTraceTree(t *testing.T) {
	start := time.Now().Add(-time.Hour).UnixMilli()
	end := time.Now().UnixMilli()
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		traceCfg config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.SearchTraceTreeRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.SearchTraceTreeResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(&service.GetTraceResp{
					TraceId: "trace-1",
					Spans:   loop_span.SpanList{},
				}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 12,
					TraceID:     "trace-1",
					StartTime:   start,
					EndTime:     end,
				},
			},
			want: &trace.SearchTraceTreeResponse{
				Spans: []*span.OutputSpan{},
				TracesAdvanceInfo: &trace.TraceAdvanceInfo{
					TraceID: "trace-1",
					Tokens:  &trace.TokenCost{},
				},
			},
			wantErr: false,
		},
		{
			name: "trace service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 12,
					TraceID:     "trace-1",
					StartTime:   start,
					EndTime:     end,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("permission denied"))
				return fields{
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 12,
					TraceID:     "trace-1",
					StartTime:   start,
					EndTime:     end,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid request",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 0,
					TraceID:     "trace-1",
					StartTime:   start,
					EndTime:     end,
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
			app := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				traceConfig:  fields.traceCfg,
			}
			got, err := app.SearchTraceTree(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_validateSearchTraceTreeReq(t *testing.T) {
	validStart := time.Now().Add(-time.Hour).UnixMilli()
	validEnd := time.Now().UnixMilli()
	type fields struct {
		traceCfg config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.SearchTraceTreeRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
		wantStart    *int64
		wantEnd      *int64
	}{
		{
			name: "nil request",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: nil,
			},
			wantErr: true,
		},
		{
			name: "invalid workspace",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 0,
					TraceID:     "trace-1",
					StartTime:   validStart,
					EndTime:     validEnd,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid trace id",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 1,
					TraceID:     "",
					StartTime:   validStart,
					EndTime:     validEnd,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid time range",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				return fields{
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 1,
					TraceID:     "trace-1",
					StartTime:   int64(0),
					EndTime:     int64(0),
				},
			},
			wantErr: true,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				return fields{
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.SearchTraceTreeRequest{
					WorkspaceID: 1,
					TraceID:     "trace-1",
					StartTime:   validStart,
					EndTime:     validEnd,
				},
			},
			wantErr:   false,
			wantStart: ptr.Of(validStart),
			wantEnd:   ptr.Of(validEnd),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			app := &TraceApplication{
				traceConfig: fields.traceCfg,
			}
			err := app.validateSearchTraceTreeReq(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr && tt.args.req != nil {
				if tt.wantStart != nil {
					assert.Equal(t, *tt.wantStart, tt.args.req.GetStartTime())
				}
				if tt.wantEnd != nil {
					assert.Equal(t, *tt.wantEnd, tt.args.req.GetEndTime())
				}
			}
		})
	}
}

func TestTraceApplication_buildSearchTraceTreeSvcReq(t *testing.T) {
	start := time.Now().Add(-time.Hour).UnixMilli()
	end := time.Now().UnixMilli()
	app := &TraceApplication{}
	tests := []struct {
		name    string
		req     *trace.SearchTraceTreeRequest
		wantErr bool
		check   func(t *testing.T, got *service.GetTraceReq)
	}{
		{
			name: "default platform",
			req: &trace.SearchTraceTreeRequest{
				WorkspaceID: 1,
				TraceID:     "trace-1",
				StartTime:   start,
				EndTime:     end,
			},
			check: func(t *testing.T, got *service.GetTraceReq) {
				assert.Equal(t, int64(1), got.WorkspaceID)
				assert.Equal(t, "trace-1", got.TraceID)
				assert.Equal(t, start, got.StartTime)
				assert.Equal(t, end, got.EndTime)
				assert.False(t, got.WithDetail)
				assert.Equal(t, loop_span.PlatformCozeLoop, got.PlatformType)
				assert.Nil(t, got.Filters)
			},
		},
		{
			name: "custom platform with filters",
			req: func() *trace.SearchTraceTreeRequest {
				platformType := commondto.PlatformTypePrompt
				return &trace.SearchTraceTreeRequest{
					WorkspaceID:  2,
					TraceID:      "trace-2",
					StartTime:    start,
					EndTime:      end,
					PlatformType: &platformType,
					Filters: &filter.FilterFields{
						FilterFields: []*filter.FilterField{{}},
					},
				}
			}(),
			check: func(t *testing.T, got *service.GetTraceReq) {
				assert.Equal(t, int64(2), got.WorkspaceID)
				assert.Equal(t, "trace-2", got.TraceID)
				assert.Equal(t, start, got.StartTime)
				assert.Equal(t, end, got.EndTime)
				assert.Equal(t, loop_span.PlatformType(commondto.PlatformTypePrompt), got.PlatformType)
				if assert.NotNil(t, got.Filters) {
					assert.Len(t, got.Filters.FilterFields, 1)
				}
			},
		},
		{
			name: "invalid filters",
			req: func() *trace.SearchTraceTreeRequest {
				invalid := filter.QueryRelation("invalid")
				return &trace.SearchTraceTreeRequest{
					WorkspaceID: 3,
					TraceID:     "trace-3",
					StartTime:   start,
					EndTime:     end,
					Filters: &filter.FilterFields{
						QueryAndOr: &invalid,
					},
				}
			}(),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := app.buildSearchTraceTreeSvcReq(tt.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Nil(t, got)
				return
			}
			if tt.check != nil {
				checkFn := tt.check
				checkFn(t, got)
			}
		})
	}
}

func TestTraceApplication_BatchGetTracesAdvanceInfo(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		traceCfg config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.BatchGetTracesAdvanceInfoRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.BatchGetTracesAdvanceInfoResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				mockSvc.EXPECT().GetTracesAdvanceInfo(gomock.Any(), gomock.Any()).Return(&service.GetTracesAdvanceInfoResp{}, nil)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.BatchGetTracesAdvanceInfoRequest{
					WorkspaceID: 123,
					Traces: []*trace.TraceQueryParams{
						{
							TraceID:   "123",
							StartTime: time.Now().Add(-time.Hour).UnixMilli(),
							EndTime:   time.Now().UnixMilli(),
						},
					},
				},
			},
			want: &trace.BatchGetTracesAdvanceInfoResponse{
				TracesAdvanceInfo: []*trace.TraceAdvanceInfo{},
			},
			wantErr: false,
		},
		{
			name: "error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(100))
				mockSvc.EXPECT().GetTracesAdvanceInfo(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.BatchGetTracesAdvanceInfoRequest{
					WorkspaceID: 123,
					Traces: []*trace.TraceQueryParams{
						{
							TraceID:   "123",
							StartTime: time.Now().Add(-time.Hour).UnixMilli(),
							EndTime:   time.Now().UnixMilli(),
						},
					},
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				traceConfig:  fields.traceCfg,
			}
			got, err := tr.BatchGetTracesAdvanceInfo(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_IngestTracesInner(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		benefit  benefit.IBenefitService
		tenant   tenant.ITenantProvider
	}
	type args struct {
		ctx context.Context
		req *trace.IngestTracesRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.IngestTracesResponse
		wantErr      bool
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockBenefit := benefitmock.NewMockIBenefitService(ctrl)
				mockBenefit.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{IsEnough: true, AccountAvailable: true, StorageDuration: 7}, nil)
				mockTenant := tenantmock.NewMockITenantProvider(ctrl)
				mockTenant.EXPECT().GetIngestTenant(gomock.Any(), gomock.Any()).Return("")
				mockSvc.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceSvc: mockSvc,
					benefit:  mockBenefit,
					tenant:   mockTenant,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.IngestTracesRequest{
					Spans: []*span.InputSpan{
						{
							WorkspaceID: "1",
							TagsString:  map[string]string{"user_id": "user1"},
						},
					},
				},
			},
			want:    trace.NewIngestTracesResponse(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			app := &TraceApplication{
				traceService: fields.traceSvc,
				benefit:      fields.benefit,
				tenant:       fields.tenant,
			}
			got, err := app.IngestTracesInner(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_GetTracesMetaInfo(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *trace.GetTracesMetaInfoRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.GetTracesMetaInfoResponse
		wantErr      bool
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockSvc.EXPECT().GetTracesMetaInfo(gomock.Any(), gomock.Any()).Return(&service.GetTracesMetaInfoResp{FilesMetas: map[string]*config.FieldMeta{}}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.GetTracesMetaInfoRequest{WorkspaceID: ptr.Of(int64(1))},
			},
			want: &trace.GetTracesMetaInfoResponse{FieldMetas: map[string]*trace.FieldMeta{}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			app := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
			}
			got, err := app.GetTracesMetaInfo(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_CreateManualAnnotation(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		tagSvc   rpc.ITagRPCAdapter
	}
	type args struct {
		ctx context.Context
		req *trace.CreateManualAnnotationRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.CreateManualAnnotationResponse
		wantErr      bool
	}{
		{
			name: "fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.CreateManualAnnotationRequest{
					Annotation: &annodto.Annotation{
						WorkspaceID: ptr.Of("1"),
						Key:         ptr.Of("test"),
						Value:       ptr.Of("test"),
						ValueType:   ptr.Of(annodto.ValueTypeString),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "fail because of invalid tag",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{
					TagKeyId:       1,
					TagContentType: rpc.TagContentTypeContinuousNumber,
				}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.CreateManualAnnotationRequest{
					Annotation: &annodto.Annotation{
						WorkspaceID: ptr.Of("1"),
						Key:         ptr.Of("1"),
						Value:       ptr.Of("test"),
						ValueType:   ptr.Of(annodto.ValueTypeString),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{
					TagKeyId:       1,
					TagContentType: rpc.TagContentTypeFreeText,
				}, nil)
				mockSvc.EXPECT().CreateManualAnnotation(gomock.Any(), gomock.Any()).Return(&service.CreateManualAnnotationResp{
					AnnotationID: "123",
				}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.CreateManualAnnotationRequest{
					Annotation: &annodto.Annotation{
						WorkspaceID: ptr.Of("1"),
						Key:         ptr.Of("1"),
						Value:       ptr.Of("test"),
						ValueType:   ptr.Of(annodto.ValueTypeString),
					},
				},
			},
			want: &trace.CreateManualAnnotationResponse{
				AnnotationID: ptr.Of("123"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				tagSvc:       fields.tagSvc,
			}
			got, err := tr.CreateManualAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_UpdateManualAnnotation(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		tagSvc   rpc.ITagRPCAdapter
	}
	type args struct {
		ctx context.Context
		req *trace.UpdateManualAnnotationRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.UpdateManualAnnotationRequest{
					Annotation: &annodto.Annotation{
						WorkspaceID: ptr.Of("1"),
						Key:         ptr.Of("test"),
						Value:       ptr.Of("test"),
						ValueType:   ptr.Of(annodto.ValueTypeString),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "fail because of invalid tag",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{
					TagKeyId:       1,
					TagContentType: rpc.TagContentTypeContinuousNumber,
				}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.UpdateManualAnnotationRequest{
					Annotation: &annodto.Annotation{
						WorkspaceID: ptr.Of("1"),
						Key:         ptr.Of("1"),
						Value:       ptr.Of("test"),
						ValueType:   ptr.Of(annodto.ValueTypeString),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{
					TagKeyId:       1,
					TagContentType: rpc.TagContentTypeFreeText,
				}, nil)
				mockSvc.EXPECT().UpdateManualAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.UpdateManualAnnotationRequest{
					Annotation: &annodto.Annotation{
						WorkspaceID: ptr.Of("1"),
						Key:         ptr.Of("1"),
						Value:       ptr.Of("test"),
						ValueType:   ptr.Of(annodto.ValueTypeString),
					},
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				tagSvc:       fields.tagSvc,
			}
			_, err := tr.UpdateManualAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceApplication_DeleteManualAnnotation(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		tagSvc   rpc.ITagRPCAdapter
	}
	type args struct {
		ctx context.Context
		req *trace.DeleteManualAnnotationRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
	}{
		{
			name: "fail",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("fail"))
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.DeleteManualAnnotationRequest{
					WorkspaceID:   1,
					AnnotationKey: "1",
				},
			},
			wantErr: true,
		},
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockTag.EXPECT().GetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(&rpc.TagInfo{
					TagKeyId:       1,
					TagContentType: rpc.TagContentTypeFreeText,
				}, nil)
				mockSvc.EXPECT().DeleteManualAnnotation(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.DeleteManualAnnotationRequest{
					WorkspaceID:   1,
					AnnotationKey: "1",
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				tagSvc:       fields.tagSvc,
			}
			_, err := tr.DeleteManualAnnotation(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestTraceApplication_ExportTracesToDataset(t *testing.T) {
	type fields struct {
		traceExportService service.ITraceExportService
		authSvc            rpc.IAuthProvider
		traceConfig        config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.ExportTracesToDatasetRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ExportTracesToDatasetResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockExportSvc := svcmock.NewMockITraceExportService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConfig := confmock.NewMockITraceConfig(ctrl)

				mockConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceExport, "123", gomock.Any()).Return(nil)
				mockExportSvc.EXPECT().ExportTracesToDataset(gomock.Any(), gomock.Any()).Return(&service.ExportTracesToDatasetResponse{
					SuccessCount: 10,
					DatasetID:    1,
					DatasetName:  "test-dataset",
				}, nil)

				return fields{
					traceExportService: mockExportSvc,
					authSvc:            mockAuth,
					traceConfig:        mockConfig,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					SpanIds: []*trace.SpanID{
						{TraceID: "trace1", SpanID: "span1"},
					}, FieldMappings: []*dataset0.FieldMapping{
						{
							FieldSchema: &dataset0.FieldSchema{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
							TraceFieldKey:      "input",
							TraceFieldJsonpath: "$.input",
						},
					},
				},
			},
			want: &trace.ExportTracesToDatasetResponse{
				SuccessCount: ptr.Of(int32(10)),
				DatasetID:    ptr.Of(int64(1)),
				DatasetName:  ptr.Of("test-dataset"),
			},
			wantErr: false,
		},
		{
			name: "invalid request case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExportTracesToDatasetRequest{
					WorkspaceID: 0, // invalid workspace ID
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "auth permission error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConfig := confmock.NewMockITraceConfig(ctrl)

				mockConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceExport, "123", gomock.Any()).Return(fmt.Errorf("permission denied"))

				return fields{
					authSvc:     mockAuth,
					traceConfig: mockConfig,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					SpanIds: []*trace.SpanID{
						{TraceID: "trace1", SpanID: "span1"},
					},
					FieldMappings: []*dataset0.FieldMapping{
						{
							FieldSchema: &dataset0.FieldSchema{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
							TraceFieldKey:      "input",
							TraceFieldJsonpath: "$.input",
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockExportSvc := svcmock.NewMockITraceExportService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConfig := confmock.NewMockITraceConfig(ctrl)

				mockConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceExport, "123", gomock.Any()).Return(nil)
				mockExportSvc.EXPECT().ExportTracesToDataset(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("service error"))

				return fields{
					traceExportService: mockExportSvc,
					authSvc:            mockAuth,
					traceConfig:        mockConfig,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExportTracesToDatasetRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					SpanIds: []*trace.SpanID{
						{TraceID: "trace1", SpanID: "span1"},
					},
					FieldMappings: []*dataset0.FieldMapping{
						{
							FieldSchema: &dataset0.FieldSchema{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
							TraceFieldKey:      "input",
							TraceFieldJsonpath: "$.input",
						},
					},
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
			tr := &TraceApplication{
				traceExportService: fields.traceExportService,
				authSvc:            fields.authSvc,
				traceConfig:        fields.traceConfig,
			}
			got, err := tr.ExportTracesToDataset(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_PreviewExportTracesToDataset(t *testing.T) {
	type fields struct {
		traceExportService service.ITraceExportService
		authSvc            rpc.IAuthProvider
		traceConfig        config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.PreviewExportTracesToDatasetRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.PreviewExportTracesToDatasetResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockExportSvc := svcmock.NewMockITraceExportService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConfig := confmock.NewMockITraceConfig(ctrl)

				mockConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTracePreviewExport, "123", gomock.Any()).Return(nil)
				mockExportSvc.EXPECT().PreviewExportTracesToDataset(gomock.Any(), gomock.Any()).Return(&service.PreviewExportTracesToDatasetResponse{
					Items: []*entity.DatasetItem{
						{
							ID: 1,
							FieldData: []*entity.FieldData{
								{
									Key:  "input",
									Name: "Input",
								},
							},
						},
					},
				}, nil)

				return fields{
					traceExportService: mockExportSvc,
					authSvc:            mockAuth,
					traceConfig:        mockConfig,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.PreviewExportTracesToDatasetRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					SpanIds: []*trace.SpanID{
						{TraceID: "trace1", SpanID: "span1"},
					},
					FieldMappings: []*dataset0.FieldMapping{
						{
							FieldSchema: &dataset0.FieldSchema{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
							TraceFieldKey:      "input",
							TraceFieldJsonpath: "$.input",
						},
					},
				},
			},
			want: &trace.PreviewExportTracesToDatasetResponse{
				Items: []*dataset0.Item{
					{
						Status: dataset0.ItemStatusSuccess,
						FieldList: []*dataset0.FieldData{
							{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid request case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.PreviewExportTracesToDatasetRequest{
					WorkspaceID: 0, // invalid workspace ID
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "auth permission error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConfig := confmock.NewMockITraceConfig(ctrl)

				mockConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTracePreviewExport, "123", gomock.Any()).Return(fmt.Errorf("permission denied"))

				return fields{
					authSvc:     mockAuth,
					traceConfig: mockConfig,
				}
			}, args: args{
				ctx: context.Background(),
				req: &trace.PreviewExportTracesToDatasetRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					SpanIds: []*trace.SpanID{
						{TraceID: "trace1", SpanID: "span1"},
					},
					FieldMappings: []*dataset0.FieldMapping{
						{
							FieldSchema: &dataset0.FieldSchema{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
							TraceFieldKey:      "input",
							TraceFieldJsonpath: "$.input",
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service error case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockExportSvc := svcmock.NewMockITraceExportService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockConfig := confmock.NewMockITraceConfig(ctrl)

				mockConfig.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTracePreviewExport, "123", gomock.Any()).Return(nil)
				mockExportSvc.EXPECT().PreviewExportTracesToDataset(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("service error"))

				return fields{
					traceExportService: mockExportSvc,
					authSvc:            mockAuth,
					traceConfig:        mockConfig,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.PreviewExportTracesToDatasetRequest{
					WorkspaceID: 123,
					StartTime:   time.Now().Add(-time.Hour).UnixMilli(),
					EndTime:     time.Now().UnixMilli(),
					SpanIds: []*trace.SpanID{
						{TraceID: "trace1", SpanID: "span1"},
					},
					FieldMappings: []*dataset0.FieldMapping{
						{
							FieldSchema: &dataset0.FieldSchema{
								Key:  ptr.Of("input"),
								Name: ptr.Of("Input"),
							},
							TraceFieldKey:      "input",
							TraceFieldJsonpath: "$.input",
						},
					},
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
			tr := &TraceApplication{
				traceExportService: fields.traceExportService,
				authSvc:            fields.authSvc,
				traceConfig:        fields.traceConfig,
			}
			got, err := tr.PreviewExportTracesToDataset(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_ChangeEvaluatorScore(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *trace.ChangeEvaluatorScoreRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ChangeEvaluatorScoreResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				expectedAnnotation := &annodto.Annotation{}
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskCreate, "123", false).Return(nil)
				mockSvc.EXPECT().ChangeEvaluatorScore(gomock.Any(), gomock.Any()).Return(&service.ChangeEvaluatorScoreResp{
					Annotation: expectedAnnotation,
				}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ChangeEvaluatorScoreRequest{
					WorkspaceID:  123,
					AnnotationID: "anno",
					SpanID:       "span",
					StartTime:    time.Now().UnixMilli(),
					Correction: &annodto.Correction{
						Score: ptr.Of(1.0),
					},
				},
			},
			want: &trace.ChangeEvaluatorScoreResponse{
				Annotation: &annodto.Annotation{},
			},
			wantErr: false,
		},
		{
			name: "invalid request",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ChangeEvaluatorScoreRequest{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskCreate, "123", false).Return(fmt.Errorf("permission denied"))
				return fields{
					auth: mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ChangeEvaluatorScoreRequest{
					WorkspaceID:  123,
					AnnotationID: "anno",
					SpanID:       "span",
					StartTime:    time.Now().UnixMilli(),
					Correction: &annodto.Correction{
						Score: ptr.Of(1.0),
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskCreate, "123", false).Return(nil)
				mockSvc.EXPECT().ChangeEvaluatorScore(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ChangeEvaluatorScoreRequest{
					WorkspaceID:  123,
					AnnotationID: "anno",
					SpanID:       "span",
					StartTime:    time.Now().UnixMilli(),
					Correction: &annodto.Correction{
						Score: ptr.Of(1.0),
					},
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
			}
			got, err := tr.ChangeEvaluatorScore(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_ListAnnotationEvaluators(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
	}
	type args struct {
		ctx context.Context
		req *trace.ListAnnotationEvaluatorsRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ListAnnotationEvaluatorsResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				evaluators := []*annodto.AnnotationEvaluator{{}}
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, "123", false).Return(nil)
				mockSvc.EXPECT().ListAnnotationEvaluators(gomock.Any(), gomock.Any()).Return(&service.ListAnnotationEvaluatorsResp{Evaluators: evaluators}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationEvaluatorsRequest{
					WorkspaceID: 123,
					Name:        ptr.Of("foo"),
				},
			},
			want:    &trace.ListAnnotationEvaluatorsResponse{Evaluators: []*annodto.AnnotationEvaluator{{}}},
			wantErr: false,
		},
		{
			name: "invalid request",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationEvaluatorsRequest{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, "123", false).Return(fmt.Errorf("permission denied"))
				return fields{
					auth: mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationEvaluatorsRequest{
					WorkspaceID: 123,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceTaskList, "123", false).Return(nil)
				mockSvc.EXPECT().ListAnnotationEvaluators(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationEvaluatorsRequest{
					WorkspaceID: 123,
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
			}
			got, err := tr.ListAnnotationEvaluators(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_ExtractSpanInfo(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		traceCfg config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		req *trace.ExtractSpanInfoRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ExtractSpanInfoResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				spanInfos := []*trace.SpanInfo{{SpanID: "span1"}}
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceRead, "123", false).Return(nil)
				mockSvc.EXPECT().ExtractSpanInfo(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, req *service.ExtractSpanInfoRequest) (*service.ExtractSpanInfoResp, error) {
						assert.Equal(t, int64(123), req.WorkspaceID)
						assert.Equal(t, "trace", req.TraceID)
						assert.Equal(t, []string{"span"}, req.SpanIds)
						return &service.ExtractSpanInfoResp{SpanInfos: spanInfos}, nil
					},
				)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExtractSpanInfoRequest{
					WorkspaceID: 123,
					TraceID:     "trace",
					SpanIds:     []string{"span"},
					StartTime:   ptr.Of(time.Now().Add(-time.Hour).UnixMilli()),
					EndTime:     ptr.Of(time.Now().UnixMilli()),
				},
			},
			want:    &trace.ExtractSpanInfoResponse{SpanInfos: []*trace.SpanInfo{{SpanID: "span1"}}},
			wantErr: false,
		},
		{
			name:         "invalid workspace",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx: context.Background(),
				req: &trace.ExtractSpanInfoRequest{},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:         "span length exceeded",
			fieldsGetter: func(ctrl *gomock.Controller) fields { return fields{} },
			args: args{
				ctx: context.Background(),
				req: &trace.ExtractSpanInfoRequest{
					WorkspaceID: 123,
					SpanIds:     make([]string, MaxSpanLength+1),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "permission error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceRead, "123", false).Return(fmt.Errorf("permission denied"))
				return fields{
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExtractSpanInfoRequest{
					WorkspaceID: 123,
					TraceID:     "trace",
					SpanIds:     []string{"span"},
					StartTime:   ptr.Of(time.Now().Add(-time.Hour).UnixMilli()),
					EndTime:     ptr.Of(time.Now().UnixMilli()),
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockCfg := confmock.NewMockITraceConfig(ctrl)
				mockCfg.EXPECT().GetTraceDataMaxDurationDay(gomock.Any(), gomock.Any()).Return(int64(30))
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceRead, "123", false).Return(nil)
				mockSvc.EXPECT().ExtractSpanInfo(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					traceCfg: mockCfg,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ExtractSpanInfoRequest{
					WorkspaceID: 123,
					TraceID:     "trace",
					SpanIds:     []string{"span"},
					StartTime:   ptr.Of(time.Now().Add(-time.Hour).UnixMilli()),
					EndTime:     ptr.Of(time.Now().UnixMilli()),
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				traceConfig:  fields.traceCfg,
			}
			got, err := tr.ExtractSpanInfo(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTraceApplication_ListAnnotations(t *testing.T) {
	type fields struct {
		traceSvc service.ITraceService
		auth     rpc.IAuthProvider
		tagSvc   rpc.ITagRPCAdapter
		evalSvc  rpc.IEvaluatorRPCAdapter
		userSvc  rpc.IUserProvider
	}
	type args struct {
		ctx context.Context
		req *trace.ListAnnotationsRequest
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         *trace.ListAnnotationsResponse
		wantErr      bool
	}{
		{
			name: "success case",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockTag := rpcmock.NewMockITagRPCAdapter(ctrl)
				mockEval := rpcmock.NewMockIEvaluatorRPCAdapter(ctrl)
				mockUser := rpcmock.NewMockIUserProvider(ctrl)
				now := time.Now()
				annotations := loop_span.AnnotationList{
					{
						ID:             "ann1",
						SpanID:         "span1",
						TraceID:        "trace1",
						WorkspaceID:    "123",
						StartTime:      now,
						AnnotationType: loop_span.AnnotationTypeManualFeedback,
						Key:            "100",
						Value:          loop_span.NewStringValue("free text"),
						Status:         loop_span.AnnotationStatusNormal,
						Reasoning:      "manual",
						CreatedBy:      "user1",
						UpdatedBy:      "user1",
						CreatedAt:      now,
						UpdatedAt:      now,
					},
					{
						ID:             "ann2",
						SpanID:         "span2",
						TraceID:        "trace1",
						WorkspaceID:    "123",
						StartTime:      now,
						AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
						Value: loop_span.AnnotationValue{
							ValueType:  loop_span.AnnotationValueTypeDouble,
							FloatValue: 0.8,
						},
						Reasoning: "auto",
						Status:    loop_span.AnnotationStatusNormal,
						Metadata: loop_span.AutoEvaluateMetadata{
							TaskID:             1,
							EvaluatorRecordID:  2,
							EvaluatorVersionID: 3,
						},
						CreatedBy: "user2",
						UpdatedBy: "user2",
						CreatedAt: now,
						UpdatedAt: now,
					},
				}
				userMap := map[string]*domaincommon.UserInfo{
					"user1": {UserID: "user1", Name: "User One"},
					"user2": {UserID: "user2", Name: "User Two"},
				}
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceRead, "123", false).Return(nil)
				mockSvc.EXPECT().ListAnnotations(gomock.Any(), gomock.Any()).Return(&service.ListAnnotationsResp{Annotations: annotations}, nil)
				mockUser.EXPECT().GetUserInfo(gomock.Any(), gomock.Any()).Return(nil, userMap, nil)
				mockEval.EXPECT().BatchGetEvaluatorVersions(gomock.Any(), gomock.Any()).Return(nil, map[int64]*rpc.Evaluator{
					3: {EvaluatorName: "eval", EvaluatorVersion: "v1"},
				}, nil)
				mockTag.EXPECT().BatchGetTagInfo(gomock.Any(), int64(123), gomock.Any()).Return(map[int64]*rpc.TagInfo{
					100: {
						TagKeyName:     "key-name",
						TagContentType: rpc.TagContentTypeFreeText,
					},
				}, nil)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
					tagSvc:   mockTag,
					evalSvc:  mockEval,
					userSvc:  mockUser,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationsRequest{
					WorkspaceID:     123,
					SpanID:          "span1",
					TraceID:         "trace1",
					StartTime:       time.Now().Add(-time.Hour).UnixMilli(),
					DescByUpdatedAt: ptr.Of(true),
					PlatformType:    ptr.Of(commondto.PlatformTypeCozeloop),
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "permission error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceRead, "123", false).Return(assert.AnError)
				return fields{auth: mockAuth}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationsRequest{WorkspaceID: 123},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "service error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				mockSvc := svcmock.NewMockITraceService(ctrl)
				mockAuth := rpcmock.NewMockIAuthProvider(ctrl)
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceRead, "123", false).Return(nil)
				mockSvc.EXPECT().ListAnnotations(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
				return fields{
					traceSvc: mockSvc,
					auth:     mockAuth,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &trace.ListAnnotationsRequest{WorkspaceID: 123},
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
			tr := &TraceApplication{
				traceService: fields.traceSvc,
				authSvc:      fields.auth,
				tagSvc:       fields.tagSvc,
				evalSvc:      fields.evalSvc,
				userSvc:      fields.userSvc,
			}
			got, err := tr.ListAnnotations(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				return
			}
			assert.NotNil(t, got)
			assert.Len(t, got.Annotations, 2)
			manual := got.Annotations[0]
			assert.NotNil(t, manual.ManualFeedback)
			assert.Equal(t, int64(100), manual.ManualFeedback.TagKeyID)
			assert.Equal(t, "key-name", manual.ManualFeedback.TagKeyName)
			assert.Equal(t, "free text", manual.ManualFeedback.GetTagValue())
			if manual.BaseInfo != nil && manual.BaseInfo.CreatedBy != nil {
				assert.Equal(t, "User One", manual.BaseInfo.CreatedBy.GetName())
			}
			auto := got.Annotations[1]
			assert.NotNil(t, auto.AutoEvaluate)
			if auto.BaseInfo != nil && auto.BaseInfo.CreatedBy != nil {
				assert.Equal(t, "User Two", auto.BaseInfo.CreatedBy.GetName())
			}
			if auto.AutoEvaluate != nil {
				assert.Equal(t, "eval", auto.AutoEvaluate.EvaluatorName)
				assert.Equal(t, "v1", auto.AutoEvaluate.EvaluatorVersion)
				if auto.AutoEvaluate.EvaluatorResult_ != nil && auto.AutoEvaluate.EvaluatorResult_.Score != nil {
					assert.Equal(t, 0.8, *auto.AutoEvaluate.EvaluatorResult_.Score)
				}
			}
		})
	}
}
