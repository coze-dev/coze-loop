// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceUniqTraceMetric struct{}

func (m *ServiceUniqTraceMetric) Name() string {
	return entity.MetricNameServiceUniqTrace
}

func (m *ServiceUniqTraceMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceUniqTraceMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceUniqTraceMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "uniq(%s)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldTraceId,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ServiceUniqTraceMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildALLSpanFilter(ctx, env)
}

func (m *ServiceUniqTraceMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceUniqTraceMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewServiceUniqTraceMetric() entity.IMetricDefinition {
	return &ServiceUniqTraceMetric{}
}
