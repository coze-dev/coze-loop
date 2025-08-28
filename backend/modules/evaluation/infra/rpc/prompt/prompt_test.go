// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPromptRPCAdapter_parseRuntimeParam(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{
			name: "normal runtime parameters",
			raw: `{
  "model_config" : {
    "model_id" : "1",
    "temperature": 0.7,
    "max_tokens": 100
  }
}`,
			wantErr: false,
		},
		{
			name:    "empty parameters",
			raw:     "",
			wantErr: false,
		},
		{
			name: "only model_config",
			raw: `{
  "model_config" : {
    "model_id" : "123",
    "model_name": "test_model",
    "temperature": 0.5,
    "top_p": 0.9,
    "max_tokens": 200,
    "json_ext": "{\"key\":\"value\"}"
  }
}`,
			wantErr: false,
		},
		{
			name: "invalid JSON",
			raw: `{
  "model_config" : {
    "model_id" : 
  }
}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := PromptRPCAdapter{}
			result, err := adapter.parseRuntimeParam(ctx, tt.raw)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)

				// Validate parsing results for non-empty parameters
				if tt.raw != "" && tt.name != "invalid JSON" {
					assert.NotNil(t, result.ModelConfig)
				}
			}
		})
	}
}

func TestPromptRPCAdapter_ExecutePrompt_RuntimeParam(t *testing.T) {
	// 由于缺少mock生成，我们只测试parseRuntimeParam方法的逻辑
	adapter := &PromptRPCAdapter{}
	ctx := context.Background()

	tests := []struct {
		name         string
		runtimeParam string
		wantErr      bool
		wantNil      bool
		wantModelID  int64
	}{
		{
			name:         "正常运行时参数解析",
			runtimeParam: `{"model_config":{"model_id":"123","model_name":"test_model","max_tokens":100,"temperature":0.7,"top_p":0.9}}`,
			wantErr:      false,
			wantNil:      false,
			wantModelID:  123,
		},
		{
			name:         "运行时参数解析失败",
			runtimeParam: `{"model_config":invalid_json}`,
			wantErr:      true,
			wantNil:      false,
		},
		{
			name:         "ModelConfig为nil的情况",
			runtimeParam: `{"model_config":null}`,
			wantErr:      false,
			wantNil:      false,
		},
		{
			name:         "空运行时参数",
			runtimeParam: "",
			wantErr:      false,
			wantNil:      false,
		},
		{
			name:         "部分ModelConfig字段",
			runtimeParam: `{"model_config":{"model_id":"456","temperature":0.5}}`,
			wantErr:      false,
			wantNil:      false,
			wantModelID:  456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.parseRuntimeParam(ctx, tt.runtimeParam)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantNil {
					assert.Nil(t, result)
				} else {
					assert.NotNil(t, result)
					if tt.wantModelID != 0 && result.ModelConfig != nil {
						assert.Equal(t, tt.wantModelID, result.ModelConfig.ModelID)
					}
				}
			}
		})
	}
}
