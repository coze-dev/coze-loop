// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceTraceCountMetric struct{}

func (m *ServiceTraceCountMetric) Name() string {
	return entity.MetricNameServiceTraceCount
}

func (m *ServiceTraceCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ServiceTraceCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceTraceCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *ServiceTraceCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceTraceCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceTraceCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func NewServiceTraceCountMetric() entity.IMetricDefinition {
	return &ServiceTraceCountMetric{}
}
