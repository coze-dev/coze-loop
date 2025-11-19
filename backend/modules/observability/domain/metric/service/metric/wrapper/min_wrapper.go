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

type MinWrapper struct {
	originalMetric entity.IMetricDefinition
}

func (m *MinWrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &MinWrapper{
		originalMetric: definition,
	}
}

func (m *MinWrapper) Name() string {
	return fmt.Sprintf("%s_min", m.originalMetric.Name())
}

func (m *MinWrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *MinWrapper) Source() entity.MetricSource {
	return m.originalMetric.Source()
}

func (m *MinWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	originExpr := m.originalMetric.Expression(granularity)
	return &entity.Expression{
		Expression: fmt.Sprintf("min(%s)", originExpr.Expression),
		Fields:     originExpr.Fields,
	}
}

func (m *MinWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return m.originalMetric.Where(ctx, f, env)
}

func (m *MinWrapper) GroupBy() []*entity.Dimension {
	return m.originalMetric.GroupBy()
}

func (m *MinWrapper) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeMin,
	}
}

func NewMinWrapper() entity.IMetricWrapper {
	return &MinWrapper{}
}
