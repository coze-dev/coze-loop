// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type Settings struct {
	// query parameters
	WorkspaceId           int64
	ThirdPartyWorkspaceID string
	PlatformType          loop_span.PlatformType
	QueryStartTime        int64 // ms
	QueryEndTime          int64 // ms
	QueryTenants          []string
	SpanDoubleCheck       bool
	QueryTraceID          string
	QueryLogID            string
	// Scene 指定处理器场景，如果设置则优先使用该场景的处理器
	Scene string
}

type Factory interface {
	CreateProcessor(context.Context, Settings) (Processor, error)
}

type Processor interface {
	Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error)
}
