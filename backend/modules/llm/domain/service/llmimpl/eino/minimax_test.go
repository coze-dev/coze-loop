// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package eino

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestMiniMaxBuilder(t *testing.T) {
	t.Run("basic_minimax_builder", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: "test-api-key",
				Model:  "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
		assert.Equal(t, entity.ProtocolMiniMax, llm.protocol)
	})

	t.Run("minimax_with_custom_base_url", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				BaseURL: "https://custom.minimax.io/v1",
				APIKey:  "test-api-key",
				Model:   "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_with_timeout", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey:    "test-api-key",
				Model:     "MiniMax-M2.5",
				TimeoutMs: ptr.Of(int64(30000)),
			},
		}
		llm, err := NewLLM(context.Background(), model)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_with_response_format", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: "test-api-key",
				Model:  "MiniMax-M2.7",
				ProtocolConfigMiniMax: &entity.ProtocolConfigMiniMax{
					ResponseFormatType: "json_object",
				},
			},
		}
		llm, err := NewLLM(context.Background(), model)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_with_runtime_options", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: "test-api-key",
				Model:  "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model,
			entity.WithTemperature(0.7),
			entity.WithTopP(0.9),
			entity.WithMaxTokens(1024),
			entity.WithStop([]string{"stop1"}),
		)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_with_runtime_response_format", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: "test-api-key",
				Model:  "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model,
			entity.WithResponseFormat(&entity.ResponseFormat{Type: "json_object"}),
		)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_nil_protocol_config", func(t *testing.T) {
		model := &entity.Model{
			Protocol:       entity.ProtocolMiniMax,
			ProtocolConfig: nil,
		}
		_, err := NewLLM(context.Background(), model)
		assert.Error(t, err)
	})

	t.Run("minimax_m25_highspeed", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: "test-api-key",
				Model:  "MiniMax-M2.5-highspeed",
			},
		}
		llm, err := NewLLM(context.Background(), model)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_with_penalty_params", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: "test-api-key",
				Model:  "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model,
			entity.WithFrequencyPenalty(0.5),
			entity.WithPresencePenalty(0.3),
		)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})

	t.Run("minimax_default_base_url", func(t *testing.T) {
		// When BaseURL is empty, miniMaxBuilder should use the default MiniMax API URL
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				BaseURL: "",
				APIKey:  "test-api-key",
				Model:   "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model)
		assert.NoError(t, err)
		assert.NotNil(t, llm)
	})
}
