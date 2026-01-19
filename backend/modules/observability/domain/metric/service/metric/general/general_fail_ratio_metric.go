// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	service_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralFailRatioMetric struct {
	entity.MetricFillNull
}

// Span错误率
func (m *GeneralFailRatioMetric) Name() string {
	return entity.MetricNameGeneralFailRatio
}

func (m *GeneralFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *GeneralFailRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *GeneralFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *GeneralFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralFailRatioMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		service_metric.NewServiceSpanErrorCountMetric(),
		service_metric.NewServiceSpanCountMetric(),
	}
}

func (m *GeneralFailRatioMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *GeneralFailRatioMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewGeneralFailRatioMetric() entity.IMetricDefinition {
	return &GeneralFailRatioMetric{}
}
