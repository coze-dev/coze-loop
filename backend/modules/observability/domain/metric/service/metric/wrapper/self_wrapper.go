// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type SelfWrapper struct {
	originalMetric entity.IMetricDefinition
}

func (a *SelfWrapper) Wrap(entity.IMetricDefinition) entity.IMetricDefinition {
	return a.originalMetric
}

func (a *SelfWrapper) Name() string {
	return a.originalMetric.Name()
}

func (a *SelfWrapper) Type() entity.MetricType {
	return a.originalMetric.Type()
}

func (a *SelfWrapper) Source() entity.MetricSource {
	return a.originalMetric.Source()
}

func (a *SelfWrapper) Expression(granularity entity.MetricGranularity) string {
	return a.originalMetric.Expression(granularity)
}

func (a *SelfWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return a.originalMetric.Where(ctx, f, env)
}

func (a *SelfWrapper) GroupBy() []*entity.Dimension {
	return a.originalMetric.GroupBy()
}

func (a *SelfWrapper) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewSelfWrapper() entity.IMetricWrapper {
	return &SelfWrapper{}
}
