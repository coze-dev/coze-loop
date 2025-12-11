// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tool

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ToolTotalSuccessCountMetric struct{}

func (m *ToolTotalSuccessCountMetric) Name() string {
	return entity.MetricNameToolTotalSuccessCount
}

func (m *ToolTotalSuccessCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolTotalSuccessCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ToolTotalSuccessCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "countIf(1, %s = 0)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ToolTotalSuccessCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalSuccessCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ToolTotalSuccessCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *ToolTotalSuccessCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewToolTotalSuccessCountMetric() entity.IMetricDefinition {
	return &ToolTotalSuccessCountMetric{}
}
