// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelTPMMaxMetric Tokens Per Minute最大值指标
type ModelTPMMaxMetric struct{}

func (m *ModelTPMMaxMetric) Name() string {
	return entity.MetricNameModelTPMMax
}

func (m *ModelTPMMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTPMMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTPMMaxMetric) Expression(granularity entity.MetricGranularity) string {
	return "max((tags_long['input_tokens']+tags_long['output_tokens'])/(duration / 60000000))"
}

func (m *ModelTPMMaxMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTPMMaxMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelTPMMaxMetric() entity.IMetricDefinition {
	return &ModelTPMMaxMetric{}
}
