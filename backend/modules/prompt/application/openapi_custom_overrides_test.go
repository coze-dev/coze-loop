// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestPromptOpenAPIApplicationImpl_applyCustomOverrides(t *testing.T) {
	t.Parallel()

	app := &PromptOpenAPIApplicationImpl{}

	t.Run("nil input", func(t *testing.T) {
		t.Parallel()

		got, err := app.applyCustomOverrides(nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("no overrides returns original", func(t *testing.T) {
		t.Parallel()

		original := &entity.Prompt{
			ID:        1,
			SpaceID:   2,
			PromptKey: "k",
			PromptCommit: &entity.PromptCommit{
				PromptDetail: &entity.PromptDetail{},
			},
		}

		got, err := app.applyCustomOverrides(original, &openapi.ExecuteRequest{})
		assert.NoError(t, err)
		assert.Same(t, original, got)
	})

	t.Run("override with missing commit/detail returns original", func(t *testing.T) {
		t.Parallel()

		original := &entity.Prompt{ID: 1}
		req := &openapi.ExecuteRequest{
			CustomTools: []*openapi.Tool{
				{Type: ptr.Of(openapi.ToolTypeFunction)},
			},
		}
		got, err := app.applyCustomOverrides(original, req)
		assert.NoError(t, err)
		assert.Same(t, original, got)

		original = &entity.Prompt{ID: 1, PromptCommit: &entity.PromptCommit{}}
		got, err = app.applyCustomOverrides(original, req)
		assert.NoError(t, err)
		assert.Same(t, original, got)
	})

	t.Run("override tools deep-copies and does not mutate original", func(t *testing.T) {
		t.Parallel()

		original := &entity.Prompt{
			ID:        1,
			SpaceID:   2,
			PromptKey: "k",
			PromptCommit: &entity.PromptCommit{
				PromptDetail: &entity.PromptDetail{
					Tools: []*entity.Tool{
						{Type: entity.ToolTypeFunction, Function: &entity.Function{Name: "old"}},
					},
				},
			},
		}
		req := &openapi.ExecuteRequest{
			CustomTools: []*openapi.Tool{
				{
					Type: ptr.Of(openapi.ToolTypeFunction),
					Function: &openapi.Function{
						Name:        ptr.Of("new"),
						Description: ptr.Of("desc"),
						Parameters:  ptr.Of(`{}`),
					},
				},
			},
		}

		got, err := app.applyCustomOverrides(original, req)
		assert.NoError(t, err)
		assert.NotSame(t, original, got)

		assert.Equal(t, "old", original.PromptCommit.PromptDetail.Tools[0].Function.Name)
		assert.Equal(t, "new", got.PromptCommit.PromptDetail.Tools[0].Function.Name)
	})

	t.Run("override toolcall config and model config", func(t *testing.T) {
		t.Parallel()

		original := &entity.Prompt{
			ID:        1,
			SpaceID:   2,
			PromptKey: "k",
			PromptCommit: &entity.PromptCommit{
				PromptDetail: &entity.PromptDetail{
					ToolCallConfig: &entity.ToolCallConfig{
						ToolChoice: entity.ToolChoiceTypeNone,
					},
					ModelConfig: &entity.ModelConfig{
						ModelID: 1,
					},
				},
			},
		}
		req := &openapi.ExecuteRequest{
			CustomToolCallConfig: &openapi.ToolCallConfig{
				ToolChoice: ptr.Of(openapi.ToolChoiceTypeAuto),
			},
			CustomModelConfig: &openapi.ModelConfig{
				ModelID:     ptr.Of(int64(123)),
				Temperature: ptr.Of(0.7),
			},
		}

		got, err := app.applyCustomOverrides(original, req)
		assert.NoError(t, err)
		assert.NotSame(t, original, got)

		assert.Equal(t, entity.ToolChoiceTypeNone, original.PromptCommit.PromptDetail.ToolCallConfig.ToolChoice)
		assert.Equal(t, entity.ToolChoiceTypeAuto, got.PromptCommit.PromptDetail.ToolCallConfig.ToolChoice)

		assert.Equal(t, int64(1), original.PromptCommit.PromptDetail.ModelConfig.ModelID)
		assert.Equal(t, int64(123), got.PromptCommit.PromptDetail.ModelConfig.ModelID)
		assert.Equal(t, ptr.Of(0.7), got.PromptCommit.PromptDetail.ModelConfig.Temperature)
	})

	t.Run("custom model config with model_id unset/0 does not override", func(t *testing.T) {
		t.Parallel()

		original := &entity.Prompt{
			ID:        1,
			SpaceID:   2,
			PromptKey: "k",
			PromptCommit: &entity.PromptCommit{
				PromptDetail: &entity.PromptDetail{
					ModelConfig: &entity.ModelConfig{ModelID: 7},
				},
			},
		}
		req := &openapi.ExecuteRequest{
			CustomModelConfig: &openapi.ModelConfig{
				ModelID: ptr.Of(int64(0)), // IsSetModelID=true but value=0
			},
		}

		got, err := app.applyCustomOverrides(original, req)
		assert.NoError(t, err)
		assert.NotSame(t, original, got)
		assert.Equal(t, int64(7), got.PromptCommit.PromptDetail.ModelConfig.ModelID)
	})
}
