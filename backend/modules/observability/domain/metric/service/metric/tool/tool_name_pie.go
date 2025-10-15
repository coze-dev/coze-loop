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

type ToolNamePieMetric struct{}

func (m *ToolNamePieMetric) Name() string {
	return entity.MetricNameToolNamePie
}

func (m *ToolNamePieMetric) Type() entity.MetricType {
	return entity.MetricOperatorPie
}

func (m *ToolNamePieMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolNamePieMetric) Expression(granularity entity.MetricGranularity) string {
	return "1"
}

func (m *ToolNamePieMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolNamePieMetric) GroupBy() []*entity.Dimension {
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

func NewToolNamePieMetric() entity.IMetricDefinition {
	return &ToolNamePieMetric{}
}
