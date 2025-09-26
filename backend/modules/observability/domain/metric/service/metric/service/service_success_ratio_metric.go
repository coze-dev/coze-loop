// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceSuccessRatioMetric 服务请求成功率指标
type ServiceSuccessRatioMetric struct{}

func (m *ServiceSuccessRatioMetric) Name() string {
	return entity.MetricNameServiceSuccessRatio
}

func (m *ServiceSuccessRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceSuccessRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceSuccessRatioMetric) Expression(granularity entity.MetricGranularity) string {
	return "countIf(1, status_code = 0) / count()"
}

func (m *ServiceSuccessRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceSuccessRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceSuccessRatioMetric() entity.IMetricDefinition {
	return &ServiceSuccessRatioMetric{}
}