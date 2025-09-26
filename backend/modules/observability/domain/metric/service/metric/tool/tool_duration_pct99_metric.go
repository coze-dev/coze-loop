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

// ToolDurationPct99Metric 工具调用耗时99分位指标
type ToolDurationPct99Metric struct{}

func (m *ToolDurationPct99Metric) Name() string {
	return entity.MetricNameToolDurationPct99
}

func (m *ToolDurationPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)(duration)"
}

func (m *ToolDurationPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolDurationPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolDurationPct99Metric() entity.IMetricDefinition {
	return &ToolDurationPct99Metric{}
}