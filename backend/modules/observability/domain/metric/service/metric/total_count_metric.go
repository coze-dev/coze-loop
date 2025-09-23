// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// TotalCountMetric 使用次数指标
type TotalCountMetric struct{}

func (m *TotalCountMetric) Name() string {
	return entity.MetricNameTotalCount
}

func (m *TotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *TotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *TotalCountMetric) Expression(granularity entity.MetricGranularity) string {
	return "count()"
}

func (m *TotalCountMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	// 需要RootSpan筛选
	return filter.BuildRootSpanFilter(ctx, env)
}

func (m *TotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}
