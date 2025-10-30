// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/smithy-go/ptr"
	metric2 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/metric"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/metric"
	mconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/metric"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"golang.org/x/sync/errgroup"
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
	if err := m.validateGetMetricsReq(ctx, req); err != nil {
		return nil, err
	}
	if err := m.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceMetricRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10), false); err != nil {
		return nil, err
	}
	var (
		metrics         map[string]*entity.Metric
		comparedMetrics map[string]*entity.Metric
		eGroup          errgroup.Group
	)
	eGroup.Go(func() error {
		defer goroutine.Recovery(ctx)
		sReq := m.buildGetMetricsReq(req)
		sResp, err := m.metricService.QueryMetrics(ctx, sReq)
		if err != nil {
			return err
		}
		metrics = sResp.Metrics
		return nil
	})
	compare := mconv.CompareDTO2DO(req.GetCompare())
	if newStart, newEnd, do := m.shouldCompareWith(req.GetStartTime(), req.GetEndTime(), compare); do {
		eGroup.Go(func() error {
			defer goroutine.Recovery(ctx)
			sReq := m.buildGetMetricsReq(req)
			sReq.StartTime = newStart
			sReq.EndTime = newEnd
			sResp, err := m.metricService.QueryMetrics(ctx, sReq)
			if err != nil {
				return err
			}
			comparedMetrics = sResp.Metrics
			return nil
		})
	}
	if err := eGroup.Wait(); err != nil {
		return nil, err
	}
	resp := &metric.GetMetricsResponse{
		Metrics:         make(map[string]*metric2.Metric),
		ComparedMetrics: make(map[string]*metric2.Metric),
	}
	for k, v := range metrics {
		resp.Metrics[k] = mconv.MetricDO2DTO(v)
	}
	for k, v := range comparedMetrics {
		resp.ComparedMetrics[k] = mconv.MetricDO2DTO(v)
	}
	return resp, nil
}

func (m *MetricApplication) validateGetMetricsReq(ctx context.Context, req *metric.GetMetricsRequest) error {
	if req.StartTime > req.EndTime {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start_time cannot be greater than end_time"))
	}
	switch entity.MetricGranularity(req.GetGranularity()) {
	case entity.MetricGranularity1Min:
		if req.EndTime-req.StartTime > 3*time.Hour.Milliseconds() {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid granularity"))
		}
	case entity.MetricGranularity1Hour:
		if req.EndTime-req.StartTime > 6*24*time.Hour.Milliseconds() {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid granularity"))
		}
	}
	return nil
}

func (m *MetricApplication) buildGetMetricsReq(req *metric.GetMetricsRequest) *service.QueryMetricsReq {
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
	if sReq.Granularity == "" {
		sReq.Granularity = entity.MetricGranularity1Day
	}
	return sReq
}

func (m *MetricApplication) shouldCompareWith(start, end int64, c *entity.Compare) (int64, int64, bool) {
	if c == nil {
		return 0, 0, false
	}
	switch c.Type {
	case entity.MetricCompareTypeMoM:
		return start - (end - start), start, true
	case entity.MetricCompareTypeYoY:
		shiftMill := c.Shift * 1000
		return start - shiftMill, end - shiftMill, true
	default:
		return 0, 0, false
	}
}

// 取最近七天内数据
func (m *MetricApplication) GetDrillDownValues(ctx context.Context, req *metric.GetDrillDownValuesRequest) (r *metric.GetDrillDownValuesResponse, err error) {
	if err := m.validateGetDrillDownValuesReq(ctx, req); err != nil {
		return nil, err
	}
	if err := m.authSvc.CheckWorkspacePermission(ctx,
		rpc.AuthActionTraceMetricRead,
		strconv.FormatInt(req.GetWorkspaceID(), 10), false); err != nil {
		return nil, err
	}
	var metricName string
	switch req.DrillDownValueType {
	case metric2.DrillDownValueTypeModelName:
		metricName = entity.MetricNameModelNamePie
	case metric2.DrillDownValueTypeToolName:
		metricName = entity.MetricNameToolNamePie
	default:
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid drill_down_value_type"))
	}
	sReq := &service.QueryMetricsReq{
		PlatformType: loop_span.PlatformType(req.GetPlatformType()),
		WorkspaceID:  req.GetWorkspaceID(),
		MetricsNames: []string{metricName},
		StartTime:    req.GetStartTime(),
		EndTime:      req.GetEndTime(),
		FilterFields: tconv.FilterFieldsDTO2DO(req.Filters),
	}
	sevenDayMills := 7 * 24 * time.Hour.Milliseconds()
	if sReq.EndTime-sReq.StartTime > sevenDayMills {
		sReq.StartTime = sReq.EndTime - sevenDayMills
	}
	sResp, err := m.metricService.QueryMetrics(ctx, sReq)
	if err != nil {
		return nil, err
	}
	resp := &metric.GetDrillDownValuesResponse{}
	metricVal := sResp.Metrics[metricName]
	if metricVal != nil {
		for k := range metricVal.Pie {
			mp := make(map[string]string)
			_ = json.Unmarshal([]byte(k), &mp)
			if val := mp["name"]; val != "" {
				resp.DrillDownValues = append(resp.DrillDownValues, &metric.DrillDownValue{
					Value:       val,
					DisplayName: ptr.String(val),
				})
			}
		}
	}
	return resp, nil
}

func (m *MetricApplication) validateGetDrillDownValuesReq(ctx context.Context, req *metric.GetDrillDownValuesRequest) error {
	if req.StartTime > req.EndTime {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start_time cannot be greater than end_time"))
	}
	return nil
}
