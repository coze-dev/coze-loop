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

type SumWrapper struct {
	originalMetric entity.IMetricDefinition
}

func (a *SumWrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &SumWrapper{
		originalMetric: definition,
	}
}

func (a *SumWrapper) Name() string {
	return fmt.Sprintf("%s_sum", a.originalMetric.Name())
}

func (a *SumWrapper) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (a *SumWrapper) Source() entity.MetricSource {
	return a.originalMetric.Source()
}

func (a *SumWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	originExpr := a.originalMetric.Expression(granularity)
	return &entity.Expression{
		Expression: fmt.Sprintf("sum(%s)", originExpr.Expression),
		Fields:     originExpr.Fields,
	}
}

func (a *SumWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return a.originalMetric.Where(ctx, f, env)
}

func (a *SumWrapper) GroupBy() []*entity.Dimension {
	return a.originalMetric.GroupBy()
}

func (a *SumWrapper) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewSumWrapper() entity.IMetricWrapper {
	return &SumWrapper{}
}
