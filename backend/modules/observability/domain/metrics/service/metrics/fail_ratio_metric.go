// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

// FailRatioMetric Span错误率指标
type FailRatioMetric struct{}

func (m *FailRatioMetric) Name() string {
	return "fail_ratio"
}

func (m *FailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *FailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *FailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *FailRatioMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}