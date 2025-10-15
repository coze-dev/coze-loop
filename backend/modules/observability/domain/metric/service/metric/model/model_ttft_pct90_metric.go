// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTTFTPct90Metric Time To First Token 90分位指标
type ModelTTFTPct90Metric struct{}

func (m *ModelTTFTPct90Metric) Name() string {
	return entity.MetricNameModelTTFTPct90
}

func (m *ModelTTFTPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTTFTPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTTFTPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)(tags_long['latency_first_resp'])/1000"
}

func (m *ModelTTFTPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTTFTPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTTFTPct90Metric() entity.IMetricDefinition {
	return &ModelTTFTPct90Metric{}
}
