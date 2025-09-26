// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTTFTMaxMetric Time To First Token最大值指标
type ModelTTFTMaxMetric struct{}

func (m *ModelTTFTMaxMetric) Name() string {
	return entity.MetricNameModelTTFTMax
}

func (m *ModelTTFTMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTTFTMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTTFTMaxMetric) Expression(granularity entity.MetricGranularity) string {
	return "max(tags_long['latency_first_resp'])"
}

func (m *ModelTTFTMaxMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTTFTMaxMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTTFTMaxMetric() entity.IMetricDefinition {
	return &ModelTTFTMaxMetric{}
}