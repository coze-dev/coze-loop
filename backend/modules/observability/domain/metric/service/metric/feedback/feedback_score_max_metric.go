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

type FeedbackScoreMaxMetric struct{}

func (m *FeedbackScoreMaxMetric) Name() string {
	return entity.MetricNameFeedbackScoreMax
}

func (m *FeedbackScoreMaxMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackScoreMaxMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackScoreMaxMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "max(value_float)"}
}

func (m *FeedbackScoreMaxMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackScoreMaxMetric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackScoreMaxMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func (m *FeedbackScoreMaxMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeMax,
	}
}

func NewFeedbackScoreMaxMetric() entity.IMetricDefinition {
	return &FeedbackScoreMaxMetric{}
}
