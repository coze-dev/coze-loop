// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"testing"

	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	workspacemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/lib/otel"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestOpenAPIApplication_unpackOtelSpace(t *testing.T) {
	type fields struct {
		auth      func(ctrl *gomock.Controller) *rpcmocks.MockIAuthProvider
		workspace func(ctrl *gomock.Controller) *workspacemocks.MockIWorkSpaceProvider
	}
	type args struct {
		ctx          context.Context
		outerSpaceID string
		reqSpanProto *otel.ExportTraceServiceRequest
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   map[string]int // spaceID -> number of spans
	}{
		{
			name:   "reqSpanProto is nil",
			fields: fields{},
			args: args{
				reqSpanProto: nil,
			},
			want: nil,
		},
		{
			name: "spaceID in span attributes",
			fields: fields{
				auth: func(ctrl *gomock.Controller) *rpcmocks.MockIAuthProvider {
					return rpcmocks.NewMockIAuthProvider(ctrl)
				},
				workspace: func(ctrl *gomock.Controller) *workspacemocks.MockIWorkSpaceProvider {
					return workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				},
			},
			args: args{
				ctx:          context.Background(),
				outerSpaceID: "outer",
				reqSpanProto: &otel.ExportTraceServiceRequest{
					ResourceSpans: []*otel.ResourceSpans{
						{
							ScopeSpans: []*otel.ScopeSpans{
								{
									Spans: []*otel.Span{
										{
											Attributes: []*otel.KeyValue{
												{
													Key: otel.OtelAttributeWorkSpaceID,
													Value: &otel.AnyValue{
														Value: &otel.AnyValue_StringValue{
															StringValue: "inner",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]int{"inner": 1},
		},
		{
			name: "spaceID not in attributes, use outerSpaceID",
			fields: fields{
				auth: func(ctrl *gomock.Controller) *rpcmocks.MockIAuthProvider {
					return rpcmocks.NewMockIAuthProvider(ctrl)
				},
				workspace: func(ctrl *gomock.Controller) *workspacemocks.MockIWorkSpaceProvider {
					return workspacemocks.NewMockIWorkSpaceProvider(ctrl)
				},
			},
			args: args{
				ctx:          context.Background(),
				outerSpaceID: "outer",
				reqSpanProto: &otel.ExportTraceServiceRequest{
					ResourceSpans: []*otel.ResourceSpans{
						{
							ScopeSpans: []*otel.ScopeSpans{
								{
									Spans: []*otel.Span{
										{
											Attributes: []*otel.KeyValue{},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]int{"outer": 1},
		},
		{
			name: "both empty, call GetIngestWorkSpaceID",
			fields: fields{
				auth: func(ctrl *gomock.Controller) *rpcmocks.MockIAuthProvider {
					m := rpcmocks.NewMockIAuthProvider(ctrl)
					m.EXPECT().GetClaim(gomock.Any()).Return(nil)
					return m
				},
				workspace: func(ctrl *gomock.Controller) *workspacemocks.MockIWorkSpaceProvider {
					m := workspacemocks.NewMockIWorkSpaceProvider(ctrl)
					m.EXPECT().GetIngestWorkSpaceID(gomock.Any(), gomock.Any(), gomock.Any()).Return("mocked")
					return m
				},
			},
			args: args{
				ctx:          context.Background(),
				outerSpaceID: "",
				reqSpanProto: &otel.ExportTraceServiceRequest{
					ResourceSpans: []*otel.ResourceSpans{
						{
							ScopeSpans: []*otel.ScopeSpans{
								{
									Spans: []*otel.Span{
										{
											Attributes: []*otel.KeyValue{},
										},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]int{"mocked": 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var authProvider *rpcmocks.MockIAuthProvider
			if tt.fields.auth != nil {
				authProvider = tt.fields.auth(ctrl)
			}
			var workspaceProvider *workspacemocks.MockIWorkSpaceProvider
			if tt.fields.workspace != nil {
				workspaceProvider = tt.fields.workspace(ctrl)
			}

			o := &OpenAPIApplication{
				auth:      authProvider,
				workspace: workspaceProvider,
			}
			got := o.unpackOtelSpace(tt.args.ctx, tt.args.outerSpaceID, tt.args.reqSpanProto)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, len(tt.want), len(got))
				for k, v := range tt.want {
					assert.Equal(t, v, len(got[k]))
				}
			}
		})
	}
}
