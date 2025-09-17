// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ToolTotalCountMetric 工具调用次数指标
type ToolTotalCountMetric struct{}

func (m *ToolTotalCountMetric) Name() string {
	return "tool_total_count"
}

func (m *ToolTotalCountMetric) Type() string {
	return "summary"
}

func (m *ToolTotalCountMetric) Source() string {
	return "ck"
}

func (m *ToolTotalCountMetric) Expression() string {
	return "count()"
}

func (m *ToolTotalCountMetric) Where(filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	// 直接返回工具Span筛选条件，不使用Filter接口，因为这是固定的筛选逻辑
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