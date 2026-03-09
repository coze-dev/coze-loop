// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package workflow

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
)

func TestNewWorkflowProvider(t *testing.T) {
	provider := NewWorkflowProvider()
	assert.NotNil(t, provider)
	assert.Implements(t, (*interface{ BatchGetWorkflows(context.Context, loop_span.SpanList) (map[string]string, error) })(nil), provider)
}

func TestWorkflowProvider_BatchGetWorkflows(t *testing.T) {
	provider := NewWorkflowProvider()
	ctx := context.Background()

	tests := []struct {
		name     string
		spans    loop_span.SpanList
		wantErr  bool
		wantSize int
	}{
		{
			name:     "empty spans list",
			spans:    loop_span.SpanList{},
			wantErr:  false,
			wantSize: 0,
		},
		{
			name: "nil spans list",
			spans:    nil,
			wantErr:  false,
			wantSize: 0,
		},
		{
			name: "spans with need workflow",
			spans: loop_span.SpanList{
				{
					TraceID:     "trace-1",
					SpanID:      "span-1",
					WorkspaceID: "ws-1",
					Encryption: loop_span.EncryptionInfo{
						NeedWorkflow: true,
					},
				},
				{
					TraceID:     "trace-2",
					SpanID:      "span-2",
					WorkspaceID: "ws-2",
					Encryption: loop_span.EncryptionInfo{
						NeedWorkflow: true,
					},
				},
			},
			wantErr:  false,
			wantSize: 0, // 当前实现返回空 map
		},
		{
			name: "spans without need workflow",
			spans: loop_span.SpanList{
				{
					TraceID:     "trace-1",
					SpanID:      "span-1",
					WorkspaceID: "ws-1",
					Encryption: loop_span.EncryptionInfo{
						NeedWorkflow: false,
					},
				},
			},
			wantErr:  false,
			wantSize: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.BatchGetWorkflows(ctx, tt.spans)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantSize, len(got))
			}
		})
	}
}
