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

type Pct90Wrapper struct {
	originalMetric entity.IMetricDefinition
}

func (p *Pct90Wrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &Pct90Wrapper{
		originalMetric: definition,
	}
}

func (p *Pct90Wrapper) Name() string {
	return fmt.Sprintf("%s_pct90", p.originalMetric.Name())
}

func (p *Pct90Wrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (p *Pct90Wrapper) Source() entity.MetricSource {
	return p.originalMetric.Source()
}

func (p *Pct90Wrapper) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("quantile(0.9)(%s)", p.originalMetric.Expression(granularity))
}

func (p *Pct90Wrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return p.originalMetric.Where(ctx, f, env)
}

func (p *Pct90Wrapper) GroupBy() []*entity.Dimension {
	return p.originalMetric.GroupBy()
}

func NewPct90Wrapper() entity.IMetricWrapper {
	return &Pct90Wrapper{}
}
