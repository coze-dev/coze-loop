// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelDurationPct99Metric 模型调用总耗时99分位指标
type ModelDurationPct99Metric struct{}

func (m *ModelDurationPct99Metric) Name() string {
	return entity.MetricNameModelDurationPct99
}

func (m *ModelDurationPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelDurationPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelDurationPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)(duration/1000)"
}

func (m *ModelDurationPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelDurationPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelDurationPct99Metric() entity.IMetricDefinition {
	return &ModelDurationPct99Metric{}
}
