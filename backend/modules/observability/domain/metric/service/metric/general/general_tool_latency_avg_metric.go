// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// GeneralToolLatencyAvgMetric 工具调用平均耗时指标
type GeneralToolLatencyAvgMetric struct{}

func (m *GeneralToolLatencyAvgMetric) Name() string {
	return entity.MetricNameGeneralToolLatencyAvg
}

func (m *GeneralToolLatencyAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralToolLatencyAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralToolLatencyAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(duration / 1000) / count()"
}

func (m *GeneralToolLatencyAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *GeneralToolLatencyAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralToolLatencyAvgMetric() entity.IMetricDefinition {
	return &GeneralToolLatencyAvgMetric{}
}