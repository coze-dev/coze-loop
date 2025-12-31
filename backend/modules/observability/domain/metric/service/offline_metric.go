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
	QueryTimeout    time.Duration
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
		drillDownVals := m.buildDrillDownFields(
			m.pMetrics.PlatformMetricDefs[metric.platformType],
			m.pMetrics.MetricGroups[metric.groupName], metric.metricDef)
		logs.CtxInfo(ctx, "traverse metric %s for %s, drill down combination count: %d",
			metric.metricDef.Name(), metric.platformType, len(drillDownVals))
		for _, drillDownVal := range drillDownVals {
			param := &metricTraverseParam{
				PlatformType:    metric.platformType,
				MetricDef:       metric.metricDef,
				WorkspaceID:     req.WorkspaceID,
				DrillDownValues: drillDownVal,
				StartDate:       req.StartDate,
				StartAt:         startAt,
				EndAt:           endAt,
				QueryTimeout:    req.QueryTimeout,
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
				time.Sleep(20 * time.Second)
			} else {
				logs.CtxInfo(ctx, "traverse metric %s at %s successfully, cost %s",
					metric.metricDef.Name(), metric.platformType, time.Since(st))
				resp.Statistic.Success++
			}
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
	metricGroupBelong := make(map[string]string)
	for groupName, metricGroup := range m.pMetrics.MetricGroups {
		for _, metricDef := range metricGroup.MetricDefinitions {
			metricGroupBelong[metricDef.Name()] = groupName
		}
	}
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
					actualGroupName := metricGroupBelong[metric.Name()]
					if actualGroupName == "" {
						return nil, fmt.Errorf("metric %s not found in any group defined", metric.Name())
					}
					ret = append(ret, &traverseMetric{
						platformType: platformType,
						groupName:    actualGroupName,
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
	err := backoff.RetryWithMaxTimes(ctx, 2, func() error {
		if param.QueryTimeout > 0 {
			iCtx, cancel := context.WithTimeout(ctx, param.QueryTimeout)
			defer cancel()
			ctx = iCtx
		}
		resp, err := m.queryOnlineMetrics(ctx, qReq)
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
) [][]*loop_span.FilterField {
	var fields []*loop_span.FilterField
	// platform drill down
	for _, obj := range platformCfg.DrillDownObjects {
		fields = append(fields, m.pMetrics.DrillDownObjects[obj])
	}
	// group drill down
	for _, obj := range groupCfg.DrillDownObjects {
		fields = append(fields, m.pMetrics.DrillDownObjects[obj])
	}
	ret := make([][]*loop_span.FilterField, 0)
	if definition.OExpression().AggrType == entity.MetricOfflineAggrTypeAvg {
		// 对于AVG类型而言的计算都是不准确的, 需要准确就需要完全地下钻
		ret = allDrillDownFields(fields)
	} else {
		ret = append(ret, fields)
	}
	for index := range ret {
		ret[index] = append(ret[index], &loop_span.FilterField{
			FieldName: loop_span.SpanFieldSpaceId,
			FieldType: loop_span.FieldTypeString,
		})
	}
	return ret
}

func allDrillDownFields(fields []*loop_span.FilterField) [][]*loop_span.FilterField {
	var dfs func(int) [][]*loop_span.FilterField
	dfs = func(idx int) [][]*loop_span.FilterField {
		if idx >= len(fields) {
			return [][]*loop_span.FilterField{{}}
		}
		ret := make([][]*loop_span.FilterField, 0)
		rest := dfs(idx + 1)
		for _, r := range rest {
			ret = append(ret, r)
			ret = append(ret, append(r, fields[idx]))
		}
		return ret
	}
	return dfs(0)
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
		metricNames: []string{metricName},
		mInfo:       &metricInfo{},
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
	subFilters := &loop_span.FilterFields{
		FilterFields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldSpaceId,
				FieldType: loop_span.FieldTypeString,
				Values:    []string{strconv.FormatInt(req.WorkspaceID, 10)},
				QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
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
	}
	extraFilters := m.buildExtraFilter(req, mDef)
	if len(extraFilters) > 0 {
		subFilters.FilterFields = append(subFilters.FilterFields, extraFilters...)
	}
	param.Filters = &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				SubFilter: subFilters,
			},
			{
				SubFilter: req.FilterFields,
			},
		},
	}
	mBuilder.mRepoReq = param
	return mBuilder, nil
}

func (m *MetricsService) buildExtraFilter(req *QueryMetricsReq, mDef entity.IMetricDefinition) []*loop_span.FilterField {
	if mDef.OExpression().AggrType != entity.MetricOfflineAggrTypeAvg {
		return nil
	} else if m.pMetrics.PlatformMetricDefs[req.PlatformType] == nil {
		// not expected to be here
		return nil
	}
	requestFieldName := make(map[string]bool)
	_ = req.FilterFields.Traverse(func(f *loop_span.FilterField) error {
		if f.FieldName != "" {
			requestFieldName[f.FieldName] = true
		}
		return nil
	})
	drillDownKeys := make([]string, 0)
	drillDownKeys = append(drillDownKeys, m.pMetrics.PlatformMetricDefs[req.PlatformType].DrillDownObjects...)
	drillDownKeys = append(drillDownKeys, m.metricDrillDown[mDef.Name()]...)
	ret := make([]*loop_span.FilterField, 0)
	for _, key := range drillDownKeys {
		field := m.pMetrics.DrillDownObjects[key]
		if field == nil {
			continue
		} else if requestFieldName[field.FieldName] {
			continue
		}
		ret = append(ret, &loop_span.FilterField{
			FieldName: field.FieldName,
			FieldType: field.FieldType,
			QueryType: ptr.Of(loop_span.QueryTypeEnumNotExist),
		})
	}
	return ret
}
