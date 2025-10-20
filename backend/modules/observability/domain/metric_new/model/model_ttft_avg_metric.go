// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric_new/wrapper"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ModelTTFTAvgMetric struct{}

func (m *ModelTTFTAvgMetric) Name() string {
	return entity.MetricNameModelTTFT
}

func (m *ModelTTFTAvgMetric) Type() entity.MetricType {
	return entity.MetricTypeTimeSeries
}

func (m *ModelTTFTAvgMetric) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (m *ModelTTFTAvgMetric) Expression(granularity entity.MetricGranularity) string {
	return "tags_long['latency_first_resp']/1000"
}

func (m *ModelTTFTAvgMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter(ctx, env)
}

func (m *ModelTTFTAvgMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ModelTTFTAvgMetric) Wrappers() []entity.IMetricWrapper {
	return []entity.IMetricWrapper{
		wrapper.NewAvgWrapper(),
		wrapper.NewMinWrapper(),
		wrapper.NewMaxWrapper(),
		wrapper.NewPct50Wrapper(),
		wrapper.NewPct90Wrapper(),
		wrapper.NewPct99Wrapper(),
	}
}

func NewModelTTFTAvgMetric() entity.IMetricDefinition {
	return &ModelTTFTAvgMetric{}
}
