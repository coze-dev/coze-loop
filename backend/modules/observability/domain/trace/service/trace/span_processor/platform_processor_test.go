// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestPlatformProcessor_Transform(t *testing.T) {
	type fields struct {
		cfg loop_span.SpanTransCfgList
	}
	type args struct {
		ctx   context.Context
		spans loop_span.SpanList
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         loop_span.SpanList
		wantErr      bool
	}{
		{
			name: "transform successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					cfg: loop_span.SpanTransCfgList{},
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{{
					TraceID: "123",
					SpanID:  "456",
				}},
			},
			want: loop_span.SpanList{{
				TraceID: "123",
				SpanID:  "456",
			}},
			wantErr: false,
		},
		{
			name: "transform successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					cfg: loop_span.SpanTransCfgList{
						{
							SpanFilter: &loop_span.FilterFields{
								QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
								FilterFields: []*loop_span.FilterField{
									{
										FieldName: loop_span.SpanFieldTraceId,
										FieldType: loop_span.FieldTypeString,
										QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
										Values:    []string{"1234"},
									},
								},
							},
						},
					},
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{
					{
						TraceID: "123",
						SpanID:  "456",
					},
					{
						TraceID: "1234",
						SpanID:  "4567",
					},
				},
			},
			want: loop_span.SpanList{{
				TraceID: "1234",
				SpanID:  "4567",
			}},
			wantErr: false,
		},
		{
			name: "transform successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					cfg: loop_span.SpanTransCfgList{
						{
							SpanFilter: &loop_span.FilterFields{
								QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
								FilterFields: []*loop_span.FilterField{
									{
										FieldName: loop_span.SpanFieldTraceId,
										FieldType: loop_span.FieldTypeString,
										QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
										Values:    []string{"1234"},
									},
								},
							},
							TagFilter: &loop_span.TagFilter{
								KeyBlackList: []string{"custom1"},
							},
							InputFilter: &loop_span.InputFilter{
								KeyWhiteList: []string{"input1"},
							},
							OutputFilter: &loop_span.OutputFilter{
								KeyWhiteList: []string{"output1"},
							},
						},
					},
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{
					{
						TraceID: "123",
						SpanID:  "456",
					},
					{
						TraceID: "1234",
						SpanID:  "4567",
						TagsString: map[string]string{
							"custom1": "1",
							"custom2": "2",
						},
						Input:  `{"input1": 1, "input2": 2}`,
						Output: `{"output1": 1, "output2": 2}`,
					},
				},
			},
			want: loop_span.SpanList{{
				TraceID: "1234",
				SpanID:  "4567",
				TagsString: map[string]string{
					"custom2": "2",
				},
				Input:  `{"input1":1}`,
				Output: `{"output1":1}`,
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			p := &PlatformProcessor{
				cfg: fields.cfg,
			}
			got, err := p.Transform(tt.args.ctx, tt.args.spans)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestPlatformProcessorFactory_CreateProcessor(t *testing.T) {
	type fields struct {
		traceConfig config.ITraceConfig
	}
	type args struct {
		ctx context.Context
		set Settings
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		want         Processor
		wantErr      bool
	}{
		{
			name: "create processor successfully",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetPlatformSpansTrans(gomock.Any(), gomock.Any()).Return(loop_span.SpanTransCfgList{
					{},
				}, nil)
				return fields{
					traceConfig: confMock,
				}
			},
			args: args{
				ctx: context.Background(),
				set: Settings{
					PlatformType: "cozeloop",
				},
			},
			want: &PlatformProcessor{
				cfg: loop_span.SpanTransCfgList{
					{},
				},
			},
			wantErr: false,
		},
		{
			name: "create processor failed when config returns error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confMock := confmocks.NewMockITraceConfig(ctrl)
				confMock.EXPECT().GetPlatformSpansTrans(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("config error"))
				return fields{
					traceConfig: confMock,
				}
			},
			args: args{
				ctx: context.Background(),
				set: Settings{
					PlatformType: "coze_loop",
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
			p := &PlatformProcessorFactory{
				traceConfig: fields.traceConfig,
			}
			got, err := p.CreateProcessor(tt.args.ctx, tt.args.set)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}
