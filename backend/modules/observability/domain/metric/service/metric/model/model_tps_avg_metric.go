// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPSAvgMetric Tokens Per Second平均值指标
type ModelTPSAvgMetric struct{}

func (m *ModelTPSAvgMetric) Name() string {
	return entity.MetricNameModelTPSAvg
}

func (m *ModelTPSAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPSAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPSAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "avg((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 1000000))"
}

func (m *ModelTPSAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPSAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPSAvgMetric() entity.IMetricDefinition {
	return &ModelTPSAvgMetric{}
}
