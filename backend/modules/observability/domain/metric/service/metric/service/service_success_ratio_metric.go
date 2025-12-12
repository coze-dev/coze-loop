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

type ServiceSuccessRatioMetric struct {
	entity.MetricFillNull
}

func (m *ServiceSuccessRatioMetric) Name() string {
	return entity.MetricNameServiceSuccessRatio
}

func (m *ServiceSuccessRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceSuccessRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceSuccessRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ServiceSuccessRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *ServiceSuccessRatioMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewServiceTraceSuccessCountMetric()),
		wrapper.NewTimeSeriesWrapper().Wrap(NewServiceTraceCountMetric()),
	}
}

func (m *ServiceSuccessRatioMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ServiceSuccessRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceSuccessRatioMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewServiceSuccessRatioMetric() entity.IMetricDefinition {
	return &ServiceSuccessRatioMetric{}
}
