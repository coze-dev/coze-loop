// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	model_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/model"
	service_metric "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type AgentModelExecutionStepAvgMetric struct{}

func (m *AgentModelExecutionStepAvgMetric) Name() string {
	return entity.MetricNameAgentModelStepAvg
}

func (m *AgentModelExecutionStepAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *AgentModelExecutionStepAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *AgentModelExecutionStepAvgMetric) Expression(entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{}
}

func (m *AgentModelExecutionStepAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *AgentModelExecutionStepAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *AgentModelExecutionStepAvgMetric) GetMetrics() []entity.IMetricDefinition {
	return []entity.IMetricDefinition{
		wrapper.NewTimeSeriesWrapper().Wrap(model_metric.NewModelTotalCountMetric()),
		wrapper.NewTimeSeriesWrapper().Wrap(service_metric.NewServiceTraceCountMetric()),
	}
}

func (m *AgentModelExecutionStepAvgMetric) Operator() entity.MetricOperator {
	return entity.MetricOperatorDivide
}

func (m *AgentModelExecutionStepAvgMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewAgentModelExecutionStepAvgMetric() entity.IMetricDefinition {
	return &AgentModelExecutionStepAvgMetric{}
}
