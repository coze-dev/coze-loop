// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

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

type AvgWrapper struct {
	originalMetric entity.IMetricDefinition
}

func (a *AvgWrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &AvgWrapper{
		originalMetric: definition,
	}
}

func (a *AvgWrapper) Name() string {
	return fmt.Sprintf("%s_avg", a.originalMetric.Name())
}

func (a *AvgWrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (a *AvgWrapper) Source() entity.MetricSource {
	return a.originalMetric.Source()
}

func (a *AvgWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	originExpr := a.originalMetric.Expression(granularity)
	return &entity.Expression{
		Expression: fmt.Sprintf("avg(%s)", originExpr.Expression),
		Fields:     originExpr.Fields,
	}
}

func (a *AvgWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return a.originalMetric.Where(ctx, f, env)
}

func (a *AvgWrapper) GroupBy() []*entity.Dimension {
	return a.originalMetric.GroupBy()
}

func NewAvgWrapper() entity.IMetricWrapper {
	return &AvgWrapper{}
}
