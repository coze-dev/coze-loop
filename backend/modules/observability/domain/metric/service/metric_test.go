// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"reflect"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
)

func TestMetricsService_formatMetrics(t *testing.T) {
	tests := []struct {
		name     string
		data     []map[string]any
		mInfo    *metricInfo
		expected map[string]*entity.Metric
	}{
		// TimeSeries 类型测试用例
		{
			name: "TimeSeries - 单个指标，单个时间点",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - 单个指标，多个时间点",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,
				},
				{
					"time_bucket": "2023-01-01T01:00:00Z",
					"metric1":     200,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
							{Timestamp: "2023-01-01T01:00:00Z", Value: "200"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - 多个指标，多个时间点",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,
					"metric2":     50,
				},
				{
					"time_bucket": "2023-01-01T01:00:00Z",
					"metric1":     200,
					"metric2":     75,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
					{Expression: "sum(value)", Alias: "metric2"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
							{Timestamp: "2023-01-01T01:00:00Z", Value: "200"},
						},
					},
				},
				"metric2": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "50"},
							{Timestamp: "2023-01-01T01:00:00Z", Value: "75"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - 带 GroupBy 的时间序列",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,
					"region":      "us-east",
				},
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     150,
					"region":      "us-west",
				},
				{
					"time_bucket": "2023-01-01T01:00:00Z",
					"metric1":     200,
					"region":      "us-east",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"us-east": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
							{Timestamp: "2023-01-01T01:00:00Z", Value: "200"},
						},
						"us-west": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "150"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - 多个 GroupBy 字段",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,
					"region":      "us-east",
					"env":         "prod",
				},
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     150,
					"region":      "us-west",
					"env":         "dev",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"us-east-prod": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
						},
						"us-west-dev": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "150"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - 空数据",
			data: []map[string]any{},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{},
		},
		{
			name: "TimeSeries - 缺少 time_bucket",
			data: []map[string]any{
				{
					"metric1": 100,
					"region":  "us-east",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"us-east": {
							{Timestamp: "", Value: "100"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - GroupBy 值为 nil",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,
					"region":      nil,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
						},
					},
				},
			},
		},
		{
			name: "TimeSeries - 指标值为各种类型",
			data: []map[string]any{
				{
					"time_bucket": "2023-01-01T00:00:00Z",
					"metric1":     100,      // int
					"metric2":     100.5,    // float
					"metric3":     "string", // string
					"metric4":     nil,      // nil
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeTimeSeries,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
					{Expression: "sum(value)", Alias: "metric2"},
					{Expression: "count(distinct id)", Alias: "metric3"},
					{Expression: "avg(value)", Alias: "metric4"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100"},
						},
					},
				},
				"metric2": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "100.5"},
						},
					},
				},
				"metric3": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: "string"},
						},
					},
				},
				"metric4": {
					TimeSeries: map[string][]*entity.MetricPoint{
						"": {
							{Timestamp: "2023-01-01T00:00:00Z", Value: ""},
						},
					},
				},
			},
		},

		// Summary 类型测试用例
		{
			name: "Summary - 单个指标",
			data: []map[string]any{
				{
					"metric1": 100,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeSummary,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Summary: "100",
				},
			},
		},
		{
			name: "Summary - 多个指标",
			data: []map[string]any{
				{
					"metric1": 100,
					"metric2": 200.5,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeSummary,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
					{Expression: "sum(value)", Alias: "metric2"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Summary: "100",
				},
				"metric2": {
					Summary: "200.5",
				},
			},
		},
		{
			name: "Summary - 多条数据（后面的会覆盖前面的）",
			data: []map[string]any{
				{
					"metric1": 100,
				},
				{
					"metric1": 200,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeSummary,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Summary: "200",
				},
			},
		},
		{
			name: "Summary - 空数据",
			data: []map[string]any{},
			mInfo: &metricInfo{
				mType: entity.MetricTypeSummary,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{},
		},
		{
			name: "Summary - 指标值为各种类型",
			data: []map[string]any{
				{
					"metric1": 100,      // int
					"metric2": 100.5,    // float
					"metric3": "string", // string
					"metric4": nil,      // nil
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypeSummary,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
					{Expression: "sum(value)", Alias: "metric2"},
					{Expression: "count(distinct id)", Alias: "metric3"},
					{Expression: "avg(value)", Alias: "metric4"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Summary: "100",
				},
				"metric2": {
					Summary: "100.5",
				},
				"metric3": {
					Summary: "string",
				},
				"metric4": {
					Summary: "",
				},
			},
		},

		// Pie 类型测试用例
		{
			name: "Pie - 单个指标，单个分组",
			data: []map[string]any{
				{
					"metric1": 100,
					"region":  "us-east",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Pie: map[string]string{
						"us-east": "100",
					},
				},
			},
		},
		{
			name: "Pie - 单个指标，多个分组",
			data: []map[string]any{
				{
					"metric1": 100,
					"region":  "us-east",
				},
				{
					"metric1": 150,
					"region":  "us-west",
				},
				{
					"metric1": 200,
					"region":  "eu-central",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Pie: map[string]string{
						"us-east":    "100",
						"us-west":    "150",
						"eu-central": "200",
					},
				},
			},
		},
		{
			name: "Pie - 多个指标",
			data: []map[string]any{
				{
					"metric1": 100,
					"metric2": 50,
					"region":  "us-east",
				},
				{
					"metric1": 150,
					"metric2": 75,
					"region":  "us-west",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
					{Expression: "sum(value)", Alias: "metric2"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Pie: map[string]string{
						"us-east": "100",
						"us-west": "150",
					},
				},
				"metric2": {
					Pie: map[string]string{
						"us-east": "50",
						"us-west": "75",
					},
				},
			},
		},
		{
			name: "Pie - 多个 GroupBy 字段",
			data: []map[string]any{
				{
					"metric1": 100,
					"region":  "us-east",
					"env":     "prod",
				},
				{
					"metric1": 150,
					"region":  "us-west",
					"env":     "dev",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Pie: map[string]string{
						"us-east-prod": "100",
						"us-west-dev":  "150",
					},
				},
			},
		},
		{
			name: "Pie - 空数据",
			data: []map[string]any{},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{},
		},
		{
			name: "Pie - GroupBy 值为 nil",
			data: []map[string]any{
				{
					"metric1": 100,
					"region":  nil,
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Pie: map[string]string{
						"": "100",
					},
				},
			},
		},
		{
			name: "Pie - 指标值为各种类型",
			data: []map[string]any{
				{
					"metric1": 100,      // int
					"metric2": 100.5,    // float
					"metric3": "string", // string
					"metric4": nil,      // nil
					"region":  "us-east",
				},
			},
			mInfo: &metricInfo{
				mType: entity.MetricTypePie,
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
					{Expression: "sum(value)", Alias: "metric2"},
					{Expression: "count(distinct id)", Alias: "metric3"},
					{Expression: "avg(value)", Alias: "metric4"},
				},
			},
			expected: map[string]*entity.Metric{
				"metric1": {
					Pie: map[string]string{
						"us-east": "100",
					},
				},
				"metric2": {
					Pie: map[string]string{
						"us-east": "100.5",
					},
				},
				"metric3": {
					Pie: map[string]string{
						"us-east": "string",
					},
				},
				"metric4": {
					Pie: map[string]string{
						"us-east": "",
					},
				},
			},
		},

		// 异常情况测试
		{
			name: "未知指标类型 - 应该返回空结果",
			data: []map[string]any{
				{
					"metric1": 100,
				},
			},
			mInfo: &metricInfo{
				mType: "unknown_type",
				mAggregation: []*entity.Dimension{
					{Expression: "count(*)", Alias: "metric1"},
				},
			},
			expected: map[string]*entity.Metric{},
		},
		{
			name: "mAggregation 为空",
			data: []map[string]any{
				{
					"metric1": 100,
				},
			},
			mInfo: &metricInfo{
				mType:        entity.MetricTypeSummary,
				mAggregation: []*entity.Dimension{},
			},
			expected: map[string]*entity.Metric{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &MetricsService{}
			result := s.formatMetrics(tt.data, tt.mInfo)

			if !compareMetrics(result, tt.expected) {
				t.Errorf("formatMetrics() = %+v, expected %+v", result, tt.expected)
			}
		})
	}
}

// compareMetrics 比较两个 Metric map 是否相等
func compareMetrics(a, b map[string]*entity.Metric) bool {
	if len(a) != len(b) {
		return false
	}

	for key, metricA := range a {
		metricB, exists := b[key]
		if !exists {
			return false
		}

		// 比较 Summary
		if metricA.Summary != metricB.Summary {
			return false
		}

		// 比较 Pie - 需要处理 nil 情况
		if metricA.Pie == nil && metricB.Pie == nil {
			// 都为 nil，继续
		} else if metricA.Pie == nil || metricB.Pie == nil {
			return false
		} else if !reflect.DeepEqual(metricA.Pie, metricB.Pie) {
			return false
		}

		// 比较 TimeSeries - 需要处理 nil 情况
		if metricA.TimeSeries == nil && metricB.TimeSeries == nil {
			// 都为 nil，继续
		} else if metricA.TimeSeries == nil || metricB.TimeSeries == nil {
			return false
		} else {
			if len(metricA.TimeSeries) != len(metricB.TimeSeries) {
				return false
			}
			for tsKey, pointsA := range metricA.TimeSeries {
				pointsB, exists := metricB.TimeSeries[tsKey]
				if !exists {
					return false
				}
				if !reflect.DeepEqual(pointsA, pointsB) {
					return false
				}
			}
		}
	}

	return true
}

// TestMetricsService_formatMetrics_NilMInfo 测试 mInfo 为 nil 的情况
func TestMetricsService_formatMetrics_NilMInfo(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic when mInfo is nil, but got none")
		}
	}()

	s := &MetricsService{}
	data := []map[string]any{
		{
			"metric1": 100,
		},
	}
	s.formatMetrics(data, nil)
}