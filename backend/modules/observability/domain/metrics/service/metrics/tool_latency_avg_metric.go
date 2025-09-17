// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ToolLatencyAvgMetric 工具调用平均耗时指标
type ToolLatencyAvgMetric struct{}

func (m *ToolLatencyAvgMetric) Name() string {
	return "tool_latency_avg"
}

func (m *ToolLatencyAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolLatencyAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolLatencyAvgMetric) Expression() string {
	return "sum(duration / 1000) / count()"
}

func (m *ToolLatencyAvgMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolLatencyAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}