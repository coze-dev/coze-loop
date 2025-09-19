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

const (
	MetricTypeTimeSeries MetricType = "time_series" // 时间序列
	MetricTypeSummary    MetricType = "summary"     // 汇总
	MetricTypePie        MetricType = "pie"         // 饼图

	MetricSourceCK   MetricSource = "ck"
	MetricSourceCoze MetricSource = "coze"

	MetricsGranularity5Min  = "5min"
	MetricsGranularity1Hour = "1hour"
	MetricsGranularity1Day  = "1day"
)

const (
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
	Expression string // 字段名或表达式
	Alias      string // 别名
}

type IMetricDefinition interface {
	Name() string                                                                                      // 指标名，全局唯一
	Type() string                                                                                      // 指标类型：time_series/summary/pie
	Source() string                                                                                    // 数据来源：span/coze
	Expression() string                                                                                // 计算表达式
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
