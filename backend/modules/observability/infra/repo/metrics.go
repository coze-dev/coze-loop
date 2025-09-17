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
		Tables:       []string{"spans"}, // 默认查询spans表
		Aggregations: convertDimensions(param.Aggregations),
		GroupBys:     convertDimensions(param.GroupBys),
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
	// 调用扩展后的traceRepo.GetMetrics方法
	data, err := m.traceRepo.GetMetrics(ctx, &traceRepo.GetMetricsParam{
		Tables:       []string{"spans"}, // 默认查询spans表
		Aggregations: convertDimensions(param.Aggregations),
		GroupBys:     convertDimensions(param.GroupBys),
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

// GetPie 获取饼图指标数据
func (m *MetricsCkRepoImpl) GetPie(ctx context.Context, param *entity.GetMetricsParam) (*entity.GetMetricsResult, error) {
	// 调用扩展后的traceRepo.GetMetrics方法
	data, err := m.traceRepo.GetMetrics(ctx, &traceRepo.GetMetricsParam{
		Tables:       []string{"spans"}, // 默认查询spans表
		Aggregations: convertDimensions(param.Aggregations),
		GroupBys:     convertDimensions(param.GroupBys),
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

// convertDimensions 转换维度类型
func convertDimensions(dimensions []*entity.Dimension) []*traceRepo.Dimension {
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