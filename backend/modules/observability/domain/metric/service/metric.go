// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_service "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"
)

const defaultGroupKey = "all"

type QueryMetricsReq struct {
	PlatformType    loop_span.PlatformType
	WorkspaceID     int64
	MetricsNames    []string
	Granularity     entity.MetricGranularity
	FilterFields    *loop_span.FilterFields
	DrillDownFields []*loop_span.FilterField
	StartTime       int64
	EndTime         int64
	GroupBySpaceID  bool
}

type QueryMetricsResp struct {
	Metrics map[string]*entity.Metric
}

type TraverseMetricsReq struct {
	PlatformTypes []loop_span.PlatformType
	MetricsNames  []string
	WorkspaceID   int64
	StartDate     string // e.g. 2025-11-17
}

type metricTraverseParam struct {
	PlatformType    loop_span.PlatformType
	WorkspaceID     int64
	MetricDef       entity.IMetricDefinition
	DrillDownValues []*loop_span.FilterField
	StartDate       string
	StartAt         int64 // ms
	EndAt           int64 // ms
}

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsService
type IMetricsService interface {
	QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error)
	TraverseMetrics(ctx context.Context, req *TraverseMetricsReq) error
}

type MetricsService struct {
	metricRepo     repo.IMetricRepo
	oMetricRepo    repo.IOfflineMetricRepo
	metricDefMap   map[string]entity.IMetricDefinition
	buildHelper    trace_service.TraceFilterProcessorBuilder
	tenantProvider tenant.ITenantProvider
	traceConfig    config.ITraceConfig
	pMetrics       *entity.PlatformMetrics
	metricProducer mq.IMetricProducer
}

func NewMetricsService(
	metricRepo repo.IMetricRepo,
	oMetricRepo repo.IOfflineMetricRepo,
	tenantProvider tenant.ITenantProvider,
	buildHelper trace_service.TraceFilterProcessorBuilder,
	traceConfig config.ITraceConfig,
	pMetrics *entity.PlatformMetrics,
	metricProducer mq.IMetricProducer,
) (IMetricsService, error) {
	metricDefMap := make(map[string]entity.IMetricDefinition)
	for _, metricGroup := range pMetrics.MetricGroups {
		var groupMetrics []entity.IMetricDefinition
		for _, def := range metricGroup.MetricDefinitions {
			var metrics []entity.IMetricDefinition
			if mAdapter, ok := def.(entity.IMetricAdapter); ok {
				for _, wrapper := range mAdapter.Wrappers() {
					metrics = append(metrics, wrapper.Wrap(def))
				}
			} else {
				metrics = append(metrics, def)
			}
			for _, def := range metrics {
				if metricDefMap[def.Name()] != nil {
					return nil, fmt.Errorf("duplicate metric name %s", def.Name())
				}
				metricDefMap[def.Name()] = def
			}
			groupMetrics = append(groupMetrics, metrics...)
		}
		metricGroup.MetricDefinitions = groupMetrics // expand wrapper metrics
	}
	// check compound metric all valid
	for _, def := range metricDefMap {
		compoundMetric, ok := def.(entity.IMetricCompound)
		if !ok {
			continue
		}
		for _, metric := range compoundMetric.GetMetrics() {
			if _, ok := metric.(entity.IMetricConst); ok {
				continue
			} else if _, ok := metric.(entity.IMetricCompound); ok {
				return nil, fmt.Errorf("nested compound metric %s is not allowed", metric.Name())
			}
			if metricDefMap[metric.Name()] == nil {
				return nil, fmt.Errorf("metric name %s not registered", metric.Name())
			}
		}
	}
	// check group by key valid
	for _, def := range metricDefMap {
		dimensions := def.GroupBy()
		for _, dimension := range dimensions {
			filedName := dimension.Field.FieldName
			fieldType := dimension.Field.FieldType
			isValidFieldName := false
			for _, field := range pMetrics.DrillDownObjects {
				if filedName != field.FieldName {
					continue
				} else if fieldType != field.FieldType {
					continue
				} else {
					isValidFieldName = true
					break
				}
			}
			if !isValidFieldName {
				return nil, fmt.Errorf("metric name %s group by field %s not valid", def.Name(), filedName)
			} else if dimension.Alias == "" {
				return nil, fmt.Errorf("metric name %s group by field %s alias not valid", def.Name(), filedName)
			}
		}
	}
	logs.Info("%d metrics registered", len(metricDefMap))
	return &MetricsService{
		metricRepo:     metricRepo,
		oMetricRepo:    oMetricRepo,
		metricDefMap:   metricDefMap,
		tenantProvider: tenantProvider,
		buildHelper:    buildHelper,
		traceConfig:    traceConfig,
		metricProducer: metricProducer,
		pMetrics:       pMetrics,
	}, nil
}

type metricQueryBuilder struct {
	metricNames   []string                 // metric names
	filter        span_filter.Filter       // platform filter
	spanEnv       *span_filter.SpanEnv     // platform span env
	requestFilter *loop_span.FilterFields  // request filter
	granularity   entity.MetricGranularity // granularity
	mInfo         *metricInfo              // aggregated metric info
	mRepoReq      *repo.GetMetricsParam    // metric repo request
}

type metricInfo struct {
	mType        entity.MetricType
	mAggregation []*entity.Dimension
	mGroupBy     []*entity.Dimension
	mWhere       []*loop_span.FilterField
}

func (m *MetricsService) QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	if len(req.MetricsNames) == 0 {
		return &QueryMetricsResp{}, nil
	}
	for _, metricName := range req.MetricsNames {
		mVal, ok := m.metricDefMap[metricName]
		if !ok {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode,
				errorx.WithExtraMsg(fmt.Sprintf("metric definition %s not found", metricName)))
		}
		if _, ok := mVal.(entity.IMetricCompound); ok {
			if len(req.MetricsNames) != 1 {
				return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
			} else {
				return m.queryCompoundMetric(ctx, req, mVal)
			}
		}
	}
	return m.queryMetrics(ctx, req)
}

func (m *MetricsService) queryCompoundMetric(ctx context.Context, req *QueryMetricsReq, mDef entity.IMetricDefinition) (*QueryMetricsResp, error) {
	mCompound := mDef.(entity.IMetricCompound)
	metrics := mCompound.GetMetrics()
	if len(metrics) == 0 {
		return &QueryMetricsResp{}, nil
	}
	logs.CtxInfo(ctx, "query compound metric %s", mDef.Name(), lo.Map(metrics, func(m entity.IMetricDefinition, _ int) string { return m.Name() }))
	var (
		metricsResp = make([]*QueryMetricsResp, len(metrics))
		eGroup      errgroup.Group
		lock        sync.Mutex
	)
	for i, metric := range metrics {
		eGroup.Go(func(t int) func() error {
			return func() error {
				defer goroutine.Recovery(ctx)
				var (
					resp *QueryMetricsResp
					err  error
				)
				// 常量指标
				if _, ok := metric.(entity.IMetricConst); ok {
					resp = &QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							metric.Name(): {
								Summary: metric.Expression(req.Granularity).Expression,
							},
						},
					}
				} else {
					sReq := &QueryMetricsReq{
						PlatformType:    req.PlatformType,
						WorkspaceID:     req.WorkspaceID,
						MetricsNames:    []string{metric.Name()},
						Granularity:     req.Granularity,
						FilterFields:    req.FilterFields,
						DrillDownFields: req.DrillDownFields,
						StartTime:       req.StartTime,
						EndTime:         req.EndTime,
					}
					resp, err = m.queryMetrics(ctx, sReq)
				}
				lock.Lock()
				defer lock.Unlock()
				if err == nil {
					metricsResp[t] = resp
				}
				return err
			}
		}(i))
	}
	if err := eGroup.Wait(); err != nil {
		return nil, err
	}
	// 复合指标计算...
	switch mCompound.Operator() {
	case entity.MetricOperatorDivide:
		// time_series相除/summary相除/time_series除summary
		return m.divideMetrics(ctx, metricsResp, mCompound.GetMetrics(), mDef)
	case entity.MetricOperatorPie:
		// summary指标组合构成饼图
		return m.pieMetrics(ctx, metricsResp, mDef.Name())
	default:
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
}

func (m *MetricsService) queryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	qCfg := m.traceConfig.GetMetricQueryConfig(ctx)
	if !qCfg.SupportOffline {
		return m.queryOnlineMetrics(ctx, req)
	} else {
		return m.queryOfflineMetrics(ctx, req)
	}
	// todo 按照时间划分
}

func (m *MetricsService) queryOnlineMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	mBuilder, err := m.buildOnlineMetricQuery(ctx, req)
	if err != nil {
		return nil, err
	} else if mBuilder == nil {
		return &QueryMetricsResp{}, nil // 不再查询...
	}
	st := time.Now()
	result, err := m.metricRepo.GetMetrics(ctx, mBuilder.mRepoReq)
	if err != nil {
		return nil, err
	}
	logs.CtxInfo(ctx, "get metrics for %v successfully, cost %v", mBuilder.metricNames, time.Since(st))
	return &QueryMetricsResp{
		Metrics: m.formatMetrics(result.Data, mBuilder),
	}, nil
}

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

func (m *MetricsService) buildOnlineMetricQuery(ctx context.Context, req *QueryMetricsReq) (*metricQueryBuilder, error) {
	filter, err := m.buildHelper.BuildPlatformRelatedFilter(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	tenants, err := m.tenantProvider.GetMetricTenantsByPlatformType(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	param := &repo.GetMetricsParam{
		Tenants: tenants,
		StartAt: req.StartTime,
		EndAt:   req.EndTime,
	}
	mBuilder := &metricQueryBuilder{
		metricNames:   req.MetricsNames,
		filter:        filter,
		spanEnv:       &span_filter.SpanEnv{WorkspaceID: req.WorkspaceID},
		requestFilter: req.FilterFields,
		granularity:   req.Granularity,
	}
	if err := m.buildMetricInfo(ctx, mBuilder); err != nil {
		return nil, err
	}
	if mBuilder.mInfo.mType == entity.MetricTypeTimeSeries {
		param.Granularity = req.Granularity
	}
	mFilter, err := m.buildFilter(ctx, mBuilder)
	if err != nil {
		return nil, err
	} else if mFilter == nil {
		return nil, nil
	}
	param.Aggregations = mBuilder.mInfo.mAggregation
	param.GroupBys = mBuilder.mInfo.mGroupBy
	param.Filters = mFilter
	for _, field := range req.DrillDownFields {
		if field == nil {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
		}
		param.GroupBys = append(param.GroupBys, &entity.Dimension{
			Field: field,
			Alias: field.FieldName,
		})
	}
	mBuilder.mRepoReq = param
	// rewrite filter
	if req.GroupBySpaceID {
		_ = param.Filters.Traverse(func(f *loop_span.FilterField) error {
			if f.FieldName == loop_span.SpanFieldSpaceId { // always true
				if len(f.Values) != 0 && f.Values[0] == "0" { // space id not passed
					f.QueryType = ptr.Of(loop_span.QueryTypeEnumAlwaysTrue)
				}
			}
			return nil
		})
	}
	return mBuilder, nil
}

func (m *MetricsService) buildOfflineMetricQuery(ctx context.Context, req *QueryMetricsReq, metricName string) (*metricQueryBuilder, error) {
	mBuilder := &metricQueryBuilder{
		metricNames: req.MetricsNames,
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
	param.Filters = &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter: &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{
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
				SubFilter: mBuilder.requestFilter,
			},
		},
	}
	mBuilder.mRepoReq = param
	return mBuilder, nil
}

func (m *MetricsService) buildMetricInfo(ctx context.Context, builder *metricQueryBuilder) error {
	var (
		mInfos = make([]*metricInfo, 0)
		err    error
	)
	for _, metricName := range builder.metricNames {
		metricDef, ok := m.metricDefMap[metricName]
		if !ok {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode,
				errorx.WithExtraMsg(fmt.Sprintf("metric definition %s not found", metricName)))
		}
		mInfo := &metricInfo{}
		mInfo.mType = metricDef.Type()
		mInfo.mGroupBy = metricDef.GroupBy()
		mInfo.mWhere, err = metricDef.Where(ctx, builder.filter, builder.spanEnv)
		if err != nil {
			return errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
		}
		expr := metricDef.Expression(builder.granularity)
		mInfo.mAggregation = []*entity.Dimension{{
			Expression: expr,
			Alias:      metricDef.Name(), // 聚合指标的别名是指标名，以此后续来拆分数据
		}}
		mInfos = append(mInfos, mInfo)
	}
	if len(mInfos) == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	out := mInfos[0]
	for i := 1; i < len(mInfos); i++ {
		mInfo := mInfos[i]
		if mInfo.mType != out.mType {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("metric types not the same"))
		} else if !reflect.DeepEqual(out.mWhere, mInfo.mWhere) {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("metric condition not the same"))
		} else if !reflect.DeepEqual(out.mGroupBy, mInfo.mGroupBy) {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("metric groupby not the same"))
		}
		out.mAggregation = append(out.mAggregation, mInfo.mAggregation...)
	}
	builder.mInfo = out
	return nil
}

func (m *MetricsService) buildFilter(ctx context.Context, mBuilder *metricQueryBuilder) (*loop_span.FilterFields, error) {
	basicFilter, forceQuery, err := mBuilder.filter.BuildBasicSpanFilter(ctx, mBuilder.spanEnv)
	if err != nil {
		return nil, err
	} else if len(basicFilter) == 0 && !forceQuery {
		return nil, nil
	}
	basicFilter = append(basicFilter, mBuilder.mInfo.mWhere...)
	return &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter:  &loop_span.FilterFields{FilterFields: basicFilter},
			},
			{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter:  mBuilder.requestFilter,
			},
		},
	}, nil
}

/*
	[{
		"time_bucket": "xx",
		"aggregation_1": "xxx",
		"aggregation_2": "xxx",
		"group_by_1": "xx",
		"group_by_2": "xx",
	}]
*/
const timeBucketKey = "time_bucket"

func (m *MetricsService) formatMetrics(data []map[string]any, mBuilder *metricQueryBuilder) map[string]*entity.Metric {
	mInfo := mBuilder.mInfo
	if len(mInfo.mAggregation) == 0 {
		return map[string]*entity.Metric{}
	}
	switch mInfo.mType {
	case entity.MetricTypeTimeSeries:
		return m.formatTimeSeriesData(data, mBuilder)
	case entity.MetricTypeSummary:
		return m.formatSummaryData(data, mInfo)
	case entity.MetricTypePie:
		return m.formatPieData(data, mInfo)
	default:
		return map[string]*entity.Metric{}
	}
}

func (m *MetricsService) formatTimeSeriesData(data []map[string]any, mBuilder *metricQueryBuilder) map[string]*entity.Metric {
	ret := make(map[string]*entity.Metric)
	metricNameMap := lo.Associate(mBuilder.mInfo.mAggregation,
		func(item *entity.Dimension) (string, bool) {
			ret[item.Alias] = &entity.Metric{
				TimeSeries: make(entity.TimeSeries),
			}
			return item.Alias, true
		})
	for _, dataItem := range data {
		groupByVals := make(map[string]string)
		for k, v := range dataItem {
			if !metricNameMap[k] && k != timeBucketKey {
				groupByVals[k] = conv.ToString(v)
			}
		}
		val := defaultGroupKey
		if len(groupByVals) > 0 {
			if data, err := json.Marshal(groupByVals); err == nil {
				val = string(data)
			}
		}
		for k, v := range dataItem {
			if metricNameMap[k] {
				ret[k].TimeSeries[val] = append(ret[k].TimeSeries[val], &entity.MetricPoint{
					Timestamp: conv.ToString(dataItem[timeBucketKey]),
					Value:     getMetricValue(v),
				})
			}
		}
	}
	// 零值填充
	t := entity.NewTimeIntervals(mBuilder.mRepoReq.StartAt, mBuilder.mRepoReq.EndAt, mBuilder.granularity)
	for metricName := range metricNameMap {
		if len(ret[metricName].TimeSeries) == 0 {
			ret[metricName].TimeSeries[defaultGroupKey] = []*entity.MetricPoint{}
		}
		m.fillTimeSeriesData(t, metricName, ret[metricName])
	}
	return ret
}

func (m *MetricsService) fillTimeSeriesData(intervals []string, metricName string, metricVal *entity.Metric) {
	fillVal := "0"
	if fill, ok := m.metricDefMap[metricName].(entity.IMetricFill); ok {
		fillVal = fill.Interpolate()
	}
	for key, timeSeries := range metricVal.TimeSeries {
		mp := lo.Associate(timeSeries, func(item *entity.MetricPoint) (string, string) {
			return item.Timestamp, item.Value
		})
		tmp := make([]*entity.MetricPoint, 0)
		for _, st := range intervals {
			val := fillVal
			if mp[st] != "" {
				val = mp[st]
			}
			tmp = append(tmp, &entity.MetricPoint{
				Timestamp: st,
				Value:     val,
			})
		}
		metricVal.TimeSeries[key] = tmp
	}
}

func (m *MetricsService) formatSummaryData(data []map[string]any, mInfo *metricInfo) map[string]*entity.Metric {
	ret := make(map[string]*entity.Metric)
	if len(data) == 1 {
		for k, v := range data[0] {
			ret[k] = &entity.Metric{
				Summary: getMetricValue(v),
			}
		}
		return ret
	}
	// 正常不应该有下钻, 有下钻就转换为Pie......
	return m.formatPieData(data, mInfo)
}

func (m *MetricsService) formatPieData(data []map[string]any, mInfo *metricInfo) map[string]*entity.Metric {
	ret := make(map[string]*entity.Metric)
	metricNameMap := lo.Associate(mInfo.mAggregation,
		func(item *entity.Dimension) (string, bool) {
			ret[item.Alias] = &entity.Metric{
				Pie: make(map[string]string),
			}
			return item.Alias, true
		})
	for _, dataItem := range data {
		groupByVals := make(map[string]string)
		for k, v := range dataItem {
			if !metricNameMap[k] {
				groupByVals[k] = getMetricValue(v)
			}
		}
		val := defaultGroupKey
		if len(groupByVals) > 0 {
			if data, err := json.Marshal(groupByVals); err == nil {
				val = string(data)
			}
		}
		for k, v := range dataItem {
			if metricNameMap[k] {
				ret[k].Pie[val] = getMetricValue(v)
			}
		}
	}
	return ret
}

func (m *MetricsService) divideMetrics(ctx context.Context, resp []*QueryMetricsResp,
	compoundMetrics []entity.IMetricDefinition, newMetric entity.IMetricDefinition,
) (*QueryMetricsResp, error) {
	if len(resp) != 2 || len(compoundMetrics) != 2 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode)
	} else if resp[0] == nil || resp[1] == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	numerator := resp[0].Metrics[compoundMetrics[0].Name()]
	denominator := resp[1].Metrics[compoundMetrics[1].Name()]
	if numerator == nil || denominator == nil {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	ret := &QueryMetricsResp{
		Metrics: make(map[string]*entity.Metric),
	}
	if numerator.TimeSeries != nil && denominator.TimeSeries != nil {
		ret.Metrics[newMetric.Name()] = &entity.Metric{
			TimeSeries: divideTimeSeries(ctx, numerator.TimeSeries, denominator.TimeSeries),
		}
	} else if numerator.Summary != "" && denominator.Summary != "" {
		ret.Metrics[newMetric.Name()] = &entity.Metric{
			Summary: divideNumber(numerator.Summary, denominator.Summary),
		}
	} else if numerator.TimeSeries != nil && denominator.Summary != "" {
		ret.Metrics[newMetric.Name()] = &entity.Metric{
			TimeSeries: divideTimeSeriesBySummary(ctx, numerator.TimeSeries, denominator.Summary),
		}
	}
	return ret, nil
}

func (m *MetricsService) mergeMetrics(onlineMetric, offlineMetric map[string]entity.Metric) map[string]entity.Metric {
	ret := make(map[string]entity.Metric)
	for metricName, metric := range onlineMetric {
		oMetric, ok := offlineMetric[metricName]
		if !ok { // just use online metric
			ret[metricName] = metric
		}
		if metric.TimeSeries != nil || oMetric.TimeSeries != nil {
			ret[metricName] = m.mergeTimeSeriesMetric(metric, oMetric)
		} else if metric.Summary != "" || oMetric.Summary != "" {
			ret[metricName] = m.mergeSummaryMetric(metric, oMetric)
		} else if metric.Pie != nil || oMetric.Pie != nil {
			ret[metricName] = m.mergePieMetric(metric, oMetric)
		} else { // not expect to be here
			ret[metricName] = metric
		}
	}
	return ret
}

func (m *MetricsService) mergeSummaryMetric(onlineMetric, offlineMetric entity.Metric) entity.Metric {
	return entity.Metric{
		Summary: addNumber(onlineMetric.Summary, offlineMetric.Summary),
	}
}

func (m *MetricsService) mergeTimeSeriesMetric(onlineMetric, offlineMetric entity.Metric) entity.Metric {
	ret := make(entity.TimeSeries)
	for k, val := range onlineMetric.TimeSeries {
		ret[k] = val
	}
	for k, val := range offlineMetric.TimeSeries {
		ret[k] = val
	}
	return entity.Metric{
		TimeSeries: ret,
	}
}

func (m *MetricsService) mergePieMetric(onlineMetric, offlineMetric entity.Metric) entity.Metric {
	// 这里的marhsal实现默认会key排序, 不再次排序
	ret := make(map[string]string)
	for k, val := range onlineMetric.Pie {
		ret[k] = val
	}
	for k, val := range offlineMetric.Pie {
		if ret[k] != "" {
			ret[k] = addNumber(ret[k], val)
		} else {
			ret[k] = val
		}
	}
	return entity.Metric{
		Pie: ret,
	}
}

func divideNumber(a, b string) string {
	numerator, errA := strconv.ParseFloat(a, 64)
	denominator, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return ""
	}
	if math.IsNaN(numerator) ||
		math.IsNaN(denominator) ||
		math.IsInf(numerator, 0) ||
		math.IsInf(denominator, 0) {
		return ""
	}
	if numerator >= 0 && denominator > 0 {
		return strconv.FormatFloat(numerator/denominator, 'f', -1, 64)
	}
	return ""
}

func addNumber(a, b string) string {
	numA, _ := strconv.ParseFloat(a, 64)
	numB, _ := strconv.ParseFloat(b, 64)
	return strconv.FormatFloat(numA+numB, 'f', -1, 64)
}

func divideTimeSeries(ctx context.Context, a, b entity.TimeSeries) entity.TimeSeries {
	ret := make(entity.TimeSeries)
	for k, val := range a {
		anotherVal := b[k]
		if len(val) == 0 || len(anotherVal) == 0 {
			continue
		} else if len(val) != len(anotherVal) {
			logs.CtxWarn(ctx, "time series length mismatch, not expected to be here")
			continue
		}
		sort.Slice(val, func(i, j int) bool {
			return val[i].Timestamp < val[j].Timestamp
		})
		sort.Slice(anotherVal, func(i, j int) bool {
			return anotherVal[i].Timestamp < anotherVal[j].Timestamp
		})
		// 正常情况下这里的key是一样的, 都是完全补齐的时间戳; 不支持下钻后相除...
		ret[k] = make([]*entity.MetricPoint, 0)
		for i := 0; i < len(val); i++ {
			dividedVal := divideNumber(val[i].Value, anotherVal[i].Value)
			if dividedVal == "" {
				dividedVal = "null" // 无法除, 那就是null
			}
			ret[k] = append(ret[k], &entity.MetricPoint{
				Timestamp: val[i].Timestamp,
				Value:     dividedVal,
			})
		}
	}
	return ret
}

func divideTimeSeriesBySummary(ctx context.Context, a entity.TimeSeries, b string) entity.TimeSeries {
	ret := make(entity.TimeSeries)
	for k, val := range a {
		ret[k] = make([]*entity.MetricPoint, 0)
		for i := 0; i < len(val); i++ {
			dividedVal := divideNumber(val[i].Value, b)
			if dividedVal == "" {
				dividedVal = "null" // 无法除, 那就是null
			}
			ret[k] = append(ret[k], &entity.MetricPoint{
				Timestamp: val[i].Timestamp,
				Value:     dividedVal,
			})
		}
	}
	return ret
}

// 预期就是多个Summary结果组合
func (m *MetricsService) pieMetrics(ctx context.Context, resp []*QueryMetricsResp, newMetricName string) (*QueryMetricsResp, error) {
	ret := &QueryMetricsResp{
		Metrics: make(map[string]*entity.Metric),
	}
	ret.Metrics[newMetricName] = &entity.Metric{
		Pie: make(map[string]string),
	}
	for _, r := range resp {
		if r == nil {
			continue
		}
		for metricName, metricVal := range r.Metrics {
			ret.Metrics[newMetricName].Pie[metricName] = metricVal.Summary
		}
	}
	return ret, nil
}

func getMetricValue(v any) string {
	ret := conv.ToString(v)
	if ret == "NaN" || ret == "+Inf" || ret == "-Inf" {
		return "null"
	}
	return ret
}

func (m *MetricsService) TraverseMetrics(ctx context.Context, req *TraverseMetricsReq) error {
	startAt, endAt, err := m.parseStartDate(req.StartDate)
	if err != nil {
		return err
	}
	if len(req.PlatformTypes) == 0 {
		req.PlatformTypes = lo.Keys(m.pMetrics.PlatformMetricDefs)
	}
	metrics := m.buildTraverseMetrics(ctx, req)
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
		if err := m.traverseMetric(ctx, param); err != nil {
			logs.CtxError(ctx, "fail to process metric %s, %v", metric.metricDef.Name(), err)
			continue
		}
	}
	return nil
}

type traverseMetric struct {
	platformType loop_span.PlatformType
	groupName    string
	metricDef    entity.IMetricDefinition
}

func (m *MetricsService) buildTraverseMetrics(ctx context.Context, req *TraverseMetricsReq) []*traverseMetric {
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
					key := fmt.Sprintf("%s_%s", platformType, metric.Name())
					if seen[key] {
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
	logs.CtxInfo(ctx, "metrics to be traversed: %v", metricNames)
	return ret
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
	resp, err := m.queryOnlineMetrics(ctx, qReq)
	if err != nil {
		return err
	}
	mEvents := m.extractMetrics(metricName, resp.Metrics[metricName])
	for _, mEvent := range mEvents {
		mEvent.PlatformType = string(qReq.PlatformType)
		mEvent.StartDate = param.StartDate
		mEvent.MetricName = metricName
	}
	return m.metricProducer.EmitMetrics(ctx, mEvents)
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
	_, ok := m.metricDefMap[metricName]
	if !ok {
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
