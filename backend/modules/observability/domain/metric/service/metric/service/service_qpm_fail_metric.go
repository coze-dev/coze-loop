// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceQPMFailMetric 服务Fail QPM指标
type ServiceQPMFailMetric struct{}

func (m *ServiceQPMFailMetric) Name() string {
	return entity.MetricNameServiceQPMFail
}

func (m *ServiceQPMFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPMFailMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code != 0) * 60/%d", entity.GranularityToSecond(granularity))
}

func (m *ServiceQPMFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceQPMFailMetric() entity.IMetricDefinition {
	return &ServiceQPMFailMetric{}
}