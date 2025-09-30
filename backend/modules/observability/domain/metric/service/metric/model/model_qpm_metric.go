// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelQPMMetric 模型QPM指标
type ModelQPMMetric struct{}

func (m *ModelQPMMetric) Name() string {
	return entity.MetricNameModelQPM
}

func (m *ModelQPMMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPMMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("count()/%d", 60*entity.GranularityToSecond(granularity))
}

func (m *ModelQPMMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPMMetric() entity.IMetricDefinition {
	return &ModelQPMMetric{}
}
