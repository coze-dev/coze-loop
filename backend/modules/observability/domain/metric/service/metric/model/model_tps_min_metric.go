// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPSMinMetric Tokens Per Second最小值指标
type ModelTPSMinMetric struct{}

func (m *ModelTPSMinMetric) Name() string {
	return entity.MetricNameModelTPSMin
}

func (m *ModelTPSMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPSMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPSMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min(sum(tags_long['input_tokens']+tags_long['output_tokens']) * 1000/sum(duration))"
}

func (m *ModelTPSMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPSMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPSMinMetric() entity.IMetricDefinition {
	return &ModelTPSMinMetric{}
}