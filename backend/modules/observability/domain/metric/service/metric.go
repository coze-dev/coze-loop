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
	timeutil "github.com/coze-dev/coze-loop/backend/pkg/time"
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
	Source          span_filter.SourceType
}

type QueryMetricsResp struct {
	Metrics map[string]*entity.Metric
}

type TraverseMetricsReq struct {
	PlatformTypes []loop_span.PlatformType
	MetricsNames  []string
	WorkspaceID   int64
	StartDate     string // e.g. 2025-11-17
	QueryTimeout  time.Duration
}

type TraverseMetricsResp struct {
	Statistic TraverseMetricStatistic
	Failures  []*TraverseMetricDetail
}

type TraverseMetricStatistic struct {
	Total   int
	Success int
	Failure int
}

type TraverseMetricDetail struct {
	PlatformType loop_span.PlatformType
	MetricName   string
	Error        error
	TimeCost     time.Duration
}

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsService
type IMetricsService interface {
	QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error)
	TraverseMetrics(ctx context.Context, req *TraverseMetricsReq) (*TraverseMetricsResp, error)
	GetMetricGroupBy(metricName string) ([]string, error)
}

type MetricsService struct {
	metricRepo      repo.IMetricRepo
	oMetricRepo     repo.IOfflineMetricRepo
	metricDefMap    map[string]entity.IMetricDefinition
	metricDrillDown map[string][]string
	buildHelper     trace_service.TraceFilterProcessorBuilder
	tenantProvider  tenant.ITenantProvider
	traceConfig     config.ITraceConfig
	pMetrics        *entity.PlatformMetrics
}

func NewMetricsService(
	metricRepo repo.IMetricRepo,
	oMetricRepo repo.IOfflineMetricRepo,
	tenantProvider tenant.ITenantProvider,
	buildHelper trace_service.TraceFilterProcessorBuilder,
	traceConfig config.ITraceConfig,
	pMetrics *entity.PlatformMetrics,
) (IMetricsService, error) {
	ret := &MetricsService{
		metricRepo:     metricRepo,
		oMetricRepo:    oMetricRepo,
		tenantProvider: tenantProvider,
		buildHelper:    buildHelper,
		traceConfig:    traceConfig,
		pMetrics:       pMetrics,
	}
	if err := ret.registerMetrics(); err != nil {
		return nil, err
	}
	if err := ret.checkMetricsValid(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (m *MetricsService) registerMetrics() error {
	metricDefMap := make(map[string]entity.IMetricDefinition)
	metricDrillDown := make(map[string][]string)
	for _, metricGroup := range m.pMetrics.MetricGroups {
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
				if def.Name() == "" {
					return fmt.Errorf("metric name is blank")
				}
				if metricDefMap[def.Name()] != nil {
					return fmt.Errorf("duplicate metric name %s", def.Name())
				}
				metricDefMap[def.Name()] = def
			}
			groupMetrics = append(groupMetrics, metrics...)
		}
		metricGroup.MetricDefinitions = groupMetrics // expand wrapper metrics
		for _, def := range groupMetrics {
			name := def.Name()
			if _, ok := metricDrillDown[name]; ok {
				return fmt.Errorf("metric name already belongs %s", name)
			}
			metricDrillDown[name] = metricGroup.DrillDownObjects
		}
	}
	m.metricDefMap = metricDefMap
	m.metricDrillDown = metricDrillDown
	logs.Info("%d metrics registered", len(metricDefMap))
	return nil
}

func (m *MetricsService) checkMetricsValid() error {
	for _, metricDef := range m.metricDefMap {
		if err := m.checkMetricValid(metricDef); err != nil {
			return err
		}
	}
	return nil
}

func (m *MetricsService) GetMetricGroupBy(metricName string) ([]string, error) {
	metricDef, ok := m.metricDefMap[metricName]
	if !ok {
		return nil, fmt.Errorf("metric definition %s not found", metricName)
	}
	dims := metricDef.GroupBy()
	keys := make([]string, 0, len(dims))
	for _, dim := range dims {
		if dim.Alias == "" {
			return nil, fmt.Errorf("%s groupby dimension has no alias", metricName)
		}
		keys = append(keys, dim.Alias)
	}
	return keys, nil
}

func (m *MetricsService) checkMetricValid(def entity.IMetricDefinition) error {
	if _, ok := def.(entity.IMetricConst); ok {
		return nil // skip const metric
	} else if compoundMetric, ok := def.(entity.IMetricCompound); ok {
		// check compound metric all valid
		for _, nestMetric := range compoundMetric.GetMetrics() {
			if _, ok := nestMetric.(entity.IMetricConst); ok {
				continue
			} else if _, ok := nestMetric.(entity.IMetricCompound); ok {
				return fmt.Errorf("nested compound metric %s is not allowed", nestMetric.Name())
			}
			if m.metricDefMap[nestMetric.Name()] == nil {
				return fmt.Errorf("metric name %s not registered", nestMetric.Name())
			}
		}
	} else {
		// check group by valid
		dimensions := def.GroupBy()
		for _, dimension := range dimensions {
			filedName := dimension.Field.FieldName
			fieldType := dimension.Field.FieldType
			isValidFieldName := false
			for _, field := range m.pMetrics.DrillDownObjects {
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
				return fmt.Errorf("metric name %s group by field %s not valid", def.Name(), filedName)
			} else if dimension.Alias == "" {
				return fmt.Errorf("metric name %s group by field %s alias not valid", def.Name(), filedName)
			}
		}
		// check offline expression valid
		expr := def.OExpression()
		if expr.AggrType == "" {
			return fmt.Errorf("metric name %s offline expression aggr type not valid", def.Name())
		}
	}
	return nil
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
	qCfg := m.traceConfig.GetMetricQueryConfig(ctx)
	val := qCfg.SpaceConfigs[strconv.FormatInt(req.WorkspaceID, 10)]
	if val != nil && val.DisableQuery {
		return &QueryMetricsResp{}, nil
	}
	if req.FilterFields != nil {
		if err := req.FilterFields.Traverse(processSpecificFilter); err != nil {
			return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
		}
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
	if !qCfg.SupportOffline { // 不支持离线指标
		return m.queryOnlineMetrics(ctx, req)
	} else if m.pMetrics.PlatformMetricDefs[req.PlatformType] == nil { // 不在离线指标计算范围内
		return m.queryOnlineMetrics(ctx, req)
	}
	boundary := getDaysBeforeTimeStamp(qCfg.OfflineCriticalPoint)
	logs.CtxInfo(ctx, "query offline critical point at %d", boundary)
	if req.StartTime >= boundary {
		return m.queryOnlineMetrics(ctx, req)
	} else if req.EndTime < boundary {
		return m.queryOfflineMetrics(ctx, req)
	} else {
		start, end := req.StartTime, req.EndTime
		req.StartTime, req.EndTime = boundary, end
		onlineMetric, err := m.queryOnlineMetrics(ctx, req)
		if err != nil {
			return nil, err
		}
		req.StartTime, req.EndTime = start, boundary-1
		offlineMetric, err := m.queryOfflineMetrics(ctx, req)
		if err != nil {
			return nil, err
		}
		return &QueryMetricsResp{
			Metrics: m.mergeMetrics(onlineMetric.Metrics, offlineMetric.Metrics),
		}, nil
	}
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
		WorkSpaceID: strconv.FormatInt(req.WorkspaceID, 10),
		Tenants:     tenants,
		StartAt:     req.StartTime,
		EndAt:       req.EndTime,
	}
	mBuilder := &metricQueryBuilder{
		metricNames: req.MetricsNames,
		filter:      filter,
		spanEnv: &span_filter.SpanEnv{
			WorkspaceID: req.WorkspaceID,
			Source:      req.Source,
		},
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
					f.QueryType = ptr.Of(loop_span.QueryTypeEnumExist)
				}
			}
			return nil
		})
		mBuilder.mRepoReq.Source = repo.MetricSourceOffline
	}
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
	metricNameMap := lo.Associate(mInfo.mAggregation,
		func(item *entity.Dimension) (string, bool) {
			ret[item.Alias] = &entity.Metric{
				Pie: make(map[string]string),
			}
			return item.Alias, true
		})
	if len(data) == 0 {
		return ret
	} else if len(data) == 1 {
		isSummaryData := true
		for k, v := range data[0] {
			if _, ok := metricNameMap[k]; !ok {
				isSummaryData = false
				break
			}
			ret[k] = &entity.Metric{
				Summary: getMetricValue(v),
			}
		}
		if isSummaryData {
			return ret
		}
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

func (m *MetricsService) mergeMetrics(onlineMetrics, offlineMetrics map[string]*entity.Metric) map[string]*entity.Metric {
	ret := make(map[string]*entity.Metric)
	metricNames := lo.UniqKeys(onlineMetrics, offlineMetrics)
	for _, metricName := range metricNames {
		offlineMetric := offlineMetrics[metricName]
		onlineMetric := onlineMetrics[metricName]
		if onlineMetric == nil {
			ret[metricName] = offlineMetric
			continue
		} else if offlineMetric == nil {
			ret[metricName] = onlineMetric
			continue
		}
		if onlineMetric.TimeSeries != nil || offlineMetric.TimeSeries != nil {
			ret[metricName] = m.mergeTimeSeriesMetric(onlineMetric, offlineMetric)
		} else if onlineMetric.Summary != "" || offlineMetric.Summary != "" {
			ret[metricName] = m.mergeSummaryMetric(onlineMetric, offlineMetric)
		} else if len(onlineMetric.Pie) > 0 || len(offlineMetric.Pie) > 0 {
			ret[metricName] = m.mergePieMetric(onlineMetric, offlineMetric)
		}
	}
	return ret
}

func (m *MetricsService) mergeSummaryMetric(onlineMetric, offlineMetric *entity.Metric) *entity.Metric {
	return &entity.Metric{
		Summary: addNumber(onlineMetric.Summary, offlineMetric.Summary),
	}
}

func (m *MetricsService) mergeTimeSeriesMetric(onlineMetric, offlineMetric *entity.Metric) *entity.Metric {
	ret := make(entity.TimeSeries)
	// order; offline first
	for k, val := range offlineMetric.TimeSeries {
		ret[k] = val
	}
	for k, val := range onlineMetric.TimeSeries {
		ret[k] = append(ret[k], val...)
	}
	return &entity.Metric{
		TimeSeries: ret,
	}
}

func (m *MetricsService) mergePieMetric(onlineMetric, offlineMetric *entity.Metric) *entity.Metric {
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
	return &entity.Metric{
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

func getMetricValue(v any) string {
	ret := conv.ToString(v)
	if ret == "NaN" || ret == "+Inf" || ret == "-Inf" {
		return "null"
	}
	return ret
}

func getDaysBeforeTimeStamp(days int) int64 {
	now := time.Now()
	daysBefore := time.Date(now.Year(), now.Month(), now.Day()-days, 0, 0, 0, 0, now.Location())
	return daysBefore.UnixMilli()
}

// TODO: 合并，有三个地方实现一样...
func processSpecificFilter(f *loop_span.FilterField) error {
	switch f.FieldName {
	case loop_span.SpanFieldStatus:
		if err := processStatusFilter(f); err != nil {
			return err
		}
	case loop_span.SpanFieldDuration,
		loop_span.SpanFieldLatencyFirstResp,
		loop_span.SpanFieldStartTimeFirstResp,
		loop_span.SpanFieldStartTimeFirstTokenResp,
		loop_span.SpanFieldLatencyFirstTokenResp,
		loop_span.SpanFieldReasoningDuration:
		if err := processLatencyFilter(f); err != nil {
			return err
		}
	}
	return nil
}

func processStatusFilter(f *loop_span.FilterField) error {
	if f.QueryType == nil || *f.QueryType != loop_span.QueryTypeEnumIn {
		return fmt.Errorf("status filter should use in operator")
	}
	f.FieldName = loop_span.SpanFieldStatusCode
	f.FieldType = loop_span.FieldTypeLong
	checkSuccess, checkError := false, false
	for _, val := range f.Values {
		switch val {
		case loop_span.SpanStatusSuccess:
			checkSuccess = true
		case loop_span.SpanStatusError:
			checkError = true
		default:
			return fmt.Errorf("invalid status code field value")
		}
	}
	if checkSuccess && checkError {
		f.QueryType = ptr.Of(loop_span.QueryTypeEnumAlwaysTrue)
		f.Values = nil
	} else if checkSuccess {
		f.Values = []string{"0"}
	} else if checkError {
		f.QueryType = ptr.Of(loop_span.QueryTypeEnumNotIn)
		f.Values = []string{"0"}
	} else {
		return fmt.Errorf("invalid status code query")
	}
	return nil
}

// ms -> us
func processLatencyFilter(f *loop_span.FilterField) error {
	if f.FieldType != loop_span.FieldTypeLong {
		return fmt.Errorf("latency field type should be long ")
	}
	micros := make([]string, 0)
	for _, val := range f.Values {
		integer, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("fail to parse long value %s, %v", val, err)
		}
		integer = timeutil.MillSec2MicroSec(integer)
		micros = append(micros, strconv.FormatInt(integer, 10))
	}
	f.Values = micros
	return nil
}
