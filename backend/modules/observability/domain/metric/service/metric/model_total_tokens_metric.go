// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTotalTokensMetric 模型Tokens消耗指标
type ModelTotalTokensMetric struct{}

func (m *ModelTotalTokensMetric) Name() string {
	return entity.MetricNameModelTotalTokens
}

func (m *ModelTotalTokensMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelTotalTokensMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTotalTokensMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['input_tokens'] + tags_long['output_tokens'])"
}

func (m *ModelTotalTokensMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTotalTokensMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}
