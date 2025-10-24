// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type MultiWrapper struct {
	wrappers []entity.IMetricWrapper
}

func (a *MultiWrapper) Wrap(definition entity.IMetricDefinition) entity.IMetricDefinition {
	for _, wrapper := range a.wrappers {
		definition = wrapper.Wrap(definition)
	}
	return definition
}

func (a *MultiWrapper) Name() string {
	return "not_supposed_to_be_here"
}

func (a *MultiWrapper) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (a *MultiWrapper) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (a *MultiWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (a *MultiWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (a *MultiWrapper) GroupBy() []*entity.Dimension {
	return nil
}

func NewMultiWrapper(wrappers ...entity.IMetricWrapper) entity.IMetricWrapper {
	return &MultiWrapper{
		wrappers: wrappers,
	}
}
