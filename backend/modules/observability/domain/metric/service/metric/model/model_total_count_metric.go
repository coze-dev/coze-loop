// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ModelTotalCountMetric struct{}

func (m *ModelTotalCountMetric) Name() string {
	return entity.MetricNameModelTotalCount
}

func (m *ModelTotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelTotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelTotalCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *ModelTotalCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ModelTotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelTotalCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *ModelTotalCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewModelTotalCountMetric() entity.IMetricDefinition {
	return &ModelTotalCountMetric{}
}
