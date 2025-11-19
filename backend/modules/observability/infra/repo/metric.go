// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	metric_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
)

func NewOfflineMetricRepoImpl(
	oMetricDao ck.IOfflineMetricDao,
	traceConfig config.ITraceConfig, ) (metric_repo.IOfflineMetricRepo, error) {
	return &OfflineMetricRepoImpl{
		offlineMetricDao: oMetricDao,
		traceConfig:      traceConfig,
	}, nil
}

type OfflineMetricRepoImpl struct {
	offlineMetricDao ck.IOfflineMetricDao
	traceConfig      config.ITraceConfig
}

func (o *OfflineMetricRepoImpl) GetMetrics(ctx context.Context, param *metric_repo.GetMetricsParam) (*metric_repo.GetMetricsResult, error) {
	cfg, err := o.traceConfig.GetMetricPlatformTenants(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := o.offlineMetricDao.GetMetrics(ctx, &ck.GetMetricsParam{
		Tables:       []string{cfg.Table},
		Aggregations: param.Aggregations,
		GroupBys:     param.GroupBys,
		Filters:      param.Filters,
		StartAt:      time_util.MillSec2MicroSec(param.StartAt),
		EndAt:        time_util.MillSec2MicroSec(param.EndAt),
		Granularity:  param.Granularity,
	})
	if err != nil {
		return nil, err
	}
	return &metric_repo.GetMetricsResult{
		Data: resp,
	}, nil
}
