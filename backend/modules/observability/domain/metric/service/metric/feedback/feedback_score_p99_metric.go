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

type FeedbackScoreP99Metric struct{}

func (m *FeedbackScoreP99Metric) Name() string {
	return entity.MetricNameFeedbackScoreP99
}

func (m *FeedbackScoreP99Metric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackScoreP99Metric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackScoreP99Metric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return nil
}

func (m *FeedbackScoreP99Metric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackScoreP99Metric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackScoreP99Metric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackScoreP99Metric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewFeedbackScoreP99Metric() entity.IMetricDefinition {
	return &FeedbackScoreP99Metric{}
}
