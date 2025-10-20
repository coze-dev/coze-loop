package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ServiceQPMAllMetric struct{}

func (m *ServiceQPMAllMetric) Name() string {
	return entity.MetricNameServiceQPMAll
}

func (m *ServiceQPMAllMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMAllMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPMAllMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("count()/%d", entity.GranularityToSecond(granularity)/60)
}

func (m *ServiceQPMAllMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMAllMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPMAllMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewServiceQPMAllMetric() entity.IMetricDefinition {
	return &ServiceQPMAllMetric{}
}

type ServiceQPMSuccessMetric struct{}

func (m *ServiceQPMSuccessMetric) Name() string {
	return entity.MetricNameServiceQPMSuccess
}

func (m *ServiceQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPMSuccessMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code = 0)/%d", entity.GranularityToSecond(granularity)/60)
}

func (m *ServiceQPMSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPMSuccessMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewServiceQPMSuccessMetric() entity.IMetricDefinition {
	return &ServiceQPMSuccessMetric{}
}

type ServiceQPMFailMetric struct{}

func (m *ServiceQPMFailMetric) Name() string {
	return entity.MetricNameServiceQPMFail
}

func (m *ServiceQPMFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ServiceQPMFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ServiceQPMFailMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code != 0)/%d", entity.GranularityToSecond(granularity)/60)
}

func (m *ServiceQPMFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *ServiceQPMFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ServiceQPMFailMetric) Wrappers() []entity.IMetricWrapper {
	return nil
}

func NewServiceQPMFailMetric() entity.IMetricDefinition {
	return &ServiceQPMFailMetric{}
}
