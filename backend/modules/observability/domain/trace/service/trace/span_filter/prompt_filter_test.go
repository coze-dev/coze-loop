// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestPromptFilter_BuildBasicSpanFilter(t *testing.T) {
	tests := []struct {
		name string
		env  *SpanEnv
		want []*loop_span.FilterField
	}{
		{
			name: "success",
			env:  &SpanEnv{WorkspaceId: 123},
			want: []*loop_span.FilterField{
				{
					FieldName: loop_span.SpanFieldSpaceId,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{"123"},
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
				{
					FieldName: loop_span.SpanFieldCallType,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{"PromptPlayground", "PromptDebug"},
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &PromptFilter{}
			got, _, _ := f.BuildBasicSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptFilter_BuildRootSpanFilter(t *testing.T) {
	tests := []struct {
		name string
		env  *SpanEnv
		want []*loop_span.FilterField
	}{
		{
			name: "success",
			env:  &SpanEnv{},
			want: []*loop_span.FilterField{
				{
					FieldName: loop_span.SpanFieldParentID,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{"0", ""},
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &PromptFilter{}
			got, _ := f.BuildRootSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptFilter_BuildLLMSpanFilter(t *testing.T) {
	tests := []struct {
		name string
		env  *SpanEnv
		want []*loop_span.FilterField
	}{
		{
			name: "success",
			env:  &SpanEnv{},
			want: []*loop_span.FilterField{
				{
					FieldName: loop_span.SpanFieldSpanType,
					FieldType: loop_span.FieldTypeString,
					Values:    []string{"model"},
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &PromptFilter{}
			got, _ := f.BuildLLMSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptFilter_BuildALLSpanFilter(t *testing.T) {
	tests := []struct {
		name     string
		env      *SpanEnv
		transCfg loop_span.SpanTransCfgList
		want     []*loop_span.FilterField
	}{
		{
			name:     "empty config",
			env:      &SpanEnv{},
			transCfg: nil,
			want:     nil,
		},
		{
			name: "with config",
			env:  &SpanEnv{},
			transCfg: loop_span.SpanTransCfgList{
				{
					SpanFilter: &loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{
								FieldName: "test_field",
								FieldType: loop_span.FieldTypeString,
								Values:    []string{"test_value"},
								QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
							},
						},
					},
				},
			},
			want: []*loop_span.FilterField{
				{
					SubFilter: &loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
						FilterFields: []*loop_span.FilterField{
							{
								SubFilter: &loop_span.FilterFields{
									QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
									FilterFields: []*loop_span.FilterField{
										{
											FieldName: "test_field",
											FieldType: loop_span.FieldTypeString,
											Values:    []string{"test_value"},
											QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &PromptFilter{transCfg: tt.transCfg}
			got, _ := f.BuildALLSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPromptFilterFactory_CreateFilter(t *testing.T) {
	type fields struct {
		c config.ITraceConfig
	}
	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		want         Filter
		wantErr      bool
	}{
		{
			name: "success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confmock := confmocks.NewMockITraceConfig(ctrl)
				confmock.EXPECT().GetPlatformSpansTrans(gomock.Any()).Return(&config.SpanTransHandlerConfig{
					PlatformCfg: map[string]loop_span.SpanTransCfgList{
						string(loop_span.PlatformPrompt): {
							{
								SpanFilter: &loop_span.FilterFields{
									QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
									FilterFields: []*loop_span.FilterField{
										{
											FieldName: "test_field",
											FieldType: loop_span.FieldTypeString,
											Values:    []string{"test_value"},
											QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
										},
									},
								},
							},
						},
					},
				}, nil)
				return fields{confmock}
			},
			want:    &PromptFilter{},
			wantErr: false,
		},
		{
			name: "error getting config",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confmock := confmocks.NewMockITraceConfig(ctrl)
				confmock.EXPECT().GetPlatformSpansTrans(gomock.Any()).Return(nil, assert.AnError)
				return fields{confmock}
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "config not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				confmock := confmocks.NewMockITraceConfig(ctrl)
				confmock.EXPECT().GetPlatformSpansTrans(gomock.Any()).Return(&config.SpanTransHandlerConfig{
					PlatformCfg: map[string]loop_span.SpanTransCfgList{},
				}, nil)
				return fields{confmock}
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
			f := &PromptFilterFactory{traceConfig: fields.c}
			got, err := f.CreateFilter(context.Background())
			assert.Equal(t, tt.wantErr, err != nil)
			if !tt.wantErr {
				assert.IsType(t, tt.want, got)
			}
		})
	}
}
