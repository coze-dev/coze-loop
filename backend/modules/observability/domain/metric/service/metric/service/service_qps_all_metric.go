// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	consts "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/const"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceQPSAllMetric struct{}

func (m *ServiceQPSAllMetric) Name() string {
	return entity.MetricNameServiceQPSAll
}

func (m *ServiceQPSAllMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPSAllMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceQPSAllMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ServiceQPSAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPSAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPSAllMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewServiceTraceCountMetric()),
		consts.NewConstSecondMetric(),
	}
}

func (m *ServiceQPSAllMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ServiceQPSAllMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewServiceQPSAllMetric() entity.IMetricDefinition {
	return &ServiceQPSAllMetric{}
}
