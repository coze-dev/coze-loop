// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceDurationPct99Metric 整体耗时99分位指标
type ServiceDurationPct99Metric struct{}

func (m *ServiceDurationPct99Metric) Name() string {
	return entity.MetricNameServiceDurationPct99
}

func (m *ServiceDurationPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceDurationPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceDurationPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)(duration)"
}

func (m *ServiceDurationPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceDurationPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceDurationPct99Metric() entity.IMetricDefinition {
	return &ServiceDurationPct99Metric{}
}