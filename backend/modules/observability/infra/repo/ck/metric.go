// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import "context"

type IOfflineMetricDao interface {
	GetMetrics(ctx context.Context, param *GetMetricsParam) ([]map[string]any, error)
}

func NewOfflineMetricDaoImpl() (IOfflineMetricDao, error) {
	return new(OfflineMetricDaoImpl), nil
}

type OfflineMetricDaoImpl struct {
}

func (o *OfflineMetricDaoImpl) GetMetrics(ctx context.Context, param *GetMetricsParam) ([]map[string]any, error) {
	return nil, nil
}
