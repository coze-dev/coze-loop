package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type GeneralFailRatioMetric struct {
	entity.MetricFillNull
}

func (m *GeneralFailRatioMetric) Name() string {
	return entity.MetricNameGeneralFailRatio
}

func (m *GeneralFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralFailRatioMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return entity.NewExpression(
		"countIf(1, status_code != 0) / count()",
		entity.NewLongField(loop_span.SpanFieldStatusCode),
	)
}

func (m *GeneralFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildALLSpanFilter(ctx, env)
}

func (m *GeneralFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewGeneralFailRatioMetric() entity.IMetricDefinition {
	return &GeneralFailRatioMetric{}
}
