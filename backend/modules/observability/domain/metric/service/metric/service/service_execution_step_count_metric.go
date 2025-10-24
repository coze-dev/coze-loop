package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type ServiceExecutionStepCountMetric struct{}

func (m *ServiceExecutionStepCountMetric) Name() string {
	return entity.MetricNameServiceExecutionStepCount
}

func (m *ServiceExecutionStepCountMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceExecutionStepCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceExecutionStepCountMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (m *ServiceExecutionStepCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool", "model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ServiceExecutionStepCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceExecutionStepCountMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewSelfWrapper(),
		wrapper.NewTimeSeriesWrapper(),
	}
}

func NewServiceExecutionStepCountMetric() entity.IMetricDefinition {
	return &ServiceExecutionStepCountMetric{}
}
