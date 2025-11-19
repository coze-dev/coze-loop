// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	consts "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/const"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
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
	return entity.MetricSourceInnerStorage
}

func (m *ModelQPMAllMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ModelQPMAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelQPMAllMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewModelTotalCountMetric()),
		consts.NewConstMinuteMetric(),
	}
}

func (m *ModelQPMAllMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ModelQPMAllMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewModelQPMAllMetric() entity.IMetricDefinition {
	return &ModelQPMAllMetric{}
}
