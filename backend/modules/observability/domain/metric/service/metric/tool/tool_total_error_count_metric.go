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

type ToolTotalErrorCountMetric struct{}

func (m *ToolTotalErrorCountMetric) Name() string {
	return entity.MetricNameToolTotalErrorCount
}

func (m *ToolTotalErrorCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolTotalErrorCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ToolTotalErrorCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "countIf(1, %s != 0)",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ToolTotalErrorCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalErrorCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ToolTotalErrorCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *ToolTotalErrorCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewToolTotalErrorCountMetric() entity.IMetricDefinition {
	return &ToolTotalErrorCountMetric{}
}
