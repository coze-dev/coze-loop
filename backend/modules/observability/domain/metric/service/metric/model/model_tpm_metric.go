// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

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

type ModelTPMMetric struct{}

func (m *ModelTPMMetric) Name() string {
	return entity.MetricNameModelTPM
}

func (m *ModelTPMMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelTPMMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "(%s+%s)/(%s / 60000000)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldInputTokens,
				FieldType: loop_span.FieldTypeLong,
			},
			{
				FieldName: loop_span.SpanFieldOutputTokens,
				FieldType: loop_span.FieldTypeLong,
			},
			{
				FieldName: loop_span.SpanFieldDuration,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ModelTPMMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelTPMMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewAvgWrapper(),
		wrapper.NewMinWrapper(),
		wrapper.NewMaxWrapper(),
		wrapper.NewPct50Wrapper(),
		wrapper.NewPct90Wrapper(),
		wrapper.NewPct99Wrapper(),
	}
}

func NewModelTPMMetric() entity.IMetricDefinition {
	return &ModelTPMMetric{}
}
