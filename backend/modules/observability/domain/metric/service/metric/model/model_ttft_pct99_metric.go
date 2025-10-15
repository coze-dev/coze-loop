// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTTFTPct99Metric Time To First Token 99分位指标
type ModelTTFTPct99Metric struct{}

func (m *ModelTTFTPct99Metric) Name() string {
	return entity.MetricNameModelTTFTPct99
}

func (m *ModelTTFTPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTTFTPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTTFTPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)(tags_long['latency_first_resp'])/1000"
}

func (m *ModelTTFTPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTTFTPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTTFTPct99Metric() entity.IMetricDefinition {
	return &ModelTTFTPct99Metric{}
}
