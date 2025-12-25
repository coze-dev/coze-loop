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
		Metadata:     do.Metadata,
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
		Role:             ptr.Of(RoleDO2DTO(do.Role)),
		ReasoningContent: do.ReasoningContent,
		Content:          do.Content,
		Parts:            OpenAPIBatchContentPartDO2DTO(do.Parts),
		ToolCallID:       do.ToolCallID,
		ToolCalls:        OpenAPIBatchToolCallDO2DTO(do.ToolCalls),
		SkipRender:       do.SkipRender,
		Metadata:         do.Metadata,
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
		ToolChoice:              ptr.Of(prompt.ToolChoiceType(do.ToolChoice)),
		ToolChoiceSpecification: OpenAPIToolChoiceSpecificationDO2DTO(do.ToolChoiceSpecification),
	}
}

func OpenAPIToolChoiceSpecificationDO2DTO(do *entity.ToolChoiceSpecification) *openapi.ToolChoiceSpecification {
	if do == nil {
		return nil
	}
	return &openapi.ToolChoiceSpecification{
		Type: ptr.Of(prompt.ToolType(do.Type)),
		Name: ptr.Of(do.Name),
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

func OpenAPIBatchContentPartDO2DTO(dos []*entity.ContentPart) []*openapi.ContentPart {
	if dos == nil {
		return nil
	}
	parts := make([]*openapi.ContentPart, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		parts = append(parts, OpenAPIContentPartDO2DTO(do))
	}
	return parts
}

func OpenAPIContentPartDO2DTO(do *entity.ContentPart) *openapi.ContentPart {
	if do == nil {
		return nil
	}
	var imageURL *string
	if do.ImageURL != nil {
		imageURL = ptr.Of(do.ImageURL.URL)
	}
	var videoURL *string
	var config *openapi.MediaConfig
	if do.VideoURL != nil {
		if do.VideoURL.URL != "" {
			videoURL = ptr.Of(do.VideoURL.URL)
		}
	}
	// Set Config with fps if available
	if do.MediaConfig != nil && do.MediaConfig.Fps != nil {
		config = &openapi.MediaConfig{
			Fps: do.MediaConfig.Fps,
		}
	}
	return &openapi.ContentPart{
		Type:       ptr.Of(OpenAPIContentTypeDO2DTO(do.Type)),
		Text:       do.Text,
		ImageURL:   imageURL,
		VideoURL:   videoURL,
		Base64Data: do.Base64Data,
		Config:     config,
	}
}

func OpenAPIContentTypeDO2DTO(do entity.ContentType) openapi.ContentType {
	switch do {
	case entity.ContentTypeText:
		return openapi.ContentTypeText
	case entity.ContentTypeImageURL:
		return openapi.ContentTypeImageURL
	case entity.ContentTypeVideoURL:
		return openapi.ContentTypeVideoURL
	case entity.ContentTypeBase64Data:
		return openapi.ContentTypeBase64Data
	case entity.ContentTypeMultiPartVariable:
		return openapi.ContentTypeMultiPartVariable
	default:
		return openapi.ContentTypeText
	}
}

// OpenAPIBatchMessageDTO2DO 将openapi Message转换为entity Message
func OpenAPIBatchMessageDTO2DO(dtos []*openapi.Message) []*entity.Message {
	if len(dtos) == 0 {
		return nil
	}
	dos := make([]*entity.Message, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		dos = append(dos, OpenAPIMessageDTO2DO(dto))
	}
	return dos
}

// OpenAPIMessageDTO2DO 将openapi Message转换为entity Message
func OpenAPIMessageDTO2DO(dto *openapi.Message) *entity.Message {
	if dto == nil {
		return nil
	}
	return &entity.Message{
		Role:             RoleDTO2DO(dto.GetRole()),
		ReasoningContent: dto.ReasoningContent,
		Content:          dto.Content,
		Parts:            OpenAPIBatchContentPartDTO2DO(dto.Parts),
		ToolCallID:       dto.ToolCallID,
		ToolCalls:        OpenAPIBatchToolCallDTO2DO(dto.ToolCalls),
		SkipRender:       dto.SkipRender,
		Metadata:         dto.Metadata,
	}
}

// OpenAPIBatchContentPartDTO2DO 将openapi ContentPart转换为entity ContentPart
func OpenAPIBatchContentPartDTO2DO(dtos []*openapi.ContentPart) []*entity.ContentPart {
	if dtos == nil {
		return nil
	}
	parts := make([]*entity.ContentPart, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		parts = append(parts, OpenAPIContentPartDTO2DO(dto))
	}
	return parts
}

// OpenAPIContentPartDTO2DO 将openapi ContentPart转换为entity ContentPart
func OpenAPIContentPartDTO2DO(dto *openapi.ContentPart) *entity.ContentPart {
	if dto == nil {
		return nil
	}
	var imageURL *entity.ImageURL
	if dto.ImageURL != nil && *dto.ImageURL != "" {
		imageURL = &entity.ImageURL{
			URL: *dto.ImageURL,
		}
	}
	var videoURL *entity.VideoURL
	if dto.VideoURL != nil && *dto.VideoURL != "" {
		videoURL = &entity.VideoURL{
			URL: *dto.VideoURL,
		}
	}
	var mediaConfig *entity.MediaConfig
	// Set MediaConfig from Config if available
	if dto.Config != nil && dto.Config.Fps != nil {
		mediaConfig = &entity.MediaConfig{
			Fps: dto.Config.Fps,
		}
	}
	return &entity.ContentPart{
		Type:        OpenAPIContentTypeDTO2DO(dto.GetType()),
		Text:        dto.Text,
		ImageURL:    imageURL,
		VideoURL:    videoURL,
		Base64Data:  dto.Base64Data,
		MediaConfig: mediaConfig,
	}
}

// OpenAPIContentTypeDTO2DO 将openapi ContentType转换为entity ContentType
func OpenAPIContentTypeDTO2DO(dto openapi.ContentType) entity.ContentType {
	switch dto {
	case openapi.ContentTypeText:
		return entity.ContentTypeText
	case openapi.ContentTypeImageURL:
		return entity.ContentTypeImageURL
	case openapi.ContentTypeVideoURL:
		return entity.ContentTypeVideoURL
	case openapi.ContentTypeBase64Data:
		return entity.ContentTypeBase64Data
	case openapi.ContentTypeMultiPartVariable:
		return entity.ContentTypeMultiPartVariable
	default:
		return entity.ContentTypeText
	}
}

// OpenAPIBatchVariableValDTO2DO 将openapi VariableVal转换为entity VariableVal
func OpenAPIBatchVariableValDTO2DO(dtos []*openapi.VariableVal) []*entity.VariableVal {
	if len(dtos) == 0 {
		return nil
	}
	dos := make([]*entity.VariableVal, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		dos = append(dos, OpenAPIVariableValDTO2DO(dto))
	}
	return dos
}

// OpenAPIVariableValDTO2DO 将openapi VariableVal转换为entity VariableVal
func OpenAPIVariableValDTO2DO(dto *openapi.VariableVal) *entity.VariableVal {
	if dto == nil {
		return nil
	}
	return &entity.VariableVal{
		Key:                 dto.GetKey(),
		Value:               dto.Value,
		PlaceholderMessages: OpenAPIBatchMessageDTO2DO(dto.PlaceholderMessages),
		MultiPartValues:     OpenAPIBatchContentPartDTO2DO(dto.MultiPartValues),
	}
}

// OpenAPITokenUsageDO2DTO 将entity TokenUsage转换为openapi TokenUsage
func OpenAPITokenUsageDO2DTO(do *entity.TokenUsage) *openapi.TokenUsage {
	if do == nil {
		return nil
	}
	return &openapi.TokenUsage{
		InputTokens:  ptr.Of(int32(do.InputTokens)),
		OutputTokens: ptr.Of(int32(do.OutputTokens)),
	}
}

// OpenAPIBatchToolCallDO2DTO 将entity ToolCall转换为openapi ToolCall
func OpenAPIBatchToolCallDO2DTO(dos []*entity.ToolCall) []*openapi.ToolCall {
	if dos == nil {
		return nil
	}
	toolCalls := make([]*openapi.ToolCall, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		toolCalls = append(toolCalls, OpenAPIToolCallDO2DTO(do))
	}
	return toolCalls
}

// OpenAPIToolCallDO2DTO 将entity ToolCall转换为openapi ToolCall
func OpenAPIToolCallDO2DTO(do *entity.ToolCall) *openapi.ToolCall {
	if do == nil {
		return nil
	}
	return &openapi.ToolCall{
		Index:        ptr.Of(int32(do.Index)),
		ID:           ptr.Of(do.ID),
		Type:         ptr.Of(OpenAPIToolTypeDO2DTO(do.Type)),
		FunctionCall: OpenAPIFunctionCallDO2DTO(do.FunctionCall),
	}
}

// OpenAPIToolTypeDO2DTO 将entity ToolType转换为openapi ToolType
func OpenAPIToolTypeDO2DTO(do entity.ToolType) openapi.ToolType {
	switch do {
	case entity.ToolTypeFunction:
		return openapi.ToolTypeFunction
	case entity.ToolTypeGoogleSearch:
		return openapi.ToolTypeGoogleSearch
	default:
		return openapi.ToolTypeFunction
	}
}

// OpenAPIFunctionCallDO2DTO 将entity FunctionCall转换为openapi FunctionCall
func OpenAPIFunctionCallDO2DTO(do *entity.FunctionCall) *openapi.FunctionCall {
	if do == nil {
		return nil
	}
	return &openapi.FunctionCall{
		Name:      ptr.Of(do.Name),
		Arguments: do.Arguments,
	}
}

// OpenAPIBatchToolCallDTO2DO 将openapi ToolCall转换为entity ToolCall
func OpenAPIBatchToolCallDTO2DO(dtos []*openapi.ToolCall) []*entity.ToolCall {
	if dtos == nil {
		return nil
	}
	toolCalls := make([]*entity.ToolCall, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		toolCalls = append(toolCalls, OpenAPIToolCallDTO2DO(dto))
	}
	return toolCalls
}

// OpenAPIToolCallDTO2DO 将openapi ToolCall转换为entity ToolCall
func OpenAPIToolCallDTO2DO(dto *openapi.ToolCall) *entity.ToolCall {
	if dto == nil {
		return nil
	}
	return &entity.ToolCall{
		Index:        int64(dto.GetIndex()),
		ID:           dto.GetID(),
		Type:         OpenAPIToolTypeDTO2DO(dto.GetType()),
		FunctionCall: OpenAPIFunctionCallDTO2DO(dto.FunctionCall),
	}
}

// OpenAPIToolTypeDTO2DO 将openapi ToolType转换为entity ToolType
func OpenAPIToolTypeDTO2DO(dto openapi.ToolType) entity.ToolType {
	switch dto {
	case openapi.ToolTypeFunction:
		return entity.ToolTypeFunction
	case openapi.ToolTypeGoogleSearch:
		return entity.ToolTypeGoogleSearch
	default:
		return entity.ToolTypeFunction
	}
}

// OpenAPIFunctionCallDTO2DO 将openapi FunctionCall转换为entity FunctionCall
func OpenAPIFunctionCallDTO2DO(dto *openapi.FunctionCall) *entity.FunctionCall {
	if dto == nil {
		return nil
	}
	return &entity.FunctionCall{
		Name:      dto.GetName(),
		Arguments: dto.Arguments,
	}
}

// OpenAPIPromptBasicDO2DTO 将entity Prompt转换为openapi PromptBasic
func OpenAPIPromptBasicDO2DTO(do *entity.Prompt) *openapi.PromptBasic {
	if do == nil || do.PromptBasic == nil {
		return nil
	}
	return &openapi.PromptBasic{
		ID:            ptr.Of(do.ID),
		WorkspaceID:   ptr.Of(do.SpaceID),
		PromptKey:     ptr.Of(do.PromptKey),
		DisplayName:   ptr.Of(do.PromptBasic.DisplayName),
		Description:   ptr.Of(do.PromptBasic.Description),
		LatestVersion: ptr.Of(do.PromptBasic.LatestVersion),
		CreatedBy:     ptr.Of(do.PromptBasic.CreatedBy),
		UpdatedBy:     ptr.Of(do.PromptBasic.UpdatedBy),
		CreatedAt:     ptr.Of(do.PromptBasic.CreatedAt.UnixMilli()),
		UpdatedAt:     ptr.Of(do.PromptBasic.UpdatedAt.UnixMilli()),
		LatestCommittedAt: func() *int64 {
			if do.PromptBasic.LatestCommittedAt == nil {
				return nil
			}
			return ptr.Of(do.PromptBasic.LatestCommittedAt.UnixMilli())
		}(),
	}
}

// OpenAPIBatchToolDTO2DO 将openapi Tool转换为entity Tool
func OpenAPIBatchToolDTO2DO(dtos []*openapi.Tool) []*entity.Tool {
	if dtos == nil {
		return nil
	}
	var tools []*entity.Tool
	for _, dto := range dtos {
		if dto != nil {
			tools = append(tools, OpenAPIToolDTO2DO(dto))
		}
	}
	return tools
}

// OpenAPIToolDTO2DO 将openapi Tool转换为entity Tool
func OpenAPIToolDTO2DO(dto *openapi.Tool) *entity.Tool {
	if dto == nil {
		return nil
	}
	return &entity.Tool{
		Type:     OpenAPIToolTypeDTO2DO(dto.GetType()),
		Function: OpenAPIFunctionDTO2DO(dto.Function),
	}
}

// OpenAPIFunctionDTO2DO 将openapi Function转换为entity Function
func OpenAPIFunctionDTO2DO(dto *openapi.Function) *entity.Function {
	if dto == nil {
		return nil
	}
	return &entity.Function{
		Name:        dto.GetName(),
		Description: dto.GetDescription(),
		Parameters:  dto.GetParameters(),
	}
}

// OpenAPIToolCallConfigDTO2DO 将openapi ToolCallConfig转换为entity ToolCallConfig
func OpenAPIToolCallConfigDTO2DO(dto *openapi.ToolCallConfig) *entity.ToolCallConfig {
	if dto == nil {
		return nil
	}
	return &entity.ToolCallConfig{
		ToolChoice:              OpenAPIToolChoiceTypeDTO2DO(dto.GetToolChoice()),
		ToolChoiceSpecification: OpenAPIToolChoiceSpecificationDTO2DO(dto.ToolChoiceSpecification),
	}
}

// OpenAPIToolChoiceTypeDTO2DO 将openapi ToolChoiceType转换为entity ToolChoiceType
func OpenAPIToolChoiceTypeDTO2DO(dto openapi.ToolChoiceType) entity.ToolChoiceType {
	return entity.ToolChoiceType(dto)
}

// OpenAPIToolChoiceSpecificationDTO2DO 将openapi ToolChoiceSpecification转换为entity ToolChoiceSpecification
func OpenAPIToolChoiceSpecificationDTO2DO(dto *openapi.ToolChoiceSpecification) *entity.ToolChoiceSpecification {
	if dto == nil {
		return nil
	}
	return &entity.ToolChoiceSpecification{
		Type: OpenAPIToolTypeDTO2DO(dto.GetType()),
		Name: dto.GetName(),
	}
}

// OpenAPIModelConfigDTO2DO 将domain prompt ModelConfig转换为entity ModelConfig
func OpenAPIModelConfigDTO2DO(dto *prompt.ModelConfig) *entity.ModelConfig {
	if dto == nil {
		return nil
	}
	return &entity.ModelConfig{
		ModelID:           dto.GetModelID(),
		MaxTokens:         dto.MaxTokens,
		Temperature:       dto.Temperature,
		TopK:              dto.TopK,
		TopP:              dto.TopP,
		PresencePenalty:   dto.PresencePenalty,
		FrequencyPenalty:  dto.FrequencyPenalty,
		JSONMode:          dto.JSONMode,
		Extra:             dto.Extra,
		ParamConfigValues: OpenAPIBatchParamConfigValueDTO2DO(dto.ParamConfigValues),
	}
}

// OpenAPIBatchParamConfigValueDTO2DO 将domain prompt ParamConfigValue转换为entity ParamConfigValue
func OpenAPIBatchParamConfigValueDTO2DO(dtos []*prompt.ParamConfigValue) []*entity.ParamConfigValue {
	if dtos == nil {
		return nil
	}
	var params []*entity.ParamConfigValue
	for _, dto := range dtos {
		if dto != nil {
			params = append(params, OpenAPIParamConfigValueDTO2DO(dto))
		}
	}
	return params
}

// OpenAPIParamConfigValueDTO2DO 将domain prompt ParamConfigValue转换为entity ParamConfigValue
func OpenAPIParamConfigValueDTO2DO(dto *prompt.ParamConfigValue) *entity.ParamConfigValue {
	if dto == nil {
		return nil
	}
	return &entity.ParamConfigValue{
		Name:  dto.GetName(),
		Label: dto.GetLabel(),
		Value: OpenAPIParamOptionDTO2DO(dto.Value),
	}
}

// OpenAPIParamOptionDTO2DO 将domain prompt ParamOption转换为entity ParamOption
func OpenAPIParamOptionDTO2DO(dto *prompt.ParamOption) *entity.ParamOption {
	if dto == nil {
		return nil
	}
	return &entity.ParamOption{
		Value: dto.GetValue(),
		Label: dto.GetLabel(),
	}
}
