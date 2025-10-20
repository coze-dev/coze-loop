// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelDurationAvgMetric 模型调用总耗时平均值指标
type ModelDurationAvgMetric struct{}

func (m *ModelDurationAvgMetric) Name() string {
	return entity.MetricNameModelDurationAvg
}

func (m *ModelDurationAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelDurationAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelDurationAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "avg(duration)/1000" // ms
}

func (m *ModelDurationAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelDurationAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelDurationAvgMetric() entity.IMetricDefinition {
	return &ModelDurationAvgMetric{}
}
