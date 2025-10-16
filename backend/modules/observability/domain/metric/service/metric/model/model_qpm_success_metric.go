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

// ModelQPMSuccessMetric 模型成功 QPM 指标
type ModelQPMSuccessMetric struct{}

func (m *ModelQPMSuccessMetric) Name() string {
	return entity.MetricNameModelQPMSuccess
}

func (m *ModelQPMSuccessMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMSuccessMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPMSuccessMetric) Expression(granularity entity.MetricGranularity) string {
	seconds := entity.GranularityToSecond(granularity) / 60
	if seconds == 0 {
		seconds = 1
	}
	return fmt.Sprintf("countIf(1, status_code = 0)/%d", seconds)
}

func (m *ModelQPMSuccessMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMSuccessMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPMSuccessMetric() entity.IMetricDefinition {
	return &ModelQPMSuccessMetric{}
}
