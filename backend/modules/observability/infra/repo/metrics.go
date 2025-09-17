// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	metricsRepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/repo"
	traceRepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
)

// MetricsCkRepoImpl ClickHouse指标仓储实现
type MetricsCkRepoImpl struct {
	traceRepo traceRepo.ITraceRepo
}

// NewMetricsCkRepoImpl 创建ClickHouse指标仓储实现
func NewMetricsCkRepoImpl(traceRepo traceRepo.ITraceRepo) metricsRepo.IMetricsRepo {
	return &MetricsCkRepoImpl{
		traceRepo: traceRepo,
	}
}

// GetTimeSeries 获取时间序列指标数据
func (m *MetricsCkRepoImpl) GetTimeSeries(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error) {
	// 调用扩展后的traceRepo.GetMetrics方法
	data, err := m.traceRepo.GetMetrics(ctx, &traceRepo.GetMetricsParam{
		Tables:       []string{"trace_1.fornax_90d"}, // 使用文档中的表名
		Aggregations: convertMetricsDimensions(param.Aggregations),
		GroupBys:     convertMetricsDimensions(param.GroupBys),
		Filters:      param.Filters,
		StartAt:      param.StartAt,
		EndAt:        param.EndAt,
		Granularity:  param.Granularity,
	})
	if err != nil {
		return nil, err
	}

	return &entity.GetMetricsResult{Data: data}, nil
}

// GetSummary 获取汇总指标数据
func (m *MetricsCkRepoImpl) GetSummary(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error) {
	// Summary类型不需要时间分组
	summaryParam := *param
	summaryParam.Granularity = ""
	
	// 调用扩展后的traceRepo.GetMetrics方法
	data, err := m.traceRepo.GetMetrics(ctx, &traceRepo.GetMetricsParam{
		Tables:       []string{"trace_1.fornax_90d"}, // 使用文档中的表名
		Aggregations: convertMetricsDimensions(summaryParam.Aggregations),
		GroupBys:     convertMetricsDimensions(summaryParam.GroupBys),
		Filters:      summaryParam.Filters,
		StartAt:      summaryParam.StartAt,
		EndAt:        summaryParam.EndAt,
		Granularity:  summaryParam.Granularity,
	})
	if err != nil {
		return nil, err
	}

	return &entity.GetMetricsResult{Data: data}, nil
}
// GetPie 获取饼图指标数据
func (m *MetricsCkRepoImpl) GetPie(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error) {
	// 饼图类型不需要时间分组
	pieParam := *param
	pieParam.Granularity = ""
	
	// 调用扩展后的traceRepo.GetMetrics方法
	data, err := m.traceRepo.GetMetrics(ctx, &traceRepo.GetMetricsParam{
		Tables:       []string{"trace_1.fornax_90d"}, // 使用文档中的表名
		Aggregations: convertMetricsDimensions(pieParam.Aggregations),
		GroupBys:     convertMetricsDimensions(pieParam.GroupBys),
		Filters:      pieParam.Filters,
		StartAt:      pieParam.StartAt,
		EndAt:        pieParam.EndAt,
		Granularity:  pieParam.Granularity,
	})
	if err != nil {
		return nil, err
	}

	return &entity.GetMetricsResult{Data: data}, nil
}

// convertMetricsDimensions 转换指标维度类型
func convertMetricsDimensions(dimensions []*entity.Dimension) []*traceRepo.Dimension {
	if dimensions == nil {
		return nil
	}

	result := make([]*traceRepo.Dimension, len(dimensions))
	for i, dim := range dimensions {
		result[i] = &traceRepo.Dimension{
			Expression: dim.Expression,
			Alias:      dim.Alias,
		}
	}
	return result
}