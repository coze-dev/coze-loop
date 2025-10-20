// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelInputTokenCountMetric Input Tokens消耗指标
type ModelInputTokenCountMetric struct{}

func (m *ModelInputTokenCountMetric) Name() string {
	return entity.MetricNameModelInputTokenCount
}

func (m *ModelInputTokenCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelInputTokenCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelInputTokenCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['input_tokens'])"
}

func (m *ModelInputTokenCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelInputTokenCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelInputTokenCountMetric() entity.IMetricDefinition {
	return &ModelInputTokenCountMetric{}
}