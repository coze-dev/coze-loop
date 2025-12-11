// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
)

type IOfflineMetricDao interface {
	GetMetrics(ctx context.Context, param *GetMetricsParam) ([]map[string]any, error)
	InsertMetrics(ctx context.Context, events []*entity.MetricEvent) error
}
