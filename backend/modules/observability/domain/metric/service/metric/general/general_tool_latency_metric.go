// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	tool_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralToolLatencyMetric struct {
	entity.MetricFillNull
}

func (m *GeneralToolLatencyMetric) Name() string {
	return entity.MetricNameGeneralToolLatencyAvg
}

func (m *GeneralToolLatencyMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralToolLatencyMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *GeneralToolLatencyMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *GeneralToolLatencyMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *GeneralToolLatencyMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralToolLatencyMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewSumWrapper().Wrap(tool_metric.NewToolDurationMetric()),
		tool_metric.NewToolTotalCountMetric(),
	}
}

func (m *GeneralToolLatencyMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *GeneralToolLatencyMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewGeneralToolLatencyMetric() entity.IMetricDefinition {
	return &GeneralToolLatencyMetric{}
}
