// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelDurationPct50Metric 模型调用总耗时50分位指标
type ModelDurationPct50Metric struct{}

func (m *ModelDurationPct50Metric) Name() string {
	return entity.MetricNameModelDurationPct50
}

func (m *ModelDurationPct50Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelDurationPct50Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelDurationPct50Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.5)(duration)/1000"
}

func (m *ModelDurationPct50Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelDurationPct50Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelDurationPct50Metric() entity.IMetricDefinition {
	return &ModelDurationPct50Metric{}
}
