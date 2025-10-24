// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelQPMSuccessMetric struct{}

func (m *ModelQPMSuccessMetric) Name() string {
	return entity.MetricNameModelQPMSuccess
}

func (m *ModelQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPMSuccessMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	denominator := entity.GranularityToSecond(granularity) / 60
	expression := fmt.Sprintf("countIf(1, %%s = 0)/%d", denominator)
	return &entity.Expression{
		Expression: expression,
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ModelQPMSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPMSuccessMetric() entity.IMetricDefinition {
	return &ModelQPMSuccessMetric{}
}
