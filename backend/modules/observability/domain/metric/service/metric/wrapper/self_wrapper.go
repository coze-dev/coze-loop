// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package wrapper

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type SelfWrapper struct{}

func (a *SelfWrapper) Wrap(in entity.IMetricDefinition) entity.IMetricDefinition {
	return in
}

func (a *SelfWrapper) Name() string {
	return ""
}

func (a *SelfWrapper) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (a *SelfWrapper) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (a *SelfWrapper) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (a *SelfWrapper) Where(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (a *SelfWrapper) GroupBy() []*entity.Dimension {
	return nil
}

func NewSelfWrapper() entity.IMetricWrapper {
	return &SelfWrapper{}
}
