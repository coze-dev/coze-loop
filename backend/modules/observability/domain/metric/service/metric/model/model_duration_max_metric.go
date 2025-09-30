// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelDurationMaxMetric 模型调用总耗时最大值指标
type ModelDurationMaxMetric struct{}

func (m *ModelDurationMaxMetric) Name() string {
	return entity.MetricNameModelDurationMax
}

func (m *ModelDurationMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelDurationMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelDurationMaxMetric) Expression(granularity entity.MetricGranularity) string {
	return "max(duration/1000)"
}

func (m *ModelDurationMaxMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelDurationMaxMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelDurationMaxMetric() entity.IMetricDefinition {
	return &ModelDurationMaxMetric{}
}
