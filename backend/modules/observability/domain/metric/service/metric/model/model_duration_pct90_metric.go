// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelDurationPct90Metric 模型调用总耗时90分位指标
type ModelDurationPct90Metric struct{}

func (m *ModelDurationPct90Metric) Name() string {
	return entity.MetricNameModelDurationPct90
}

func (m *ModelDurationPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelDurationPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelDurationPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)(duration)/1000"
}

func (m *ModelDurationPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelDurationPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelDurationPct90Metric() entity.IMetricDefinition {
	return &ModelDurationPct90Metric{}
}
