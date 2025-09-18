// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// TotalCountMetric 使用次数指标
type TotalCountMetric struct{}

func (m *TotalCountMetric) Name() string {
	return entity.MetricNameTotalCount
}

func (m *TotalCountMetric) Type() string {
	return string(entity.MetricTypeSummary)
}

func (m *TotalCountMetric) Source() string {
	return string(entity.MetricSourceCK)
}

func (m *TotalCountMetric) Expression() string {
	return "count()"
}

func (m *TotalCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	// 需要RootSpan筛选
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *TotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}