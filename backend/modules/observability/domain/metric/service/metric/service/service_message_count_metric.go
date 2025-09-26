// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ServiceMessageCountMetric 消息数指标（仅SDK数据来源）
type ServiceMessageCountMetric struct{}

func (m *ServiceMessageCountMetric) Name() string {
	return entity.MetricNameServiceMessageCount
}

func (m *ServiceMessageCountMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceMessageCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceMessageCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "uniq(tags_string['message_id'])"
}

func (m *ServiceMessageCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceMessageCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceMessageCountMetric() entity.IMetricDefinition {
	return &ServiceMessageCountMetric{}
}