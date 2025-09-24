// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type MetricType string
type MetricSource string
type MetricGranularity string

const (
	MetricTypeTimeSeries MetricType = "time_series" // 时间序列
	MetricTypeSummary    MetricType = "summary"     // 汇总
	MetricTypePie        MetricType = "pie"         // 饼图

	MetricSourceCK MetricSource = "ck"

	MetricsGranularity5Min  MetricGranularity = "5min"
	MetricsGranularity1Hour MetricGranularity = "1hour"
	MetricsGranularity1Day  MetricGranularity = "1day"

	MetricNameModelFailRatio   = "model_fail_ratio"
	MetricNameToolFailRatio    = "tool_fail_ratio"
	MetricNameModelLatencyAvg  = "model_latency_avg"
	MetricNameModelTotalTokens = "model_total_tokens"
	MetricNameToolLatencyAvg   = "tool_latency_avg"
	MetricNameToolTotalCount   = "tool_total_count"
	MetricNameTotalCount       = "total_count"
	MetricNameFailRatio        = "fail_ratio"
)

type Dimension struct {
	Expression string                 // 表达式
	Field      *loop_span.FilterField // 字段名, 设计上用于聚合
	Alias      string                 // 别名
}

type IMetricDefinition interface {
	Name() string                                                                                      // 指标名，全局唯一
	Type() MetricType                                                                                  // 指标类型
	Source() MetricSource                                                                              // 数据来源
	Expression(MetricGranularity) string                                                               // 计算表达式
	Where(context.Context, span_filter.Filter, *span_filter.SpanEnv) ([]*loop_span.FilterField, error) // 筛选条件
	GroupBy() []*Dimension                                                                             // 聚合维度
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
