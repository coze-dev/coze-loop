// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceDurationPct90Metric 整体耗时90分位指标
type ServiceDurationPct90Metric struct{}

func (m *ServiceDurationPct90Metric) Name() string {
	return entity.MetricNameServiceDurationPct90
}

func (m *ServiceDurationPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceDurationPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceDurationPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)(duration)"
}

func (m *ServiceDurationPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceDurationPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceDurationPct90Metric() entity.IMetricDefinition {
	return &ServiceDurationPct90Metric{}
}