// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPOTMinMetric Time Per Output Token最小值指标
type ModelTPOTMinMetric struct{}

func (m *ModelTPOTMinMetric) Name() string {
	return entity.MetricNameModelTPOTMin
}

func (m *ModelTPOTMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPOTMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPOTMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min((duration-tags_long['latency_first_resp'])/(1000*tags_long['output_tokens']))"
}

func (m *ModelTPOTMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPOTMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPOTMinMetric() entity.IMetricDefinition {
	return &ModelTPOTMinMetric{}
}
