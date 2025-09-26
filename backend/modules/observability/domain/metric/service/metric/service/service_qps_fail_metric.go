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

// ServiceQPSFailMetric 服务Fail QPS指标
type ServiceQPSFailMetric struct{}

func (m *ServiceQPSFailMetric) Name() string {
	return entity.MetricNameServiceQPSFail
}

func (m *ServiceQPSFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPSFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPSFailMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code != 0)/%s", granularity)
}

func (m *ServiceQPSFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPSFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceQPSFailMetric() entity.IMetricDefinition {
	return &ServiceQPSFailMetric{}
}