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

// ToolDurationPct50Metric 工具调用耗时50分位指标
type ToolDurationPct50Metric struct{}

func (m *ToolDurationPct50Metric) Name() string {
	return entity.MetricNameToolDurationPct50
}

func (m *ToolDurationPct50Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationPct50Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationPct50Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.5)(duration)/1000"
}

func (m *ToolDurationPct50Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolDurationPct50Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolDurationPct50Metric() entity.IMetricDefinition {
	return &ToolDurationPct50Metric{}
}
