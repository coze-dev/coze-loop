// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestCheckProcessor_Transform(t *testing.T) {
	type fields struct {
		workspaceId int64
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
			name: "transform successfully when workspace id matches",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					workspaceId: 1,
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{
					{
						WorkspaceID: "1",
					},
					{
						WorkspaceID: "2",
					},
				},
			},
			want: loop_span.SpanList{
				{
					WorkspaceID: "1",
				},
				{
					WorkspaceID: "2",
				},
			},
			wantErr: false,
		},
		{
			name: "transform failed when workspace id not matches",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					workspaceId: 1,
				}
			},
			args: args{
				ctx: context.Background(),
				spans: loop_span.SpanList{
					{
						WorkspaceID: "2",
					},
					{
						WorkspaceID: "2",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "transform when no spans",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					workspaceId: 1,
				}
			},
			args: args{
				ctx:   context.Background(),
				spans: loop_span.SpanList{},
			},
			want:    loop_span.SpanList{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fields := tt.fieldsGetter(ctrl)
			c := &CheckProcessor{
				workspaceId: fields.workspaceId,
			}
			got, err := c.Transform(tt.args.ctx, tt.args.spans)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestCheckProcessorFactory_CreateProcessor(t *testing.T) {
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
				set: Settings{
					WorkspaceId: 1,
				},
			},
			want: &CheckProcessor{
				workspaceId: 1,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CheckProcessorFactory{}
			got, err := c.CreateProcessor(tt.args.ctx, tt.args.set)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, got, tt.want)
		})
	}
}

func TestCheckProcessor_GetName(t *testing.T) {
	assert.Equal(t, "CheckProcessor", (&CheckProcessor{}).GetName())
}
