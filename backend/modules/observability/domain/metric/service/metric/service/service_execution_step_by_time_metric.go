// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ServiceExecutionStepCountByTimeMetric struct{}

func (m *ServiceExecutionStepCountByTimeMetric) Name() string {
	return entity.MetricNameServiceExecutionStepCount
}

func (m *ServiceExecutionStepCountByTimeMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceExecutionStepCountByTimeMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceExecutionStepCountByTimeMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *ServiceExecutionStepCountByTimeMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool", "model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ServiceExecutionStepCountByTimeMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceExecutionStepCountByTimeMetric() entity.IMetricDefinition {
	return &ServiceExecutionStepCountByTimeMetric{}
}
