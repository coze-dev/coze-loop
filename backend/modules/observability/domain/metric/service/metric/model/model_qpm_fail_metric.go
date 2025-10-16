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

// ModelQPMFailMetric 模型失败 QPM 指标
type ModelQPMFailMetric struct{}

func (m *ModelQPMFailMetric) Name() string {
	return entity.MetricNameModelQPMFail
}

func (m *ModelQPMFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPMFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPMFailMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code != 0)/%d", entity.GranularityToSecond(granularity)/60)
}

func (m *ModelQPMFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPMFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPMFailMetric() entity.IMetricDefinition {
	return &ModelQPMFailMetric{}
}
