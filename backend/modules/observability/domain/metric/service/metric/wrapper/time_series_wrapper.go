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

type TimeSeriesWrapper struct {
	originalMetric entity.IMetricDefinition
}

func (a *TimeSeriesWrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	return &TimeSeriesWrapper{
		originalMetric: definition,
	}
}

func (a *TimeSeriesWrapper) Name() string {
	return fmt.Sprintf("%s_by_time", a.originalMetric.Name())
}

func (a *TimeSeriesWrapper) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (a *TimeSeriesWrapper) Source() entity.MetricSource {
	return a.originalMetric.Source()
}

func (a *TimeSeriesWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return a.originalMetric.Expression(granularity)
}

func (a *TimeSeriesWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return a.originalMetric.Where(ctx, f, env)
}

func (a *TimeSeriesWrapper) GroupBy() []*entity.Dimension {
	return a.originalMetric.GroupBy()
}

type TimeSeriesWrapperOption func(*TimeSeriesWrapper)

func NewTimeSeriesWrapper() entity.IMetricWrapper {
	return &TimeSeriesWrapper{}
}
