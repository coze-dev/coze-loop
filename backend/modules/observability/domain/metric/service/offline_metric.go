// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
)

type metricTraverseParam struct {
	PlatformType    loop_span.PlatformType
	WorkspaceID     int64
	MetricDef       entity.IMetricDefinition
	DrillDownValues []*loop_span.FilterField
	StartDate       string
	StartAt         int64 // ms
	EndAt           int64 // ms
}

func (m *MetricsService) TraverseMetrics(ctx context.Context, req *TraverseMetricsReq) (*TraverseMetricsResp, error) {
	startAt, endAt, err := m.parseStartDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	if len(req.PlatformTypes) == 0 {
		req.PlatformTypes = lo.Keys(m.pMetrics.PlatformMetricDefs)
	}
	metrics, err := m.buildTraverseMetrics(ctx, req)
	if err != nil {
		return nil, err
	}
	resp := &TraverseMetricsResp{}
	for _, metric := range metrics {
		if _, ok := metric.metricDef.(entity.IMetricCompound); ok {
			logs.CtxWarn(ctx, "skip metric compound metric %s", metric.metricDef.Name())
			continue
		}
		drillDownVal := m.buildDrillDownFields(
			m.pMetrics.PlatformMetricDefs[metric.platformType],
			m.pMetrics.MetricGroups[metric.groupName], metric.metricDef)
		param := &metricTraverseParam{
			PlatformType:    metric.platformType,
			MetricDef:       metric.metricDef,
			WorkspaceID:     req.WorkspaceID,
			DrillDownValues: drillDownVal,
			StartDate:       req.StartDate,
			StartAt:         startAt,
			EndAt:           endAt,
		}
		st := time.Now()
		resp.Statistic.Total++
		if err := m.traverseMetric(ctx, param); err != nil {
			logs.CtxError(ctx, "fail to traverse metric %s at %s, %v",
				metric.metricDef.Name(), metric.platformType, err)
			resp.Statistic.Failure++
			resp.Failures = append(resp.Failures, &TraverseMetricDetail{
				PlatformType: metric.platformType,
				MetricName:   metric.metricDef.Name(),
				Error:        err,
				TimeCost:     time.Since(st),
			})
		} else {
			logs.CtxInfo(ctx, "traverse metric %s at %s successfully, cost %s",
				metric.metricDef.Name(), metric.platformType, time.Since(st))
			resp.Statistic.Success++
		}
	}
	return resp, nil
}

type traverseMetric struct {
	platformType loop_span.PlatformType
	groupName    string
	metricDef    entity.IMetricDefinition
}

func (m *MetricsService) buildTraverseMetrics(ctx context.Context, req *TraverseMetricsReq) ([]*traverseMetric, error) {
	seen := make(map[string]bool)
	ret := make([]*traverseMetric, 0)
	for _, platformType := range req.PlatformTypes {
		platformTypeCfg := m.pMetrics.PlatformMetricDefs[platformType]
		if platformTypeCfg == nil {
			logs.CtxError(ctx, "platform type %s not found", platformType)
			continue
		}
		for _, groupName := range platformTypeCfg.MetricGroups {
			metricGroup := m.pMetrics.MetricGroups[groupName]
			if metricGroup == nil {
				continue
			}
			for _, metricDef := range metricGroup.MetricDefinitions {
				if !m.shouldTraverseMetric(metricDef.Name(), req.MetricsNames) {
					continue
				}
				metrics := []entity.IMetricDefinition{metricDef}
				if compound, ok := metricDef.(entity.IMetricCompound); ok {
					metrics = compound.GetMetrics()
				}
				for _, metric := range metrics {
					// special case
					key := fmt.Sprintf("%s_%s", platformType, metric.Name())
					if _, ok := metric.(entity.IMetricConst); ok {
						continue
					} else if m.metricDefMap[metric.Name()] == nil {
						return nil, fmt.Errorf("metric %s not found", metric.Name())
					} else if seen[key] {
						continue
					}
					seen[key] = true
					ret = append(ret, &traverseMetric{
						platformType: platformType,
						groupName:    groupName,
						metricDef:    metric,
					})
				}
			}
		}
	}
	metricNames := lo.Map(ret, func(item *traverseMetric, _ int) string {
		return fmt.Sprintf("%s_%s", item.platformType, item.metricDef.Name())
	})
	logs.CtxInfo(ctx, "metrics to be traversed: %v, count: %d", metricNames, len(metricNames))
	return ret, nil
}

func (m *MetricsService) traverseMetric(ctx context.Context, param *metricTraverseParam) error {
	metricName := param.MetricDef.Name()
	qReq := &QueryMetricsReq{
		PlatformType:    param.PlatformType,
		WorkspaceID:     param.WorkspaceID,
		MetricsNames:    []string{metricName},
		DrillDownFields: param.DrillDownValues,
		Granularity:     entity.MetricGranularity1Day,
		StartTime:       param.StartAt,
		EndTime:         param.EndAt,
		GroupBySpaceID:  true,
	}
	var mResp *QueryMetricsResp
	err := backoff.RetryWithMaxTimes(ctx, 3, func() error {
		iCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		resp, err := m.queryOnlineMetrics(iCtx, qReq)
		if err != nil {
			return err
		}
		mResp = resp
		return nil
	})
	if err != nil {
		return err
	}
	mEvents := m.extractMetrics(metricName, mResp.Metrics[metricName])
	for _, mEvent := range mEvents {
		mEvent.PlatformType = string(qReq.PlatformType)
		mEvent.StartDate = param.StartDate
		mEvent.MetricName = metricName
	}
	return m.oMetricRepo.InsertMetrics(ctx, mEvents)
}

func (m *MetricsService) parseStartDate(startDate string) (int64, int64, error) {
	startAt, err := time.ParseInLocation(time.DateOnly, startDate, time.Local)
	if err != nil {
		return 0, 0, fmt.Errorf("fail to parse start date, %v", err)
	}
	endAt := time.Date(startAt.Year(), startAt.Month(), startAt.Day(), 23, 59, 59, 999999999, startAt.Location())
	return startAt.UnixMilli(), endAt.UnixMilli(), nil
}

func (m *MetricsService) shouldTraverseMetric(metricName string, reqMetrics []string) bool {
	if len(reqMetrics) != 0 && !lo.Contains(reqMetrics, metricName) {
		return false
	}
	return true
}

func (m *MetricsService) buildDrillDownFields(
	platformCfg *entity.PlatformMetricDef,
	groupCfg *entity.MetricGroup,
	definition entity.IMetricDefinition,
) []*loop_span.FilterField {
	var ret []*loop_span.FilterField
	// platform drill down
	for _, obj := range platformCfg.DrillDownObjects {
		ret = append(ret, m.pMetrics.DrillDownObjects[obj])
	}
	// group drill down
	for _, obj := range groupCfg.DrillDownObjects {
		ret = append(ret, m.pMetrics.DrillDownObjects[obj])
	}
	// metric drill down
	for _, obj := range definition.GroupBy() {
		ret = append(ret, obj.Field)
	}
	// unique
	ret = lo.UniqBy(ret, func(item *loop_span.FilterField) string {
		return item.FieldName
	})
	ret = append(ret, &loop_span.FilterField{
		FieldName: loop_span.SpanFieldSpaceId,
		FieldType: loop_span.FieldTypeString,
	})
	return ret
}

func (m *MetricsService) extractMetrics(metricName string, metric *entity.Metric) []*entity.MetricEvent {
	def := m.metricDefMap[metricName]
	if def == nil {
		return nil
	}
	objectKeys := make(map[string]string)
	for _, obj := range def.GroupBy() {
		objectKeys[obj.Alias] = obj.Field.FieldName
	}
	for _, obj := range m.pMetrics.DrillDownObjects {
		objectKeys[obj.FieldName] = obj.FieldName
	}
	var events []*entity.MetricEvent
	switch def.Type() {
	case entity.MetricTypeTimeSeries:
		for k, v := range metric.TimeSeries {
			event := &entity.MetricEvent{
				ObjectKeys:  make(map[string]string),
				MetricValue: lo.Ternary(len(v) > 0, v[0].Value, ""),
			}
			if k != defaultGroupKey {
				mp := make(map[string]string)
				_ = json.Unmarshal([]byte(k), &mp)
				for objName, objVal := range mp {
					if val := objectKeys[objName]; val != "" {
						event.ObjectKeys[val] = objVal
					} else if objName == loop_span.SpanFieldSpaceId {
						event.WorkspaceID = objVal
					}
				}
			}
			events = append(events, event)
		}
	case entity.MetricTypeSummary:
		if metric.Summary != "" {
			event := &entity.MetricEvent{
				MetricValue: metric.Summary,
			}
			events = append(events, event)
			break
		}
		fallthrough
	case entity.MetricTypePie:
		for k, v := range metric.Pie {
			event := &entity.MetricEvent{
				ObjectKeys:  make(map[string]string),
				MetricValue: v,
			}
			if k != defaultGroupKey {
				mp := make(map[string]string)
				_ = json.Unmarshal([]byte(k), &mp)
				for objName, objVal := range mp {
					if val := objectKeys[objName]; val != "" {
						event.ObjectKeys[val] = objVal
					} else if objName == loop_span.SpanFieldSpaceId {
						event.WorkspaceID = objVal
					}
				}
			}
			events = append(events, event)
		}
	}
	return events
}

// query offline metrics
func (m *MetricsService) queryOfflineMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	// 离线指标拆开计算
	retMetric := make(map[string]*entity.Metric)
	for _, metricName := range req.MetricsNames {
		mBuilder, err := m.buildOfflineMetricQuery(ctx, req, metricName)
		if err != nil {
			return nil, err
		}
		st := time.Now()
		result, err := m.oMetricRepo.GetMetrics(ctx, mBuilder.mRepoReq)
		if err != nil {
			return nil, err
		}
		logs.CtxInfo(ctx, "get offline metrics for %v successfully, cost %v", mBuilder.metricNames, time.Since(st))
		for k, v := range m.formatMetrics(result.Data, mBuilder) {
			retMetric[k] = v
		}
	}
	return &QueryMetricsResp{
		Metrics: retMetric,
	}, nil
}

func (m *MetricsService) buildOfflineMetricQuery(ctx context.Context, req *QueryMetricsReq, metricName string) (*metricQueryBuilder, error) {
	mBuilder := &metricQueryBuilder{
		mInfo: &metricInfo{},
	}
	mDef := m.metricDefMap[metricName]
	if mDef == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	mBuilder.mInfo.mType = mDef.Type()
	mBuilder.mInfo.mGroupBy = mDef.GroupBy()
	oExpression := mDef.OExpression()
	if oExpression.MetricName == "" {
		oExpression.MetricName = mDef.Name()
	}
	mBuilder.mInfo.mAggregation = append(mBuilder.mInfo.mAggregation, &entity.Dimension{
		OExpression: oExpression,
		Alias:       mDef.Name(),
	})
	param := &repo.GetMetricsParam{
		Aggregations: mBuilder.mInfo.mAggregation,
		GroupBys:     mBuilder.mInfo.mGroupBy,
		Filters:      req.FilterFields,
		StartAt:      req.StartTime,
		EndAt:        req.EndTime,
	}
	if mBuilder.mInfo.mType == entity.MetricTypeTimeSeries {
		param.Granularity = entity.MetricGranularity1Day
	}
	param.Filters = &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter: &loop_span.FilterFields{
					FilterFields: []*loop_span.FilterField{
						{
							FieldName: loop_span.SpanFieldSpaceId,
							FieldType: loop_span.FieldTypeString,
							Values:    []string{strconv.FormatInt(req.WorkspaceID, 10)},
							QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
						},
						{
							FieldName: "platform_type",
							FieldType: loop_span.FieldTypeString,
							Values:    []string{string(req.PlatformType)},
							QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						},
						{
							FieldName: "metric_name",
							FieldType: loop_span.FieldTypeString,
							Values:    []string{oExpression.MetricName},
							QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						},
					},
				},
			},
			{
				SubFilter: req.FilterFields,
			},
		},
	}
	mBuilder.mRepoReq = param
	return mBuilder, nil
}
