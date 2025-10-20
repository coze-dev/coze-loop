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

type ToolDurationMetric struct{}

func (m *ToolDurationMetric) Name() string {
	return entity.MetricNameToolDuration
}

func (m *ToolDurationMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolDurationMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolDurationMetric) Expression(granularity entity.MetricGranularity) string {
	return "duration/1000"
}

func (m *ToolDurationMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
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

func (m *ToolDurationMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ToolDurationMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewAvgWrapper(),
		wrapper.NewMinWrapper(),
		wrapper.NewMaxWrapper(),
		wrapper.NewPct50Wrapper(),
		wrapper.NewPct90Wrapper(),
		wrapper.NewPct99Wrapper(),
	}
}

func NewToolDurationMetric() entity.IMetricDefinition {
	return &ToolDurationMetric{}
}
