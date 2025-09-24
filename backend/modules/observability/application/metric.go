// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	metric2 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/metric"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/metric"
	mconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/metric"
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
	metricService  service.IMetricsService
	tenantProvider tenant.ITenantProvider
	authSvc        rpc.IAuthProvider
}

func NewMetricApplication(
	metricService service.IMetricsService,
	tenantProvider tenant.ITenantProvider,
	authSvc rpc.IAuthProvider,
) (IMetricApplication, error) {
	return &MetricApplication{
		metricService:  metricService,
		tenantProvider: tenantProvider,
		authSvc:        authSvc,
	}, nil
}

func (m *MetricApplication) GetMetrics(ctx context.Context, req *metric.GetMetricsRequest) (r *metric.GetMetricsResponse, err error) {
	if err := m.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceMetricRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10)); err != nil {
		return nil, err
	}
	sReq := &service.QueryMetricsReq{
		PlatformType:    loop_span.PlatformType(req.GetPlatformType()),
		WorkspaceID:     req.GetWorkspaceID(),
		MetricsNames:    req.GetMetricNames(),
		Granularity:     entity.MetricGranularity(req.GetGranularity()),
		StartTime:       req.GetStartTime(),
		EndTime:         req.GetEndTime(),
		FilterFields:    tconv.FilterFieldsDTO2DO(req.Filters),
		DrillDownFields: tconv.FilterFieldListDTO2DO(req.DrillDownFields),
	}
	sResp, err := m.metricService.QueryMetrics(ctx, sReq)
	if err != nil {
		return nil, err
	}
	resp := &metric.GetMetricsResponse{
		Metrics: make(map[string]*metric2.Metric),
	}
	for k, v := range sResp.Metrics {
		resp.Metrics[k] = mconv.MetricDO2DTO(v)
	}
	return resp, nil
}
