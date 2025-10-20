package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralModelLatencyMetric struct {
	entity.MetricFillNull
}

func (m *GeneralModelLatencyMetric) Name() string {
	return entity.MetricNameGeneralModelLatencyAvg
}

func (m *GeneralModelLatencyMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralModelLatencyMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralModelLatencyMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(duration) / (1000 * count())"
}

func (m *GeneralModelLatencyMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *GeneralModelLatencyMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralModelLatencyMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewGeneralModelLatencyMetric() entity.IMetricDefinition {
	return &GeneralModelLatencyMetric{}
}
