// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/metric"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type IMetricApplication interface {
	metric.MetricService
}

type MetricApplication struct {
	metricService service.MetricsService
	tenant        tenant.ITenantProvider
	authSvc       rpc.IAuthProvider
}

func (m *MetricApplication) GetMetrics(ctx context.Context, req *metric.GetMetricsRequest) (r *metric.GetMetricsResponse, err error) {
	if err := m.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceMetricRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	sReq := &service.QueryMetricsReq{
		PlatformType: loop_span.PlatformType(req.GetPlatformType()),
		WorkspaceID:  req.GetWorkspaceID(),
		MetricsNames: req.GetMetricNames(),
		Granularity:  entity.MetricGranularity(req.GetGranularity()),
		StartTime:    req.GetStartTime(),
		EndTime:      req.GetEndTime(),
		FilterFields: tconv.FilterFieldsDTO2DO(req.Filters),
	}
	sResp, err := m.metricService.QueryMetrics(ctx, sReq)
	if err != nil {
		return nil, err
	}

}
