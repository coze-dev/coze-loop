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

type ServiceQPSFailMetric struct{}

func (m *ServiceQPSFailMetric) Name() string {
	return entity.MetricNameServiceQPSFail
}

func (m *ServiceQPSFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPSFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceQPSFailMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ServiceQPSFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPSFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPSFailMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewServiceTraceErrorCountMetric()),
		consts.NewConstSecondMetric(),
	}
}

func (m *ServiceQPSFailMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ServiceQPSFailMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewServiceQPSFailMetric() entity.IMetricDefinition {
	return &ServiceQPSFailMetric{}
}
