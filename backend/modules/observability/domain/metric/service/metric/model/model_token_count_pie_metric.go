// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelTokenCountPieMetric struct{}

func (m *ModelTokenCountPieMetric) Name() string {
	return entity.MetricNameModelTokenCountPie
}

func (m *ModelTokenCountPieMetric) Type() entity.MetricType {
	return entity.MetricTypePie
}

func (m *ModelTokenCountPieMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelTokenCountPieMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "sum(%s + %s)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldInputTokens,
				FieldType: loop_span.FieldTypeLong,
			},
			{
				FieldName: loop_span.SpanFieldOutputTokens,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ModelTokenCountPieMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTokenCountPieMetric) GroupBy() []*entity.Dimension {
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

func NewModelTokenCountPieMetric() entity.IMetricDefinition {
	return &ModelTokenCountPieMetric{}
}
