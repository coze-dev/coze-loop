package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceUserCountMetric struct{}

func (m *ServiceUserCountMetric) Name() string {
	return entity.MetricNameServiceUserCount
}

func (m *ServiceUserCountMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceUserCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceUserCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "uniq(tags_string['user_id'])"
}

func (m *ServiceUserCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildALLSpanFilter(ctx, env)
}

func (m *ServiceUserCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceUserCountMetric() entity.IMetricDefinition {
	return &ServiceUserCountMetric{}
}
