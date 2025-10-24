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

type Pct50Wrapper struct {
	originalMetric entity.IMetricDefinition
}

func (p *Pct50Wrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &Pct50Wrapper{
		originalMetric: definition,
	}
}

func (p *Pct50Wrapper) Name() string {
	return fmt.Sprintf("%s_pct50", p.originalMetric.Name())
}

func (p *Pct50Wrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (p *Pct50Wrapper) Source() entity.MetricSource {
	return p.originalMetric.Source()
}

func (p *Pct50Wrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	originExpr := p.originalMetric.Expression(granularity)
	return entity.NewExpression(fmt.Sprintf("quantile(0.5)(%s)", originExpr.Expression), originExpr.Fields...)
}

func (p *Pct50Wrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return p.originalMetric.Where(ctx, f, env)
}

func (p *Pct50Wrapper) GroupBy() []*entity.Dimension {
	return p.originalMetric.GroupBy()
}

func NewPct50Wrapper() entity.IMetricWrapper {
	return &Pct50Wrapper{}
}
