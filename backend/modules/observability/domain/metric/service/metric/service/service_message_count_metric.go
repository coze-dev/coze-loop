// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ServiceMessageCountMetric struct{}

func (m *ServiceMessageCountMetric) Name() string {
	return entity.MetricNameServiceMessageCount
}

func (m *ServiceMessageCountMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceMessageCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceMessageCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "uniq(%s)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldMessageID,
				FieldType: loop_span.FieldTypeString,
			},
		},
	}
}

func (m *ServiceMessageCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	filters, err := filter.BuildALLSpanFilter(ctx, env)
	if err != nil {
		return nil, err
	}
	filters = append(filters, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldUserID,
		FieldType: loop_span.FieldTypeString,
		Values:    []string{""},
		QueryType: ptr.Of(loop_span.QueryTypeEnumNotEq),
	})
	return filters, nil
}

func (m *ServiceMessageCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceMessageCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewServiceMessageCountMetric() entity.IMetricDefinition {
	return &ServiceMessageCountMetric{}
}
