// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPOTPct50Metric Time Per Output Token 50分位指标
type ModelTPOTPct50Metric struct{}

func (m *ModelTPOTPct50Metric) Name() string {
	return entity.MetricNameModelTPOTPct50
}

func (m *ModelTPOTPct50Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPOTPct50Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPOTPct50Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.5)((duration-tags_long['latency_first_resp'])/(1000*tags_long['output_tokens']))"
}

func (m *ModelTPOTPct50Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPOTPct50Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPOTPct50Metric() entity.IMetricDefinition {
	return &ModelTPOTPct50Metric{}
}
