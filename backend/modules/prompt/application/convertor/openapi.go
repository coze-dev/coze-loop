// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func OpenAPIPromptDO2DTO(do *entity.Prompt) *openapi.Prompt {
	if do == nil {
		return nil
	}
	var promptTemplate *entity.PromptTemplate
	var tools []*entity.Tool
	var toolCallConfig *entity.ToolCallConfig
	var modelConfig *entity.ModelConfig
	if promptDetail := do.GetPromptDetail(); promptDetail != nil {
		promptTemplate = promptDetail.PromptTemplate
		tools = promptDetail.Tools
		toolCallConfig = promptDetail.ToolCallConfig
		modelConfig = promptDetail.ModelConfig
	}
	return &openapi.Prompt{
		WorkspaceID:    ptr.Of(do.SpaceID),
		PromptKey:      ptr.Of(do.PromptKey),
		Version:        ptr.Of(do.GetVersion()),
		PromptTemplate: OpenAPIPromptTemplateDO2DTO(promptTemplate),
		Tools:          OpenAPIBatchToolDO2DTO(tools),
		ToolCallConfig: OpenAPIToolCallConfigDO2DTO(toolCallConfig),
		LlmConfig:      OpenAPIModelConfigDO2DTO(modelConfig),
	}
}

func OpenAPIPromptTemplateDO2DTO(do *entity.PromptTemplate) *openapi.PromptTemplate {
	if do == nil {
		return nil
	}
	return &openapi.PromptTemplate{
		TemplateType: ptr.Of(prompt.TemplateType(do.TemplateType)),
		Messages:     OpenAPIBatchMessageDO2DTO(do.Messages),
		VariableDefs: OpenAPIBatchVariableDefDO2DTO(do.VariableDefs),
	}
}

func OpenAPIBatchMessageDO2DTO(dos []*entity.Message) []*openapi.Message {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*openapi.Message, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIMessageDO2DTO(do))
	}
	return dtos
}

func OpenAPIMessageDO2DTO(do *entity.Message) *openapi.Message {
	if do == nil {
		return nil
	}
	return &openapi.Message{
		Role:    ptr.Of(RoleDO2DTO(do.Role)),
		Content: do.Content,
	}
}

func OpenAPIBatchVariableDefDO2DTO(dos []*entity.VariableDef) []*openapi.VariableDef {
	dtos := make([]*openapi.VariableDef, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIVariableDefDO2DTO(do))
	}
	return dtos
}

func OpenAPIVariableDefDO2DTO(do *entity.VariableDef) *openapi.VariableDef {
	if do == nil {
		return nil
	}
	return &openapi.VariableDef{
		Key:  ptr.Of(do.Key),
		Desc: ptr.Of(do.Desc),
		Type: ptr.Of(prompt.VariableType(do.Type)),
	}
}

func OpenAPIBatchToolDO2DTO(dos []*entity.Tool) []*openapi.Tool {
	if dos == nil {
		return nil
	}
	dtos := make([]*openapi.Tool, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIToolDO2DTO(do))
	}
	return dtos
}

func OpenAPIToolDO2DTO(do *entity.Tool) *openapi.Tool {
	if do == nil {
		return nil
	}
	return &openapi.Tool{
		Type:     ptr.Of(prompt.ToolType(do.Type)),
		Function: OpenAPIFunctionDO2DTO(do.Function),
	}
}

func OpenAPIFunctionDO2DTO(do *entity.Function) *openapi.Function {
	if do == nil {
		return nil
	}
	return &openapi.Function{
		Name:        ptr.Of(do.Name),
		Description: ptr.Of(do.Description),
		Parameters:  ptr.Of(do.Parameters),
	}
}

func OpenAPIToolCallConfigDO2DTO(do *entity.ToolCallConfig) *openapi.ToolCallConfig {
	if do == nil {
		return nil
	}
	return &openapi.ToolCallConfig{
		ToolChoice: ptr.Of(prompt.ToolChoiceType(do.ToolChoice)),
	}
}

func OpenAPIModelConfigDO2DTO(do *entity.ModelConfig) *openapi.LLMConfig {
	if do == nil {
		return nil
	}
	return &openapi.LLMConfig{
		MaxTokens:        do.MaxTokens,
		Temperature:      do.Temperature,
		TopK:             do.TopK,
		TopP:             do.TopP,
		PresencePenalty:  do.PresencePenalty,
		FrequencyPenalty: do.FrequencyPenalty,
		JSONMode:         do.JSONMode,
	}
}
