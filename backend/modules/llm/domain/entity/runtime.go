// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type Message struct {
	Role             Role   `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`

	// if MultiModalContent is not empty, use this instead of Content
	// if MultiModalContent is empty, use Content
	MultiModalContent []*ChatMessagePart `json:"multi_content,omitempty"`

	Name string `json:"name,omitempty"`

	// only for AssistantMessage
	ToolCalls []*ToolCall `json:"tool_calls,omitempty"`

	// only for ToolMessage
	ToolCallID string `json:"tool_call_id,omitempty"`

	ResponseMeta *ResponseMeta `json:"response_meta,omitempty"`
}

func (m *Message) GetInputToken() int {
	if m == nil || m.ResponseMeta == nil || m.ResponseMeta.Usage == nil {
		return 0
	}
	return m.ResponseMeta.Usage.PromptTokens
}

func (m *Message) GetOutputToken() int {
	if m == nil || m.ResponseMeta == nil || m.ResponseMeta.Usage == nil {
		return 0
	}
	return m.ResponseMeta.Usage.CompletionTokens
}

func (m *Message) HasMultiModalContent() bool {
	if m == nil || len(m.MultiModalContent) == 0 {
		return false
	}
	for _, p := range m.MultiModalContent {
		if p.IsMultiModal() {
			return true
		}
	}
	return false
}

func (m *Message) GetImageCountAndMaxSize() (hasUrl, hasBinary bool, cnt int64, maxSizeInByte int64) {
	if !m.HasMultiModalContent() {
		return
	}
	for _, p := range m.MultiModalContent {
		if p.IsURL() {
			hasUrl = true
			cnt++
			continue
		}
		if p.IsBinary() {
			hasBinary = true
			cnt++
			if maxSizeInByte < int64(len(p.ImageURL.URL)*3/4) {
				maxSizeInByte = int64(len(p.ImageURL.URL) * 3 / 4)
			}
		}
	}
	return
}

type Role string

const (
	// Assistant is the role of an assistant, means the message is returned by ChatModel.
	RoleAssistant Role = "assistant"
	// User is the role of a user, means the message is a user message.
	RoleUser Role = "user"
	// System is the role of a system, means the message is a system message.
	RoleSystem Role = "system"
	// Tool is the role of a tool, means the message is a tool call output.
	RoleTool Role = "tool"
)

type ChatMessagePart struct {
	Type     ChatMessagePartType  `json:"type"`
	Text     string               `json:"text"`
	ImageURL *ChatMessageImageURL `json:"image_url"`
}

func (p *ChatMessagePart) IsMultiModal() bool {
	if p == nil {
		return false
	}
	return p.Type != ChatMessagePartTypeText
}

type ChatMessagePartType string

const (
	// ChatMessagePartTypeText means the part is a text.
	ChatMessagePartTypeText ChatMessagePartType = "text"
	// ChatMessagePartTypeImageURL means the part is an image url.
	ChatMessagePartTypeImageURL ChatMessagePartType = "image_url"
	// ChatMessagePartTypeAudioURL means the part is an audio url.
	ChatMessagePartTypeAudioURL ChatMessagePartType = "audio_url"
	// ChatMessagePartTypeVideoURL means the part is a video url.
	ChatMessagePartTypeVideoURL ChatMessagePartType = "video_url"
	// ChatMessagePartTypeFileURL means the part is a file url.
	ChatMessagePartTypeFileURL ChatMessagePartType = "file_url"
)

type ChatMessageImageURL struct {
	// URL can either be a traditional URL or a special URL conforming to RFC-2397 (https://www.rfc-editor.org/rfc/rfc2397).
	// double check with model implementations for detailed instructions on how to use this.
	URL string `json:"url,omitempty"`
	URI string `json:"uri,omitempty"`
	// Detail is the quality of the image url.
	Detail ImageURLDetail `json:"detail,omitempty"`

	// MIMEType is the mime type of the image, eg. "image/png".
	MIMEType string `json:"mime_type,omitempty"`
}

// ImageURLDetail is the detail of the image url.
type ImageURLDetail string

const (
	// ImageURLDetailHigh means the high quality image url.
	ImageURLDetailHigh ImageURLDetail = "high"
	// ImageURLDetailLow means the low quality image url.
	ImageURLDetailLow ImageURLDetail = "low"
	// ImageURLDetailAuto means the auto quality image url.
	ImageURLDetailAuto ImageURLDetail = "auto"
)

type ToolCall struct {
	// Index is used when there are multiple tool calls in a message.
	// In stream mode, it's used to identify the chunk of the tool call for merging.
	Index *int64 `json:"index,omitempty"`
	// ID is the id of the tool call, it can be used to identify the specific tool call.
	ID string `json:"id"`
	// Type is the type of the tool call, default is "function".
	Type string `json:"type"`
	// Function is the function call to be made.
	Function *FunctionCall `json:"function"`

	// Extra is used to store extra information for the tool call.
	Extra map[string]any `json:"extra,omitempty"`
}

// FunctionCall is the function call in a message.
// It's used in Assistant Message.
type FunctionCall struct {
	// Name is the name of the function to call, it can be used to identify the specific function.
	Name string `json:"name,omitempty"`
	// Arguments is the arguments to call the function with, in JSON format.
	Arguments string `json:"arguments,omitempty"`
}

// ResponseMeta collects meta information about a chat response.
type ResponseMeta struct {
	// FinishReason is the reason why the chat response is finished.
	// It's usually "stop", "length", "tool_calls", "content_filter", "null". This is defined by chat model implementation.
	FinishReason string `json:"finish_reason,omitempty"`
	// Usage is the token usage of the chat response, whether usage exists depends on whether the chat model implementation returns.
	Usage *TokenUsage `json:"usage,omitempty"`
}

// TokenUsage Represents the token usage of chat model request.
type TokenUsage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`
	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completion_tokens"`
	// TotalTokens is the total number of tokens in the request.
	TotalTokens int `json:"total_tokens"`
}

type ToolChoice string

const (
	ToolChoiceAuto     ToolChoice = "auto"
	ToolChoiceRequired ToolChoice = "required"
	ToolChoiceNone     ToolChoice = "none"
)

type ToolInfo struct {
	// The unique name of the tool that clearly communicates its purpose.
	Name string
	// Used to tell the model how/when/why to use the tool.
	// You can provide few-shot examples as a part of the description.
	Desc string

	ToolDefType ToolDefType

	Def string
}

type ToolDefType string

const (
	ToolDefTypeOpenAPIV3 ToolDefType = "open_api_v3"
)

type StreamRespParseResult struct {
	RespMsgs          []*Message
	LastRespMsg       *Message
	ReasoningDuration time.Duration
	FirstTokenLatency time.Duration
}

func (p *ChatMessagePart) IsURL() bool {
	if p == nil || p.Type != ChatMessagePartTypeImageURL || p.ImageURL == nil ||
		p.ImageURL.MIMEType != "" {
		return false
	}
	return true
}

func (p *ChatMessagePart) IsBinary() bool {
	if p == nil || p.Type != ChatMessagePartTypeImageURL || p.ImageURL == nil ||
		p.ImageURL.MIMEType == "" {
		return false
	}
	return true
}

type ResponseFormatType string

const (
	ResponseFormatTypeText ResponseFormatType = "text"
	ResponseFormatTypeJSON ResponseFormatType = "json_object"
)

type ResponseFormat struct {
	Type ResponseFormatType `json:"type,omitempty"`
}
