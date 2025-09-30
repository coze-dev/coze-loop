// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPMMinMetric Tokens Per Minute最小值指标
type ModelTPMMinMetric struct{}

func (m *ModelTPMMinMetric) Name() string {
	return entity.MetricNameModelTPMMin
}

func (m *ModelTPMMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPMMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 60000000))"
}

func (m *ModelTPMMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPMMinMetric() entity.IMetricDefinition {
	return &ModelTPMMinMetric{}
}
