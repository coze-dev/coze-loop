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

type FeedbackScoreMinMetric struct{}

func (m *FeedbackScoreMinMetric) Name() string {
	return entity.MetricNameFeedbackScoreMin
}

func (m *FeedbackScoreMinMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackScoreMinMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackScoreMinMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return nil
}

func (m *FeedbackScoreMinMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackScoreMinMetric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackScoreMinMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackScoreMinMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeMin,
	}
}

func NewFeedbackScoreMinMetric() entity.IMetricDefinition {
	return &FeedbackScoreMinMetric{}
}
