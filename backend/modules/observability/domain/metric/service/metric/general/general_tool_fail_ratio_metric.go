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

type GeneralToolFailRatioMetric struct {
	entity.MetricFillNull
}

func (m *GeneralToolFailRatioMetric) Name() string {
	return entity.MetricNameGeneralToolFailRatio
}

func (m *GeneralToolFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralToolFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralToolFailRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{
		Expression: "countIf(1, %s != 0) / count()",
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *GeneralToolFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *GeneralToolFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralToolFailRatioMetric() entity.IMetricDefinition {
	return &GeneralToolFailRatioMetric{}
}
