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

type ServiceQPMSuccessMetric struct{}

func (m *ServiceQPMSuccessMetric) Name() string {
	return entity.MetricNameServiceQPMSuccess
}

func (m *ServiceQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceQPMSuccessMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ServiceQPMSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPMSuccessMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewServiceTraceSuccessCountMetric()),
		consts.NewConstMinuteMetric(),
	}
}

func (m *ServiceQPMSuccessMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ServiceQPMSuccessMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewServiceQPMSuccessMetric() entity.IMetricDefinition {
	return &ServiceQPMSuccessMetric{}
}
