// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestAgentAdapter_CallTraceAgent(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(ctx context.Context) (*AgentAdapter, context.Context)
		wantErr bool
		errCheck func(t *testing.T, err error)
	}{
		{
			name: "success case",
			setup: func(ctx context.Context) (*AgentAdapter, context.Context) {
				adapter := &AgentAdapter{}
				return adapter, ctx
			},
			wantErr: false,
			errCheck: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			adapter, ctx := tt.setup(ctx)

			result, err := adapter.CallTraceAgent(ctx, 123, "http://example.com")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, int64(0), result)
			}

			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
		})
	}
}

func TestAgentAdapter_GetReport(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(ctx context.Context) (*AgentAdapter, context.Context)
		wantErr bool
		errCheck func(t *testing.T, err error)
	}{
		{
			name: "success case",
			setup: func(ctx context.Context) (*AgentAdapter, context.Context) {
				adapter := &AgentAdapter{}
				return adapter, ctx
			},
			wantErr: false,
			errCheck: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			adapter, ctx := tt.setup(ctx)

			report, status, err := adapter.GetReport(ctx, 123, 456)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "", report)
				assert.Equal(t, entity.ReportStatus_Unknown, status)
			}

			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
		})
	}
}