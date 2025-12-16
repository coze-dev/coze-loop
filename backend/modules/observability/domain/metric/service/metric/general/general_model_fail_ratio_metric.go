// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	model_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralModelFailRatioMetric struct {
	entity.MetricFillNull
}

func (m *GeneralModelFailRatioMetric) Name() string {
	return entity.MetricNameGeneralModelFailRatio
}

func (m *GeneralModelFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralModelFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *GeneralModelFailRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *GeneralModelFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *GeneralModelFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralModelFailRatioMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		model_metric.NewModelTotalErrorCountMetricc(),
		model_metric.NewModelTotalCountMetric(),
	}
}

func (m *GeneralModelFailRatioMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *GeneralModelFailRatioMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewGeneralModelFailRatioMetric() entity.IMetricDefinition {
	return &GeneralModelFailRatioMetric{}
}
