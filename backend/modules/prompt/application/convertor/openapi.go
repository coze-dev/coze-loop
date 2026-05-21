// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	domainopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func OpenAPIPromptDO2DTO(do *entity.Prompt) *domainopenapi.Prompt {
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
	return &domainopenapi.Prompt{
		WorkspaceID:    ptr.Of(do.SpaceID),
		PromptKey:      ptr.Of(do.PromptKey),
		Version:        ptr.Of(do.GetVersion()),
		PromptTemplate: OpenAPIPromptTemplateDO2DTO(promptTemplate),
		Tools:          OpenAPIBatchToolDO2DTO(tools),
		ToolCallConfig: OpenAPIToolCallConfigDO2DTO(toolCallConfig),
		LlmConfig:      OpenAPIModelConfigDO2DTO(modelConfig),
	}
}

func OpenAPIPromptTemplateDO2DTO(do *entity.PromptTemplate) *domainopenapi.PromptTemplate {
	if do == nil {
		return nil
	}
	return &domainopenapi.PromptTemplate{
		TemplateType: ptr.Of(domainopenapi.TemplateType(do.TemplateType)),
		Messages:     OpenAPIBatchMessageDO2DTO(do.Messages),
		VariableDefs: OpenAPIBatchVariableDefDO2DTO(do.VariableDefs),
		Metadata:     do.Metadata,
	}
}

func OpenAPIBatchMessageDO2DTO(dos []*entity.Message) []*domainopenapi.Message {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*domainopenapi.Message, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIMessageDO2DTO(do))
	}
	return dtos
}

func OpenAPIMessageDO2DTO(do *entity.Message) *domainopenapi.Message {
	if do == nil {
		return nil
	}
	return &domainopenapi.Message{
		Role:             ptr.Of(RoleDO2DTO(do.Role)),
		ReasoningContent: do.ReasoningContent,
		Content:          do.Content,
		Parts:            OpenAPIBatchContentPartDO2DTO(do.Parts),
		ToolCallID:       do.ToolCallID,
		ToolCalls:        OpenAPIBatchToolCallDO2DTO(do.ToolCalls),
		SkipRender:       do.SkipRender,
		Signature:        do.Signature,
		Metadata:         do.Metadata,
	}
}

func OpenAPIBatchVariableDefDO2DTO(dos []*entity.VariableDef) []*domainopenapi.VariableDef {
	dtos := make([]*domainopenapi.VariableDef, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIVariableDefDO2DTO(do))
	}
	return dtos
}

func OpenAPIVariableDefDO2DTO(do *entity.VariableDef) *domainopenapi.VariableDef {
	if do == nil {
		return nil
	}
	return &domainopenapi.VariableDef{
		Key:  ptr.Of(do.Key),
		Desc: ptr.Of(do.Desc),
		Type: ptr.Of(domainopenapi.VariableType(do.Type)),
	}
}

func OpenAPIBatchToolDO2DTO(dos []*entity.Tool) []*domainopenapi.Tool {
	if dos == nil {
		return nil
	}
	dtos := make([]*domainopenapi.Tool, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, OpenAPIToolDO2DTO(do))
	}
	return dtos
}

func OpenAPIToolDO2DTO(do *entity.Tool) *domainopenapi.Tool {
	if do == nil {
		return nil
	}
	return &domainopenapi.Tool{
		Type:     ptr.Of(domainopenapi.ToolType(do.Type)),
		Function: OpenAPIFunctionDO2DTO(do.Function),
	}
}

func OpenAPIFunctionDO2DTO(do *entity.Function) *domainopenapi.Function {
	if do == nil {
		return nil
	}
	return &domainopenapi.Function{
		Name:        ptr.Of(do.Name),
		Description: ptr.Of(do.Description),
		Parameters:  ptr.Of(do.Parameters),
	}
}

func OpenAPIToolCallConfigDO2DTO(do *entity.ToolCallConfig) *domainopenapi.ToolCallConfig {
	if do == nil {
		return nil
	}
	return &domainopenapi.ToolCallConfig{
		ToolChoice:              ptr.Of(domainopenapi.ToolChoiceType(do.ToolChoice)),
		ToolChoiceSpecification: OpenAPIToolChoiceSpecificationDO2DTO(do.ToolChoiceSpecification),
	}
}

func OpenAPIToolChoiceSpecificationDO2DTO(do *entity.ToolChoiceSpecification) *domainopenapi.ToolChoiceSpecification {
	if do == nil {
		return nil
	}
	return &domainopenapi.ToolChoiceSpecification{
		Type: ptr.Of(domainopenapi.ToolType(do.Type)),
		Name: ptr.Of(do.Name),
	}
}

func OpenAPIModelConfigDO2DTO(do *entity.ModelConfig) *domainopenapi.LLMConfig {
	if do == nil {
		return nil
	}
	return &domainopenapi.LLMConfig{
		MaxTokens:        do.MaxTokens,
		Temperature:      do.Temperature,
		TopK:             do.TopK,
		TopP:             do.TopP,
		PresencePenalty:  do.PresencePenalty,
		FrequencyPenalty: do.FrequencyPenalty,
		JSONMode:         do.JSONMode,
		Thinking:         OpenAPIThinkingConfigDO2DTO(do.Thinking),
		Extra:            do.Extra,
	}
}

func OpenAPIBatchContentPartDO2DTO(dos []*entity.ContentPart) []*domainopenapi.ContentPart {
	if dos == nil {
		return nil
	}
	parts := make([]*domainopenapi.ContentPart, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		parts = append(parts, OpenAPIContentPartDO2DTO(do))
	}
	return parts
}

func OpenAPIContentPartDO2DTO(do *entity.ContentPart) *domainopenapi.ContentPart {
	if do == nil {
		return nil
	}
	var imageURL *string
	if do.ImageURL != nil {
		imageURL = ptr.Of(do.ImageURL.URL)
	}
	var videoURL *string
	var config *domainopenapi.MediaConfig
	if do.VideoURL != nil {
		if do.VideoURL.URL != "" {
			videoURL = ptr.Of(do.VideoURL.URL)
		}
	}
	// Set Config with fps if available
	if do.MediaConfig != nil && do.MediaConfig.Fps != nil {
		config = &domainopenapi.MediaConfig{
			Fps: do.MediaConfig.Fps,
		}
	}
	return &domainopenapi.ContentPart{
		Type:       ptr.Of(OpenAPIContentTypeDO2DTO(do.Type)),
		Text:       do.Text,
		ImageURL:   imageURL,
		VideoURL:   videoURL,
		Base64Data: do.Base64Data,
		Config:     config,
		Signature:  do.Signature,
	}
}

func OpenAPIContentTypeDO2DTO(do entity.ContentType) domainopenapi.ContentType {
	switch do {
	case entity.ContentTypeText:
		return domainopenapi.ContentTypeText
	case entity.ContentTypeImageURL:
		return domainopenapi.ContentTypeImageURL
	case entity.ContentTypeVideoURL:
		return domainopenapi.ContentTypeVideoURL
	case entity.ContentTypeBase64Data:
		return domainopenapi.ContentTypeBase64Data
	case entity.ContentTypeMultiPartVariable:
		return domainopenapi.ContentTypeMultiPartVariable
	default:
		return domainopenapi.ContentTypeText
	}
}

// OpenAPIBatchMessageDTO2DO 将openapi Message转换为entity Message
func OpenAPIBatchMessageDTO2DO(dtos []*domainopenapi.Message) []*entity.Message {
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
func OpenAPIMessageDTO2DO(dto *domainopenapi.Message) *entity.Message {
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
		Signature:        dto.Signature,
		Metadata:         dto.Metadata,
	}
}

// OpenAPIBatchContentPartDTO2DO 将openapi ContentPart转换为entity ContentPart
func OpenAPIBatchContentPartDTO2DO(dtos []*domainopenapi.ContentPart) []*entity.ContentPart {
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
func OpenAPIContentPartDTO2DO(dto *domainopenapi.ContentPart) *entity.ContentPart {
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
		Signature:   dto.Signature,
	}
}

// OpenAPIContentTypeDTO2DO 将openapi ContentType转换为entity ContentType
func OpenAPIContentTypeDTO2DO(dto domainopenapi.ContentType) entity.ContentType {
	switch dto {
	case domainopenapi.ContentTypeText:
		return entity.ContentTypeText
	case domainopenapi.ContentTypeImageURL:
		return entity.ContentTypeImageURL
	case domainopenapi.ContentTypeVideoURL:
		return entity.ContentTypeVideoURL
	case domainopenapi.ContentTypeBase64Data:
		return entity.ContentTypeBase64Data
	case domainopenapi.ContentTypeMultiPartVariable:
		return entity.ContentTypeMultiPartVariable
	default:
		return entity.ContentTypeText
	}
}

// OpenAPIBatchVariableValDTO2DO 将openapi VariableVal转换为entity VariableVal
func OpenAPIBatchVariableValDTO2DO(dtos []*domainopenapi.VariableVal) []*entity.VariableVal {
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
func OpenAPIVariableValDTO2DO(dto *domainopenapi.VariableVal) *entity.VariableVal {
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
func OpenAPITokenUsageDO2DTO(do *entity.TokenUsage) *domainopenapi.TokenUsage {
	if do == nil {
		return nil
	}
	return &domainopenapi.TokenUsage{
		InputTokens:  ptr.Of(int32(do.InputTokens)),
		OutputTokens: ptr.Of(int32(do.OutputTokens)),
	}
}

// OpenAPIBatchToolCallDO2DTO 将entity ToolCall转换为openapi ToolCall
func OpenAPIBatchToolCallDO2DTO(dos []*entity.ToolCall) []*domainopenapi.ToolCall {
	if dos == nil {
		return nil
	}
	toolCalls := make([]*domainopenapi.ToolCall, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		toolCalls = append(toolCalls, OpenAPIToolCallDO2DTO(do))
	}
	return toolCalls
}

// OpenAPIToolCallDO2DTO 将entity ToolCall转换为openapi ToolCall
func OpenAPIToolCallDO2DTO(do *entity.ToolCall) *domainopenapi.ToolCall {
	if do == nil {
		return nil
	}
	return &domainopenapi.ToolCall{
		Index:        ptr.Of(int32(do.Index)),
		ID:           ptr.Of(do.ID),
		Type:         ptr.Of(OpenAPIToolTypeDO2DTO(do.Type)),
		FunctionCall: OpenAPIFunctionCallDO2DTO(do.FunctionCall),
		Signature:    do.Signature,
	}
}

// OpenAPIToolTypeDO2DTO 将entity ToolType转换为openapi ToolType
func OpenAPIToolTypeDO2DTO(do entity.ToolType) domainopenapi.ToolType {
	switch do {
	case entity.ToolTypeFunction:
		return domainopenapi.ToolTypeFunction
	case entity.ToolTypeGoogleSearch:
		return domainopenapi.ToolTypeGoogleSearch
	default:
		return domainopenapi.ToolTypeFunction
	}
}

// OpenAPIFunctionCallDO2DTO 将entity FunctionCall转换为openapi FunctionCall
func OpenAPIFunctionCallDO2DTO(do *entity.FunctionCall) *domainopenapi.FunctionCall {
	if do == nil {
		return nil
	}
	return &domainopenapi.FunctionCall{
		Name:      ptr.Of(do.Name),
		Arguments: do.Arguments,
	}
}

// OpenAPIBatchToolCallDTO2DO 将openapi ToolCall转换为entity ToolCall
func OpenAPIBatchToolCallDTO2DO(dtos []*domainopenapi.ToolCall) []*entity.ToolCall {
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
func OpenAPIToolCallDTO2DO(dto *domainopenapi.ToolCall) *entity.ToolCall {
	if dto == nil {
		return nil
	}
	return &entity.ToolCall{
		Index:        int64(dto.GetIndex()),
		ID:           dto.GetID(),
		Type:         OpenAPIToolTypeDTO2DO(dto.GetType()),
		FunctionCall: OpenAPIFunctionCallDTO2DO(dto.FunctionCall),
		Signature:    dto.Signature,
	}
}

// OpenAPIToolTypeDTO2DO 将openapi ToolType转换为entity ToolType
func OpenAPIToolTypeDTO2DO(dto domainopenapi.ToolType) entity.ToolType {
	switch dto {
	case domainopenapi.ToolTypeFunction:
		return entity.ToolTypeFunction
	case domainopenapi.ToolTypeGoogleSearch:
		return entity.ToolTypeGoogleSearch
	default:
		return entity.ToolTypeFunction
	}
}

// OpenAPIFunctionCallDTO2DO 将openapi FunctionCall转换为entity FunctionCall
func OpenAPIFunctionCallDTO2DO(dto *domainopenapi.FunctionCall) *entity.FunctionCall {
	if dto == nil {
		return nil
	}
	return &entity.FunctionCall{
		Name:      dto.GetName(),
		Arguments: dto.Arguments,
	}
}

// OpenAPIPromptBasicDO2DTO 将entity Prompt转换为openapi PromptBasic
func OpenAPIPromptBasicDO2DTO(do *entity.Prompt) *domainopenapi.PromptBasic {
	if do == nil || do.PromptBasic == nil {
		return nil
	}
	return &domainopenapi.PromptBasic{
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
func OpenAPIBatchToolDTO2DO(dtos []*domainopenapi.Tool) []*entity.Tool {
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
func OpenAPIToolDTO2DO(dto *domainopenapi.Tool) *entity.Tool {
	if dto == nil {
		return nil
	}
	return &entity.Tool{
		Type:     OpenAPIToolTypeDTO2DO(dto.GetType()),
		Function: OpenAPIFunctionDTO2DO(dto.Function),
	}
}

// OpenAPIFunctionDTO2DO 将openapi Function转换为entity Function
func OpenAPIFunctionDTO2DO(dto *domainopenapi.Function) *entity.Function {
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
func OpenAPIToolCallConfigDTO2DO(dto *domainopenapi.ToolCallConfig) *entity.ToolCallConfig {
	if dto == nil {
		return nil
	}
	return &entity.ToolCallConfig{
		ToolChoice:              OpenAPIToolChoiceTypeDTO2DO(dto.GetToolChoice()),
		ToolChoiceSpecification: OpenAPIToolChoiceSpecificationDTO2DO(dto.ToolChoiceSpecification),
	}
}

// OpenAPIToolChoiceTypeDTO2DO 将openapi ToolChoiceType转换为entity ToolChoiceType
func OpenAPIToolChoiceTypeDTO2DO(dto domainopenapi.ToolChoiceType) entity.ToolChoiceType {
	return entity.ToolChoiceType(dto)
}

// OpenAPIToolChoiceSpecificationDTO2DO 将openapi ToolChoiceSpecification转换为entity ToolChoiceSpecification
func OpenAPIToolChoiceSpecificationDTO2DO(dto *domainopenapi.ToolChoiceSpecification) *entity.ToolChoiceSpecification {
	if dto == nil {
		return nil
	}
	return &entity.ToolChoiceSpecification{
		Type: OpenAPIToolTypeDTO2DO(dto.GetType()),
		Name: dto.GetName(),
	}
}

// OpenAPIModelConfigDTO2DO 将openapi ModelConfig转换为entity ModelConfig
func OpenAPIModelConfigDTO2DO(dto *domainopenapi.ModelConfig) *entity.ModelConfig {
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
		Thinking:          OpenAPIThinkingConfigDTO2DO(dto.Thinking),
		ParamConfigValues: OpenAPIBatchParamConfigValueDTO2DO(dto.ParamConfigValues),
	}
}

// OpenAPIThinkingConfigDTO2DO 将openapi ThinkingConfig转换为entity ThinkingConfig
func OpenAPIThinkingConfigDTO2DO(dto *domainopenapi.ThinkingConfig) *entity.ThinkingConfig {
	if dto == nil {
		return nil
	}
	return &entity.ThinkingConfig{
		BudgetTokens:    dto.BudgetTokens,
		ThinkingOption:  OpenAPIThinkingOptionDTO2DO(dto.ThinkingOption),
		ReasoningEffort: OpenAPIReasoningEffortDTO2DO(dto.ReasoningEffort),
	}
}

// OpenAPIThinkingOptionDTO2DO 将openapi ThinkingOption转换为entity ThinkingOption
func OpenAPIThinkingOptionDTO2DO(dto *domainopenapi.ThinkingOption) *entity.ThinkingOption {
	if dto == nil {
		return nil
	}
	var result entity.ThinkingOption
	switch *dto {
	case domainopenapi.ThinkingOptionDisabled:
		result = entity.ThinkingOptionDisabled
	case domainopenapi.ThinkingOptionEnabled:
		result = entity.ThinkingOptionEnabled
	case domainopenapi.ThinkingOptionAuto:
		result = entity.ThinkingOptionAuto
	default:
		return nil
	}
	return &result
}

// OpenAPIReasoningEffortDTO2DO 将openapi ReasoningEffort转换为entity ReasoningEffort
func OpenAPIReasoningEffortDTO2DO(dto *domainopenapi.ReasoningEffort) *entity.ReasoningEffort {
	if dto == nil {
		return nil
	}
	var result entity.ReasoningEffort
	switch *dto {
	case domainopenapi.ReasoningEffortMinimal:
		result = entity.ReasoningEffortMinimal
	case domainopenapi.ReasoningEffortLow:
		result = entity.ReasoningEffortLow
	case domainopenapi.ReasoningEffortMedium:
		result = entity.ReasoningEffortMedium
	case domainopenapi.ReasoningEffortHigh:
		result = entity.ReasoningEffortHigh
	default:
		return nil
	}
	return &result
}

// OpenAPIThinkingConfigDO2DTO 将entity ThinkingConfig转换为openapi ThinkingConfig
func OpenAPIThinkingConfigDO2DTO(do *entity.ThinkingConfig) *domainopenapi.ThinkingConfig {
	if do == nil {
		return nil
	}
	return &domainopenapi.ThinkingConfig{
		BudgetTokens:    do.BudgetTokens,
		ThinkingOption:  OpenAPIThinkingOptionDO2DTO(do.ThinkingOption),
		ReasoningEffort: OpenAPIReasoningEffortDO2DTO(do.ReasoningEffort),
	}
}

// OpenAPIThinkingOptionDO2DTO 将entity ThinkingOption转换为openapi ThinkingOption
func OpenAPIThinkingOptionDO2DTO(do *entity.ThinkingOption) *domainopenapi.ThinkingOption {
	if do == nil {
		return nil
	}
	var result domainopenapi.ThinkingOption
	switch *do {
	case entity.ThinkingOptionDisabled:
		result = domainopenapi.ThinkingOptionDisabled
	case entity.ThinkingOptionEnabled:
		result = domainopenapi.ThinkingOptionEnabled
	case entity.ThinkingOptionAuto:
		result = domainopenapi.ThinkingOptionAuto
	default:
		return nil
	}
	return &result
}

// OpenAPIReasoningEffortDO2DTO 将entity ReasoningEffort转换为openapi ReasoningEffort
func OpenAPIReasoningEffortDO2DTO(do *entity.ReasoningEffort) *domainopenapi.ReasoningEffort {
	if do == nil {
		return nil
	}
	var result domainopenapi.ReasoningEffort
	switch *do {
	case entity.ReasoningEffortMinimal:
		result = domainopenapi.ReasoningEffortMinimal
	case entity.ReasoningEffortLow:
		result = domainopenapi.ReasoningEffortLow
	case entity.ReasoningEffortMedium:
		result = domainopenapi.ReasoningEffortMedium
	case entity.ReasoningEffortHigh:
		result = domainopenapi.ReasoningEffortHigh
	default:
		return nil
	}
	return &result
}

// OpenAPIBatchParamConfigValueDTO2DO 将openapi ParamConfigValue转换为entity ParamConfigValue
func OpenAPIBatchParamConfigValueDTO2DO(dtos []*domainopenapi.ParamConfigValue) []*entity.ParamConfigValue {
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

// OpenAPIParamConfigValueDTO2DO 将openapi ParamConfigValue转换为entity ParamConfigValue
func OpenAPIParamConfigValueDTO2DO(dto *domainopenapi.ParamConfigValue) *entity.ParamConfigValue {
	if dto == nil {
		return nil
	}
	return &entity.ParamConfigValue{
		Name:  dto.GetName(),
		Label: dto.GetLabel(),
		Value: OpenAPIParamOptionDTO2DO(dto.Value),
	}
}

// OpenAPIParamOptionDTO2DO 将openapi ParamOption转换为entity ParamOption
func OpenAPIParamOptionDTO2DO(dto *domainopenapi.ParamOption) *entity.ParamOption {
	if dto == nil {
		return nil
	}
	return &entity.ParamOption{
		Value: dto.GetValue(),
		Label: dto.GetLabel(),
	}
}

// OpenAPIResponseAPIConfigDTO2DO 将openapi ResponseAPIConfig转换为entity ResponseAPIConfig
func OpenAPIResponseAPIConfigDTO2DO(dto *domainopenapi.ResponseAPIConfig) *entity.ResponseAPIConfig {
	if dto == nil {
		return nil
	}
	return &entity.ResponseAPIConfig{
		PreviousResponseID: dto.PreviousResponseID,
		EnableCaching:      dto.EnableCaching,
		SessionID:          dto.SessionID,
	}
}
