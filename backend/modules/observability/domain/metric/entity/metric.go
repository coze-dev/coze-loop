// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
)

type MetricType string
type MetricSource string
type MetricGranularity string
type MetricCompareType string
type MetricOperator string

const (
	MetricTypeTimeSeries MetricType = "time_series" // 时间序列
	MetricTypeSummary    MetricType = "summary"     // 汇总
	MetricTypePie        MetricType = "pie"         // 饼图

	MetricSourceCK MetricSource = "ck"

	MetricGranularity1Min  MetricGranularity = "1min"
	MetricGranularity1Hour MetricGranularity = "1hour"
	MetricGranularity1Day  MetricGranularity = "1day"
	MetricGranularity1Week MetricGranularity = "1week"

	MetricCompareTypeYoY = "yoy" // 同比
	MetricCompareTypeMoM = "mom" // 环比

	// General 指标概览
	MetricNameGeneralTotalCount       = "general_total_count"
	MetricNameGeneralFailRatio        = "general_fail_ratio"
	MetricNameGeneralModelFailRatio   = "general_model_fail_ratio"
	MetricNameGeneralModelLatencyAvg  = "general_model_latency_avg"
	MetricNameGeneralModelTotalTokens = "general_model_total_tokens"
	MetricNameGeneralToolTotalCount   = "general_tool_total_count"
	MetricNameGeneralToolFailRatio    = "general_tool_fail_ratio"
	MetricNameGeneralToolLatencyAvg   = "general_tool_latency_avg"

	// Model 模型统计指标
	MetricNameModelTokenCount             = "model_token_count"
	MetricNameModelTokenCountByTime       = "model_token_count_by_time"
	MetricNameModelTokenCountPie          = "model_token_count_pie"
	MetricNameModelInputTokenCount        = "model_input_token_count"
	MetricNameModelInputTokenCountByTime  = "model_input_token_count_by_time"
	MetricNameModelOutputTokenCount       = "model_output_token_count"
	MetricNameModelOutputTokenCountByTime = "model_output_token_count_by_time"
	MetricNameModelSystemTokenCount       = "model_system_token_count"
	MetricNameModelToolChoiceTokenCount   = "model_tool_choice_token_count"
	MetricNameModelQPS                    = "model_qps"
	MetricNameModelQPSSuccess             = "model_qps_success"
	MetricNameModelQPSFail                = "model_qps_fail"
	MetricNameModelQPM                    = "model_qpm"
	MetricNameModelQPMSuccess             = "model_qpm_success"
	MetricNameModelQPMFail                = "model_qpm_fail"
	MetricNameModelSuccessRatio           = "model_success_ratio"
	MetricNameModelTPSAvg                 = "model_tps_avg"
	MetricNameModelTPSMin                 = "model_tps_min"
	MetricNameModelTPSMax                 = "model_tps_max"
	MetricNameModelTPSPct50               = "model_tps_pct50"
	MetricNameModelTPSPct90               = "model_tps_pct90"
	MetricNameModelTPSPct99               = "model_tps_pct99"
	MetricNameModelTPMAvg                 = "model_tpm_avg"
	MetricNameModelTPMMin                 = "model_tpm_min"
	MetricNameModelTPMMax                 = "model_tpm_max"
	MetricNameModelTPMPct50               = "model_tpm_pct50"
	MetricNameModelTPMPct90               = "model_tpm_pct90"
	MetricNameModelTPMPct99               = "model_tpm_pct99"
	MetricNameModelDurationAvg            = "model_duration_avg"
	MetricNameModelDurationMin            = "model_duration_min"
	MetricNameModelDurationMax            = "model_duration_max"
	MetricNameModelDurationPct50          = "model_duration_pct50"
	MetricNameModelDurationPct90          = "model_duration_pct90"
	MetricNameModelDurationPct99          = "model_duration_pct99"
	MetricNameModelTTFTAvg                = "model_ttft_avg"
	MetricNameModelTTFTMin                = "model_ttft_min"
	MetricNameModelTTFTMax                = "model_ttft_max"
	MetricNameModelTTFTPct50              = "model_ttft_pct50"
	MetricNameModelTTFTPct90              = "model_ttft_pct90"
	MetricNameModelTTFTPct99              = "model_ttft_pct99"
	MetricNameModelTPOTAvg                = "model_tpot_avg"
	MetricNameModelTPOTMin                = "model_tpot_min"
	MetricNameModelTPOTMax                = "model_tpot_max"
	MetricNameModelTPOTPct50              = "model_tpot_pct50"
	MetricNameModelTPOTPct90              = "model_tpot_pct90"
	MetricNameModelTPOTPct99              = "model_tpot_pct99"
	MetricNameModelNamePie                = "model_name_pie"

	// Tool 工具统计指标
	MetricNameToolTotalCount       = "tool_total_count"
	MetricNameToolTotalCountByTime = "tool_total_count_by_time"
	MetricNameToolDurationAvg      = "tool_duration_avg"
	MetricNameToolDurationMin      = "tool_duration_min"
	MetricNameToolDurationMax      = "tool_duration_max"
	MetricNameToolDurationPct50    = "tool_duration_pct50"
	MetricNameToolDurationPct90    = "tool_duration_pct90"
	MetricNameToolDurationPct99    = "tool_duration_pct99"
	MetricNameToolSuccessRatio     = "tool_success_ratio"
	MetricNameToolNamePie          = "tool_name_pie"

	// Service 服务调用指标
	MetricNameServiceTraceCountTotal    = "service_trace_count_total"
	MetricNameServiceTraceCount         = "service_trace_count"
	MetricNameServiceSpanCount          = "service_span_count"
	MetricNameServiceUserCount          = "service_user_count"
	MetricNameServiceMessageCount       = "service_message_count"
	MetricNameServiceQPSAll             = "service_qps_all"
	MetricNameServiceQPSSuccess         = "service_qps_success"
	MetricNameServiceQPSFail            = "service_qps_fail"
	MetricNameServiceQPMAll             = "service_qpm_all"
	MetricNameServiceQPMSuccess         = "service_qpm_success"
	MetricNameServiceQPMFail            = "service_qpm_fail"
	MetricNameServiceDurationAvg        = "service_duration_avg"
	MetricNameServiceDurationMin        = "service_duration_min"
	MetricNameServiceDurationMax        = "service_duration_max"
	MetricNameServiceDurationPct50      = "service_duration_pct50"
	MetricNameServiceDurationPct90      = "service_duration_pct90"
	MetricNameServiceDurationPct99      = "service_duration_pct99"
	MetricNameServiceSuccessRatio       = "service_success_ratio"
	MetricNameServiceExecutionStepCount = "service_execution_step_count"

	// 复合指标计算
	MetricOperatorDivide = "divide"
	MetricOperatorPie    = "pie"
)

type Compare struct {
	Type  MetricCompareType
	Shift int64 // shift seconds
}
type Dimension struct {
	Expression string                 // 表达式
	Field      *loop_span.FilterField // 字段名, 设计上用于聚合
	Alias      string                 // 别名
}

type IMetricFill interface {
	Interpolate() string
}

type IMetricCompound interface {
	GetMetrics() []IMetricDefinition
	Operator() MetricOperator
}

type MetricFillNull struct{}

func (f *MetricFillNull) Interpolate() string {
	return "null"
}

type IMetricDefinition interface {
	Name() string                                                                                      // 指标名，全局唯一
	Type() MetricType                                                                                  // 指标类型
	Source() MetricSource                                                                              // 数据来源
	Expression(MetricGranularity) string                                                               // 计算表达式
	Where(context.Context, span_filter.Filter, *span_filter.SpanEnv) ([]*loop_span.FilterField, error) // 筛选条件
	GroupBy() []*Dimension                                                                             // 聚合维度
}

type TimeSeries map[string][]*MetricPoint

type Metric struct {
	Summary    string
	Pie        map[string]string
	TimeSeries TimeSeries
}

type MetricPoint struct {
	Timestamp string
	Value     string
}

func GranularityToSecond(g MetricGranularity) int64 {
	switch g {
	case MetricGranularity1Min:
		return 60
	case MetricGranularity1Hour:
		return 3600
	case MetricGranularity1Day, MetricGranularity1Week:
		return 86400
	default:
		return 86400
	}
}

func NewTimeIntervals(startTime, endTime int64, granularity MetricGranularity) []string {
	var truncatedTime int64
	var intervalMills = GranularityToSecond(granularity) * 1000
	switch granularity {
	case MetricGranularity1Min:
		truncatedTime = startTime - (startTime % intervalMills)
	case MetricGranularity1Hour:
		truncatedTime = startTime - (startTime % intervalMills)
	case MetricGranularity1Day, MetricGranularity1Week:
		t := time.UnixMilli(startTime)
		truncatedTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).UnixMilli()
	default:
		t := time.UnixMilli(startTime)
		truncatedTime = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).UnixMilli()
	}
	ret := make([]string, 0)
	for truncatedTime <= endTime {
		tmp := strconv.FormatInt(truncatedTime, 10)
		ret = append(ret, tmp)
		truncatedTime += intervalMills
	}
	return ret
}
