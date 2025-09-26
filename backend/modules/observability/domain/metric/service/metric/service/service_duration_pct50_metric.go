// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceDurationPct50Metric 整体耗时50分位指标
type ServiceDurationPct50Metric struct{}

func (m *ServiceDurationPct50Metric) Name() string {
	return entity.MetricNameServiceDurationPct50
}

func (m *ServiceDurationPct50Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceDurationPct50Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceDurationPct50Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.5)(duration)"
}

func (m *ServiceDurationPct50Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceDurationPct50Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceDurationPct50Metric() entity.IMetricDefinition {
	return &ServiceDurationPct50Metric{}
}