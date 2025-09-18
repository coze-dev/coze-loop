// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNotifyRPCAdapter_SendLarkMessageCard(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(ctx context.Context) (*NotifyRPCAdapter, context.Context)
		wantErr  bool
		errCheck func(t *testing.T, err error)
	}{
		{
			name: "success case",
			setup: func(ctx context.Context) (*NotifyRPCAdapter, context.Context) {
				adapter := &NotifyRPCAdapter{}
				return adapter, ctx
			},
			wantErr: false,
			errCheck: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "with empty userID",
			setup: func(ctx context.Context) (*NotifyRPCAdapter, context.Context) {
				adapter := &NotifyRPCAdapter{}
				return adapter, ctx
			},
			wantErr: false,
			errCheck: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "with empty cardID",
			setup: func(ctx context.Context) (*NotifyRPCAdapter, context.Context) {
				adapter := &NotifyRPCAdapter{}
				return adapter, ctx
			},
			wantErr: false,
			errCheck: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "with nil param",
			setup: func(ctx context.Context) (*NotifyRPCAdapter, context.Context) {
				adapter := &NotifyRPCAdapter{}
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

			var userID, cardID string
			var param map[string]string

			switch tt.name {
			case "success case":
				userID = "user123"
				cardID = "card456"
				param = map[string]string{"key": "value"}
			case "with empty userID":
				userID = ""
				cardID = "card456"
				param = map[string]string{"key": "value"}
			case "with empty cardID":
				userID = "user123"
				cardID = ""
				param = map[string]string{"key": "value"}
			case "with nil param":
				userID = "user123"
				cardID = "card456"
				param = nil
			}

			err := adapter.SendLarkMessageCard(ctx, userID, cardID, param)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.errCheck != nil {
				tt.errCheck(t, err)
			}
		})
	}
}
