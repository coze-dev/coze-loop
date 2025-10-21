// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelToolChoiceTokenCountMetric struct{}

func (m *ModelToolChoiceTokenCountMetric) Name() string {
	return entity.MetricNameModelToolChoiceTokenCount
}

func (m *ModelToolChoiceTokenCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelToolChoiceTokenCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelToolChoiceTokenCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(tags_long['_tool_choice_tokens'])"
}

func (m *ModelToolChoiceTokenCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelToolChoiceTokenCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelToolChoiceTokenCountMetric() entity.IMetricDefinition {
	return &ModelToolChoiceTokenCountMetric{}
}
