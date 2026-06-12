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

type FeedbackScoreMetric struct{}

func (m *FeedbackScoreMetric) Name() string {
	return entity.MetricNameFeedbackScore
}

func (m *FeedbackScoreMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FeedbackScoreMetric) Source() entity.MetricSource {
	return entity.MetricSourceAnnotation
}

func (m *FeedbackScoreMetric) Expression(_ entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "value_float"}
}

func (m *FeedbackScoreMetric) Where(_ context.Context, _ span_filter.Filter, _ *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: "value_type",
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"double", "long"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *FeedbackScoreMetric) GroupBy() []*entity.Dimension {
	return nil
}

func (m *FeedbackScoreMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewAvgWrapper(),
		wrapper.NewMinWrapper(),
		wrapper.NewMaxWrapper(),
		wrapper.NewPct50Wrapper(),
		wrapper.NewPct90Wrapper(),
		wrapper.NewPct99Wrapper(),
	}
}

func (m *FeedbackScoreMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewFeedbackScoreMetric() entity.IMetricDefinition {
	return &FeedbackScoreMetric{}
}
