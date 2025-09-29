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

// ModelQPSMetric 模型QPS指标
type ModelQPSMetric struct{}

func (m *ModelQPSMetric) Name() string {
	return entity.MetricNameModelQPS
}

func (m *ModelQPSMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPSMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPSMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("count()/%d", entity.GranularityToSecond(granularity))
}

func (m *ModelQPSMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPSMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPSMetric() entity.IMetricDefinition {
	return &ModelQPSMetric{}
}