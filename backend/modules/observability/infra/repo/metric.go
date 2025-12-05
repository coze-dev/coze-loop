// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	metric_repo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
)

func NewOfflineMetricRepoImpl(
	oMetricDao dao.IOfflineMetricDao,
	traceConfig config.ITraceConfig,
) (metric_repo.IOfflineMetricRepo, error) {
	return &OfflineMetricRepoImpl{
		offlineMetricDao: oMetricDao,
		traceConfig:      traceConfig,
	}, nil
}

type OfflineMetricRepoImpl struct {
	offlineMetricDao dao.IOfflineMetricDao
	traceConfig      config.ITraceConfig
}

func (o *OfflineMetricRepoImpl) GetMetrics(ctx context.Context, param *metric_repo.GetMetricsParam) (*metric_repo.GetMetricsResult, error) {
	cfg, err := o.traceConfig.GetMetricPlatformTenants(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := o.offlineMetricDao.GetMetrics(ctx, &dao.GetMetricsParam{
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

func (o *OfflineMetricRepoImpl) InsertMetrics(ctx context.Context, events []*entity.MetricEvent) error {
	return o.offlineMetricDao.InsertMetrics(ctx, events)
}
