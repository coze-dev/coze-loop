// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsService
type IMetricsService interface {
	QueryMetrics(ctx context.Context, req *entity.QueryMetricsReq) (*entity.QueryMetricsResp, error)
}

// MetricsService 指标服务实现
type MetricsService struct {
	metricsRepo repo.IMetricsRepo
	definitions []entity.IMetricDefinition
}

// NewMetricsService 创建指标服务
func NewMetricsService(metricsRepo repo.IMetricsRepo, definitions []entity.IMetricDefinition) IMetricsService {
	return &MetricsService{
		metricsRepo: metricsRepo,
		definitions: definitions,
	}
}

// QueryMetrics 查询指标数据
func (s *MetricsService) QueryMetrics(ctx context.Context, req *entity.QueryMetricsReq) (*entity.QueryMetricsResp, error) {
	// TODO: 实现指标查询逻辑
	// 1. 根据指标名称找到对应的定义
	// 2. 构建查询参数
	// 3. 调用repo层获取数据
	// 4. 格式化返回结果
	
	resp := &entity.QueryMetricsResp{
		Metrics: make(map[string]*entity.Metric),
	}
	
	// 遍历请求的指标名称
	for _, metricName := range req.MetricsNames {
		// 查找指标定义
		var definition entity.IMetricDefinition
		for _, def := range s.definitions {
			if def.Name() == metricName {
				definition = def
				break
			}
		}
		
		if definition == nil {
			continue // 跳过未找到的指标定义
		}
		
		// 获取指标的筛选条件
		whereFilters, whereErr := definition.Where(req.FilterFields)
		if whereErr != nil {
			return nil, whereErr
		}
		
		// 合并筛选条件
		mergedFilters := req.FilterFields
		if whereFilters != nil && len(whereFilters) > 0 {
			if mergedFilters == nil {
				mergedFilters = &loop_span.FilterFields{
					FilterFields: whereFilters,
				}
			} else {
				// 合并过滤条件
				if mergedFilters.FilterFields == nil {
					mergedFilters.FilterFields = whereFilters
				} else {
					mergedFilters.FilterFields = append(mergedFilters.FilterFields, whereFilters...)
				}
			}
		}
		
		// 构建查询参数
		param := &entity.GetMetricsParam{
			Tenants: []string{req.WorkspaceID},
			Aggregations: []*entity.Dimension{
				{
					Expression: definition.Expression(),
					Alias:      definition.Name(),
				},
			},
			Filters:     mergedFilters,
			StartAt:     req.StartTime,
			EndAt:       req.EndTime,
			Granularity: req.Granularity,
			GroupBys:    definition.GroupBy(),
		}
		
		// 根据指标类型调用不同的查询方法
		var result *entity.GetMetricsResult
		var err error
		
		switch definition.Type() {
		case entity.MetricTypeTimeSeries:
			result, err = s.metricsRepo.GetTimeSeries(ctx, param)
		case entity.MetricTypeSummary:
			result, err = s.metricsRepo.GetSummary(ctx, param)
		case entity.MetricTypePie:
			result, err = s.metricsRepo.GetPie(ctx, param)
		default:
			continue
		}
		
		if err != nil {
			return nil, err
		}
		
		// 格式化结果
		metric := &entity.Metric{}
		if result != nil && len(result.Data) > 0 {
			// TODO: 根据指标类型格式化数据
			// 这里需要根据具体的数据格式进行转换
		}
		
		resp.Metrics[metricName] = metric
	}
	
	return resp, nil
}