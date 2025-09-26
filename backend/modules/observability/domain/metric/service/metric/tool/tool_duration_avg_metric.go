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

// ToolDurationAvgMetric 工具调用耗时平均值指标
type ToolDurationAvgMetric struct{}

func (m *ToolDurationAvgMetric) Name() string {
	return entity.MetricNameToolDurationAvg
}

func (m *ToolDurationAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "avg(duration)"
}

func (m *ToolDurationAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolDurationAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolDurationAvgMetric() entity.IMetricDefinition {
	return &ToolDurationAvgMetric{}
}