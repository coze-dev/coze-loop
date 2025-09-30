// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// GeneralModelLatencyAvgMetric 模型调用平均耗时指标
type GeneralModelLatencyAvgMetric struct {
	entity.MetricFillNull
}

func (m *GeneralModelLatencyAvgMetric) Name() string {
	return entity.MetricNameGeneralModelLatencyAvg
}

func (m *GeneralModelLatencyAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralModelLatencyAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralModelLatencyAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(duration / 1000) / count()" // ms
}

func (m *GeneralModelLatencyAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *GeneralModelLatencyAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralModelLatencyAvgMetric() entity.IMetricDefinition {
	return &GeneralModelLatencyAvgMetric{}
}
