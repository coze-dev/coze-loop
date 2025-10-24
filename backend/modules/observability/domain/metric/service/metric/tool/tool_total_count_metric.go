package tool

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ToolTotalCountMetric struct{}

func (m *ToolTotalCountMetric) Name() string {
	return entity.MetricNameToolTotalCount
}

func (m *ToolTotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolTotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolTotalCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return entity.NewExpression("count()")
}

func (m *ToolTotalCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ToolTotalCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func NewToolTotalCountMetric() entity.IMetricDefinition {
	return &ToolTotalCountMetric{}
}
