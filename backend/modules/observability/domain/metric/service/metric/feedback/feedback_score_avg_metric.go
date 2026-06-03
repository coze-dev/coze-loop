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

type FeedbackScoreAvgMetric struct{}

func (m *FeedbackScoreAvgMetric) Name() string {
	return entity.MetricNameFeedbackScoreAvg
}

func (m *FeedbackScoreAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackScoreAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceOfflineOnly
}

func (m *FeedbackScoreAvgMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return nil
}

func (m *FeedbackScoreAvgMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackScoreAvgMetric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackScoreAvgMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackScoreAvgMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewFeedbackScoreAvgMetric() entity.IMetricDefinition {
	return &FeedbackScoreAvgMetric{}
}
