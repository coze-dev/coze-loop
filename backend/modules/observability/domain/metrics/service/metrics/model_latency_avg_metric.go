// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/filter"
)

// ModelLatencyAvgMetric 模型调用平均耗时指标
type ModelLatencyAvgMetric struct{}

func (m *ModelLatencyAvgMetric) Name() string {
	return "model_latency_avg"
}

func (m *ModelLatencyAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelLatencyAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelLatencyAvgMetric) Expression() string {
	return "sum(duration / 1000) / count()"
}

func (m *ModelLatencyAvgMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter()
}

func (m *ModelLatencyAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}