// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelSystemTokenCountMetric struct{}

func (m *ModelSystemTokenCountMetric) Name() string {
	return entity.MetricNameModelSystemTokenCount
}

func (m *ModelSystemTokenCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelSystemTokenCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelSystemTokenCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "sum(%s)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: "_system_tokens",
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ModelSystemTokenCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelSystemTokenCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelSystemTokenCountMetric() entity.IMetricDefinition {
	return &ModelSystemTokenCountMetric{}
}
