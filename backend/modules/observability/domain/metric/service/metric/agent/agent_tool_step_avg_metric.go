// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	service_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	tool_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/tool"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type AgentToolExecutionStepAvgMetric struct{}

func (m *AgentToolExecutionStepAvgMetric) Name() string {
	return entity.MetricNameAgentToolStepAvg
}

func (m *AgentToolExecutionStepAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *AgentToolExecutionStepAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *AgentToolExecutionStepAvgMetric) Expression(entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *AgentToolExecutionStepAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *AgentToolExecutionStepAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *AgentToolExecutionStepAvgMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(tool_metric.NewToolTotalCountMetric()),
		wrapper.NewTimeSeriesWrapper().Wrap(service_metric.NewServiceTraceCountMetric()),
	}
}

func (m *AgentToolExecutionStepAvgMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *AgentToolExecutionStepAvgMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewAgentToolExecutionStepAvgMetric() entity.IMetricDefinition {
	return &AgentToolExecutionStepAvgMetric{}
}
