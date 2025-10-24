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

type ModelQPMAllMetric struct{}

func (m *ModelQPMAllMetric) Name() string {
	return entity.MetricNameModelQPMAll
}

func (m *ModelQPMAllMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMAllMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPMAllMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	expression := fmt.Sprintf("count()/%d", entity.GranularityToSecond(granularity)/60)
	return &entity.Expression{Expression: expression}
}

func (m *ModelQPMAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPMAllMetric() entity.IMetricDefinition {
	return &ModelQPMAllMetric{}
}
