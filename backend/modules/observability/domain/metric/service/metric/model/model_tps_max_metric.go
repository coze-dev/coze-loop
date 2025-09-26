// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPSMaxMetric Tokens Per Second最大值指标
type ModelTPSMaxMetric struct{}

func (m *ModelTPSMaxMetric) Name() string {
	return entity.MetricNameModelTPSMax
}

func (m *ModelTPSMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPSMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPSMaxMetric) Expression(granularity entity.MetricGranularity) string {
	return "max(sum(tags_long['input_tokens']+tags_long['output_tokens']) * 1000/sum(duration))"
}

func (m *ModelTPSMaxMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPSMaxMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPSMaxMetric() entity.IMetricDefinition {
	return &ModelTPSMaxMetric{}
}