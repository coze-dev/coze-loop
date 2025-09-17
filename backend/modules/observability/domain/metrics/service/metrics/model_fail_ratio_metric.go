// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

// ModelFailRatioMetric 模型调用错误率指标
type ModelFailRatioMetric struct{}

func (m *ModelFailRatioMetric) Name() string {
	return "model_fail_ratio"
}

func (m *ModelFailRatioMetric) Type() string {
	return "summary"
}

func (m *ModelFailRatioMetric) Source() string {
	return "ck"
}

func (m *ModelFailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ModelFailRatioMetric) Where(filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(context.Background(), env)
}

func (m *ModelFailRatioMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}