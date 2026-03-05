// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeExecuteRequest(t *testing.T) {
	t.Parallel()

	t.Run("nil request", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, normalizeExecuteRequest(nil))
	})

	t.Run("release_label fills prompt_identifier.label", func(t *testing.T) {
		t.Parallel()
		req := &openapi.ExecuteRequest{
			PromptIdentifier: &openapi.PromptQuery{
				PromptKey: ptr.Of("p1"),
			},
			ReleaseLabel: ptr.Of("prod"),
		}
		normalized := normalizeExecuteRequest(req)
		assert.Equal(t, "prod", normalized.GetPromptIdentifier().GetLabel())
		assert.Equal(t, "prod", normalized.GetReleaseLabel())
	})

	t.Run("custom_tool_config fallback to custom_tool_call_config", func(t *testing.T) {
		t.Parallel()
		req := &openapi.ExecuteRequest{
			CustomToolConfig: &openapi.ToolCallConfig{
				ToolChoice: ptr.Of(openapi.ToolChoiceTypeAuto),
			},
		}
		normalized := normalizeExecuteRequest(req)
		assert.NotNil(t, normalized.CustomToolCallConfig)
		assert.Equal(t, openapi.ToolChoiceTypeAuto, normalized.CustomToolCallConfig.GetToolChoice())
	})

	t.Run("custom_tools without config defaults to auto", func(t *testing.T) {
		t.Parallel()
		req := &openapi.ExecuteRequest{
			CustomTools: []*openapi.Tool{
				{
					Type: ptr.Of(openapi.ToolTypeFunction),
				},
			},
		}
		normalized := normalizeExecuteRequest(req)
		assert.NotNil(t, normalized.CustomToolCallConfig)
		assert.Equal(t, openapi.ToolChoiceTypeAuto, normalized.CustomToolCallConfig.GetToolChoice())
	})
}

