// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_context_extractor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

//go:generate mockgen -destination=mocks/span_context_extractor.go -package=mocks . ISpanContextExtractor
type ISpanContextExtractor interface {
	GetCallType(ctx context.Context, spans *loop_span.Span) string
	GetBenefitSource(ctx context.Context, callType string) int64
}
