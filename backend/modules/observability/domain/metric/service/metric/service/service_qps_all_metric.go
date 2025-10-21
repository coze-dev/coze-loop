package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceQPSAllMetric struct{}

func (m *ServiceQPSAllMetric) Name() string {
	return entity.MetricNameServiceQPSAll
}

func (m *ServiceQPSAllMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPSAllMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPSAllMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("count()/%d", entity.GranularityToSecond(granularity))
}

func (m *ServiceQPSAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPSAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceQPSAllMetric() entity.IMetricDefinition {
	return &ServiceQPSAllMetric{}
}
