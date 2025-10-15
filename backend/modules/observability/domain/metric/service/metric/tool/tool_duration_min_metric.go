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

// ToolDurationMinMetric 工具调用耗时最小值指标
type ToolDurationMinMetric struct{}

func (m *ToolDurationMinMetric) Name() string {
	return entity.MetricNameToolDurationMin
}

func (m *ToolDurationMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min(duration)/1000"
}

func (m *ToolDurationMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolDurationMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolDurationMinMetric() entity.IMetricDefinition {
	return &ToolDurationMinMetric{}
}
