package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralTotalCountMetric struct{}

func (m *GeneralTotalCountMetric) Name() string {
	return entity.MetricNameGeneralTotalCount
}

func (m *GeneralTotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralTotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralTotalCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *GeneralTotalCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *GeneralTotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralTotalCountMetric() entity.IMetricDefinition {
	return &GeneralTotalCountMetric{}
}
