// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
)

func NewOfflineMetricDaoImpl() (dao.IOfflineMetricDao, error) {
	return new(OfflineMetricDaoImpl), nil
}

type OfflineMetricDaoImpl struct {
}

func (o *OfflineMetricDaoImpl) GetMetrics(ctx context.Context, param *dao.GetMetricsParam) ([]map[string]any, error) {
	return nil, nil
}

func (o *OfflineMetricDaoImpl) InsertMetrics(ctx context.Context, events []*entity.MetricEvent) error {
	return nil
}
