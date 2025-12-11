// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type GeneralToolTotalCountMetric struct{}

func (m *GeneralToolTotalCountMetric) Name() string {
	return entity.MetricNameGeneralToolTotalCount
}

func (m *GeneralToolTotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralToolTotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *GeneralToolTotalCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *GeneralToolTotalCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *GeneralToolTotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralToolTotalCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewGeneralToolTotalCountMetric() entity.IMetricDefinition {
	return &GeneralToolTotalCountMetric{}
}
