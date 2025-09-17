// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/filter"
)

// ModelTotalTokensMetric 模型Tokens消耗指标
type ModelTotalTokensMetric struct{}

func (m *ModelTotalTokensMetric) Name() string {
	return "model_total_tokens"
}

func (m *ModelTotalTokensMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ModelTotalTokensMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTotalTokensMetric) Expression() string {
	return "sum(tags_long['input_tokens'] + tags_long['output_tokens'])"
}

func (m *ModelTotalTokensMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter()
}

func (m *ModelTotalTokensMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}