// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

type LLMCallParam struct {
	SpaceID       int64
	PromptID      int64
	PromptKey     string
	PromptVersion string
	Scenario      entity.Scenario
	UserID        *string

	Messages          []*entity.Message
	Tools             []*entity.Tool
	ToolCallConfig    *entity.ToolCallConfig
	ModelConfig       *entity.ModelConfig
	ResponseAPIConfig *entity.ResponseAPIConfig
}

type LLMStreamingCallParam struct {
	LLMCallParam
	ResultStream chan<- *entity.ReplyItem
}

//go:generate mockgen -destination=mocks/llm_provider.go -package=mocks . ILLMProvider
type ILLMProvider interface {
	Call(ctx context.Context, param LLMCallParam) (*entity.ReplyItem, error)
	StreamingCall(ctx context.Context, param LLMStreamingCallParam) (*entity.ReplyItem, error)
}
