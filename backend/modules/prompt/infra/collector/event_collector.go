// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

//go:generate mockgen -destination=mocks/event_collector.go -package=mocks . ICollectorProvider
type ICollectorProvider interface {
	CollectPromptHubEvent(ctx context.Context, spaceID int64, prompts []*entity.Prompt)
}

type EventCollectorProviderImpl struct{}

func NewEventCollectorProvider() ICollectorProvider {
	return &EventCollectorProviderImpl{}
}

func (c *EventCollectorProviderImpl) CollectPromptHubEvent(ctx context.Context, spaceID int64, prompts []*entity.Prompt) {
}
