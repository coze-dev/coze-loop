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

type ModelQPMSuccessMetric struct{}

func (m *ModelQPMSuccessMetric) Name() string {
	return entity.MetricNameModelQPMSuccess
}

func (m *ModelQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelQPMSuccessMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ModelQPMSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelQPMSuccessMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewModelTotalSuccessCountMetric()),
		consts.NewConstMinuteMetric(),
	}
}

func (m *ModelQPMSuccessMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ModelQPMSuccessMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewModelQPMSuccessMetric() entity.IMetricDefinition {
	return &ModelQPMSuccessMetric{}
}
