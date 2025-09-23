// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ToolLatencyAvgMetric 工具调用平均耗时指标
type ToolLatencyAvgMetric struct{}

func (m *ToolLatencyAvgMetric) Name() string {
	return entity.MetricNameToolLatencyAvg
}

func (m *ToolLatencyAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolLatencyAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolLatencyAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(duration / 1000) / count()"
}

func (m *ToolLatencyAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
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

func (m *ToolLatencyAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}
