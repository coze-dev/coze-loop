// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"reflect"

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
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/samber/lo"
)

type QueryMetricsReq struct {
	PlatformType    loop_span.PlatformType
	WorkspaceID     int64
	MetricsNames    []string
	Granularity     entity.MetricGranularity
	FilterFields    *loop_span.FilterFields
	DrillDownFields []*loop_span.FilterField
	StartTime       int64
	EndTime         int64
}

type QueryMetricsResp struct {
	Metrics map[string]*entity.Metric
}

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsService
type IMetricsService interface {
	QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error)
}

type MetricsService struct {
	metricRepo     repo.IMetricRepo
	metricDefMap   map[string]entity.IMetricDefinition
	buildHelper    trace_service.TraceFilterProcessorBuilder
	tenantProvider tenant.ITenantProvider
}

func NewMetricsService(
	metricRepo repo.IMetricRepo,
	metricDefs []entity.IMetricDefinition,
	tenantProvider tenant.ITenantProvider,
	buildHelper trace_service.TraceFilterProcessorBuilder,
) (IMetricsService, error) {
	metricDefMap := make(map[string]entity.IMetricDefinition)
	for _, def := range metricDefs {
		if metricDefMap[def.Name()] != nil {
			return nil, fmt.Errorf("duplicate metric name %s", def.Name())
		}
		metricDefMap[def.Name()] = def
	}
	return &MetricsService{
		metricRepo:     metricRepo,
		metricDefMap:   metricDefMap,
		tenantProvider: tenantProvider,
		buildHelper:    buildHelper,
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
	mBuilder, err := m.buildMetricQuery(ctx, req)
	if err != nil {
		return nil, err
	} else if mBuilder == nil {
		return &QueryMetricsResp{}, nil // 不再查询...
	}
	result, err := m.metricRepo.GetMetrics(ctx, mBuilder.mRepoReq)
	if err != nil {
		return nil, err
	}
	return &QueryMetricsResp{
		Metrics: m.formatMetrics(result.Data, mBuilder),
	}, nil
}

func (m *MetricsService) buildMetricQuery(ctx context.Context, req *QueryMetricsReq) (*metricQueryBuilder, error) {
	filter, err := m.buildHelper.BuildPlatformRelatedFilter(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	tenants, err := m.tenantProvider.GetTenantsByPlatformType(ctx, req.PlatformType)
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
		mInfo.mAggregation = []*entity.Dimension{{
			Expression: metricDef.Expression(builder.granularity),
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
	switch mInfo.mType {
	case entity.MetricTypeTimeSeries:
		return m.formatTimeSeriesData(data, mBuilder)
	case entity.MetricTypeSummary: // 预期不会有聚合, 有就是参数问题
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
				TimeSeries: make(map[string][]*entity.MetricPoint),
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
		val := "all"
		if len(groupByVals) > 0 {
			if data, err := json.Marshal(groupByVals); err == nil {
				val = string(data)
			}
		}
		for k, v := range dataItem {
			if metricNameMap[k] {
				ret[k].TimeSeries[val] = append(ret[k].TimeSeries[val], &entity.MetricPoint{
					Timestamp: conv.ToString(dataItem[timeBucketKey]),
					Value:     conv.ToString(v),
				})
			}
		}
		// 填充零值...
		t := entity.NewTimeIntervals(mBuilder.mRepoReq.StartAt, mBuilder.mRepoReq.EndAt, mBuilder.granularity)
		for metricName, metricVal := range ret {
			m.fillTimeSeriesData(t, metricName, metricVal)
		}
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
	for _, name := range mInfo.mAggregation {
		ret[name.Alias] = &entity.Metric{}
	}
	for _, dataItem := range data {
		for k, v := range dataItem { // 预期不应该有下钻, 有就是参数问题
			ret[k] = &entity.Metric{
				Summary: conv.ToString(v),
			}
			break
		}
	}
	return ret
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
				groupByVals[k] = conv.ToString(v)
			}
		}
		val := "all"
		if len(groupByVals) > 0 {
			if data, err := json.Marshal(groupByVals); err == nil {
				val = string(data)
			}
		}
		for k, v := range dataItem {
			if metricNameMap[k] {
				ret[k].Pie[val] = conv.ToString(v)
			}
		}
	}
	return ret
}
