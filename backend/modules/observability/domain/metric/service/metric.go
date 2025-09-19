// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type QueryMetricsReq struct {
	WorkspaceID  string
	StartTime    int64
	EndTime      int64
	PlatformType string
	MetricsNames []string
	Granularity  string
	FilterFields *loop_span.FilterFields
}

type QueryMetricsResp struct {
	Metrics map[string]*entity.Metric
}

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricsService
type IMetricsService interface {
	QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error)
}

type MetricsService struct {
	metricsRepo           repo.IMetricsRepo
	definitions           []entity.IMetricDefinition
	definitionsMap        map[entity.MetricName]entity.IMetricDefinition
	platformFilterFactory span_filter.PlatformFilterFactory
	tenantProvider        tenant.ITenantProvider
}

func NewMetricsService(
	metricsRepo repo.IMetricsRepo,
	definitions []entity.IMetricDefinition,
	platformFilterFactory span_filter.PlatformFilterFactory,
	tenantProvider tenant.ITenantProvider,
) IMetricsService {
	// 构建 definitionsMap 以优化查找性能
	definitionsMap := make(map[entity.MetricName]entity.IMetricDefinition)
	for _, def := range definitions {
		definitionsMap[def.Name()] = def
	}

	return &MetricsService{
		metricsRepo:           metricsRepo,
		definitions:           definitions,
		definitionsMap:        definitionsMap,
		platformFilterFactory: platformFilterFactory,
		tenantProvider:        tenantProvider,
	}
}

// getTenants 根据 PlatformType 获取 Tenants
func (s *MetricsService) getTenants(ctx context.Context, platformType string) ([]string, error) {
	return s.tenantProvider.GetTenantsByPlatformType(ctx, loop_span.PlatformType(platformType))
}

func (s *MetricsService) QueryMetrics(ctx context.Context, req *QueryMetricsReq) (*QueryMetricsResp, error) {
	resp := &QueryMetricsResp{
		Metrics: make(map[string]*entity.Metric),
	}

	// 1. 根据 PlatformType 获取 Tenants
	tenants, err := s.getTenants(ctx, req.PlatformType)
	if err != nil {
		return nil, err
	}

	// 遍历请求的指标名称
	for _, metricName := range req.MetricsNames {
		// 2. 使用 map 快速查找指标定义
		definition, exists := s.definitionsMap[entity.MetricName(metricName)]
		if !exists {
			continue // 跳过未找到的指标定义
		}

		// 获取平台相关的Filter
		platformFilter, filterErr := s.platformFilterFactory.GetFilter(ctx, loop_span.PlatformType(req.PlatformType))
		if filterErr != nil {
			return nil, filterErr
		}

		// 构建SpanEnv
		spanEnv := &span_filter.SpanEnv{
			WorkspaceID:           0, // TODO: 从req中获取WorkspaceID
			ThirdPartyWorkspaceID: req.WorkspaceID,
		}

		// 获取指标的筛选条件
		whereFilters, whereErr := definition.Where(ctx, platformFilter, spanEnv)
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

		// 3. 构造参数调用 TraceRepo 的 GetMetrics
		param := &repo.GetMetricsParam{
			Tenants: tenants,
			Aggregations: []*entity.Dimension{
				{
					Expression: definition.Expression(entity.MetricGranularity(req.Granularity)),
					Alias:      string(definition.Name()),
				},
			},
			Filters:     mergedFilters,
			StartAt:     req.StartTime,
			EndAt:       req.EndTime,
			Granularity: req.Granularity,
			GroupBys:    definition.GroupBy(),
		}

		// 统一使用 GetMetrics 方法调用 repo
		result, err := s.metricsRepo.GetMetrics(ctx, param)
		if err != nil {
			return nil, err
		}

		// 4. 结构转换，返回结果
		metric := &entity.Metric{}
		if result != nil && len(result.Data) > 0 {
			// 根据指标类型格式化数据
			switch definition.Type() {
			case entity.MetricTypeTimeSeries:
				// 处理时间序列数据
				metric.TimeSeries = s.formatTimeSeriesData(result.Data)
			case entity.MetricTypeSummary:
				// 处理汇总数据
				metric.Summary = s.formatSummaryData(result.Data)
			case entity.MetricTypePie:
				// 处理饼图数据
				metric.Pie = s.formatPieData(result.Data)
			}
		}

		resp.Metrics[metricName] = metric
	}

	return resp, nil
}

// formatTimeSeriesData 格式化时间序列数据
func (s *MetricsService) formatTimeSeriesData(data []map[string]any) map[string][]*entity.MetricPoint {
	timeSeries := make(map[string][]*entity.MetricPoint)
	
	for _, row := range data {
		// 这里需要根据实际的数据结构进行解析
		// 假设数据包含 timestamp, value 等字段
		if timestamp, ok := row["timestamp"]; ok {
			if value, ok := row["value"]; ok {
				key := "default" // 可以根据 GroupBy 字段确定 key
				if timeSeries[key] == nil {
					timeSeries[key] = make([]*entity.MetricPoint, 0)
				}
				
				point := &entity.MetricPoint{
					Timestamp: toString(timestamp),
					Value:     toString(value),
				}
				timeSeries[key] = append(timeSeries[key], point)
			}
		}
	}
	
	return timeSeries
}

// formatSummaryData 格式化汇总数据
func (s *MetricsService) formatSummaryData(data []map[string]any) string {
	if len(data) == 0 {
		return "0"
	}
	
	// 取第一行数据的 value 字段作为汇总值
	if value, ok := data[0]["value"]; ok {
		return toString(value)
	}
	
	return "0"
}

// formatPieData 格式化饼图数据
func (s *MetricsService) formatPieData(data []map[string]any) map[string]string {
	pie := make(map[string]string)
	
	for _, row := range data {
		// 根据 GroupBy 字段确定 key，value 字段确定值
		var key, value string
		
		// 这里需要根据实际的数据结构进行解析
		for k, v := range row {
			if k == "value" {
				value = toString(v)
			} else if k != "timestamp" { // 非时间戳字段作为分组键
				key = toString(v)
			}
		}
		
		if key != "" && value != "" {
			pie[key] = value
		}
	}
	
	return pie
}

// toString 将 any 类型转换为字符串
func toString(v any) string {
	if v == nil {
		return ""
	}
	
	switch val := v.(type) {
	case string:
		return val
	case int, int32, int64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%f", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}