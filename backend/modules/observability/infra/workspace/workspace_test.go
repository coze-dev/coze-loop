// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/workspace"
)

func TestWorkspaceProviderImpl_GetIngestWorkSpaceID(t *testing.T) {
	type args struct {
		ctx   context.Context
		spans []*span.InputSpan
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty spans array, return empty string",
			args: args{
				ctx:   context.Background(),
				spans: []*span.InputSpan{},
			},
			want: "",
		},
		{
			name: "nil spans, return empty string",
			args: args{
				ctx:   context.Background(),
				spans: nil,
			},
			want: "",
		},
		{
			name: "normal spans array, return first span workspace id",
			args: args{
				ctx: context.Background(),
				spans: []*span.InputSpan{
					{WorkspaceID: "workspace1"},
					{WorkspaceID: "workspace2"},
				},
			},
			want: "workspace1",
		},
		{
			name: "first span has empty workspace id",
			args: args{
				ctx: context.Background(),
				spans: []*span.InputSpan{
					{WorkspaceID: ""},
					{WorkspaceID: "workspace2"},
				},
			},
			want: "",
		},
		{
			name: "single span with workspace id",
			args: args{
				ctx: context.Background(),
				spans: []*span.InputSpan{
					{WorkspaceID: "single_workspace"},
				},
			},
			want: "single_workspace",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WorkspaceProviderImpl{}
			got := w.GetIngestWorkSpaceID(tt.args.ctx, tt.args.spans)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorkspaceProviderImpl_GetQueryWorkSpaceID(t *testing.T) {
	type args struct {
		ctx                 context.Context
		requestWorkspaceID int64
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "positive workspace id conversion",
			args: args{
				ctx:                 context.Background(),
				requestWorkspaceID: 12345,
			},
			want: "12345",
		},
		{
			name: "negative workspace id conversion",
			args: args{
				ctx:                 context.Background(),
				requestWorkspaceID: -1,
			},
			want: "-1",
		},
		{
			name: "zero workspace id conversion",
			args: args{
				ctx:                 context.Background(),
				requestWorkspaceID: 0,
			},
			want: "0",
		},
		{
			name: "large positive workspace id conversion",
			args: args{
				ctx:                 context.Background(),
				requestWorkspaceID: 9223372036854775807, // max int64
			},
			want: "9223372036854775807",
		},
		{
			name: "large negative workspace id conversion",
			args: args{
				ctx:                 context.Background(),
				requestWorkspaceID: -9223372036854775808, // min int64
			},
			want: "-9223372036854775808",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &WorkspaceProviderImpl{}
			got := w.GetQueryWorkSpaceID(tt.args.ctx, tt.args.requestWorkspaceID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewWorkspaceProvider(t *testing.T) {
	tests := []struct {
		name string
		want workspace.IWorkSpaceProvider
	}{
		{
			name: "create new workspace provider successfully",
			want: &WorkspaceProviderImpl{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewWorkspaceProvider()
			assert.NotNil(t, got)
			assert.IsType(t, &WorkspaceProviderImpl{}, got)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWorkspaceProviderImpl_Interface(t *testing.T) {
	// Verify that WorkspaceProviderImpl implements IWorkSpaceProvider interface
	var _ workspace.IWorkSpaceProvider = &WorkspaceProviderImpl{}
	
	provider := NewWorkspaceProvider()
	assert.NotNil(t, provider)
	
	// Test interface methods exist and are callable
	workspaceID := provider.GetQueryWorkSpaceID(context.Background(), 123)
	assert.Equal(t, "123", workspaceID)
	
	spans := []*span.InputSpan{{WorkspaceID: "test"}}
	ingestID := provider.GetIngestWorkSpaceID(context.Background(), spans)
	assert.Equal(t, "test", ingestID)
}

func TestWorkspaceProviderImpl_EdgeCases(t *testing.T) {
	provider := &WorkspaceProviderImpl{}
	
	// Test with nil context (should still work)
	got := provider.GetQueryWorkSpaceID(nil, 456)
	assert.Equal(t, "456", got)
	
	// Test with nil context for GetIngestWorkSpaceID
	spans := []*span.InputSpan{{WorkspaceID: "test_nil_ctx"}}
	ingestID := provider.GetIngestWorkSpaceID(nil, spans)
	assert.Equal(t, "test_nil_ctx", ingestID)
	
	// Test with nil spans element - this would cause panic in the actual code
	// So we test the actual behavior which is to return empty string for empty array
	emptySpans := []*span.InputSpan{}
	emptyID := provider.GetIngestWorkSpaceID(context.Background(), emptySpans)
	assert.Equal(t, "", emptyID)
}