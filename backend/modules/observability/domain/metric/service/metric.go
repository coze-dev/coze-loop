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

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsService
type IMetricsService interface {
	QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error)
	TraverseMetrics(ctx context.Context, req *TraverseMetricsReq) error
}

type MetricsService struct {
	metricRepo     repo.IMetricRepo
	metricDefMap   map[string]entity.IMetricDefinition
	buildHelper    trace_service.TraceFilterProcessorBuilder
	tenantProvider tenant.ITenantProvider
	traceConfig    config.ITraceConfig
	metricProducer mq.IMetricProducer
}

func NewMetricsService(
	metricRepo repo.IMetricRepo,
	metricDefs []entity.IMetricDefinition,
	tenantProvider tenant.ITenantProvider,
	buildHelper trace_service.TraceFilterProcessorBuilder,
	traceConfig config.ITraceConfig,
	metricProducer mq.IMetricProducer,
) (IMetricsService, error) {
	metricDefMap := make(map[string]entity.IMetricDefinition)
	for _, def := range metricDefs {
		metrics := []entity.IMetricDefinition{}
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
	}
	logs.Info("%d metrics registered", len(metricDefMap))
	return &MetricsService{
		metricRepo:     metricRepo,
		metricDefMap:   metricDefMap,
		tenantProvider: tenantProvider,
		buildHelper:    buildHelper,
		traceConfig:    traceConfig,
		metricProducer: metricProducer,
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

func (m *MetricsService) TraverseMetrics(ctx context.Context, req *TraverseMetricsReq) error {
	cfg, err := m.traceConfig.GetMetricDefinitions(ctx)
	if err != nil {
		logs.CtxError(ctx, "fail to get metric definitions, %v", err)
		return err
	}
	startAt, err := time.ParseInLocation(time.DateOnly, req.StartDate, time.Local)
	if err != nil {
		logs.CtxError(ctx, "fail to parse time %s, %v", req.StartDate, err)
		return err
	}
	endAt := time.Date(startAt.Year(), startAt.Month(), startAt.Day(), 23, 59, 59, 999999999, startAt.Location())
	if len(req.PlatformTypes) == 0 {
		req.PlatformTypes = lo.Keys(cfg)
	}
	for _, platformType := range req.PlatformTypes {
		platformTypeCfg := cfg[platformType]
		if platformTypeCfg == nil {
			logs.CtxError(ctx, "platform type %s not found", platformType)
			continue
		}
		metricsNames := req.MetricsNames
		metricsDef := lo.Keys(platformTypeCfg.MetricDefinitions)
		if len(metricsNames) == 0 {
			metricsNames = lo.Keys(platformTypeCfg.MetricDefinitions)
		} else {
			metricsNames = lo.Intersect(metricsNames, metricsDef)
		}
		logs.CtxInfo(ctx, "Traverse metrics for %s: %v", platformType, metricsNames)
		for _, metricName := range metricsNames {
			if metricDef, ok := m.metricDefMap[metricName]; !ok {
				logs.CtxWarn(ctx, "metric definition %s not found", metricName)
				continue
			} else if _, ok = metricDef.(entity.IMetricCompound); ok {
				logs.CtxWarn(ctx, "metric definition %s is compound, ignore it", metricName)
				continue
			}
			qReq := &QueryMetricsReq{
				PlatformType:   platformType,
				WorkspaceID:    req.WorkspaceID,
				MetricsNames:   []string{metricName},
				Granularity:    entity.MetricGranularity1Day,
				StartTime:      startAt.UnixMilli(),
				EndTime:        endAt.UnixMilli(),
				GroupBySpaceID: true,
			}
			for _, obj := range platformTypeCfg.DrillDownObjects {
				qReq.DrillDownFields = append(qReq.DrillDownFields, obj.Field)
			}
			for _, obj := range platformTypeCfg.MetricDefinitions[metricName] {
				qReq.DrillDownFields = append(qReq.DrillDownFields, obj.Field)
			}
			qReq.DrillDownFields = append(qReq.DrillDownFields, &loop_span.FilterField{
				FieldName: loop_span.SpanFieldSpaceId,
				FieldType: loop_span.FieldTypeString,
			})
			resp, err := m.queryMetrics(ctx, qReq)
			if err != nil {
				logs.CtxWarn(ctx, "fail to query metric, %v", err)
				continue
			}
			metricName := qReq.MetricsNames[0]
			mEvents := m.extractMetrics(metricName, resp.Metrics[metricName], platformTypeCfg)
			for _, mEvent := range mEvents {
				mEvent.PlatformType = string(qReq.PlatformType)
				mEvent.StartDate = req.StartDate
				mEvent.MetricName = metricName
			}
			_ = m.metricProducer.EmitMetrics(ctx, mEvents)
		}
	}
	return nil
}

func (m *MetricsService) queryCompoundMetric(ctx context.Context, req *QueryMetricsReq, mDef entity.IMetricDefinition) (*QueryMetricsResp, error) {
	mCompound := mDef.(entity.IMetricCompound)
	metrics := mCompound.GetMetrics()
	if len(metrics) == 0 {
		return &QueryMetricsResp{}, nil
	}
	var (
		metricsResp = make([]*QueryMetricsResp, len(metrics))
		eGroup      errgroup.Group
		lock        sync.Mutex
	)
	for i, metric := range metrics {
		eGroup.Go(func(t int) func() error {
			return func() error {
				defer goroutine.Recovery(ctx)
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
				resp, err := m.queryMetrics(ctx, sReq)
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
		// time_series相除/summary相除
		return m.divideMetrics(ctx, metricsResp, mCompound.GetMetrics(), mDef)
	case entity.MetricOperatorPie:
		// summary指标组合构成饼图
		return m.pieMetrics(ctx, metricsResp, mDef.Name())
	default:
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
}

func (m *MetricsService) queryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	mBuilder, err := m.buildMetricQuery(ctx, req)
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
	logs.CtxInfo(ctx, "get metrics for %v successfully, cost %v", req.MetricsNames, time.Since(st))
	return &QueryMetricsResp{
		Metrics: m.formatMetrics(result.Data, mBuilder),
	}, nil
}

func (m *MetricsService) buildMetricQuery(ctx context.Context, req *QueryMetricsReq) (*metricQueryBuilder, error) {
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
				f.QueryType = ptr.Of(loop_span.QueryTypeEnumAlwaysTrue)
			}
			return nil
		})
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
	}
	return ret, nil
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

func (m *MetricsService) extractMetrics(metricName string, metric *entity.Metric, cfg *config.MetricPlatformConfig) []*entity.MetricEvent {
	def := m.metricDefMap[metricName]
	if def == nil {
		return nil
	}
	objectList := append(cfg.DrillDownObjects, cfg.MetricDefinitions[metricName]...)
	objectKeys := lo.Associate(objectList, func(item *config.MetricObjectKeyConfig) (string, *config.MetricObjectKeyConfig) {
		return item.Field.FieldName, item
	})
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
					if val := objectKeys[objName]; val != nil {
						event.ObjectKeys[val.StorageKey] = objVal
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
					if val := objectKeys[objName]; val != nil {
						event.ObjectKeys[val.StorageKey] = objVal
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
