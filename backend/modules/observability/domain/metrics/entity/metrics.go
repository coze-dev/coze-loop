// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type MetricType string

const (
	MetricTypeTimeSeries MetricType = "time_series" // 时间序列
	MetricTypeSummary    MetricType = "summary"     // 汇总
	MetricTypePie        MetricType = "pie"         // 饼图
)

type MetricSource string

const (
	MetricSourceCK   MetricSource = "ck"   // ClickHouse
	MetricSourceCoze MetricSource = "coze" // Coze系统
)

type Dimension struct {
	Expression string // 字段名或表达式
	Alias      string // 别名
}

type IMetricDefinition interface {
	Name() string                                                    // 指标名，全局唯一
	Type() MetricType                                                // 指标类型
	Source() MetricSource                                            // 数据来源
	Expression() string                                              // 计算表达式
	Where(*loop_span.FilterFields) ([]*loop_span.FilterField, error) // 筛选条件
	GroupBy() []*Dimension                                           // 聚合维度
}

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
	Metrics map[string]*Metric
}

type Metric struct {
	Summary    string
	Pie        map[string]string
	TimeSeries map[string][]*MetricPoint
}

type MetricPoint struct {
	Timestamp string
	Value     string
}

type GetMetricsParam struct {
	Tenants      []string
	Aggregations []*Dimension
	GroupBys     []*Dimension
	Filters      *loop_span.FilterFields
	StartAt      int64
	EndAt        int64
	Granularity  string
}

type GetMetricsResult struct {
	Data []map[string]any
}