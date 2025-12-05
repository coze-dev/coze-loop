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

type Pct99Wrapper struct {
	originalMetric entity.IMetricDefinition
}

func (p *Pct99Wrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &Pct99Wrapper{
		originalMetric: definition,
	}
}

func (p *Pct99Wrapper) Name() string {
	return fmt.Sprintf("%s_pct99", p.originalMetric.Name())
}

func (p *Pct99Wrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (p *Pct99Wrapper) Source() entity.MetricSource {
	return p.originalMetric.Source()
}

func (p *Pct99Wrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	originExpr := p.originalMetric.Expression(granularity)
	return &entity.Expression{
		Expression: fmt.Sprintf("quantile(0.99)(%s)", originExpr.Expression),
		Fields:     originExpr.Fields,
	}
}

func (p *Pct99Wrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return p.originalMetric.Where(ctx, f, env)
}

func (p *Pct99Wrapper) GroupBy() []*entity.Dimension {
	return p.originalMetric.GroupBy()
}

func (p *Pct99Wrapper) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewPct99Wrapper() entity.IMetricWrapper {
	return &Pct99Wrapper{}
}
