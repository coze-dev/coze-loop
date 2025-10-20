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

// ToolDurationPct90Metric 工具调用耗时90分位指标
type ToolDurationPct90Metric struct{}

func (m *ToolDurationPct90Metric) Name() string {
	return entity.MetricNameToolDurationPct90
}

func (m *ToolDurationPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)(duration)/1000"
}

func (m *ToolDurationPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolDurationPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolDurationPct90Metric() entity.IMetricDefinition {
	return &ToolDurationPct90Metric{}
}
