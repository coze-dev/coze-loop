// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceDurationMinMetric 整体耗时最小值指标
type ServiceDurationMinMetric struct{}

func (m *ServiceDurationMinMetric) Name() string {
	return entity.MetricNameServiceDurationMin
}

func (m *ServiceDurationMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceDurationMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceDurationMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min(duration)"
}

func (m *ServiceDurationMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceDurationMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceDurationMinMetric() entity.IMetricDefinition {
	return &ServiceDurationMinMetric{}
}