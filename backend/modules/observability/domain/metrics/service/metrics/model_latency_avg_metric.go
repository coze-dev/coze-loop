// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelLatencyAvgMetric 模型调用平均耗时指标
type ModelLatencyAvgMetric struct{}

func (m *ModelLatencyAvgMetric) Name() string {
	return entity.MetricNameModelLatencyAvg
}

func (m *ModelLatencyAvgMetric) Type() string {
	return string(entity.MetricTypeSummary)
}

func (m *ModelLatencyAvgMetric) Source() string {
	return string(entity.MetricSourceCK)
}

func (m *ModelLatencyAvgMetric) Expression() string {
	return "sum(duration / 1000) / count()"
}

func (m *ModelLatencyAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelLatencyAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}