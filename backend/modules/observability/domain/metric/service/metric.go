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
	PlatformType loop_span.PlatformType
	WorkspaceID  int64
	MetricsNames []string
	Granularity  entity.MetricGranularity
	FilterFields *loop_span.FilterFields
	StartTime    int64
	EndTime      int64
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
	metricDefMap   map[entity.MetricName]entity.IMetricDefinition
	buildHelper    trace_service.TraceFilterProcessorBuilder
	tenantProvider tenant.ITenantProvider
}

func NewMetricsService(
	metricRepo repo.IMetricRepo,
	metricDefs []entity.IMetricDefinition,
	tenantProvider tenant.ITenantProvider,
	buildHelper trace_service.TraceFilterProcessorBuilder,
) (IMetricsService, error) {
	metricDefMap := make(map[entity.MetricName]entity.IMetricDefinition)
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

type metricInfo struct {
	mType        entity.MetricType
	mAggregation []*entity.Dimension
	mGroupBy     []*entity.Dimension
	mWhere       []*loop_span.FilterField
}

func (s *MetricsService) QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	filter, err := s.buildHelper.BuildPlatformRelatedFilter(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	tenants, err := s.tenantProvider.GetTenantsByPlatformType(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}
	param := repo.GetMetricsParam{
		Tenants:     tenants,
		StartAt:     req.StartTime,
		EndAt:       req.EndTime,
		Granularity: req.Granularity,
	}
	spanEnv := &span_filter.SpanEnv{
		WorkspaceID: req.WorkspaceID,
	}
	metricInfos := make([]*metricInfo, 0)
	for _, metricName := range req.MetricsNames {
		metricDef, ok := s.metricDefMap[metricName]
		if !ok {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode,
				errorx.WithExtraMsg(fmt.Sprintf("metric definition %s not found", metricName)))
		}
		mInfo := &metricInfo{}
		mInfo.mType = metricDef.Type()
		mInfo.mGroupBy = metricDef.GroupBy()
		mInfo.mWhere, err = metricDef.Where(ctx, filter, spanEnv)
		if err != nil {
			return nil, errorx.WrapByCode(err, obErrorx.CommercialCommonInvalidParamCodeCode)
		}
		mInfo.mAggregation = []*entity.Dimension{{
			Expression: metricDef.Expression(req.Granularity),
			Alias:      string(metricDef.Name()), // 聚合指标的别名是指标名，以此后续来拆分数据
		}}
		metricInfos = append(metricInfos, mInfo)
	}
	mInfo, err := s.combineMetricInfos(metricInfos)
	if err != nil {
		return nil, err
	}
	mFilter, err := s.buildMetricFilter(ctx, filter, spanEnv, mInfo.mWhere, req.FilterFields)
	if err != nil {
		return nil, err
	} else if mFilter == nil {
		return &QueryMetricsResp{}, nil
	}
	param.Aggregations = mInfo.mAggregation
	param.GroupBys = mInfo.mGroupBy // todo 需要传group by,指标底层
	param.Filters = mFilter
	result, err := s.metricRepo.GetMetrics(ctx, &param)
	if err != nil {
		return nil, err
	}
	return &QueryMetricsResp{
		Metrics: s.formatMetrics(result.Data, mInfo),
	}, nil
}

// TODO: 怎么确定Basic Filter，统计能看到的还是总的？？
func (s *MetricsService) buildMetricFilter(ctx context.Context,
	filter span_filter.Filter,
	spanEnv *span_filter.SpanEnv,
	metricFilters []*loop_span.FilterField,
	requestFilter *loop_span.FilterFields,
) (*loop_span.FilterFields, error) {
	basicFilter, forceQuery, err := filter.BuildBasicSpanFilter(ctx, spanEnv)
	if err != nil {
		return nil, err
	} else if len(basicFilter) == 0 && !forceQuery {
		return nil, nil
	}
	basicFilter = append(basicFilter, metricFilters...)
	return &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter:  &loop_span.FilterFields{FilterFields: basicFilter},
			},
			{
				QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
				SubFilter:  requestFilter,
			},
		},
	}, nil
}

func (s *MetricsService) combineMetricInfos(mInfos []*metricInfo) (*metricInfo, error) {
	if len(mInfos) == 0 {
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode)
	}
	out := mInfos[0]
	for i := 1; i < len(mInfos); i++ {
		mInfo := mInfos[i]
		if mInfo.mType != out.mType {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("metric types not the same"))
		} else if !reflect.DeepEqual(out.mWhere, mInfo.mWhere) {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("metric condition not the same"))
		} else if !reflect.DeepEqual(out.mGroupBy, mInfo.mGroupBy) {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("metric groupby not the same"))
		}
		out.mAggregation = append(out.mAggregation, mInfo.mAggregation...)
	}
	return out, nil
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
