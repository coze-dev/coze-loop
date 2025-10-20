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

// ToolTotalCountMetric 工具调用量指标
type ToolTotalCountByTimeMetric struct{}

func (m *ToolTotalCountByTimeMetric) Name() string {
	return entity.MetricNameToolTotalCountByTime
}

func (m *ToolTotalCountByTimeMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolTotalCountByTimeMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolTotalCountByTimeMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *ToolTotalCountByTimeMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalCountByTimeMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewToolTotalCountByTimeMetric() entity.IMetricDefinition {
	return &ToolTotalCountByTimeMetric{}
}
