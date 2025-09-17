// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/filter"
)

// TotalCountMetric 使用次数指标
type TotalCountMetric struct{}

func (m *TotalCountMetric) Name() string {
	return "total_count"
}

func (m *TotalCountMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *TotalCountMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *TotalCountMetric) Expression() string {
	return "count()"
}

func (m *TotalCountMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	// 需要RootSpan筛选
	return filter.BuildRootSpanFilter()
}

func (m *TotalCountMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}