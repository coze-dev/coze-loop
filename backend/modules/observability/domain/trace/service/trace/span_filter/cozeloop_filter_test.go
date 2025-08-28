// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestCozeLoopFilter_BuildBasicSpanFilter(t *testing.T) {
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
					Values:    []string{"Custom"},
					QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CozeLoopFilter{}
			got, _, _ := f.BuildBasicSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCozeLoopFilter_BuildAllSpanFilter(t *testing.T) {
	tests := []struct {
		name string
		env  *SpanEnv
		want []*loop_span.FilterField
	}{
		{
			name: "success",
			env:  &SpanEnv{WorkspaceId: 123},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CozeLoopFilter{}
			got, _ := f.BuildALLSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCozeLoopFilter_BuildRootSpanFilter(t *testing.T) {
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
			f := &CozeLoopFilter{}
			got, _ := f.BuildRootSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCozeLoopFilter_BuildLlmSpanFilter(t *testing.T) {
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
			f := &CozeLoopFilter{}
			got, _ := f.BuildLLMSpanFilter(context.Background(), tt.env)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCozeLoopFilterFactory_CreateFilter(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		want    Filter
		wantErr bool
	}{
		{
			name:    "success",
			ctx:     context.Background(),
			want:    &CozeLoopFilter{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &CozeLoopFilterFactory{}
			got, err := f.CreateFilter(tt.ctx)
			assert.Equal(t, err != nil, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}
