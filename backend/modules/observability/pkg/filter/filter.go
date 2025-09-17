// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package filter 提供span筛选条件构建工具
package filter

import (
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// BuildRootSpanFilter 构建RootSpan筛选条件
func BuildRootSpanFilter() ([]*loop_span.FilterField, error) {
	// RootSpan通常是没有父级SpanID的span
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldParentID,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{""},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		},
	}, nil
}

// BuildLLMSpanFilter 构建LLM Span筛选条件
func BuildLLMSpanFilter() ([]*loop_span.FilterField, error) {
	// LLM Span通过span_type字段进行筛选
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"llm"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

// BuildToolSpanFilter 构建工具Span筛选条件
func BuildToolSpanFilter() ([]*loop_span.FilterField, error) {
	// 工具Span通过span_type字段进行筛选
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"tool"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

// BuildSpaceIDFilter 构建工作空间ID筛选条件
func BuildSpaceIDFilter(spaceID string) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpaceId,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{spaceID},
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		},
	}, nil
}

// BuildTimeRangeFilter 构建时间范围筛选条件
func BuildTimeRangeFilter(startTime, endTime int64) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldStartTime,
			FieldType: loop_span.FieldTypeLong,
			Values:    []string{fmt.Sprintf("%d", startTime)},
			QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
		},
		{
			FieldName: loop_span.SpanFieldStartTime,
			FieldType: loop_span.FieldTypeLong,
			Values:    []string{fmt.Sprintf("%d", endTime)},
			QueryType: ptr.Of(loop_span.QueryTypeEnumLte),
		},
	}, nil
}

// MergeFilters 合并多个筛选条件
func MergeFilters(filters ...[]*loop_span.FilterField) []*loop_span.FilterField {
	var result []*loop_span.FilterField
	for _, filterGroup := range filters {
		if filterGroup != nil {
			result = append(result, filterGroup...)
		}
	}
	return result
}