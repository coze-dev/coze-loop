package general

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type GeneralToolLatencyMetric struct {
	entity.MetricFillNull
}

func (m *GeneralToolLatencyMetric) Name() string {
	return entity.MetricNameGeneralToolLatencyAvg
}

func (m *GeneralToolLatencyMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *GeneralToolLatencyMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *GeneralToolLatencyMetric) Expression(granularity entity.MetricGranularity) string {
	return "sum(duration) / (1000 * count())"
}

func (m *GeneralToolLatencyMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *GeneralToolLatencyMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *GeneralToolLatencyMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewGeneralToolLatencyMetric() entity.IMetricDefinition {
	return &GeneralToolLatencyMetric{}
}
