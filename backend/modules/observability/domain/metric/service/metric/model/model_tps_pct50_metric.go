// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPSPct50Metric Tokens Per Second 50分位指标
type ModelTPSPct50Metric struct{}

func (m *ModelTPSPct50Metric) Name() string {
	return entity.MetricNameModelTPSPct50
}

func (m *ModelTPSPct50Metric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPSPct50Metric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPSPct50Metric) Expression(granularity entity.MetricGranularity) string {
	return "quantile(0.5)((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 1000000))"
}

func (m *ModelTPSPct50Metric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPSPct50Metric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPSPct50Metric() entity.IMetricDefinition {
	return &ModelTPSPct50Metric{}
}
