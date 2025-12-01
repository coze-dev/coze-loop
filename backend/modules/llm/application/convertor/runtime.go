// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	druntime "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

func MessagesDTO2DO(dtos []*druntime.Message) (dos []*entity.Message) {
	return slices.Transform(dtos, func(dto *druntime.Message, _ int) *entity.Message {
		return MessageDTO2DO(dto)
	})
}

func MessageDTO2DO(dto *druntime.Message) (do *entity.Message) {
	if dto == nil {
		return nil
	}
	return &entity.Message{
		Role:              entity.Role(dto.GetRole()),
		Content:           dto.GetContent(),
		ReasoningContent:  dto.GetReasoningContent(),
		MultiModalContent: ChatMessagePartsDTO2DO(dto.GetMultimodalContents()),
		Name:              "",
		ToolCalls:         ToolCallsDTO2DO(dto.GetToolCalls()),
		ToolCallID:        dto.GetToolCallID(),
		ResponseMeta:      ResponseMetaDTO2DO(dto.GetResponseMeta()),
	}
}

func ResponseMetaDTO2DO(dto *druntime.ResponseMeta) (do *entity.ResponseMeta) {
	if dto == nil {
		return nil
	}
	return &entity.ResponseMeta{
		FinishReason: dto.GetFinishReason(),
		Usage:        TokenUsageDTO2DO(dto.GetUsage()),
	}
}

func TokenUsageDTO2DO(dto *druntime.TokenUsage) (do *entity.TokenUsage) {
	if dto == nil {
		return nil
	}
	return &entity.TokenUsage{
		PromptTokens:     int(dto.GetPromptTokens()),
		CompletionTokens: int(dto.GetCompletionTokens()),
		TotalTokens:      int(dto.GetTotalTokens()),
	}
}

func ToolCallsDTO2DO(dtos []*druntime.ToolCall) (dos []*entity.ToolCall) {
	return slices.Transform(dtos, func(dto *druntime.ToolCall, _ int) *entity.ToolCall {
		return ToolCallDTO2DO(dto)
	})
}

func ToolCallDTO2DO(dto *druntime.ToolCall) (do *entity.ToolCall) {
	if dto == nil {
		return nil
	}
	return &entity.ToolCall{
		Index:    dto.Index,
		ID:       dto.GetID(),
		Type:     dto.GetType(),
		Function: FunctionCallDTO2DO(dto.GetFunctionCall()),
		Extra:    nil,
	}
}

func FunctionCallDTO2DO(dto *druntime.FunctionCall) (do *entity.FunctionCall) {
	if dto == nil {
		return nil
	}
	return &entity.FunctionCall{
		Name:      dto.GetName(),
		Arguments: dto.GetArguments(),
	}
}

func ChatMessagePartsDTO2DO(dtos []*druntime.ChatMessagePart) (dos []*entity.ChatMessagePart) {
	return slices.Transform(dtos, func(dto *druntime.ChatMessagePart, _ int) *entity.ChatMessagePart {
		return ChatMessagePartDTO2DO(dto)
	})
}

func ChatMessagePartDTO2DO(dto *druntime.ChatMessagePart) (do *entity.ChatMessagePart) {
	if dto == nil {
		return nil
	}
	return &entity.ChatMessagePart{
		Type:     entity.ChatMessagePartType(dto.GetType()),
		Text:     dto.GetText(),
		ImageURL: ChatMessageImageURLDTO2DO(dto.GetImageURL()),
		VideoURL: ChatMessageVideoURLDTO2DO(dto.GetVideoURL()),
	}
}

func ChatMessageImageURLDTO2DO(dto *druntime.ChatMessageImageURL) (do *entity.ChatMessageImageURL) {
	if dto == nil {
		return nil
	}
	return &entity.ChatMessageImageURL{
		URL:      dto.GetURL(),
		Detail:   entity.ImageURLDetail(dto.GetDetail()),
		MIMEType: dto.GetMimeType(),
	}
}

func ChatMessageVideoURLDTO2DO(dto *druntime.ChatMessageVideoURL) (do *entity.ChatMessageVideoURL) {
	if dto == nil {
		return nil
	}
	return &entity.ChatMessageVideoURL{
		URL:      dto.GetURL(),
		MIMEType: dto.GetMimeType(),
	}
}

func MessageDO2DTO(do *entity.Message) (dto *druntime.Message) {
	if do == nil {
		return nil
	}
	return &druntime.Message{
		Role:               druntime.Role(do.Role),
		Content:            ptr.Of(do.Content),
		ReasoningContent:   ptr.Of(do.ReasoningContent),
		MultimodalContents: ChatMessagePartsDO2DTO(do.MultiModalContent),
		ToolCalls:          ToolCallsDO2DTO(do.ToolCalls),
		ToolCallID:         ptr.Of(do.ToolCallID),
		ResponseMeta:       ResponseMetaDO2DTO(do.ResponseMeta),
	}
}

func ResponseMetaDO2DTO(do *entity.ResponseMeta) (dto *druntime.ResponseMeta) {
	if do == nil {
		return nil
	}
	return &druntime.ResponseMeta{
		FinishReason: ptr.Of(do.FinishReason),
		Usage:        TokenUsageDO2DTO(do.Usage),
	}
}

func TokenUsageDO2DTO(do *entity.TokenUsage) (dto *druntime.TokenUsage) {
	if do == nil {
		return nil
	}
	return &druntime.TokenUsage{
		PromptTokens:     ptr.Of(int64(do.PromptTokens)),
		CompletionTokens: ptr.Of(int64(do.CompletionTokens)),
		TotalTokens:      ptr.Of(int64(do.TotalTokens)),
	}
}

func ToolCallsDO2DTO(dos []*entity.ToolCall) (dtos []*druntime.ToolCall) {
	return slices.Transform(dos, func(do *entity.ToolCall, _ int) *druntime.ToolCall {
		return ToolCallDO2DTO(do)
	})
}

func ToolCallDO2DTO(do *entity.ToolCall) (dto *druntime.ToolCall) {
	if do == nil {
		return nil
	}
	return &druntime.ToolCall{
		Index:        do.Index,
		ID:           ptr.Of(do.ID),
		Type:         ptr.Of(do.Type),
		FunctionCall: FunctionCallDO2DTO(do.Function),
	}
}

func FunctionCallDO2DTO(do *entity.FunctionCall) (dto *druntime.FunctionCall) {
	if do == nil {
		return nil
	}
	return &druntime.FunctionCall{
		Name:      ptr.Of(do.Name),
		Arguments: ptr.Of(do.Arguments),
	}
}

func ChatMessagePartsDO2DTO(dos []*entity.ChatMessagePart) (dtos []*druntime.ChatMessagePart) {
	return slices.Transform(dos, func(do *entity.ChatMessagePart, _ int) *druntime.ChatMessagePart {
		return ChatMessagePartDO2DTO(do)
	})
}

func ChatMessagePartDO2DTO(do *entity.ChatMessagePart) (dto *druntime.ChatMessagePart) {
	if do == nil {
		return nil
	}
	return &druntime.ChatMessagePart{
		Type:     ptr.Of(druntime.ChatMessagePartType(do.Type)),
		Text:     ptr.Of(do.Text),
		ImageURL: ChatMessageImageURLDO2DTO(do.ImageURL),
		VideoURL: ChatMessageVideoURLDO2DTO(do.VideoURL),
	}
}

func ChatMessageImageURLDO2DTO(do *entity.ChatMessageImageURL) (dto *druntime.ChatMessageImageURL) {
	if do == nil {
		return nil
	}
	return &druntime.ChatMessageImageURL{
		URL:      ptr.Of(do.URL),
		Detail:   ptr.Of(druntime.ImageURLDetail(do.Detail)),
		MimeType: ptr.Of(do.MIMEType),
	}
}

func ChatMessageVideoURLDO2DTO(do *entity.ChatMessageVideoURL) (dto *druntime.ChatMessageVideoURL) {
	if do == nil {
		return nil
	}
	return &druntime.ChatMessageVideoURL{
		URL:      ptr.Of(do.URL),
		MimeType: ptr.Of(do.MIMEType),
	}
}
