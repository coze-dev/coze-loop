// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
)

func TestPromptRuntimeParam_GetJSONDemo(t *testing.T) {
	param := &PromptRuntimeParam{}
	demo := param.GetJSONDemo()

	assert.NotEmpty(t, demo)
	assert.Contains(t, demo, "model_config")
	assert.Contains(t, demo, "max_tokens")
	assert.Contains(t, demo, "temperature")
	assert.Contains(t, demo, "top_p")
	assert.Contains(t, demo, "json_ext")
}

func TestPromptRuntimeParam_GetJSONValue(t *testing.T) {
	param := &PromptRuntimeParam{
		ModelConfig: &ModelConfig{
			ModelID:     123,
			ModelName:   "test_model",
			MaxTokens:   gptr.Of(int32(100)),
			Temperature: gptr.Of(0.7),
			TopP:        gptr.Of(0.9),
			JSONExt:     gptr.Of(`{"key":"value"}`),
		},
	}

	jsonValue := param.GetJSONValue()
	assert.NotEmpty(t, jsonValue)
	assert.Contains(t, jsonValue, "model_config")
	assert.Contains(t, jsonValue, "123")
	assert.Contains(t, jsonValue, "test_model")
}

func TestPromptRuntimeParam_ParseFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		wantErr bool
	}{
		{
			name: "normal parsing",
			jsonStr: `{
				"model_config": {
					"model_id": "123",
					"model_name": "test_model",
					"max_tokens": 100,
					"temperature": 0.7,
					"top_p": 0.9,
					"json_ext": "{\"key\":\"value\"}"
				}
			}`,
			wantErr: false,
		},
		{
			name:    "empty JSON",
			jsonStr: "{}",
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			jsonStr: "invalid json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			param := &PromptRuntimeParam{}
			result, err := param.ParseFromJSON(tt.jsonStr)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.IsType(t, &PromptRuntimeParam{}, result)
			}
		})
	}
}

func TestNewPromptRuntimeParam(t *testing.T) {
	modelConfig := &ModelConfig{
		ModelID:   123,
		ModelName: "test_model",
	}

	param := NewPromptRuntimeParam(modelConfig)
	assert.NotNil(t, param)

	promptParam, ok := param.(*PromptRuntimeParam)
	assert.True(t, ok)
	assert.Equal(t, modelConfig, promptParam.ModelConfig)
}

func TestDummyRuntimeParam_GetJSONDemo(t *testing.T) {
	param := &DummyRuntimeParam{}
	demo := param.GetJSONDemo()

	assert.Equal(t, "{}", demo)
}

func TestDummyRuntimeParam_GetJSONValue(t *testing.T) {
	param := &DummyRuntimeParam{}
	jsonValue := param.GetJSONValue()

	assert.Equal(t, "{}", jsonValue)
}

func TestDummyRuntimeParam_ParseFromJSON(t *testing.T) {
	param := &DummyRuntimeParam{}

	tests := []struct {
		name    string
		jsonStr string
	}{
		{
			name:    "arbitrary JSON",
			jsonStr: `{"key": "value"}`,
		},
		{
			name:    "empty JSON",
			jsonStr: "{}",
		},
		{
			name:    "invalid JSON",
			jsonStr: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := param.ParseFromJSON(tt.jsonStr)

			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.IsType(t, &DummyRuntimeParam{}, result)
		})
	}
}

func TestNewDummyRuntimeParam(t *testing.T) {
	param := NewDummyRuntimeParam()
	assert.NotNil(t, param)
	assert.IsType(t, &DummyRuntimeParam{}, param)
}
