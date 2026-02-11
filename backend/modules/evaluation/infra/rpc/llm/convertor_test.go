// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	runtimedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestLLMCallParamConvert(t *testing.T) {
	param := &commonentity.LLMCallParam{
		SpaceID:     1,
		EvaluatorID: "100",
		UserID:      gptr.Of("user1"),
		Scenario:    commonentity.ScenarioEvaluator,
		Messages: []*commonentity.Message{
			{
				Role:    commonentity.RoleUser,
				Content: &commonentity.Content{Text: gptr.Of("hello")},
			},
		},
		ModelConfig: &commonentity.ModelConfig{
			ModelID:     gptr.Of(int64(100)),
			Temperature: gptr.Of(0.7),
		},
		ToolCallConfig: &commonentity.ToolCallConfig{
			ToolChoice: commonentity.ToolChoiceTypeAuto,
		},
	}

	got := LLMCallParamConvert(param)
	assert.Equal(t, int64(1), *got.BizParam.WorkspaceID)
	assert.Equal(t, "user1", *got.BizParam.UserID)
	assert.Equal(t, common.ScenarioEvaluator, *got.BizParam.Scenario)
	assert.Equal(t, "100", *got.BizParam.ScenarioEntityID)
	assert.Equal(t, runtimedto.RoleUser, got.Messages[0].Role)
}

func TestModelConfigDO2DTO(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		assert.Nil(t, ModelConfigDO2DTO(nil, nil))
	})

	t.Run("full_input", func(t *testing.T) {
		mc := &commonentity.ModelConfig{
			ModelID:     gptr.Of(int64(1)),
			Temperature: gptr.Of(0.5),
			MaxTokens:   gptr.Of(int32(100)),
			TopP:        gptr.Of(0.9),
		}
		tcc := &commonentity.ToolCallConfig{
			ToolChoice: commonentity.ToolChoiceTypeRequired,
		}
		got := ModelConfigDO2DTO(mc, tcc)
		assert.Equal(t, int64(1), got.ModelID)
		assert.Equal(t, 0.5, *got.Temperature)
		assert.Equal(t, int64(100), *got.MaxTokens)
		assert.Equal(t, 0.9, *got.TopP)
		assert.Equal(t, runtimedto.ToolChoiceRequired, *got.ToolChoice)
	})
}

func TestToolChoiceTypeDO2DTO(t *testing.T) {
	assert.Equal(t, runtimedto.ToolChoiceNone, ToolChoiceTypeDO2DTO(commonentity.ToolChoiceTypeNone))
	assert.Equal(t, runtimedto.ToolChoiceAuto, ToolChoiceTypeDO2DTO(commonentity.ToolChoiceTypeAuto))
	assert.Equal(t, runtimedto.ToolChoiceRequired, ToolChoiceTypeDO2DTO(commonentity.ToolChoiceTypeRequired))
	assert.Equal(t, runtimedto.ToolChoiceAuto, ToolChoiceTypeDO2DTO(commonentity.ToolChoiceType("unknown")))
}

func TestRoleDO2DTO(t *testing.T) {
	assert.Equal(t, runtimedto.RoleSystem, RoleDO2DTO(commonentity.RoleSystem))
	assert.Equal(t, runtimedto.RoleUser, RoleDO2DTO(commonentity.RoleUser))
	assert.Equal(t, runtimedto.RoleAssistant, RoleDO2DTO(commonentity.RoleAssistant))
	assert.Equal(t, runtimedto.RoleTool, RoleDO2DTO(commonentity.RoleTool))
	assert.Equal(t, runtimedto.RoleUser, RoleDO2DTO(commonentity.Role(999)))
}

func TestToolCallConvert(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		assert.Nil(t, ToolCallDO2DTO(nil))
		assert.Nil(t, ToolCallDTO2DO(nil))
	})

	t.Run("valid_input", func(t *testing.T) {
		do := &commonentity.ToolCall{
			Index: 0,
			ID:    "call_1",
			Type:  commonentity.ToolTypeFunction,
			FunctionCall: &commonentity.FunctionCall{
				Name:      "test_func",
				Arguments: gptr.Of("{}"),
			},
		}

		dto := ToolCallDO2DTO(do)
		assert.Equal(t, int64(0), *dto.Index)
		assert.Equal(t, "call_1", *dto.ID)
		assert.Equal(t, runtimedto.ToolTypeFunction, *dto.Type)
		assert.Equal(t, "test_func", *dto.FunctionCall.Name)

		back := ToolCallDTO2DO(dto)
		assert.Equal(t, do.Index, back.Index)
		assert.Equal(t, do.ID, back.ID)
		assert.Equal(t, do.FunctionCall.Name, back.FunctionCall.Name)
	})
}

func TestReplyItemDTO2DO(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		assert.Nil(t, ReplyItemDTO2DO(nil))
	})

	t.Run("full_input", func(t *testing.T) {
		dto := &runtimedto.Message{
			Content:          gptr.Of("content"),
			ReasoningContent: gptr.Of("reasoning"),
			ResponseMeta: &runtimedto.ResponseMeta{
				FinishReason: gptr.Of("stop"),
				Usage: &runtimedto.TokenUsage{
					PromptTokens:     gptr.Of(int64(10)),
					CompletionTokens: gptr.Of(int64(20)),
				},
			},
		}
		got := ReplyItemDTO2DO(dto)
		assert.Equal(t, "content", *got.Content)
		assert.Equal(t, "reasoning", *got.ReasoningContent)
		assert.Equal(t, "stop", got.FinishReason)
		assert.Equal(t, int64(10), got.TokenUsage.InputTokens)
		assert.Equal(t, int64(20), got.TokenUsage.OutputTokens)
	})
}

func TestScenarioDO2DTO(t *testing.T) {
	assert.Equal(t, common.ScenarioEvalTarget, ScenarioDO2DTO(commonentity.ScenarioEvalTarget))
	assert.Equal(t, common.ScenarioEvaluator, ScenarioDO2DTO(commonentity.ScenarioEvaluator))
	assert.Equal(t, common.ScenarioDefault, ScenarioDO2DTO(commonentity.Scenario("unknown")))
}

func TestToolDO2DTO(t *testing.T) {
	t.Run("nil_input", func(t *testing.T) {
		assert.Nil(t, ToolDO2DTO(nil))
	})
	t.Run("valid_input", func(t *testing.T) {
		do := &commonentity.Tool{
			Function: &commonentity.Function{
				Name:        "test",
				Description: "desc",
				Parameters:  "params",
			},
		}
		got := ToolDO2DTO(do)
		assert.Equal(t, "test", *got.Name)
		assert.Equal(t, "desc", *got.Desc)
		assert.Equal(t, "params", *got.Def)
	})
}

func TestToolsDO2DTO(t *testing.T) {
	t.Run("empty_input", func(t *testing.T) {
		assert.Nil(t, ToolsDO2DTO(nil))
	})
	t.Run("valid_input", func(t *testing.T) {
		dos := []*commonentity.Tool{{Function: &commonentity.Function{Name: "t1"}}}
		got := ToolsDO2DTO(dos)
		assert.Len(t, got, 1)
	})
}

func TestToolCallsDO2DTO(t *testing.T) {
	t.Run("empty_input", func(t *testing.T) {
		assert.Nil(t, ToolCallsDO2DTO(nil))
	})
	t.Run("valid_input", func(t *testing.T) {
		dos := []*commonentity.ToolCall{{ID: "c1"}}
		got := ToolCallsDO2DTO(dos)
		assert.Len(t, got, 1)
	})
}

func TestToolCallsDTO2DO(t *testing.T) {
	t.Run("empty_input", func(t *testing.T) {
		assert.Nil(t, ToolCallsDTO2DO(nil))
	})
	t.Run("valid_input", func(t *testing.T) {
		dtos := []*runtimedto.ToolCall{{ID: gptr.Of("c1")}}
		got := ToolCallsDTO2DO(dtos)
		assert.Len(t, got, 1)
	})
}

func TestContentTypeDO2DTO(t *testing.T) {
	assert.Equal(t, runtimedto.ChatMessagePartTypeText, ContentTypeDO2DTO(commonentity.ContentTypeText))
	assert.Equal(t, runtimedto.ChatMessagePartTypeText, ContentTypeDO2DTO(commonentity.ContentType("unknown")))
}

func TestMessageDO2DTO(t *testing.T) {
	assert.Nil(t, MessageDO2DTO(nil))
}

func TestFunctionDO2DTO(t *testing.T) {
	assert.Nil(t, FunctionDO2DTO(nil))
}

func TestFunctionDTO2DO(t *testing.T) {
	assert.Nil(t, FunctionDTO2DO(nil))
}

func TestTokenUsageDTO2DO(t *testing.T) {
	assert.Nil(t, TokenUsageDTO2DO(nil))
}
