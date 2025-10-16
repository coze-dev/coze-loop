// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTTFTAvgMetric Time To First Token平均值指标
type ModelTTFTAvgMetric struct{}

func (m *ModelTTFTAvgMetric) Name() string {
	return entity.MetricNameModelTTFTAvg
}

func (m *ModelTTFTAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTTFTAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTTFTAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "avg(tags_long['latency_first_resp'])/1000" // ms
}

func (m *ModelTTFTAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTTFTAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTTFTAvgMetric() entity.IMetricDefinition {
	return &ModelTTFTAvgMetric{}
}
