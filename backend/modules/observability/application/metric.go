// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

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
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
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
		logs.CtxError(ctx, "fail to query metrics, %v", err)
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

// 支持多维度下钻
// 约定：按照指标GourpBy中的Alias进行解析
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
		metricName = entity.MetricNameModelTotalCountPie
	case metric2.DrillDownValueTypeToolName:
		metricName = entity.MetricNameToolTotalCountPie
	case metric2.DrillDownValueTypeInnerModelName:
		metricName = "model_inner_total_count_pie"
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
	sResp, err := m.metricService.QueryMetrics(ctx, sReq)
	if err != nil {
		logs.CtxError(ctx, "fail to query metrics, %v", err)
		return nil, err
	}
	resp := &metric.GetDrillDownValuesResponse{}
	metricVal := sResp.Metrics[metricName]
	if metricVal == nil {
		return resp, nil
	}
	keys, err := m.metricService.GetMetricGroupBy(metricName)
	if err != nil {
		return nil, err
	}
	// GroupBy Key的顺序决定了返回的层级结构
	tree := m.buildDrillDownTree(metricVal.Pie, keys)
	resp.DrillDownValues = m.convertDrillDownTree(tree)
	const maxLength = 1000
	if len(resp.DrillDownValues) > maxLength {
		resp.DrillDownValues = resp.DrillDownValues[:maxLength]
	}
	return resp, nil
}

func (m *MetricApplication) validateGetDrillDownValuesReq(ctx context.Context, req *metric.GetDrillDownValuesRequest) error {
	if req.StartTime > req.EndTime {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start_time cannot be greater than end_time"))
	}
	return nil
}

func (m *MetricApplication) TraverseMetrics(ctx context.Context, req *metric.TraverseMetricsRequest) (*metric.TraverseMetricsResponse, error) {
	if req.StartDate == nil {
		req.StartDate = ptr.Of(time.Now().Add(-24 * time.Hour).Format(time.DateOnly))
	}
	sReq := &service.TraverseMetricsReq{
		MetricsNames: req.GetMetricNames(),
		StartDate:    req.GetStartDate(),
		QueryTimeout: 60 * time.Second,
	}
	for _, platformType := range req.GetPlatformTypes() {
		sReq.PlatformTypes = append(sReq.PlatformTypes, loop_span.PlatformType(platformType))
	}
	if req.WorkspaceID != nil {
		sReq.WorkspaceID = req.GetWorkspaceID()
	}
	resp, err := m.metricService.TraverseMetrics(ctx, sReq)
	if err != nil {
		logs.CtxError(ctx, "fail to traverse metrics", err)
		return nil, err
	}
	logs.CtxInfo(ctx, "Traverse %d metrics result: success %d, fail %d",
		resp.Statistic.Total, resp.Statistic.Success, resp.Statistic.Failure)
	return &metric.TraverseMetricsResponse{
		Statistic: &metric.TraverseMetricsStatistic{
			Total:   ptr.Of(int32(resp.Statistic.Total)),
			Success: ptr.Of(int32(resp.Statistic.Success)),
			Failure: ptr.Of(int32(resp.Statistic.Failure)),
		},
	}, nil
}

type drillDownNode struct {
	Key      string
	Value    string
	Total    float64
	Children map[string]*drillDownNode
}

func (m *MetricApplication) buildDrillDownTree(pie map[string]string, keys []string) []*drillDownNode {
	rootNodes := make(map[string]*drillDownNode)
	for k, v := range pie {
		valFloat, _ := strconv.ParseFloat(v, 64)
		keyMap := make(map[string]string)
		_ = json.Unmarshal([]byte(k), &keyMap)
		currentLevel := rootNodes
		for _, key := range keys {
			val := keyMap[key]
			node, ok := currentLevel[val]
			if !ok {
				node = &drillDownNode{
					Key:      key,
					Value:    val,
					Children: make(map[string]*drillDownNode),
				}
				currentLevel[val] = node
			}
			node.Total += valFloat
			currentLevel = node.Children
		}
	}
	return m.sortDrillDownNodes(rootNodes)
}

func (m *MetricApplication) sortDrillDownNodes(nodes map[string]*drillDownNode) []*drillDownNode {
	res := make([]*drillDownNode, 0, len(nodes))
	for _, node := range nodes {
		res = append(res, node)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Total != res[j].Total {
			return res[i].Total > res[j].Total
		}
		return strings.Compare(res[i].Value, res[j].Value) < 0
	})
	return res
}

func (m *MetricApplication) convertDrillDownTree(nodes []*drillDownNode) []*metric.DrillDownValue {
	res := make([]*metric.DrillDownValue, 0, len(nodes))
	for _, node := range nodes {
		val := &metric.DrillDownValue{
			Value:       node.Value,
			DisplayName: ptr.Of(node.Value),
		}
		if len(node.Children) > 0 {
			sortedChildren := m.sortDrillDownNodes(node.Children)
			val.SubDrillDownValues = m.convertDrillDownTree(sortedChildren)
		}
		res = append(res, val)
	}
	return res
}
