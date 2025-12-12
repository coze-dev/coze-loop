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

type ModelTotalCountPieMetric struct{}

func (m *ModelTotalCountPieMetric) Name() string {
	return entity.MetricNameModelTotalCountPie
}

func (m *ModelTotalCountPieMetric) Type() entity.MetricType {
	return entity.MetricTypePie
}

func (m *ModelTotalCountPieMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelTotalCountPieMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *ModelTotalCountPieMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	filters, err := filter.BuildLLMSpanFilter(ctx, env)
	if err != nil {
		return nil, err
	}
	// 聚合非空
	filters = append(filters, &loop_span.FilterField{
		FieldName: "model_name",
		FieldType: loop_span.FieldTypeString,
		Values:    []string{""},
		QueryType: ptr.Of(loop_span.QueryTypeEnumNotEq),
	})
	return filters, nil
}

func (m *ModelTotalCountPieMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{
		{
			Field: &loop_span.FilterField{
				FieldName: "model_name",
				FieldType: loop_span.FieldTypeString,
			},
			Alias: "name",
		},
	}
}

func (m *ModelTotalCountPieMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewModelTotalCountPieMetric() entity.IMetricDefinition {
	return &ModelTotalCountPieMetric{}
}
