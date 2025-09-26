// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelOutputTokenCountMetric Output Tokens消耗指标
type ModelOutputTokenCountMetric struct{}

func (m *ModelOutputTokenCountMetric) Name() string {
	return entity.MetricNameModelOutputTokenCount
}

func (m *ModelOutputTokenCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelOutputTokenCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelOutputTokenCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['output_tokens'])"
}

func (m *ModelOutputTokenCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelOutputTokenCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelOutputTokenCountMetric() entity.IMetricDefinition {
	return &ModelOutputTokenCountMetric{}
}