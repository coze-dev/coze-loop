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

type FeedbackScoreP50Metric struct{}

func (m *FeedbackScoreP50Metric) Name() string {
	return entity.MetricNameFeedbackScoreP50
}

func (m *FeedbackScoreP50Metric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackScoreP50Metric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackScoreP50Metric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "quantile(0.5)(value_float)"}
}

func (m *FeedbackScoreP50Metric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackScoreP50Metric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackScoreP50Metric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackScoreP50Metric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeAvg,
	}
}

func NewFeedbackScoreP50Metric() entity.IMetricDefinition {
	return &FeedbackScoreP50Metric{}
}
