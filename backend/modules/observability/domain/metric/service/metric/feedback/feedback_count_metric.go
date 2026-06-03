// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package feedback

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type FeedbackCountMetric struct{}

func (m *FeedbackCountMetric) Name() string {
	return entity.MetricNameFeedbackCount
}

func (m *FeedbackCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackCountMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return nil
}

func (m *FeedbackCountMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackCountMetric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackCountMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewFeedbackCountMetric() entity.IMetricDefinition {
	return &FeedbackCountMetric{}
}
