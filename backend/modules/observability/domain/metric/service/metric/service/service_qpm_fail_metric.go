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

type ServiceQPMFailMetric struct{}

func (m *ServiceQPMFailMetric) Name() string {
	return entity.MetricNameServiceQPMFail
}

func (m *ServiceQPMFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceQPMFailMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ServiceQPMFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPMFailMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewServiceTraceErrorCountMetric()),
		consts.NewConstMinuteMetric(),
	}
}

func (m *ServiceQPMFailMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ServiceQPMFailMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewServiceQPMFailMetric() entity.IMetricDefinition {
	return &ServiceQPMFailMetric{}
}
