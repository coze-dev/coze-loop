package tool

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ToolSuccessRatioMetric struct {
	entity.MetricFillNull
}

func (m *ToolSuccessRatioMetric) Name() string {
	return entity.MetricNameToolSuccessRatio
}

func (m *ToolSuccessRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ToolSuccessRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolSuccessRatioMetric) Expression(granularity entity.MetricGranularity) string {
	return "countIf(1, status_code = 0) / count()"
}

func (m *ToolSuccessRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolSuccessRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ToolSuccessRatioMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewToolSuccessRatioMetric() entity.IMetricDefinition {
	return &ToolSuccessRatioMetric{}
}
