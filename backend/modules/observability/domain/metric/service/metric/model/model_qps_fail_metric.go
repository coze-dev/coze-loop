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

// ModelQPSFailMetric 模型失败 QPS 指标
type ModelQPSFailMetric struct{}

func (m *ModelQPSFailMetric) Name() string {
	return entity.MetricNameModelQPSFail
}

func (m *ModelQPSFailMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelQPSFailMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelQPSFailMetric) Expression(granularity entity.MetricGranularity) string {
	return fmt.Sprintf("countIf(1, status_code != 0)/%d", entity.GranularityToSecond(granularity))
}

func (m *ModelQPSFailMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelQPSFailMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func NewModelQPSFailMetric() entity.IMetricDefinition {
	return &ModelQPSFailMetric{}
}
