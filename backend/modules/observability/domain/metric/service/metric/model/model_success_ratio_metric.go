// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelSuccessRatioMetric struct {
	entity.MetricFillNull
}

func (m *ModelSuccessRatioMetric) Name() string {
	return entity.MetricNameModelSuccessRatio
}

func (m *ModelSuccessRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelSuccessRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ModelSuccessRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *ModelSuccessRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *ModelSuccessRatioMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(NewModelTotalSuccessCountMetric()),
		wrapper.NewTimeSeriesWrapper().Wrap(NewModelTotalCountMetric()),
	}
}

func (m *ModelSuccessRatioMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *ModelSuccessRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelSuccessRatioMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewModelSuccessRatioMetric() entity.IMetricDefinition {
	return &ModelSuccessRatioMetric{}
}
