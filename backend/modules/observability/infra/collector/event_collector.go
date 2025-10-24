// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package collector

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/collector"
)

type EventCollectorProviderImpl struct{}

func NewEventCollectorProvider() collector.ICollectorProvider {
	return &EventCollectorProviderImpl{}
}

func (p *EventCollectorProviderImpl) CollectTraceOpenAPIEvent(ctx context.Context, method string, workspaceId int64, platformType, spanListType string, spanSize int64, errorCode int, start time.Time, isError bool) {
}
