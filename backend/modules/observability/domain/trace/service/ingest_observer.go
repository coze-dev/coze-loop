// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

// IngestTraceObserver observes spans immediately after they enter IngestTraces.
// Implementations must not mutate the span list.
type IngestTraceObserver interface {
	BeforeIngest(ctx context.Context, tenant string, spans loop_span.SpanList)
}

type noopIngestTraceObserver struct{}

func (noopIngestTraceObserver) BeforeIngest(context.Context, string, loop_span.SpanList) {}
