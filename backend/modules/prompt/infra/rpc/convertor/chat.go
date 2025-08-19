// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"strconv"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	runtimedto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func LLMCallParamConvert(param rpc.LLMCallParam) *runtime.ChatRequest {
	return &runtime.ChatRequest{
		ModelConfig: ModelConfigDO2DTO(param.ModelConfig, param.ToolCallConfig),
		Messages:    BatchMessageDO2DTO(param.Messages),
		Tools:       BatchToolDO2DTO(param.Tools),
		BizParam: &runtimedto.BizParam{
			WorkspaceID:           ptr.Of(param.SpaceID),
			UserID:                param.UserID,
			Scenario:              ptr.Of(ScenarioDO2DTO(param.Scenario)),
			ScenarioEntityID:      ptr.Of(strconv.FormatInt(param.PromptID, 10)),
			ScenarioEntityVersion: ptr.Of(param.PromptVersion),
			ScenarioEntityKey:     ptr.Of(param.PromptKey),
		},
	}
}

func ModelConfigDO2DTO(modelConfig *entity.ModelConfig, toolCallConfig *entity.ToolCallConfig) *runtimedto.ModelConfig {
	if modelConfig == nil {
		return nil
	}
	var maxTokens *int64
	if modelConfig.MaxTokens != nil {
		maxTokens = ptr.Of(int64(ptr.From(modelConfig.MaxTokens)))
	}
	var toolChoice *runtimedto.ToolChoice
	if toolCallConfig != nil {
		toolChoice = ptr.Of(ToolChoiceTypeDO2DTO(toolCallConfig.ToolChoice))
	}
	var responseFormat *runtimedto.ResponseFormat
	if modelConfig.JSONMode != nil && ptr.From(modelConfig.JSONMode) {
		responseFormat = &runtimedto.ResponseFormat{
			Type: gptr.Of(runtimedto.ResponseFormatJSONObject),
		}
	}
	return &runtimedto.ModelConfig{
		ModelID:          modelConfig.ModelID,
		Temperature:      modelConfig.Temperature,
		MaxTokens:        maxTokens,
		TopP:             modelConfig.TopP,
		ToolChoice:       toolChoice,
		ResponseFormat:   responseFormat,
		TopK:             modelConfig.TopK,
		PresencePenalty:  modelConfig.PresencePenalty,
		FrequencyPenalty: modelConfig.FrequencyPenalty,
	}
}

func ToolChoiceTypeDO2DTO(do entity.ToolChoiceType) runtimedto.ToolChoice {
	switch do {
	case entity.ToolChoiceTypeNone:
		return runtimedto.ToolChoiceNone
	case entity.ToolChoiceTypeAuto:
		return runtimedto.ToolChoiceAuto
	default:
		return runtimedto.ToolChoiceAuto
	}
}

func BatchMessageDO2DTO(dos []*entity.Message) []*runtimedto.Message {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.Message, 0, len(dos))
	for _, message := range dos {
		res = append(res, MessageDO2DTO(message))
	}
	return res
}

func MessageDO2DTO(do *entity.Message) *runtimedto.Message {
	if do == nil {
		return nil
	}
	return &runtimedto.Message{
		Role:               RoleDO2DTO(do.Role),
		Content:            do.Content,
		MultimodalContents: BatchContentPartDO2DTO(do.Parts),
		ToolCalls:          BatchToolCallDO2DTO(do.ToolCalls),
		ToolCallID:         do.ToolCallID,
		ResponseMeta:       nil,
	}
}

func RoleDO2DTO(do entity.Role) runtimedto.Role {
	switch do {
	case entity.RoleSystem:
		return runtimedto.RoleSystem
	case entity.RoleUser:
		return runtimedto.RoleUser
	case entity.RoleAssistant:
		return runtimedto.RoleAssistant
	case entity.RoleTool:
		return runtimedto.RoleTool
	default:
		return runtimedto.RoleUser
	}
}

func BatchContentPartDO2DTO(dos []*entity.ContentPart) []*runtimedto.ChatMessagePart {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.ChatMessagePart, 0, len(dos))
	for _, part := range dos {
		res = append(res, ContentPartDO2DTO(part))
	}
	return res
}

func ContentPartDO2DTO(do *entity.ContentPart) *runtimedto.ChatMessagePart {
	if do == nil {
		return nil
	}
	return &runtimedto.ChatMessagePart{
		Type:     ptr.Of(ContentTypeDO2DTO(do.Type)),
		Text:     do.Text,
		ImageURL: ImageURLDO2DTO(do.ImageURL),
	}
}

func ContentTypeDO2DTO(do entity.ContentType) runtimedto.ChatMessagePartType {
	switch do {
	case entity.ContentTypeText:
		return runtimedto.ChatMessagePartTypeText
	case entity.ContentTypeImageURL:
		return runtimedto.ChatMessagePartTypeImageURL
	default:
		return runtimedto.ChatMessagePartTypeText
	}
}

func ImageURLDO2DTO(do *entity.ImageURL) *runtimedto.ChatMessageImageURL {
	if do == nil {
		return nil
	}
	return &runtimedto.ChatMessageImageURL{
		URL: ptr.Of(do.URL),
	}
}

func BatchToolCallDO2DTO(dos []*entity.ToolCall) []*runtimedto.ToolCall {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.ToolCall, 0, len(dos))
	for _, toolCall := range dos {
		res = append(res, ToolCallDO2DTO(toolCall))
	}
	return res
}

func ToolCallDO2DTO(do *entity.ToolCall) *runtimedto.ToolCall {
	if do == nil {
		return nil
	}
	return &runtimedto.ToolCall{
		Index:        ptr.Of(do.Index),
		ID:           ptr.Of(do.ID),
		Type:         ptr.Of(ToolTypeDO2DTO(do.Type)),
		FunctionCall: FunctionDO2DTO(do.FunctionCall),
	}
}

func ToolTypeDO2DTO(do entity.ToolType) runtimedto.ToolType {
	switch do {
	default:
		return runtimedto.ToolTypeFunction
	}
}

func FunctionDO2DTO(do *entity.FunctionCall) *runtimedto.FunctionCall {
	if do == nil {
		return nil
	}
	return &runtimedto.FunctionCall{
		Name:      ptr.Of(do.Name),
		Arguments: do.Arguments,
	}
}

func BatchToolDO2DTO(dos []*entity.Tool) []*runtimedto.Tool {
	if len(dos) == 0 {
		return nil
	}
	res := make([]*runtimedto.Tool, 0, len(dos))
	for _, tool := range dos {
		res = append(res, ToolDO2DTO(tool))
	}
	return res
}

func ToolDO2DTO(do *entity.Tool) *runtimedto.Tool {
	if do == nil || do.Function == nil {
		return nil
	}
	return &runtimedto.Tool{
		Name:    ptr.Of(do.Function.Name),
		Desc:    ptr.Of(do.Function.Description),
		DefType: ptr.Of(runtimedto.ToolDefTypeOpenAPIV3),
		Def:     ptr.Of(do.Function.Parameters),
	}
}

func ScenarioDO2DTO(do entity.Scenario) common.Scenario {
	switch do {
	case entity.ScenarioPromptDebug:
		return common.ScenarioPromptDebug
	case entity.ScenarioEvalTarget:
		return common.ScenarioEvalTarget
	default:
		return common.ScenarioDefault
	}
}

//========================================================

func ReplyItemDTO2DO(dto *runtimedto.Message) *entity.ReplyItem {
	if dto == nil {
		return nil
	}
	var finishReason string
	var tokenUsage *entity.TokenUsage
	if dto.ResponseMeta != nil {
		finishReason = ptr.From(dto.ResponseMeta.FinishReason)
		tokenUsage = TokenUsageDTO2DO(dto.ResponseMeta.Usage)
	}
	return &entity.ReplyItem{
		Message:      MessageDTO2DO(dto),
		FinishReason: finishReason,
		TokenUsage:   tokenUsage,
	}
}

func MessageDTO2DO(dto *runtimedto.Message) *entity.Message {
	if dto == nil {
		return nil
	}
	return &entity.Message{
		Role:             RoleDTO2DO(dto.Role),
		ReasoningContent: dto.ReasoningContent,
		Content:          dto.Content,
		Parts:            BatchMultimodalContentDTO2DO(dto.MultimodalContents),
		ToolCallID:       dto.ToolCallID,
		ToolCalls:        BatchToolCallDTO2DO(dto.ToolCalls),
	}
}

func RoleDTO2DO(dto runtimedto.Role) entity.Role {
	switch dto {
	case runtimedto.RoleSystem:
		return entity.RoleSystem
	case runtimedto.RoleUser:
		return entity.RoleUser
	case runtimedto.RoleAssistant:
		return entity.RoleAssistant
	case runtimedto.RoleTool:
		return entity.RoleTool
	default:
		return entity.RoleAssistant
	}
}

func BatchMultimodalContentDTO2DO(dtos []*runtimedto.ChatMessagePart) []*entity.ContentPart {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.ContentPart, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, MultimodalContentDTO2DO(dto))
	}
	return res
}

func MultimodalContentDTO2DO(dto *runtimedto.ChatMessagePart) *entity.ContentPart {
	if dto == nil {
		return nil
	}
	return &entity.ContentPart{
		Type:     ContentTypeDTO2DO(dto.GetType()),
		Text:     dto.Text,
		ImageURL: ImageURLDTO2DO(dto.ImageURL),
	}
}

func ContentTypeDTO2DO(dto runtimedto.ChatMessagePartType) entity.ContentType {
	switch dto {
	case runtimedto.ChatMessagePartTypeText:
		return entity.ContentTypeText
	case runtimedto.ChatMessagePartTypeImageURL:
		return entity.ContentTypeImageURL
	default:
		return entity.ContentTypeText
	}
}

func ImageURLDTO2DO(dto *runtimedto.ChatMessageImageURL) *entity.ImageURL {
	if dto == nil {
		return nil
	}
	return &entity.ImageURL{
		URL: ptr.From(dto.URL),
	}
}

func BatchToolCallDTO2DO(dtos []*runtimedto.ToolCall) []*entity.ToolCall {
	if len(dtos) == 0 {
		return nil
	}
	res := make([]*entity.ToolCall, 0, len(dtos))
	for _, dto := range dtos {
		res = append(res, ToolCallDTO2DO(dto))
	}
	return res
}

func ToolCallDTO2DO(dto *runtimedto.ToolCall) *entity.ToolCall {
	if dto == nil {
		return nil
	}
	return &entity.ToolCall{
		Index:        ptr.From(dto.Index),
		ID:           ptr.From(dto.ID),
		Type:         ToolTypeDTO2DO(ptr.From(dto.Type)),
		FunctionCall: FunctionDTO2DO(dto.FunctionCall),
	}
}

func ToolTypeDTO2DO(dto runtimedto.ToolType) entity.ToolType {
	switch dto {
	default:
		return entity.ToolTypeFunction
	}
}

func FunctionDTO2DO(dto *runtimedto.FunctionCall) *entity.FunctionCall {
	if dto == nil {
		return nil
	}
	return &entity.FunctionCall{
		Name:      ptr.From(dto.Name),
		Arguments: dto.Arguments,
	}
}

func TokenUsageDTO2DO(dto *runtimedto.TokenUsage) *entity.TokenUsage {
	if dto == nil {
		return nil
	}
	return &entity.TokenUsage{
		InputTokens:  ptr.From(dto.PromptTokens),
		OutputTokens: ptr.From(dto.CompletionTokens),
	}
}
