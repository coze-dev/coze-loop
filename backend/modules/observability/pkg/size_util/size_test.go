// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package size_util

import (
	"testing"
	"unsafe"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
)

func TestSizeOfString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{name: "empty string", input: "", expected: 0},
		{name: "ascii string", input: "hello", expected: 5},
		{name: "unicode string", input: "你好", expected: 6},
		{name: "mixed string", input: "hi你好", expected: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, SizeOfString(tt.input))
		})
	}
}

func TestSizeOfSpanNew_NilMaps(t *testing.T) {
	span := &loop_span.Span{
		StartTime:      1000,
		SpanID:         "span-123",
		ParentID:       "parent-456",
		LogID:          "log-789",
		TraceID:        "trace-abc",
		DurationMicros: 5000,
		PSM:            "test.psm",
		CallType:       "rpc",
		WorkspaceID:    "ws-001",
		SpanName:       "test_span",
		SpanType:       "model",
		Method:         "POST",
		StatusCode:     200,
		Input:          "input_data",
		Output:         "output_data",
		ObjectStorage:  "{}",
	}

	expected := int(unsafe.Sizeof(span.StartTime)) +
		len("span-123") +
		len("parent-456") +
		len("log-789") +
		len("trace-abc") +
		int(unsafe.Sizeof(span.DurationMicros)) +
		len("test.psm") +
		len("rpc") +
		len("ws-001") +
		len("test_span") +
		len("model") +
		len("POST") +
		int(unsafe.Sizeof(span.StatusCode)) +
		len("input_data") +
		len("output_data") +
		len("{}")

	result := SizeOfSpanNew(span)
	assert.Equal(t, expected, result)
}

func TestSizeOfSpanNew_WithMaps(t *testing.T) {
	span := &loop_span.Span{
		SystemTagsString: map[string]string{"key1": "value1", "key2": "value2"},
		SystemTagsLong:   map[string]int64{"long_key": 100},
		SystemTagsDouble: map[string]float64{"double_key": 1.5},
		TagsString:       map[string]string{"tag_str": "val"},
		TagsLong:         map[string]int64{"tag_long": 200},
		TagsDouble:       map[string]float64{"tag_double": 2.5},
		TagsByte:         map[string]string{"tag_byte": "byte_val"},
		TagsBool:         map[string]bool{"tag_bool": true},
	}

	result := SizeOfSpanNew(span)

	// 计算基础字段（全为零值）
	expectedBase := int(unsafe.Sizeof(span.StartTime)) +
		int(unsafe.Sizeof(span.DurationMicros)) +
		int(unsafe.Sizeof(span.StatusCode))

	// 计算 map 部分
	expectedMaps := 0
	// SystemTagsString
	expectedMaps += len("key1") + len("value1") + len("key2") + len("value2")
	// SystemTagsLong
	expectedMaps += len("long_key") + int(unsafe.Sizeof(int64(0)))
	// SystemTagsDouble
	expectedMaps += len("double_key") + int(unsafe.Sizeof(float64(0)))
	// TagsString
	expectedMaps += len("tag_str") + len("val")
	// TagsLong
	expectedMaps += len("tag_long") + int(unsafe.Sizeof(int64(0)))
	// TagsDouble
	expectedMaps += len("tag_double") + int(unsafe.Sizeof(float64(0)))
	// TagsByte
	expectedMaps += len("tag_byte") + len("byte_val")
	// TagsBool
	expectedMaps += len("tag_bool") + int(unsafe.Sizeof(bool(false)))

	assert.Equal(t, expectedBase+expectedMaps, result)
}

func TestSizeOfSpanNew_EmptySpan(t *testing.T) {
	span := &loop_span.Span{}

	expected := int(unsafe.Sizeof(span.StartTime)) +
		int(unsafe.Sizeof(span.DurationMicros)) +
		int(unsafe.Sizeof(span.StatusCode))

	result := SizeOfSpanNew(span)
	assert.Equal(t, expected, result)
}

func TestSizeOfSpanNew_EmptyMaps(t *testing.T) {
	span := &loop_span.Span{
		SystemTagsString: map[string]string{},
		SystemTagsLong:   map[string]int64{},
		SystemTagsDouble: map[string]float64{},
		TagsString:       map[string]string{},
		TagsLong:         map[string]int64{},
		TagsDouble:       map[string]float64{},
		TagsByte:         map[string]string{},
		TagsBool:         map[string]bool{},
	}

	expected := int(unsafe.Sizeof(span.StartTime)) +
		int(unsafe.Sizeof(span.DurationMicros)) +
		int(unsafe.Sizeof(span.StatusCode))

	result := SizeOfSpanNew(span)
	assert.Equal(t, expected, result)
}
