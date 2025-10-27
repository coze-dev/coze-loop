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

type ModelSuccessRatioMetric struct {
	entity.MetricFillNull
}

func (m *ModelSuccessRatioMetric) Name() string {
	return entity.MetricNameModelSuccessRatio
}

func (m *ModelSuccessRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelSuccessRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelSuccessRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "countIf(1, %s = 0) / count()",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ModelSuccessRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelSuccessRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelSuccessRatioMetric() entity.IMetricDefinition {
	return &ModelSuccessRatioMetric{}
}
