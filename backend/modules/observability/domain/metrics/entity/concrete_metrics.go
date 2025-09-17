// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/pkg/filter"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// TotalCountMetric 使用次数指标
type TotalCountMetric struct{}

func (m *TotalCountMetric) Name() string {
	return "total_count"
}

func (m *TotalCountMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *TotalCountMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *TotalCountMetric) Expression() string {
	return "count()"
}

func (m *TotalCountMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	// 需要RootSpan筛选
	return filter.BuildRootSpanFilter()
}

func (m *TotalCountMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// FailRatioMetric Span错误率指标
type FailRatioMetric struct{}

func (m *FailRatioMetric) Name() string {
	return "fail_ratio"
}

func (m *FailRatioMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *FailRatioMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *FailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *FailRatioMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return nil, nil
}

func (m *FailRatioMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// ModelFailRatioMetric 模型调用错误率指标
type ModelFailRatioMetric struct{}

func (m *ModelFailRatioMetric) Name() string {
	return "model_fail_ratio"
}

func (m *ModelFailRatioMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *ModelFailRatioMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *ModelFailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ModelFailRatioMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter()
}

func (m *ModelFailRatioMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// ModelLatencyAvgMetric 模型调用平均耗时指标
type ModelLatencyAvgMetric struct{}

func (m *ModelLatencyAvgMetric) Name() string {
	return "model_latency_avg"
}

func (m *ModelLatencyAvgMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *ModelLatencyAvgMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *ModelLatencyAvgMetric) Expression() string {
	return "sum(duration / 1000) / count()"
}

func (m *ModelLatencyAvgMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter()
}

func (m *ModelLatencyAvgMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// ModelTotalTokensMetric 模型Tokens消耗指标
type ModelTotalTokensMetric struct{}

func (m *ModelTotalTokensMetric) Name() string {
	return "model_total_tokens"
}

func (m *ModelTotalTokensMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *ModelTotalTokensMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *ModelTotalTokensMetric) Expression() string {
	return "sum(tags_long['input_tokens'] + tags_long['output_tokens'])"
}

func (m *ModelTotalTokensMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return filter.BuildLLMSpanFilter()
}

func (m *ModelTotalTokensMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// ToolTotalCountMetric 工具调用次数指标
type ToolTotalCountMetric struct{}

func (m *ToolTotalCountMetric) Name() string {
	return "tool_total_count"
}

func (m *ToolTotalCountMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *ToolTotalCountMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *ToolTotalCountMetric) Expression() string {
	return "count()"
}

func (m *ToolTotalCountMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolTotalCountMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// ToolFailRatioMetric 工具调用错误率指标
type ToolFailRatioMetric struct{}

func (m *ToolFailRatioMetric) Name() string {
	return "tool_fail_ratio"
}

func (m *ToolFailRatioMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *ToolFailRatioMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *ToolFailRatioMetric) Expression() string {
	return "countIf(1, status_code != 0) / count()"
}

func (m *ToolFailRatioMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolFailRatioMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}

// ToolLatencyAvgMetric 工具调用平均耗时指标
type ToolLatencyAvgMetric struct{}

func (m *ToolLatencyAvgMetric) Name() string {
	return "tool_latency_avg"
}

func (m *ToolLatencyAvgMetric) Type() MetricType {
	return MetricTypeSummary
}

func (m *ToolLatencyAvgMetric) Source() MetricSource {
	return MetricSourceCK
}

func (m *ToolLatencyAvgMetric) Expression() string {
	return "sum(duration / 1000) / count()"
}

func (m *ToolLatencyAvgMetric) Where(filterFields *loop_span.FilterFields) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (m *ToolLatencyAvgMetric) GroupBy() []*Dimension {
	return []*Dimension{}
}



// GetAllMetricDefinitions 获取所有指标定义
func GetAllMetricDefinitions() []IMetricDefinition {
	return []IMetricDefinition{
		&TotalCountMetric{},
		&FailRatioMetric{},
		&ModelFailRatioMetric{},
		&ModelLatencyAvgMetric{},
		&ModelTotalTokensMetric{},
		&ToolTotalCountMetric{},
		&ToolFailRatioMetric{},
		&ToolLatencyAvgMetric{},
	}
}