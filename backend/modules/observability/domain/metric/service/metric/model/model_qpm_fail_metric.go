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

type ModelQPMFailMetric struct{}

func (m *ModelQPMFailMetric) Name() string {
	return entity.MetricNameModelQPMFail
}

func (m *ModelQPMFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelQPMFailMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ModelQPMFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelQPMFailMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewModelTotalErrorCountMetricc()),
		consts.NewConstMinuteMetric(),
	}
}

func (m *ModelQPMFailMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ModelQPMFailMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewModelQPMFailMetric() entity.IMetricDefinition {
	return &ModelQPMFailMetric{}
}
