// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
)

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsRepo
type IMetricsRepo interface {
	GetTimeSeries(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error)
	GetSummary(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error)
	GetPie(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error)
}