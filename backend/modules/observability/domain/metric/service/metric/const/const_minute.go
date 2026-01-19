// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ConstMinuteMetric struct {
	entity.IMetricConst
}

func (m *ConstMinuteMetric) Name() string {
	return "const_per_minute"
} // does not matter

func (m *ConstMinuteMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ConstMinuteMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ConstMinuteMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	expression := fmt.Sprintf("%d", entity.GranularityToSecond(granularity)/60)
	return &entity.Expression{
		Expression: expression,
	}
}

func (m *ConstMinuteMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *ConstMinuteMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ConstMinuteMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewConstMinuteMetric() entity.IMetricDefinition {
	return &ConstMinuteMetric{}
}
