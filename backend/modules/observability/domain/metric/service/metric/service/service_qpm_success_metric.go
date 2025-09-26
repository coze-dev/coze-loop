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

// ServiceQPMSuccessMetric 服务Success QPM指标
type ServiceQPMSuccessMetric struct{}

func (m *ServiceQPMSuccessMetric) Name() string {
	return entity.MetricNameServiceQPMSuccess
}

func (m *ServiceQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPMSuccessMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code = 0) * 60/%s", granularity)
}

func (m *ServiceQPMSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceQPMSuccessMetric() entity.IMetricDefinition {
	return &ServiceQPMSuccessMetric{}
}