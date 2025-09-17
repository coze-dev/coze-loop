// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ToolTotalCountMetric 工具调用次数指标
type ToolTotalCountMetric struct{}

func (m *ToolTotalCountMetric) Name() string {
	return "tool_total_count"
}

func (m *ToolTotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolTotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolTotalCountMetric) Expression() string {
	return "count()"
}

func (m *ToolTotalCountMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}