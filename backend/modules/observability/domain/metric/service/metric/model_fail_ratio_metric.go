// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelFailRatioMetric 模型调用错误率指标
type ModelFailRatioMetric struct{}

func (m *ModelFailRatioMetric) Name() entity.MetricName {
	return entity.MetricNameModelFailRatio
}

func (m *ModelFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelFailRatioMetric) Source() string {
	return string(entity.MetricSourceCK)
}

func (m *ModelFailRatioMetric) Expression(granularity entity.MetricGranularity) string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ModelFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}