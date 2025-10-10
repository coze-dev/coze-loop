// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package collector

import (
	"context"
	"time"
)

//go:generate mockgen -destination=mocks/event_collector.go -package=mocks . ICollectorProvider
type ICollectorProvider interface {
	CollectTraceOpenAPIEvent(ctx context.Context, method string, workspaceId int64, platformType, spanListType string, spanSize int64, errorCode int, start time.Time, isError bool)
}
