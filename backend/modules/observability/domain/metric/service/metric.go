// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_service "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
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

func (s *MetricsService) QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	mBuilder, err := s.buildMetricQuery(ctx, req)
	if err != nil {
		return nil, err
	} else if mBuilder == nil {
		return &QueryMetricsResp{}, nil // 不再查询...
	}
	result, err := s.metricRepo.GetMetrics(ctx, mBuilder.mRepoReq)
	if err != nil {
		return nil, err
	}
	return &QueryMetricsResp{
		Metrics: s.formatMetrics(result.Data, mBuilder.mInfo),
	}, nil
}

func (s *MetricsService) buildMetricQuery(ctx context.Context, req *QueryMetricsReq) (*metricQueryBuilder, error) {
	filter, err := s.buildHelper.BuildPlatformRelatedFilter(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	tenants, err := s.tenantProvider.GetTenantsByPlatformType(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	param := &repo.GetMetricsParam{
		Tenants:     tenants,
		StartAt:     req.StartTime,
		EndAt:       req.EndTime,
		Granularity: req.Granularity,
	}
	mBuilder := &metricQueryBuilder{
		metricNames:   req.MetricsNames,
		filter:        filter,
		spanEnv:       &span_filter.SpanEnv{WorkspaceID: req.WorkspaceID},
		requestFilter: req.FilterFields,
		granularity:   req.Granularity,
	}
	if err := s.buildMetricInfo(ctx, mBuilder); err != nil {
		return nil, err
	}
	mFilter, err := s.buildFilter(ctx, mBuilder)
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

func (s *MetricsService) buildMetricInfo(ctx context.Context, builder *metricQueryBuilder) error {
	var (
		mInfos = make([]*metricInfo, 0)
		err    error
	)
	for _, metricName := range builder.metricNames {
		metricDef, ok := s.metricDefMap[metricName]
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

func (s *MetricsService) buildFilter(ctx context.Context, mBuilder *metricQueryBuilder) (*loop_span.FilterFields, error) {
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

func (s *MetricsService) formatMetrics(data []map[string]any, mInfo *metricInfo) map[string]*entity.Metric {
	metricNameMap := lo.Associate(mInfo.mAggregation,
		func(item *entity.Dimension) (string, bool) {
			return item.Alias, true
		})
	ret := make(map[string]*entity.Metric)
	switch mInfo.mType {
	case entity.MetricTypeTimeSeries:
		for _, dataItem := range data {
			groupByVals := []string{}
			for k, v := range dataItem {
				if !metricNameMap[k] && k != timeBucketKey {
					groupByVals = append(groupByVals, conv.ToString(v))
				}
			}
			// 这一条聚合结果对应的聚合值,如果有多个,整合成一个
			val := strings.Join(groupByVals, "-")
			if val == "" {
				val = "all"
			}
			for k, v := range dataItem {
				if metricNameMap[k] {
					if ret[k] == nil {
						ret[k] = &entity.Metric{
							TimeSeries: make(map[string][]*entity.MetricPoint),
						}
					}
					ret[k].TimeSeries[val] = append(ret[k].TimeSeries[val], &entity.MetricPoint{
						Timestamp: conv.ToString(dataItem[timeBucketKey]),
						Value:     conv.ToString(v),
					})
				}
			}
		}
	case entity.MetricTypeSummary: // 预期不会有聚合, 有就是参数问题
		for _, dataItem := range data {
			for k, v := range dataItem {
				if metricNameMap[k] {
					ret[k] = &entity.Metric{
						Summary: conv.ToString(v),
					}
				}
			}
		}
	case entity.MetricTypePie:
		for _, dataItem := range data {
			groupByVals := []string{}
			for k, v := range dataItem {
				if !metricNameMap[k] {
					groupByVals = append(groupByVals, conv.ToString(v))
				}
			}
			// 这一条聚合结果对应的聚合值,如果有多个,整合成一个
			val := strings.Join(groupByVals, "-")
			if val == "" {
				val = "all"
			}
			for k, v := range dataItem {
				if metricNameMap[k] {
					if ret[k] == nil {
						ret[k] = &entity.Metric{
							Pie: make(map[string]string),
						}
					}
					ret[k].Pie[val] = conv.ToString(v)
				}
			}
		}
	}
	return ret
}
