// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestAttrTosProcessor_Transform(t *testing.T) {
	type fields struct {
		storage rpc.IFileProvider
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
			name: "transform successfully with object storage",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := mocks.NewMockIFileProvider(ctrl)
				objectStorage.EXPECT().GetDownloadUrls(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).Return(map[string]string{
					"input":      "a",
					"output":     "a",
					"attachment": "a",
				}, nil)
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{{
					WorkspaceID:   "1",
					ObjectStorage: `{"input_tos_key":"input","output_tos_key":"output","Attachments":[{"tos_key":"attachment"}]}`,
				}},
			},
			want: loop_span.SpanList{{
				WorkspaceID:   "1",
				ObjectStorage: `{"input_tos_key":"input","output_tos_key":"output","Attachments":[{"tos_key":"attachment"}]}`,
				AttrTos: &loop_span.AttrTos{
					InputDataURL:   "a",
					OutputDataURL:  "a",
					MultimodalData: map[string]string{"attachment": "a"},
				},
			}},
			wantErr: false,
		},
		{
			name: "transform with config error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				objectStorage := mocks.NewMockIFileProvider(ctrl)
				objectStorage.EXPECT().GetDownloadUrls(gomock.Any(), gomock.Any(), gomock.Any()).MaxTimes(3).Return(nil, fmt.Errorf("error"))
				return fields{
					storage: objectStorage,
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{{
					WorkspaceID:   "1",
					ObjectStorage: `{"input_tos_key":"input","output_tos_key":"output","Attachments":[{"tos_key":"attachment"}]}`,
				}},
			},
			want: loop_span.SpanList{{
				WorkspaceID:   "1",
				ObjectStorage: `{"input_tos_key":"input","output_tos_key":"output","Attachments":[{"tos_key":"attachment"}]}`,
				AttrTos: &loop_span.AttrTos{
					InputDataURL:   "",
					OutputDataURL:  "",
					MultimodalData: map[string]string{},
				},
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			p := &AttrTosProcessor{
				fileProvider: fields.storage,
			}
			got, err := p.Transform(tt.args.ctx, tt.args.spans)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestAttrTosProcessorFactory_CreateProcessor(t *testing.T) {
	type fields struct{}
	type args struct {
		ctx context.Context
		set Settings
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Processor
		wantErr bool
	}{
		{
			name:   "create processor successfully",
			fields: fields{},
			args: args{
				ctx: context.Background(),
				set: Settings{},
			},
			want:    &AttrTosProcessor{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &AttrTosProcessorFactory{}
			got, err := c.CreateProcessor(tt.args.ctx, tt.args.set)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestAttrTosProcessor_GetName(t *testing.T) {
	assert.Equal(t, "AttrTosProcessor", (&AttrTosProcessor{}).GetName())
}
