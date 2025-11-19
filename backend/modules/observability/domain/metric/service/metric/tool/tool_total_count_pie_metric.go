// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tool

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ToolTotalCountPieMetric struct{}

func (m *ToolTotalCountPieMetric) Name() string {
	return entity.MetricNameToolTotalCountPie
}

func (m *ToolTotalCountPieMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolTotalCountPieMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ToolTotalCountPieMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *ToolTotalCountPieMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalCountPieMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{
		{
			Field: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldSpanName,
				FieldType: loop_span.FieldTypeString,
			},
			Alias: "name",
		},
	}
}

func (m *ToolTotalCountPieMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewToolTotalCountPieMetric() entity.IMetricDefinition {
	return &ToolTotalCountPieMetric{}
}
