// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelQPSSuccessMetric 模型成功 QPS 指标
type ModelQPSSuccessMetric struct{}

func (m *ModelQPSSuccessMetric) Name() string {
	return entity.MetricNameModelQPSSuccess
}

func (m *ModelQPSSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPSSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPSSuccessMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code = 0)/%d", entity.GranularityToSecond(granularity))
}

func (m *ModelQPSSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPSSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPSSuccessMetric() entity.IMetricDefinition {
	return &ModelQPSSuccessMetric{}
}
