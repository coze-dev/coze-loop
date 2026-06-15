// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package feedback

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type FeedbackValueDistributionMetric struct{}

func (m *FeedbackValueDistributionMetric) Name() string {
	return entity.MetricNameFeedbackValueDistribution
}

func (m *FeedbackValueDistributionMetric) Type() entity.MetricType {
	return entity.MetricTypePie
}

func (m *FeedbackValueDistributionMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackValueDistributionMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *FeedbackValueDistributionMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: "value_type",
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"string", "bool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *FeedbackValueDistributionMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{
		{
			Field: &loop_span.FilterField{
				FieldName: "value_string",
				FieldType: loop_span.FieldTypeString,
			},
			Alias: "name",
		},
	}
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
