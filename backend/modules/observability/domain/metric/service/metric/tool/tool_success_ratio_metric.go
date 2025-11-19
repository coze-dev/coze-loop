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

type ToolSuccessRatioMetric struct {
	entity.MetricFillNull
}

func (m *ToolSuccessRatioMetric) Name() string {
	return entity.MetricNameToolSuccessRatio
}

func (m *ToolSuccessRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolSuccessRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ToolSuccessRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ToolSuccessRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolSuccessRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ToolSuccessRatioMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewToolTotalSuccessCountMetric()),
		wrapper.NewTimeSeriesWrapper().Wrap(NewToolTotalCountMetric()),
	}
}

func (m *ToolSuccessRatioMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ToolSuccessRatioMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewToolSuccessRatioMetric() entity.IMetricDefinition {
	return &ToolSuccessRatioMetric{}
}
