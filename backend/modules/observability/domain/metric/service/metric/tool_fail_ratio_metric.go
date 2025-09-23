// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// ToolFailRatioMetric 工具调用错误率指标
type ToolFailRatioMetric struct{}

func (m *ToolFailRatioMetric) Name() string {
	return entity.MetricNameToolFailRatio
}

func (m *ToolFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ToolFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ToolFailRatioMetric) Expression(granularity entity.MetricGranularity) string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ToolFailRatioMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	// 直接返回工具Span筛选条件，不使用Filter接口，因为这是固定的筛选逻辑
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}
