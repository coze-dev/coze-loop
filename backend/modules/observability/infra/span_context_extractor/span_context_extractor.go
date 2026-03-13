// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_context_extractor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/span_context_extractor"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func NewSpanContextExtractor() span_context_extractor.ISpanContextExtractor {
	return &SpanContextExtractorImpl{}
}

type SpanContextExtractorImpl struct{}

func (s *SpanContextExtractorImpl) GetCallType(ctx context.Context, spans *loop_span.Span) string {
	return "Custom"
}

func (s *SpanContextExtractorImpl) GetBenefitSource(ctx context.Context, callType string) int64 {
	return 10
}
