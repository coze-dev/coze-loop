package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceSpanCountMetric struct{}

func (m *ServiceSpanCountMetric) Name() string {
	return entity.MetricNameServiceSpanCount
}

func (m *ServiceSpanCountMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceSpanCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceSpanCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *ServiceSpanCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildALLSpanFilter(ctx, env)
}

func (m *ServiceSpanCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceSpanCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func NewServiceSpanCountMetric() entity.IMetricDefinition {
	return &ServiceSpanCountMetric{}
}
