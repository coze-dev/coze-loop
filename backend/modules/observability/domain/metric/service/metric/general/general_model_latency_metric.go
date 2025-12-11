// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	model_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralModelLatencyMetric struct {
	entity.MetricFillNull
}

func (m *GeneralModelLatencyMetric) Name() string {
	return entity.MetricNameGeneralModelLatencyAvg
}

func (m *GeneralModelLatencyMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralModelLatencyMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *GeneralModelLatencyMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *GeneralModelLatencyMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *GeneralModelLatencyMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralModelLatencyMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewSumWrapper().Wrap(model_metric.NewModelDurationMetric()),
		model_metric.NewModelTotalCountMetric(),
	}
}

func (m *GeneralModelLatencyMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *GeneralModelLatencyMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewGeneralModelLatencyMetric() entity.IMetricDefinition {
	return &GeneralModelLatencyMetric{}
}
