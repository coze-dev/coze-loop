// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type GetMetricsParam struct {
	Tenants      []string
	Aggregations []*entity.Dimension
	GroupBys     []*entity.Dimension
	Filters      *loop_span.FilterFields
	StartAt      int64
	EndAt        int64
	Granularity  entity.MetricGranularity
}

type GetMetricsResult struct {
	Data []map[string]any
}

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsRepo
type IMetricRepo interface {
	GetMetrics(ctx context.Context, param *GetMetricsParam) (*GetMetricsResult, error)
}
