// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelDurationMinMetric 模型调用总耗时最小值指标
type ModelDurationMinMetric struct{}

func (m *ModelDurationMinMetric) Name() string {
	return entity.MetricNameModelDurationMin
}

func (m *ModelDurationMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelDurationMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelDurationMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min(duration)/1000"
}

func (m *ModelDurationMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelDurationMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelDurationMinMetric() entity.IMetricDefinition {
	return &ModelDurationMinMetric{}
}
