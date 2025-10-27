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

type ServiceQPMSuccessMetric struct{}

func (m *ServiceQPMSuccessMetric) Name() string {
	return entity.MetricNameServiceQPMSuccess
}

func (m *ServiceQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ServiceQPMSuccessMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	denominator := entity.GranularityToSecond(granularity) / 60
	expression := fmt.Sprintf("countIf(1, %%s = 0)/%d", denominator)
	return &entity.Expression{
		Expression: expression,
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
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
