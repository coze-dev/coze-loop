// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package feedback

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type FeedbackCountBySourcePieMetric struct{}

func (m *FeedbackCountBySourcePieMetric) Name() string {
	return entity.MetricNameFeedbackCountBySourcePie
}

func (m *FeedbackCountBySourcePieMetric) Type() entity.MetricType {
	return entity.MetricTypePie
}

func (m *FeedbackCountBySourcePieMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackCountBySourcePieMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *FeedbackCountBySourcePieMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackCountBySourcePieMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{
		{
			Field: &loop_span.FilterField{
				FieldName: "feedback_source",
				FieldType: loop_span.FieldTypeString,
			},
			Alias: "name",
		},
	}
}

func (m *FeedbackCountBySourcePieMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewFeedbackCountBySourcePieMetric() entity.IMetricDefinition {
	return &FeedbackCountBySourcePieMetric{}
}
