// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package eino

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
)

func TestMiniMaxIntegration(t *testing.T) {
	apiKey := os.Getenv("MINIMAX_API_KEY")
	if apiKey == "" {
		t.Skip("MINIMAX_API_KEY not set, skipping integration tests")
	}

	t.Run("minimax_m27_generate", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: apiKey,
				Model:  "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model,
			entity.WithTemperature(0.7),
			entity.WithMaxTokens(100),
		)
		require.NoError(t, err)
		require.NotNil(t, llm)

		resp, err := llm.Generate(context.Background(), []*entity.Message{
			{Role: entity.RoleUser, Content: "Say hello in one word."},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotEmpty(t, resp.Content)
		require.Equal(t, entity.RoleAssistant, resp.Role)
	})

	t.Run("minimax_m25_highspeed_generate", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: apiKey,
				Model:  "MiniMax-M2.5-highspeed",
			},
		}
		llm, err := NewLLM(context.Background(), model,
			entity.WithTemperature(0.7),
			entity.WithMaxTokens(100),
		)
		require.NoError(t, err)
		require.NotNil(t, llm)

		resp, err := llm.Generate(context.Background(), []*entity.Message{
			{Role: entity.RoleUser, Content: "What is 2+2? Answer with just the number."},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotEmpty(t, resp.Content)
	})

	t.Run("minimax_stream", func(t *testing.T) {
		model := &entity.Model{
			Protocol: entity.ProtocolMiniMax,
			ProtocolConfig: &entity.ProtocolConfig{
				APIKey: apiKey,
				Model:  "MiniMax-M2.7",
			},
		}
		llm, err := NewLLM(context.Background(), model,
			entity.WithTemperature(0.7),
			entity.WithMaxTokens(100),
		)
		require.NoError(t, err)
		require.NotNil(t, llm)

		stream, err := llm.Stream(context.Background(), []*entity.Message{
			{Role: entity.RoleUser, Content: "Count from 1 to 5."},
		})
		require.NoError(t, err)
		require.NotNil(t, stream)
	})
}
