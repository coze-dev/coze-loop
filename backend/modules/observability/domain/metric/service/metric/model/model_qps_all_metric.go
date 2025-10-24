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

type ModelQPSAllMetric struct{}

func (m *ModelQPSAllMetric) Name() string {
	return entity.MetricNameModelQPSAll
}

func (m *ModelQPSAllMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPSAllMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPSAllMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	expression := fmt.Sprintf("count()/%d", entity.GranularityToSecond(granularity))
	return entity.NewExpression(expression)
}

func (m *ModelQPSAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPSAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPSAllMetric() entity.IMetricDefinition {
	return &ModelQPSAllMetric{}
}
