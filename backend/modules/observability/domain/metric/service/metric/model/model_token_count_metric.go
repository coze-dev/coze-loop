// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelTokenCountMetric struct{}

func (m *ModelTokenCountMetric) Name() string {
	return entity.MetricNameModelTokenCount
}

func (m *ModelTokenCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelTokenCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelTokenCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
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

func (m *ModelTokenCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTokenCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelTokenCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func NewModelTokenCountMetric() entity.IMetricDefinition {
	return &ModelTokenCountMetric{}
}
