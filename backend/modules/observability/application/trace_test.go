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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/view"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad"))
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("bad"))
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockSvc.EXPECT().IngestTraces(gomock.Any(), gomock.Any()).Return(nil)
				return fields{
					traceSvc: mockSvc,
					benefit:  mockBenefit,
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
				mockAuth.EXPECT().CheckWorkspacePermission(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
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
