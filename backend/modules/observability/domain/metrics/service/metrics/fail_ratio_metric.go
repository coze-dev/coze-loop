// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// FailRatioMetric Span错误率指标
type FailRatioMetric struct{}

func (m *FailRatioMetric) Name() string {
	return "fail_ratio"
}

func (m *FailRatioMetric) Type() string {
	return "summary"
}

func (m *FailRatioMetric) Source() string {
	return "ck"
}

func (m *FailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *FailRatioMetric) Where(filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	// 错误率指标不需要额外的筛选条件
	return nil, nil
}

func (m *FailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}