// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	service_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type AgentExecutionStepAvgMetric struct{}

func (m *AgentExecutionStepAvgMetric) Name() string {
	return entity.MetricNameAgentStepAvg
}

func (m *AgentExecutionStepAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *AgentExecutionStepAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *AgentExecutionStepAvgMetric) Expression(entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *AgentExecutionStepAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *AgentExecutionStepAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *AgentExecutionStepAvgMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(service_metric.NewServiceExecutionStepCountMetric()),
		wrapper.NewTimeSeriesWrapper().Wrap(service_metric.NewServiceTraceCountMetric()),
	}
}

func (m *AgentExecutionStepAvgMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *AgentExecutionStepAvgMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewAgentExecutionStepAvgMetric() entity.IMetricDefinition {
	return &AgentExecutionStepAvgMetric{}
}
