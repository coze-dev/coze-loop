// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/entity"
	metricsRepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metrics/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck"
)

type MetricsCkRepoImpl struct {
	spansDao ck.ISpansDao
}

func NewMetricsCkRepoImpl(spansDao ck.ISpansDao) metricsRepo.IMetricsRepo {
	return &MetricsCkRepoImpl{
		spansDao: spansDao,
	}
}

// convertDimensions 将 entity.Dimension 转换为 ck.Dimension
func convertDimensions(dimensions []*entity.Dimension) []*ck.Dimension {
	if dimensions == nil {
		return nil
	}
	
	result := make([]*ck.Dimension, len(dimensions))
	for i, dim := range dimensions {
		result[i] = &ck.Dimension{
			Expression: dim.Expression,
			Alias:      dim.Alias,
		}
	}
	return result
}

func (m *MetricsCkRepoImpl) GetTimeSeries(ctx context.Context, param *metricsRepo.GetMetricsParam) (*metricsRepo.GetMetricsResult, error) {
	// 转换参数并调用 spansDao.GetMetrics 方法
	data, err := m.spansDao.GetMetrics(ctx, &ck.GetMetricsParam{
		Tables:       []string{"trace_1.fornax_90d"}, // 使用文档中的表名
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

	return &metricsRepo.GetMetricsResult{Data: data}, nil
}

func (m *MetricsCkRepoImpl) GetSummary(ctx context.Context, param *metricsRepo.GetMetricsParam) (*metricsRepo.GetMetricsResult, error) {
	// Summary类型不需要时间分组
	summaryParam := *param
	summaryParam.Granularity = ""

	// 转换参数并调用 spansDao.GetMetrics 方法
	data, err := m.spansDao.GetMetrics(ctx, &ck.GetMetricsParam{
		Tables:       []string{"trace_1.fornax_90d"}, // 使用文档中的表名
		Aggregations: convertDimensions(summaryParam.Aggregations),
		GroupBys:     convertDimensions(summaryParam.GroupBys),
		Filters:      summaryParam.Filters,
		StartAt:      summaryParam.StartAt,
		EndAt:        summaryParam.EndAt,
		Granularity:  summaryParam.Granularity,
	})
	if err != nil {
		return nil, err
	}
	return &metricsRepo.GetMetricsResult{Data: data}, nil
}

func (m *MetricsCkRepoImpl) GetPie(ctx context.Context, param *metricsRepo.GetMetricsParam) (*metricsRepo.GetMetricsResult, error) {
	// 饼图类型不需要时间分组
	pieParam := *param
	pieParam.Granularity = ""

	// 转换参数并调用 spansDao.GetMetrics 方法
	data, err := m.spansDao.GetMetrics(ctx, &ck.GetMetricsParam{
		Tables:       []string{"trace_1.fornax_90d"}, // 使用文档中的表名
		Aggregations: convertDimensions(pieParam.Aggregations),
		GroupBys:     convertDimensions(pieParam.GroupBys),
		Filters:      pieParam.Filters,
		StartAt:      pieParam.StartAt,
		EndAt:        pieParam.EndAt,
		Granularity:  pieParam.Granularity,
	})
	if err != nil {
		return nil, err
	}

	return &metricsRepo.GetMetricsResult{Data: data}, nil
}