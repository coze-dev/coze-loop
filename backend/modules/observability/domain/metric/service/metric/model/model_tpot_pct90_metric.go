// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPOTPct90Metric Time Per Output Token 90分位指标
type ModelTPOTPct90Metric struct{}

func (m *ModelTPOTPct90Metric) Name() string {
	return entity.MetricNameModelTPOTPct90
}

func (m *ModelTPOTPct90Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPOTPct90Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPOTPct90Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.9)(tags_long['output_tokens'] / (duration-tags_long['latency_first_resp']))"
}

func (m *ModelTPOTPct90Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPOTPct90Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPOTPct90Metric() entity.IMetricDefinition {
	return &ModelTPOTPct90Metric{}
}