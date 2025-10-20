// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceTraceCountTotalMetric Trace上报量指标
type ServiceTraceCountTotalMetric struct{}

func (m *ServiceTraceCountTotalMetric) Name() string {
	return entity.MetricNameServiceTraceCountTotal
}

func (m *ServiceTraceCountTotalMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ServiceTraceCountTotalMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceTraceCountTotalMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *ServiceTraceCountTotalMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceTraceCountTotalMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceTraceCountTotalMetric() entity.IMetricDefinition {
	return &ServiceTraceCountTotalMetric{}
}