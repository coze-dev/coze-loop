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

type FeedbackValueDistributionMetric struct{}

func (m *FeedbackValueDistributionMetric) Name() string {
	return entity.MetricNameFeedbackValueDistribution
}

func (m *FeedbackValueDistributionMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackValueDistributionMetric) Source() entity.MetricSource {
	return entity.MetricSourceOfflineOnly
}

func (m *FeedbackValueDistributionMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return nil
}

func (m *FeedbackValueDistributionMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackValueDistributionMetric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackValueDistributionMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackValueDistributionMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewFeedbackValueDistributionMetric() entity.IMetricDefinition {
	return &FeedbackValueDistributionMetric{}
}
