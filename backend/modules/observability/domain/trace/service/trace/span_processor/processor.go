// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type Settings struct {
	// query parameters
	WorkspaceId    int64
	PlatformType   loop_span.PlatformType
	QueryStartTime int64 // ms
	QueryEndTime   int64 // ms
	Tenant         string
}

type Factory interface {
	CreateProcessor(context.Context, Settings) (Processor, error)
}

type Processor interface {
	Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error)
}
