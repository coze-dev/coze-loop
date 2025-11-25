// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"
	"github.com/vincent-petithory/dataurl"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/manage"
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
	// llm暂时不支持toolCallConfig
	//if toolCallConfig != nil {
	//	toolChoice = ptr.Of(ToolChoiceTypeDO2DTO(toolCallConfig.ToolChoice))
	//}
	var responseFormat *runtimedto.ResponseFormat
	if modelConfig.JSONMode != nil && ptr.From(modelConfig.JSONMode) {
		responseFormat = &runtimedto.ResponseFormat{
			Type: gptr.Of(runtimedto.ResponseFormatJSONObject),
		}
	}
	return &runtimedto.ModelConfig{
		ModelID:           modelConfig.ModelID,
		Temperature:       modelConfig.Temperature,
		MaxTokens:         maxTokens,
		TopP:              modelConfig.TopP,
		ToolChoice:        toolChoice,
		ResponseFormat:    responseFormat,
		TopK:              modelConfig.TopK,
		PresencePenalty:   modelConfig.PresencePenalty,
		FrequencyPenalty:  modelConfig.FrequencyPenalty,
		ParamConfigValues: BatchParamConfigValueDO2DTO(modelConfig.ParamConfigValues),
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
	part := &runtimedto.ChatMessagePart{
		Type: ptr.Of(ContentTypeDO2DTO(do.Type, do.Base64Data)),
		Text: do.Text,
	}
	switch do.Type {
	case entity.ContentTypeImageURL:
		part.ImageURL = ImageURLDO2DTO(do.ImageURL)
	case entity.ContentTypeVideoURL:
		part.VideoURL = VideoURLDO2DTO(do.VideoURL, do.MediaConfig)
	case entity.ContentTypeBase64Data:
		imageURL, videoURL := base64DataToMedia(do)
		if videoURL != nil {
			part.Type = ptr.Of(runtimedto.ChatMessagePartTypeVideoURL)
			part.VideoURL = videoURL
		} else if imageURL != nil {
			part.Type = ptr.Of(runtimedto.ChatMessagePartTypeImageURL)
			part.ImageURL = imageURL
		}
	}
	return part
}

func ContentTypeDO2DTO(contentType entity.ContentType, base64Data *string) runtimedto.ChatMessagePartType {
	switch contentType {
	case entity.ContentTypeText:
		return runtimedto.ChatMessagePartTypeText
	case entity.ContentTypeImageURL:
		return runtimedto.ChatMessagePartTypeImageURL
	case entity.ContentTypeVideoURL:
		return runtimedto.ChatMessagePartTypeVideoURL
	case entity.ContentTypeBase64Data:
		imageURL, videoURL := base64DataToMedia(&entity.ContentPart{Base64Data: base64Data})
		if videoURL != nil {
			return runtimedto.ChatMessagePartTypeVideoURL
		}
		if imageURL != nil {
			return runtimedto.ChatMessagePartTypeImageURL
		}
		return runtimedto.ChatMessagePartTypeImageURL
	default:
		return runtimedto.ChatMessagePartTypeText
	}
}

func ImageURLDO2DTO(url *entity.ImageURL) *runtimedto.ChatMessageImageURL {
	if url == nil {
		return nil
	}
	return &runtimedto.ChatMessageImageURL{
		URL: ptr.Of(url.URL),
	}
}

func VideoURLDO2DTO(url *entity.VideoURL, mediaConfig *entity.MediaConfig) *runtimedto.ChatMessageVideoURL {
	if url == nil {
		return nil
	}
	var detail *runtimedto.VideoURLDetail
	if mediaConfig != nil && mediaConfig.Fps != nil {
		detail = &runtimedto.VideoURLDetail{
			Fps: mediaConfig.Fps,
		}
	}
	return &runtimedto.ChatMessageVideoURL{
		URL:    ptr.Of(url.URL),
		Detail: detail,
	}
}

func base64DataToMedia(part *entity.ContentPart) (*runtimedto.ChatMessageImageURL, *runtimedto.ChatMessageVideoURL) {
	if part == nil || part.Base64Data == nil || ptr.From(part.Base64Data) == "" {
		return nil, nil
	}
	dataURL, _ := dataurl.DecodeString(ptr.From(part.Base64Data))
	if dataURL == nil {
		return nil, nil
	}
	mimeType := dataURL.ContentType()
	if strings.HasPrefix(mimeType, runtimedto.MimePrefixImage) {
		return &runtimedto.ChatMessageImageURL{
			URL:      part.Base64Data,
			MimeType: ptr.Of(mimeType),
		}, nil
	}
	if strings.HasPrefix(mimeType, runtimedto.MimePrefixVideo) {
		videoURL := &runtimedto.ChatMessageVideoURL{
			URL:      part.Base64Data,
			MimeType: ptr.Of(mimeType),
		}
		// Preserve fps from MediaConfig if available
		if part.MediaConfig != nil && part.MediaConfig.Fps != nil {
			videoURL.Detail = &runtimedto.VideoURLDetail{
				Fps: part.MediaConfig.Fps,
			}
		}
		return nil, videoURL
	}
	return nil, nil
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
	case entity.ScenarioPTaaS:
		return common.ScenarioPromptAsAService
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
	videoURL, mediaConfig := VideoURLDTO2DO(dto.VideoURL)
	return &entity.ContentPart{
		Type:        ContentTypeDTO2DO(dto.GetType()),
		Text:        dto.Text,
		ImageURL:    ImageURLDTO2DO(dto.ImageURL),
		VideoURL:    videoURL,
		MediaConfig: mediaConfig,
	}
}

func ContentTypeDTO2DO(dto runtimedto.ChatMessagePartType) entity.ContentType {
	switch dto {
	case runtimedto.ChatMessagePartTypeText:
		return entity.ContentTypeText
	case runtimedto.ChatMessagePartTypeImageURL:
		return entity.ContentTypeImageURL
	case runtimedto.ChatMessagePartTypeVideoURL:
		return entity.ContentTypeVideoURL
	default:
		return entity.ContentTypeText
	}
}

func ImageURLDTO2DO(dto *runtimedto.ChatMessageImageURL) *entity.ImageURL {
	if dto == nil {
		return nil
	}
	url := ptr.From(dto.URL)
	// If mimetype is provided and URL is base64 string, convert to dataurl format
	if dto.MimeType != nil && ptr.From(dto.MimeType) != "" && !strings.HasPrefix(url, "data:") {
		url = fmt.Sprintf("data:%s;base64,%s", ptr.From(dto.MimeType), url)
	}
	return &entity.ImageURL{
		URL: url,
	}
}

func VideoURLDTO2DO(dto *runtimedto.ChatMessageVideoURL) (*entity.VideoURL, *entity.MediaConfig) {
	if dto == nil {
		return nil, nil
	}
	var mediaConfig *entity.MediaConfig
	if dto.Detail != nil && dto.Detail.Fps != nil {
		mediaConfig = &entity.MediaConfig{
			Fps: dto.Detail.Fps,
		}
	}
	url := ptr.From(dto.URL)
	// If mimetype is provided and URL is base64 string, convert to dataurl format
	if dto.MimeType != nil && ptr.From(dto.MimeType) != "" && !strings.HasPrefix(url, "data:") {
		url = fmt.Sprintf("data:%s;base64,%s", ptr.From(dto.MimeType), url)
	}
	return &entity.VideoURL{
		URL: url,
	}, mediaConfig
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

func BatchParamConfigValueDO2DTO(dos []*entity.ParamConfigValue) []*runtimedto.ParamConfigValue {
	if dos == nil {
		return nil
	}
	result := make([]*runtimedto.ParamConfigValue, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		result = append(result, ParamConfigValueDO2DTO(do))
	}
	return result
}

func ParamConfigValueDO2DTO(do *entity.ParamConfigValue) *runtimedto.ParamConfigValue {
	if do == nil {
		return nil
	}
	return &runtimedto.ParamConfigValue{
		Name:  ptr.Of(do.Name),
		Label: ptr.Of(do.Label),
		Value: ParamOptionDO2DTO(do.Value),
	}
}

func ParamOptionDO2DTO(do *entity.ParamOption) *manage.ParamOption {
	if do == nil {
		return nil
	}
	return &manage.ParamOption{
		Value: ptr.Of(do.Value),
		Label: ptr.Of(do.Label),
	}
}
