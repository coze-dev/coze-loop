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

type ModelQPSFailMetric struct{}

func (m *ModelQPSFailMetric) Name() string {
	return entity.MetricNameModelQPSFail
}

func (m *ModelQPSFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPSFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelQPSFailMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ModelQPSFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPSFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelQPSFailMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewModelTotalErrorCountMetricc()),
		consts.NewConstSecondMetric(),
	}
}

func (m *ModelQPSFailMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ModelQPSFailMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewModelQPSFailMetric() entity.IMetricDefinition {
	return &ModelQPSFailMetric{}
}
