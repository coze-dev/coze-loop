// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	tool_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralToolFailRatioMetric struct {
	entity.MetricFillNull
}

func (m *GeneralToolFailRatioMetric) Name() string {
	return entity.MetricNameGeneralToolFailRatio
}

func (m *GeneralToolFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralToolFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *GeneralToolFailRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *GeneralToolFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *GeneralToolFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralToolFailRatioMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		tool_metric.NewToolTotalErrorCountMetric(),
		tool_metric.NewToolTotalCountMetric(),
	}
}

func (m *GeneralToolFailRatioMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *GeneralToolFailRatioMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewGeneralToolFailRatioMetric() entity.IMetricDefinition {
	return &GeneralToolFailRatioMetric{}
}
