// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

func TestToolResultsCollector_CollectToolResults(t *testing.T) {
	collector := NewToolResultsCollector()

	t.Run("nil mock tools returns empty map", func(t *testing.T) {
		got, err := collector.CollectToolResults(context.Background(), CollectToolResultsParam{
			MockTools: nil,
		})
		assert.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("builds tool result map", func(t *testing.T) {
		got, err := collector.CollectToolResults(context.Background(), CollectToolResultsParam{
			MockTools: []*entity.MockTool{
				{Name: "tool_a", MockResponse: "{\"ok\":true}"},
				{Name: "tool_b", MockResponse: "b"},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"tool_a": "{\"ok\":true}",
			"tool_b": "b",
		}, got)
	})

	t.Run("skips nil and empty name entries", func(t *testing.T) {
		got, err := collector.CollectToolResults(context.Background(), CollectToolResultsParam{
			MockTools: []*entity.MockTool{
				nil,
				{Name: "", MockResponse: "ignored"},
				{Name: "tool_a", MockResponse: "a"},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, map[string]string{
			"tool_a": "a",
		}, got)
	})
}

