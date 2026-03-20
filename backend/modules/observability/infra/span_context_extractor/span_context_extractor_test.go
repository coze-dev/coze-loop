// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_context_extractor

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
)

func TestSpanContextExtractorImpl(t *testing.T) {
	ext := NewSpanContextExtractor()
	_, ok := ext.(*SpanContextExtractorImpl)
	assert.True(t, ok)

	assert.Equal(t, "Custom", ext.GetCallType(context.Background(), &loop_span.Span{}))
	assert.Equal(t, int64(10), ext.GetBenefitSource(context.Background(), "any"))
}
