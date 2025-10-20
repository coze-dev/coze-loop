package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric_new/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceTraceMetric struct{}

func (m *ServiceTraceMetric) Name() string {
	return entity.MetricNameServiceTraceCountTotal
}

func (m *ServiceTraceMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ServiceTraceMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceTraceMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *ServiceTraceMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceTraceMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceTraceMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewTimeSeriesWrapper(wrapper.WithTimeSeriesName(entity.MetricNameServiceTraceCount)),
	}
}

func NewServiceTraceMetric() entity.IMetricDefinition {
	return &ServiceTraceMetric{}
}
