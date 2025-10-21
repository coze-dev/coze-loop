package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralModelFailRatioMetric struct {
	entity.MetricFillNull
}

func (m *GeneralModelFailRatioMetric) Name() string {
	return entity.MetricNameGeneralModelFailRatio
}

func (m *GeneralModelFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralModelFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralModelFailRatioMetric) Expression(granularity entity.MetricGranularity) string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *GeneralModelFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *GeneralModelFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralModelFailRatioMetric() entity.IMetricDefinition {
	return &GeneralModelFailRatioMetric{}
}
