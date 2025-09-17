// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/filter"
)

// ModelFailRatioMetric 模型调用错误率指标
type ModelFailRatioMetric struct{}

func (m *ModelFailRatioMetric) Name() string {
	return "model_fail_ratio"
}

func (m *ModelFailRatioMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelFailRatioMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelFailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ModelFailRatioMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter()
}

func (m *ModelFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}