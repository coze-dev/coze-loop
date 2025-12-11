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

type ServiceTraceErrorCountMetric struct{}

func (m *ServiceTraceErrorCountMetric) Name() string {
	return entity.MetricNameServiceTraceErrorCount
}

func (m *ServiceTraceErrorCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ServiceTraceErrorCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceTraceErrorCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "countIf(1, %s != 0)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ServiceTraceErrorCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceTraceErrorCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceTraceErrorCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *ServiceTraceErrorCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewServiceTraceErrorCountMetric() entity.IMetricDefinition {
	return &ServiceTraceErrorCountMetric{}
}
