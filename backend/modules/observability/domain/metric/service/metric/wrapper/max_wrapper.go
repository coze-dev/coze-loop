// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type MaxWrapper struct {
	originalMetric entity.IMetricDefinition
}

func (m *MaxWrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &MaxWrapper{
		originalMetric: definition,
	}
}

func (m *MaxWrapper) Name() string {
	return fmt.Sprintf("%s_max", m.originalMetric.Name())
}

func (m *MaxWrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *MaxWrapper) Source() entity.MetricSource {
	return m.originalMetric.Source()
}

func (m *MaxWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	originExpr := m.originalMetric.Expression(granularity)
	return &entity.Expression{
		Expression: fmt.Sprintf("max(%s)", originExpr.Expression),
		Fields:     originExpr.Fields,
	}
}

func (m *MaxWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return m.originalMetric.Where(ctx, f, env)
}

func (m *MaxWrapper) GroupBy() []*entity.Dimension {
	return m.originalMetric.GroupBy()
}

func (m *MaxWrapper) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeMax,
	}
}

func NewMaxWrapper() entity.IMetricWrapper {
	return &MaxWrapper{}
}
