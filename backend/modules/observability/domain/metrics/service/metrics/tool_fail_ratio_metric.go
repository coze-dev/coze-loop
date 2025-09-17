// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ToolFailRatioMetric 工具调用错误率指标
type ToolFailRatioMetric struct{}

func (m *ToolFailRatioMetric) Name() string {
	return "tool_fail_ratio"
}

func (m *ToolFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolFailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ToolFailRatioMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}