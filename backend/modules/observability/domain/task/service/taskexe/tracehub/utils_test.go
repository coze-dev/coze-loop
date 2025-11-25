// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestProcessSpecificFilter_StatusFilter(t *testing.T) {
	tests := []struct {
		name        string
		filter      *loop_span.FilterField
		expectError bool
		validate    func(t *testing.T, result *loop_span.FilterField)
	}{
		{
			name: "status filter with success and error - should convert to always true",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatus,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				Values:    []string{loop_span.SpanStatusSuccess, loop_span.SpanStatusError},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, loop_span.SpanFieldStatusCode, result.FieldName)
				assert.Equal(t, loop_span.FieldTypeLong, result.FieldType)
				assert.Equal(t, loop_span.QueryTypeEnumAlwaysTrue, *result.QueryType)
				assert.Nil(t, result.Values)
			},
		},
		{
			name: "status filter with only success - should convert to value 0",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatus,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				Values:    []string{loop_span.SpanStatusSuccess},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, loop_span.SpanFieldStatusCode, result.FieldName)
				assert.Equal(t, loop_span.FieldTypeLong, result.FieldType)
				assert.Equal(t, loop_span.QueryTypeEnumIn, *result.QueryType)
				assert.Equal(t, []string{"0"}, result.Values)
			},
		},
		{
			name: "status filter with only error - should convert to not in value 0",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatus,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				Values:    []string{loop_span.SpanStatusError},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, loop_span.SpanFieldStatusCode, result.FieldName)
				assert.Equal(t, loop_span.FieldTypeLong, result.FieldType)
				assert.Equal(t, loop_span.QueryTypeEnumNotIn, *result.QueryType)
				assert.Equal(t, []string{"0"}, result.Values)
			},
		},
		{
			name: "status filter without in operator - should return error",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatus,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				Values:    []string{loop_span.SpanStatusSuccess},
			},
			expectError: true,
		},
		{
			name: "status filter with invalid status value - should return error",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatus,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				Values:    []string{"invalid_status"},
			},
			expectError: true,
		},
		{
			name: "status filter with empty values - should return error",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStatus,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
				Values:    []string{},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			filterCopy := *tt.filter
			err := processSpecificFilter(&filterCopy)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, &filterCopy)
				}
			}
		})
	}
}

func TestProcessSpecificFilter_LatencyFilter(t *testing.T) {
	tests := []struct {
		name        string
		filter      *loop_span.FilterField
		expectError bool
		validate    func(t *testing.T, result *loop_span.FilterField)
	}{
		{
			name: "duration filter - should convert ms to us",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldDuration,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
				Values:    []string{"100", "200"},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, loop_span.SpanFieldDuration, result.FieldName)
				assert.Equal(t, loop_span.FieldTypeLong, result.FieldType)
				assert.Equal(t, loop_span.QueryTypeEnumGte, *result.QueryType)
				assert.Equal(t, []string{"100000", "200000"}, result.Values) // 100ms -> 100000us
			},
		},
		{
			name: "latency_first_resp filter - should convert ms to us",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldLatencyFirstResp,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumLte),
				Values:    []string{"50"},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, loop_span.SpanFieldLatencyFirstResp, result.FieldName)
				assert.Equal(t, loop_span.FieldTypeLong, result.FieldType)
				assert.Equal(t, loop_span.QueryTypeEnumLte, *result.QueryType)
				assert.Equal(t, []string{"50000"}, result.Values) // 50ms -> 50000us
			},
		},
		{
			name: "start_time_first_resp filter - should convert ms to us",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStartTimeFirstResp,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
				Values:    []string{"1000"},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, loop_span.SpanFieldStartTimeFirstResp, result.FieldName)
				assert.Equal(t, []string{"1000000"}, result.Values) // 1000ms -> 1000000us
			},
		},
		{
			name: "start_time_first_token_resp filter - should convert ms to us",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldStartTimeFirstTokenResp,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumGt),
				Values:    []string{"10"},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, []string{"10000"}, result.Values) // 10ms -> 10000us
			},
		},
		{
			name: "latency_first_token_resp filter - should convert ms to us",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldLatencyFirstTokenResp,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumLt),
				Values:    []string{"5"},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, []string{"5000"}, result.Values) // 5ms -> 5000us
			},
		},
		{
			name: "reasoning_duration filter - should convert ms to us",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldReasoningDuration,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
				Values:    []string{"30"},
			},
			expectError: false,
			validate: func(t *testing.T, result *loop_span.FilterField) {
				assert.Equal(t, []string{"30000"}, result.Values) // 30ms -> 30000us
			},
		},
		{
			name: "latency filter with wrong field type - should return error",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldDuration,
				FieldType: loop_span.FieldTypeString,
				QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
				Values:    []string{"100"},
			},
			expectError: true,
		},
		{
			name: "latency filter with invalid value - should return error",
			filter: &loop_span.FilterField{
				FieldName: loop_span.SpanFieldDuration,
				FieldType: loop_span.FieldTypeLong,
				QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
				Values:    []string{"invalid"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying the original
			filterCopy := *tt.filter
			err := processSpecificFilter(&filterCopy)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, &filterCopy)
				}
			}
		})
	}
}

func TestProcessSpecificFilter_UnknownField(t *testing.T) {
	// Test with unknown field name - should not modify the filter
	filter := &loop_span.FilterField{
		FieldName: "unknown_field",
		FieldType: loop_span.FieldTypeString,
		QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		Values:    []string{"test"},
	}

	original := *filter
	err := processSpecificFilter(filter)

	assert.NoError(t, err)
	assert.Equal(t, original.FieldName, filter.FieldName)
	assert.Equal(t, original.FieldType, filter.FieldType)
	assert.Equal(t, original.QueryType, filter.QueryType)
	assert.Equal(t, original.Values, filter.Values)
}

func TestProcessSpecificFilter_NilFilter(t *testing.T) {
	// Test with nil filter - should not panic
	err := processSpecificFilter(nil)
	assert.NoError(t, err)
}
