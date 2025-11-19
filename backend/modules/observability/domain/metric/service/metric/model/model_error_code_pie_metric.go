// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ModelErrorCodePieMetric struct{}

func (m *ModelErrorCodePieMetric) Name() string {
	return entity.MetricNameModelErrorCodePie
}

func (m *ModelErrorCodePieMetric) Type() entity.MetricType {
	return entity.MetricTypePie
}

func (m *ModelErrorCodePieMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelErrorCodePieMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *ModelErrorCodePieMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	filters, err := filter.BuildLLMSpanFilter(ctx, env)
	if err != nil {
		return nil, err
	}
	// 只统计错误状态码（非0）
	filters = append(filters, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldStatusCode,
		FieldType: loop_span.FieldTypeLong,
		Values:    []string{"0"},
		QueryType: ptr.Of(loop_span.QueryTypeEnumNotEq),
	})
	return filters, nil
}

func (m *ModelErrorCodePieMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{
		{
			Field: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
			Alias: loop_span.SpanFieldStatusCode,
		},
	}
}

func (m *ModelErrorCodePieMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewModelErrorCodePieMetric() entity.IMetricDefinition {
	return &ModelErrorCodePieMetric{}
}
