// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tool

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric_new/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ToolDurationAvgMetric struct{}

func (m *ToolDurationAvgMetric) Name() string {
	return entity.MetricNameToolDuration
}

func (m *ToolDurationAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "duration/1000"
}

func (m *ToolDurationAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	_ = ctx
	_ = filter
	_ = env
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

func (m *ToolDurationAvgMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewAvgWrapper(),
		wrapper.NewMinWrapper(),
		wrapper.NewMaxWrapper(),
		wrapper.NewPct50Wrapper(),
		wrapper.NewPct90Wrapper(),
		wrapper.NewPct99Wrapper(),
	}
}

func NewToolDurationAvgMetric() entity.IMetricDefinition {
	return &ToolDurationAvgMetric{}
}
