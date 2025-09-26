// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTTFTMinMetric Time To First Token最小值指标
type ModelTTFTMinMetric struct{}

func (m *ModelTTFTMinMetric) Name() string {
	return entity.MetricNameModelTTFTMin
}

func (m *ModelTTFTMinMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTTFTMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTTFTMinMetric) Expression(granularity entity.MetricGranularity) string {
	return "min(tags_long['latency_first_resp'])"
}

func (m *ModelTTFTMinMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTTFTMinMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTTFTMinMetric() entity.IMetricDefinition {
	return &ModelTTFTMinMetric{}
}