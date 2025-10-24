package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceQPMAllMetric struct{}

func (m *ServiceQPMAllMetric) Name() string {
	return entity.MetricNameServiceQPMAll
}

func (m *ServiceQPMAllMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMAllMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPMAllMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	expression := fmt.Sprintf("count()/%d", entity.GranularityToSecond(granularity)/60)
	return &entity.Expression{Expression: expression}
}

func (m *ServiceQPMAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceQPMAllMetric() entity.IMetricDefinition {
	return &ServiceQPMAllMetric{}
}
