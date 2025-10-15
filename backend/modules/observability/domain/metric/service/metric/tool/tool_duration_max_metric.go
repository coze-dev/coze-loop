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

// ToolDurationMaxMetric 工具调用耗时最大值指标
type ToolDurationMaxMetric struct{}

func (m *ToolDurationMaxMetric) Name() string {
	return entity.MetricNameToolDurationMax
}

func (m *ToolDurationMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationMaxMetric) Expression(granularity entity.MetricGranularity) string {
	return "max(duration)/1000"
}

func (m *ToolDurationMaxMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolDurationMaxMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolDurationMaxMetric() entity.IMetricDefinition {
	return &ToolDurationMaxMetric{}
}
