// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric_new/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceDurationAvgMetric struct{}

func (m *ServiceDurationAvgMetric) Name() string {
	return entity.MetricNameServiceDuration
}

func (m *ServiceDurationAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceDurationAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceDurationAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "duration/1000"
}

func (m *ServiceDurationAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceDurationAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceDurationAvgMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewAvgWrapper(),
		wrapper.NewMinWrapper(),
		wrapper.NewMaxWrapper(),
		wrapper.NewPct50Wrapper(),
		wrapper.NewPct90Wrapper(),
		wrapper.NewPct99Wrapper(),
	}
}

func NewServiceDurationAvgMetric() entity.IMetricDefinition {
	return &ServiceDurationAvgMetric{}
}
