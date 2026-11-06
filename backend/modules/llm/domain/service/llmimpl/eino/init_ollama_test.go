// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package eino

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
)

func TestOllamaChatModelConfig_MaxTokens(t *testing.T) {
	model := &entity.Model{ProtocolConfig: &entity.ProtocolConfig{BaseURL: "http://localhost:11434", Model: "llama3"}}

	t.Run("max tokens is passed as num_predict", func(t *testing.T) {
		cfg := ollamaChatModelConfig(model, entity.WithMaxTokens(256))
		assert.Equal(t, 256, cfg.Options.NumPredict)
	})

	t.Run("num_predict stays unset without max tokens", func(t *testing.T) {
		cfg := ollamaChatModelConfig(model)
		assert.Equal(t, 0, cfg.Options.NumPredict)
	})
}
