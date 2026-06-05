// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package feedback

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type FeedbackCountByKeyPieMetric struct{}

func (m *FeedbackCountByKeyPieMetric) Name() string {
	return entity.MetricNameFeedbackCountByKeyPie
}

func (m *FeedbackCountByKeyPieMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackCountByKeyPieMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackCountByKeyPieMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return nil
}

func (m *FeedbackCountByKeyPieMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FeedbackCountByKeyPieMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{
		{
			Field: &loop_span.FilterField{
				FieldName: "annotation_key",
				FieldType: loop_span.FieldTypeString,
			},
			Alias: "name",
		},
	}
}

func (m *FeedbackCountByKeyPieMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func (m *FeedbackCountByKeyPieMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType: entity.MetricOfflineAggrTypeSum,
	}
}

func NewFeedbackCountByKeyPieMetric() entity.IMetricDefinition {
	return &FeedbackCountByKeyPieMetric{}
}
