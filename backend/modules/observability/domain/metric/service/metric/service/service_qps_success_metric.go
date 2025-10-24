package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceQPSSuccessMetric struct{}

func (m *ServiceQPSSuccessMetric) Name() string {
	return entity.MetricNameServiceQPSSuccess
}

func (m *ServiceQPSSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPSSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPSSuccessMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	expression := fmt.Sprintf("countIf(1, status_code = 0)/%d", entity.GranularityToSecond(granularity))
	return &entity.Expression{
		Expression: expression,
		Fields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldStatusCode,
				FieldType: loop_span.FieldTypeLong,
			},
		},
	}
}

func (m *ServiceQPSSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPSSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewServiceQPSSuccessMetric() entity.IMetricDefinition {
	return &ServiceQPSSuccessMetric{}
}
