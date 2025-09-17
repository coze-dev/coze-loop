// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTotalTokensMetric 模型Tokens消耗指标
type ModelTotalTokensMetric struct{}

func (m *ModelTotalTokensMetric) Name() string {
	return "model_total_tokens"
}

func (m *ModelTotalTokensMetric) Type() string {
	return "summary"
}

func (m *ModelTotalTokensMetric) Source() string {
	return "ck"
}

func (m *ModelTotalTokensMetric) Expression() string {
	return "sum(tags_long['input_tokens'] + tags_long['output_tokens'])"
}

func (m *ModelTotalTokensMetric) Where(filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(context.Background(), env)
}

func (m *ModelTotalTokensMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}