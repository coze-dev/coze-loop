// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralModelTotalTokensMetric struct{}

func (m *GeneralModelTotalTokensMetric) Name() string {
	return entity.MetricNameGeneralModelTotalTokens
}

func (m *GeneralModelTotalTokensMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralModelTotalTokensMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralModelTotalTokensMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
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

func (m *GeneralModelTotalTokensMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *GeneralModelTotalTokensMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralModelTotalTokensMetric() entity.IMetricDefinition {
	return &GeneralModelTotalTokensMetric{}
}
