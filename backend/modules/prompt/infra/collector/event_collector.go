// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

//go:generate mockgen -destination=mocks/event_collector.go -package=mocks . ICollectorProvider
type ICollectorProvider interface {
	CollectPromptHubEvent(ctx context.Context, spaceID int64, prompts []*entity.Prompt)
	CollectPTaaSEvent(ctx context.Context, executeLog *ExecuteLog)
}

type ExecuteLog struct {
	SpaceID      int64     `json:"space_id,omitempty"`
	PromptKey    string    `json:"prompt_key,omitempty"`
	Version      string    `json:"version,omitempty"`
	Stream       bool      `json:"stream,omitempty"`
	InputTokens  int64     `json:"input_tokens,omitempty"`
	OutputTokens int64     `json:"output_tokens,omitempty"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	EndedAt      time.Time `json:"ended_at,omitempty"`
	StatusCode   int32     `json:"status_code,omitempty"`
}

type EventCollectorProviderImpl struct{}

func NewEventCollectorProvider() ICollectorProvider {
	return &EventCollectorProviderImpl{}
}

func (c *EventCollectorProviderImpl) CollectPromptHubEvent(ctx context.Context, spaceID int64, prompts []*entity.Prompt) {
}

func (c *EventCollectorProviderImpl) CollectPTaaSEvent(ctx context.Context, executeLog *ExecuteLog) {
}
