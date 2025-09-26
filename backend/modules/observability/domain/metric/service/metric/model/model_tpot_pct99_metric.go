// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPOTPct99Metric Time Per Output Token 99分位指标
type ModelTPOTPct99Metric struct{}

func (m *ModelTPOTPct99Metric) Name() string {
	return entity.MetricNameModelTPOTPct99
}

func (m *ModelTPOTPct99Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPOTPct99Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPOTPct99Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.99)(tags_long['output_tokens'] / (duration-tags_long['latency_first_resp']))"
}

func (m *ModelTPOTPct99Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPOTPct99Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPOTPct99Metric() entity.IMetricDefinition {
	return &ModelTPOTPct99Metric{}
}