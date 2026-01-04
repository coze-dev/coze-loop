// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracer

import (
	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	commonentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ChatMessagePartType string

const (
	ChatMessagePartTypeText        ChatMessagePartType = "text"
	ChatMessagePartTypeImageBinary ChatMessagePartType = "image_binary"
	ChatMessagePartTypeImageURL    ChatMessagePartType = "image_url"
)

func ConvertPrompt2Ob(originMessages []*commonentity.Message, variables []*tracespec.PromptArgument) *tracespec.PromptInput {
	return &tracespec.PromptInput{
		Templates: gslice.Map(originMessages, ConvertMsg2Ob),
		Arguments: variables,
	}
}

func ConvertModel2Ob(originMessages []*commonentity.Message, tools []*commonentity.Tool) (tags map[string]any) {
	msgsOb := gslice.Map(originMessages, ConvertMsg2Ob)
	toolsOb := gslice.Map(tools, ConvertTool2Ob)
	modelInput := &tracespec.ModelInput{
		Messages: msgsOb,
		Tools:    toolsOb,
	}
	tags = make(map[string]any)
	tags[tracespec.Input] = Convert2TraceString(modelInput)
	return tags
}

func ConvertTool2Ob(originTool *commonentity.Tool) (obTool *tracespec.ModelTool) {
	if originTool == nil {
		return nil
	}
	obTool = &tracespec.ModelTool{
		Type: "function",
		Function: &tracespec.ModelToolFunction{
			Name:        originTool.Function.Name,
			Parameters:  []byte(originTool.Function.Parameters),
			Description: originTool.Function.Description,
		},
	}
	return obTool
}

func ConvertMsg2Ob(msg *commonentity.Message) (obMsg *tracespec.ModelMessage) {
	if msg == nil {
		return nil
	}
	obMsg = &tracespec.ModelMessage{
		Role:      ConvertPromptMessageType2String(msg.Role),
		Content:   gptr.Indirect(msg.Content.Text),
		Parts:     make([]*tracespec.ModelMessagePart, 0),
		Name:      "",
		ToolCalls: make([]*tracespec.ModelToolCall, 0),
	}
	for _, part := range msg.Content.MultiPart {
		obMsg.Parts = append(obMsg.Parts, ConvertContent2Ob(part))
	}

	return obMsg
}

func ConvertContent2Ob(content *commonentity.Content) *tracespec.ModelMessagePart {
	var contentType string
	switch gptr.Indirect(content.ContentType) {
	case commonentity.ContentTypeText:
		contentType = string(ChatMessagePartTypeText)
	case commonentity.ContentTypeImage:
		contentType = string(ChatMessagePartTypeImageURL)
	case commonentity.ContentTypeMultipartVariable:
		contentType = string(commonentity.ContentTypeMultipartVariable)
	default:
		contentType = string(ChatMessagePartTypeText)
	}
	part := &tracespec.ModelMessagePart{
		Type: tracespec.ModelMessagePartType(contentType),
		Text: gptr.Indirect(content.Text),
	}
	if content.Image != nil {
		part.ImageURL = &tracespec.ModelImageURL{
			Name:   gptr.Indirect(content.Image.Name),
			URL:    gptr.Indirect(content.Image.URL),
			Detail: "",
		}
	}

	return part
}

func ConvertPromptMessageType2String(messageType commonentity.Role) string {
	switch messageType {
	case commonentity.RoleSystem:
		return tracespec.VRoleSystem
	case commonentity.RoleUser:
		return tracespec.VRoleUser
	case commonentity.RoleAssistant:
		return tracespec.VRoleAssistant
	case commonentity.RoleTool:
		return tracespec.VRoleTool
	}
	return tracespec.VRoleSystem
}

func ConvertEvaluatorToolCall2Ob(evaluatorToolCall *commonentity.Tool) (toolCall *tracespec.ModelToolCall) {
	toolCall = &tracespec.ModelToolCall{
		Type: "function",
		Function: &tracespec.ModelToolCallFunction{
			Name:      evaluatorToolCall.Function.Name,
			Arguments: evaluatorToolCall.Function.Parameters,
		},
	}

	return toolCall
}

func Convert2TraceString(input any) string {
	str, err := sonic.MarshalString(input)
	if err != nil {
		return ""
	}

	return str
}

func ContentToSpanParts(parts []*commonentity.Content) []*tracespec.ModelMessagePart {
	if parts == nil {
		return nil
	}
	spanParts := make([]*tracespec.ModelMessagePart, 0)
	for _, part := range parts {
		if part == nil {
			continue
		}
		partSpan := &tracespec.ModelMessagePart{}
		switch gptr.Indirect(part.ContentType) {
		case commonentity.ContentTypeText:
			partSpan.Text = gptr.Indirect(part.Text)
			partSpan.Type = tracespec.ModelMessagePartTypeText
		case commonentity.ContentTypeImage:
			partSpan.Type = tracespec.ModelMessagePartTypeImage
			if part.Image != nil {
				partSpan.ImageURL = &tracespec.ModelImageURL{
					URL:  gptr.Indirect(part.Image.URL),
					Name: gptr.Indirect(part.Image.Name),
				}
			}
		}
		spanParts = append(spanParts, partSpan)
	}
	return spanParts
}

func ConvertContentDO2DTO(content *entity.Content) *common.Content {
	var result *common.Content
	if content == nil {
		return result
	}
	var multiPart []*common.Content
	if content.MultiPart != nil {
		for _, part := range content.MultiPart {
			multiPart = append(multiPart, ConvertContentDO2DTO(part))
		}
	}
	result = &common.Content{
		ContentType: entity.CommonContentTypeDO2DTO(content.GetContentType()),
		Text:        gptr.Of(content.GetText()),
		Image: &common.Image{
			Name: gptr.Of(content.GetImage().GetName()),
			URL:  gptr.Of(content.GetImage().GetUrl()),
		},
		Audio: &common.Audio{
			Name: gptr.Of(content.GetAudio().GetName()),
			URL:  gptr.Of(content.GetAudio().GetUrl()),
		},
		Video: &common.Video{
			Name: gptr.Of(content.GetVideo().GetName()),
			URL:  gptr.Of(content.GetVideo().GetUrl()),
		},
		MultiPart: multiPart,
	}
	return result
}

func ConvertContentTypeDTO2DO(contentType common.ContentType) entity.ContentType {
	switch contentType {
	case common.ContentTypeText:
		return entity.ContentType_Text
	case common.ContentTypeImage:
		return entity.ContentType_Image
	case common.ContentTypeAudio:
		return entity.ContentType_Audio
	case common.ContentTypeVideo:
		return entity.ContentType_Video
	case common.ContentTypeMultiPart:
		return entity.ContentType_MultiPart
	default:
		return entity.ContentType_Text
	}
}
