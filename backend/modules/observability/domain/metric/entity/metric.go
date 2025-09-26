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
	MetricNameTotalCount       = "total_count"
	MetricNameFailRatio        = "fail_ratio"

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
	MetricNameModelTokenCount      = "model_token_count"
	MetricNameModelInputTokenCount = "model_input_token_count"
	MetricNameModelOutputTokenCount = "model_output_token_count"
	MetricNameModelQPS             = "model_qps"
	MetricNameModelQPM             = "model_qpm"
	MetricNameModelSuccessRatio    = "model_success_ratio"
	MetricNameModelTPSAvg          = "model_tps_avg"
	MetricNameModelTPSMin          = "model_tps_min"
	MetricNameModelTPSMax          = "model_tps_max"
	MetricNameModelTPSPct50        = "model_tps_pct50"
	MetricNameModelTPSPct90        = "model_tps_pct90"
	MetricNameModelTPSPct99        = "model_tps_pct99"
	MetricNameModelTPMAvg          = "model_tpm_avg"
	MetricNameModelTPMMin          = "model_tpm_min"
	MetricNameModelTPMMax          = "model_tpm_max"
	MetricNameModelTPMPct50        = "model_tpm_pct50"
	MetricNameModelTPMPct90        = "model_tpm_pct90"
	MetricNameModelTPMPct99        = "model_tpm_pct99"
	MetricNameModelDurationAvg     = "model_duration_avg"
	MetricNameModelDurationMin     = "model_duration_min"
	MetricNameModelDurationMax     = "model_duration_max"
	MetricNameModelDurationPct50   = "model_duration_pct50"
	MetricNameModelDurationPct90   = "model_duration_pct90"
	MetricNameModelDurationPct99   = "model_duration_pct99"
	MetricNameModelTTFTAvg         = "model_ttft_avg"
	MetricNameModelTTFTMin         = "model_ttft_min"
	MetricNameModelTTFTMax         = "model_ttft_max"
	MetricNameModelTTFTPct50       = "model_ttft_pct50"
	MetricNameModelTTFTPct90       = "model_ttft_pct90"
	MetricNameModelTTFTPct99       = "model_ttft_pct99"
	MetricNameModelTPOTAvg         = "model_tpot_avg"
	MetricNameModelTPOTMin         = "model_tpot_min"
	MetricNameModelTPOTMax         = "model_tpot_max"
	MetricNameModelTPOTPct50       = "model_tpot_pct50"
	MetricNameModelTPOTPct90       = "model_tpot_pct90"
	MetricNameModelTPOTPct99       = "model_tpot_pct99"

	// Tool 工具统计指标
	MetricNameToolTotalCount     = "tool_total_count"
	MetricNameToolDurationAvg    = "tool_duration_avg"
	MetricNameToolDurationMin    = "tool_duration_min"
	MetricNameToolDurationMax    = "tool_duration_max"
	MetricNameToolDurationPct50  = "tool_duration_pct50"
	MetricNameToolDurationPct90  = "tool_duration_pct90"
	MetricNameToolDurationPct99  = "tool_duration_pct99"
	MetricNameToolSuccessRatio   = "tool_success_ratio"

	// Service 服务调用指标
	MetricNameServiceTraceCountTotal = "service_trace_count_total"
	MetricNameServiceTraceCount      = "service_trace_count"
	MetricNameServiceSpanCount       = "service_span_count"
	MetricNameServiceUserCount       = "service_user_count"
	MetricNameServiceMessageCount    = "service_message_count"
	MetricNameServiceQPSAll          = "service_qps_all"
	MetricNameServiceQPSSuccess      = "service_qps_success"
	MetricNameServiceQPSFail         = "service_qps_fail"
	MetricNameServiceQPMAll          = "service_qpm_all"
	MetricNameServiceQPMSuccess      = "service_qpm_success"
	MetricNameServiceQPMFail         = "service_qpm_fail"
	MetricNameServiceDurationAvg     = "service_duration_avg"
	MetricNameServiceDurationMin     = "service_duration_min"
	MetricNameServiceDurationMax     = "service_duration_max"
	MetricNameServiceDurationPct50   = "service_duration_pct50"
	MetricNameServiceDurationPct90   = "service_duration_pct90"
	MetricNameServiceDurationPct99   = "service_duration_pct99"
	MetricNameServiceSuccessRatio    = "service_success_ratio"
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