// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type ConstSecondMetric struct {
	entity.IMetricConst
}

func (m *ConstSecondMetric) Name() string {
	return "const_per_second"
} // does not matter

func (m *ConstSecondMetric) Type() entity.MetricType {
	return entity.MetricTypeSummary
}

func (m *ConstSecondMetric) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (m *ConstSecondMetric) Expression(granularity entity.MetricGranularity) *entity.Expression {
	expression := fmt.Sprintf("%d", entity.GranularityToSecond(granularity))
	return &entity.Expression{
		Expression: expression,
	}
}

func (m *ConstSecondMetric) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *ConstSecondMetric) GroupBy() []*entity.Dimension {
	return []*entity.Dimension{}
}

func (m *ConstSecondMetric) OExpression() *entity.OExpression {
	return &entity.OExpression{}
}

func NewConstSecondMetric() entity.IMetricDefinition {
	return &ConstSecondMetric{}
}
