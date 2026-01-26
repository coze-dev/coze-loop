// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	runtimedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func LLMCallParamConvert(param *commonentity.LLMCallParam) *runtime.ChatRequest {
	return &runtime.ChatRequest{
		ModelConfig: ModelConfigDO2DTO(param.ModelConfig, param.ToolCallConfig),
		Messages:    MessagesDO2DTO(param.Messages),
		Tools:       ToolsDO2DTO(param.Tools),
		BizParam: &runtimedto.BizParam{
			WorkspaceID: gptr.Of(param.SpaceID),
			UserID:      param.UserID,
			Scenario:    gptr.Of(ScenarioDO2DTO(param.Scenario)),
			// 这里传prompt key
			ScenarioEntityID: gptr.Of(param.EvaluatorID),
		},
	}
}

func ModelConfigDO2DTO(modelConfig *commonentity.ModelConfig, toolCallConfig *commonentity.ToolCallConfig) *runtimedto.ModelConfig {
	if modelConfig == nil {
		return nil
	}
	var toolChoice *runtimedto.ToolChoice
	if toolCallConfig != nil {
		toolChoice = gptr.Of(ToolChoiceTypeDO2DTO(toolCallConfig.ToolChoice))
	}
	return &runtimedto.ModelConfig{
		ModelID:        modelConfig.ModelID,
		Temperature:    modelConfig.Temperature,
		MaxTokens:      gptr.Of(int64(gptr.Indirect(modelConfig.MaxTokens))),
		TopP:           modelConfig.TopP,
		ToolChoice:     toolChoice,
		Protocol:       modelConfig.Protocol,
		Identification: modelConfig.Identification,
	}
}

func ToolChoiceTypeDO2DTO(do commonentity.ToolChoiceType) runtimedto.ToolChoice {
	switch do {
	case commonentity.ToolChoiceTypeNone:
		return runtimedto.ToolChoiceNone
	case commonentity.ToolChoiceTypeAuto:
		return runtimedto.ToolChoiceAuto
	case commonentity.ToolChoiceTypeRequired:
		return runtimedto.ToolChoiceRequired
	default:
		return runtimedto.ToolChoiceAuto
	}
}

func MessagesDO2DTO(dos []*commonentity.Message) []*runtimedto.Message {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.Message, 0, len(dos))
	for _, message := range dos {
		res = append(res, MessageDO2DTO(message))
	}
	return res
}

func MessageDO2DTO(do *commonentity.Message) *runtimedto.Message {
	if do == nil {
		return nil
	}
	return &runtimedto.Message{
		Role:         RoleDO2DTO(do.Role),
		Content:      do.Content.Text,
		ResponseMeta: nil,
	}
}

func RoleDO2DTO(do commonentity.Role) runtimedto.Role {
	switch do {
	case commonentity.RoleSystem:
		return runtimedto.RoleSystem
	case commonentity.RoleUser:
		return runtimedto.RoleUser
	case commonentity.RoleAssistant:
		return runtimedto.RoleAssistant
	case commonentity.RoleTool:
		return runtimedto.RoleTool
	default:
		return runtimedto.RoleUser
	}
}

func ContentTypeDO2DTO(do commonentity.ContentType) runtimedto.ChatMessagePartType {
	switch do {
	case commonentity.ContentTypeText:
		return runtimedto.ChatMessagePartTypeText
	default:
		return runtimedto.ChatMessagePartTypeText
	}
}

func ToolCallsDO2DTO(dos []*commonentity.ToolCall) []*runtimedto.ToolCall {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.ToolCall, 0, len(dos))
	for _, toolCall := range dos {
		res = append(res, ToolCallDO2DTO(toolCall))
	}
	return res
}

func ToolCallDO2DTO(do *commonentity.ToolCall) *runtimedto.ToolCall {
	if do == nil {
		return nil
	}
	return &runtimedto.ToolCall{
		Index:        gptr.Of(do.Index),
		ID:           gptr.Of(do.ID),
		Type:         gptr.Of(ToolTypeDO2DTO(do.Type)),
		FunctionCall: FunctionDO2DTO(do.FunctionCall),
	}
}

func ToolTypeDO2DTO(do commonentity.ToolType) runtimedto.ToolType {
	switch do {
	default:
		return runtimedto.ToolTypeFunction
	}
}

func FunctionDO2DTO(do *commonentity.FunctionCall) *runtimedto.FunctionCall {
	if do == nil {
		return nil
	}
	return &runtimedto.FunctionCall{
		Name:      gptr.Of(do.Name),
		Arguments: do.Arguments,
	}
}

func ToolsDO2DTO(dos []*commonentity.Tool) []*runtimedto.Tool {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.Tool, 0, len(dos))
	for _, tool := range dos {
		res = append(res, ToolDO2DTO(tool))
	}
	return res
}

func ToolDO2DTO(do *commonentity.Tool) *runtimedto.Tool {
	if do == nil || do.Function == nil {
		return nil
	}
	return &runtimedto.Tool{
		Name:    gptr.Of(do.Function.Name),
		Desc:    gptr.Of(do.Function.Description),
		DefType: gptr.Of(runtimedto.ToolDefTypeOpenAPIV3),
		Def:     gptr.Of(do.Function.Parameters),
	}
}

func ScenarioDO2DTO(do commonentity.Scenario) common.Scenario {
	switch do {
	case commonentity.ScenarioEvalTarget:
		return common.ScenarioEvalTarget
	case commonentity.ScenarioEvaluator:
		return common.ScenarioEvaluator
	default:
		return common.ScenarioDefault
	}
}

// ========================================================

func ReplyItemDTO2DO(dto *runtimedto.Message) *commonentity.ReplyItem {
	if dto == nil {
		return nil
	}
	var finishReason string
	var tokenUsage *commonentity.TokenUsage
	if dto.ResponseMeta != nil {
		finishReason = gptr.Indirect(dto.ResponseMeta.FinishReason)
		tokenUsage = TokenUsageDTO2DO(dto.ResponseMeta.Usage)
	}
	return &commonentity.ReplyItem{
		Content:          dto.Content,
		ReasoningContent: dto.ReasoningContent,
		ToolCalls:        ToolCallsDTO2DO(dto.ToolCalls),
		FinishReason:     finishReason,
		TokenUsage:       tokenUsage,
	}
}

func ToolCallsDTO2DO(dtos []*runtimedto.ToolCall) []*commonentity.ToolCall {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*commonentity.ToolCall, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, ToolCallDTO2DO(dto))
	}
	return res
}

func ToolCallDTO2DO(dto *runtimedto.ToolCall) *commonentity.ToolCall {
	if dto == nil {
		return nil
	}
	return &commonentity.ToolCall{
		Index:        gptr.Indirect(dto.Index),
		ID:           gptr.Indirect(dto.ID),
		Type:         ToolTypeDTO2DO(gptr.Indirect(dto.Type)),
		FunctionCall: FunctionDTO2DO(dto.FunctionCall),
	}
}

func ToolTypeDTO2DO(dto runtimedto.ToolType) commonentity.ToolType {
	switch dto {
	default:
		return commonentity.ToolTypeFunction
	}
}

func FunctionDTO2DO(dto *runtimedto.FunctionCall) *commonentity.FunctionCall {
	if dto == nil {
		return nil
	}
	return &commonentity.FunctionCall{
		Name:      gptr.Indirect(dto.Name),
		Arguments: dto.Arguments,
	}
}

func TokenUsageDTO2DO(dto *runtimedto.TokenUsage) *commonentity.TokenUsage {
	if dto == nil {
		return nil
	}
	return &commonentity.TokenUsage{
		InputTokens:  gptr.Indirect(dto.PromptTokens),
		OutputTokens: gptr.Indirect(dto.CompletionTokens),
	}
}
