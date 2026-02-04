// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/json"

	"github.com/stretchr/testify/assert"
)

func TestSpan(t *testing.T) {
	t.Parallel()
	span := &Span{
		StartTime:      1234,
		TraceID:        "123",
		ParentID:       "123456",
		SpanID:         "456",
		PSM:            "1",
		LogID:          "2",
		CallType:       "custom",
		WorkspaceID:    "987",
		SpanName:       "span_name",
		SpanType:       "span_type",
		DurationMicros: 123,
		Method:         "method",
		Input:          "input",
		Output:         "output",
		ObjectStorage:  "os",
		TagsString: map[string]string{
			"tag1": "1",
		},
		TagsLong: map[string]int64{
			"tag2":          2,
			"input_tokens":  10,
			"output_tokens": 20,
		},
		TagsDouble: map[string]float64{
			"tag3": 3.0,
		},
		TagsBool: map[string]bool{
			"tag4": true,
		},
		TagsByte: map[string]string{
			"tag5": "12",
		},
		SystemTagsDouble: map[string]float64{
			"stag1": 0.0,
		},
		SystemTagsString: map[string]string{
			"stag2": "1",
		},
		SystemTagsLong: map[string]int64{
			"stag3": 2,
		},
	}
	validSpan := &Span{
		StartTime:       time.Now().Add(-time.Hour * 12).UnixMicro(),
		SpanID:          "0000000000000001",
		TraceID:         "00000000000000000000000000000001",
		DurationMicros:  0,
		LogicDeleteTime: 0,
		TagsLong: map[string]int64{
			"a": 1,
		},
		SystemTagsLong: map[string]int64{},
		SystemTagsString: map[string]string{
			"dc": "aa",
			"x":  "11",
		},
	}
	assert.Equal(t, span.GetFieldValue(SpanFieldTraceId, false, false), "123")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpanId, false, false), "456")
	assert.Equal(t, span.GetFieldValue(SpanFieldPSM, false, false), "1")
	assert.Equal(t, span.GetFieldValue(SpanFieldLogID, false, false), "2")
	assert.Equal(t, span.GetFieldValue(SpanFieldCallType, false, false), "custom")
	assert.Equal(t, span.GetFieldValue(SpanFieldDuration, false, false), int64(123))
	assert.Equal(t, span.GetFieldValue(SpanFieldStartTime, false, false), int64(1234))
	assert.Equal(t, span.GetFieldValue(SpanFieldParentID, false, false), "123456")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpaceId, false, false), "987")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpanType, false, false), "span_type")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpanName, false, false), "span_name")
	assert.Equal(t, span.GetFieldValue(SpanFieldInput, false, false), "input")
	assert.Equal(t, span.GetFieldValue(SpanFieldOutput, false, false), "output")
	assert.Equal(t, span.GetFieldValue(SpanFieldMethod, false, false), "method")
	assert.Equal(t, span.GetFieldValue(SpanFieldObjectStorage, false, false), "os")
	assert.Equal(t, span.GetFieldValue("tag1", false, false), "1")
	assert.Equal(t, span.GetFieldValue("tag2", false, false), int64(2))
	assert.Equal(t, span.GetFieldValue("tag3", false, false), 3.0)
	assert.Equal(t, span.GetFieldValue("tag4", false, false), true)
	assert.Equal(t, span.GetFieldValue("tag5", false, false), "12")
	assert.Equal(t, span.GetFieldValue("tag6", true, false), nil)
	assert.Equal(t, span.GetFieldValue("stag1", true, false), 0.0)
	assert.Equal(t, span.GetFieldValue("stag2", true, false), "1")
	assert.Equal(t, span.GetFieldValue("stag3", true, false), int64(2))
	assert.Equal(t, span.GetFieldValue("tag1", false, true), "1")
	assert.Equal(t, span.IsValidSpan() != nil, true)
	assert.Equal(t, validSpan.IsValidSpan() == nil, true)
	assert.Equal(t, span.GetSystemTags(), map[string]string{"stag1": "0", "stag2": "1", "stag3": "2"})
	assert.Equal(t, span.GetCustomTags(), map[string]string{"tag1": "1", "tag2": "2", "tag3": "3", "tag4": "true", "tag5": "12", "input_tokens": "10", "output_tokens": "20"})
	in, out, _ := span.getTokens(context.Background())
	assert.Equal(t, in, int64(10))
	assert.Equal(t, out, int64(20))
	assert.Equal(t, TTLFromInteger(4), TTL3d)
	assert.Equal(t, TTLFromInteger(3), TTL3d)
	assert.Equal(t, TTLFromInteger(7), TTL7d)
	assert.Equal(t, TTLFromInteger(30), TTL30d)
	assert.Equal(t, TTLFromInteger(90), TTL90d)
	assert.Equal(t, TTLFromInteger(180), TTL180d)
	assert.Equal(t, TTLFromInteger(365), TTL365d)

	ctx := context.Background()
	span = &Span{
		StartTime:       time.Now().Add(-24 * time.Hour).UnixMicro(),
		LogicDeleteTime: time.Now().Add(24 * 7 * time.Hour).UnixMicro(),
	}
	assert.Equal(t, span.GetTTL(ctx), TTL7d)
	span.LogicDeleteTime = time.Now().Add(24 * 30 * time.Hour).UnixMicro()
	assert.Equal(t, span.GetTTL(ctx), TTL30d)
	span.LogicDeleteTime = time.Now().Add(24 * 90 * time.Hour).UnixMicro()
	assert.Equal(t, span.GetTTL(ctx), TTL90d)
	span.LogicDeleteTime = time.Now().Add(24 * 180 * time.Hour).UnixMicro()
	assert.Equal(t, span.GetTTL(ctx), TTL180d)
	span.LogicDeleteTime = time.Now().Add(24 * 365 * time.Hour).UnixMicro()
	assert.Equal(t, span.GetTTL(ctx), TTL365d)
}

func TestSpan_AddAnnotation(t *testing.T) {
	t.Parallel()
	// 测试向空列表添加注解
	span := &Span{
		SpanID:  "test-span-id",
		TraceID: "test-trace-id",
	}

	annotation := &Annotation{
		SpanID:  "test-span-id",
		TraceID: "test-trace-id",
		Key:     "test-key",
		Value:   NewBoolValue(true),
	}

	span.AddAnnotation(annotation)

	assert.NotNil(t, span.Annotations)
	assert.Equal(t, len(span.Annotations), 1)
	assert.Equal(t, span.Annotations[0], annotation)

	// 测试向已有列表添加注解
	annotation2 := &Annotation{
		SpanID:  "test-span-id",
		TraceID: "test-trace-id",
		Key:     "test-key-2",
		Value:   NewBoolValue(false),
	}

	span.AddAnnotation(annotation2)

	assert.Equal(t, len(span.Annotations), 2)
	assert.Equal(t, span.Annotations[0], annotation)
	assert.Equal(t, span.Annotations[1], annotation2)

	// 测试添加nil注解
	span.AddAnnotation(nil)
	assert.Equal(t, len(span.Annotations), 3)
	assert.Nil(t, span.Annotations[2])
}

func TestSpan_AddManualDatasetAnnotation(t *testing.T) {
	t.Parallel()
	span := &Span{
		SpanID:      "test-span-id",
		TraceID:     "test-trace-id",
		StartTime:   time.Now().UnixMicro(),
		WorkspaceID: "test-workspace",
	}

	datasetID := int64(12345)
	userID := "test-user"
	annotationType := AnnotationTypeManualDataset

	// 测试正常创建注解
	annotation, err := span.AddManualDatasetAnnotation(datasetID, userID, annotationType)

	assert.NoError(t, err)
	assert.NotNil(t, annotation)

	// 验证注解字段设置
	assert.Equal(t, annotation.SpanID, span.SpanID)
	assert.Equal(t, annotation.TraceID, span.TraceID)
	assert.Equal(t, annotation.WorkspaceID, span.WorkspaceID)
	assert.Equal(t, annotation.AnnotationType, annotationType)
	assert.Equal(t, annotation.Key, "12345")
	assert.Equal(t, annotation.Value.BoolValue, true)
	assert.Equal(t, annotation.Value.ValueType, AnnotationValueTypeBool)
	assert.NotNil(t, annotation.Metadata)
	assert.Equal(t, annotation.Status, AnnotationStatusNormal)
	assert.Equal(t, annotation.CreatedBy, userID)
	assert.Equal(t, annotation.UpdatedBy, userID)
	assert.NotEmpty(t, annotation.ID)

	// 验证注解添加到span
	assert.Equal(t, len(span.Annotations), 1)
	assert.Equal(t, span.Annotations[0], annotation)

	// 测试添加多个注解
	annotation2, err := span.AddManualDatasetAnnotation(67890, "user2", AnnotationTypeManualFeedback)
	assert.NoError(t, err)
	assert.Equal(t, len(span.Annotations), 2)
	assert.Equal(t, span.Annotations[1], annotation2)
}

func TestSpan_ExtractByJsonpath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	span := &Span{
		Input:  `{"name": "test", "data": {"value": 123, "nested": {"key": "hello"}}}`,
		Output: `{"result": "success", "score": 0.95, "details": {"message": "completed"}}`,
		TagsString: map[string]string{
			"tag1": `{"custom": "value"}`,
		},
		TagsLong: map[string]int64{
			"count":                   42,
			SpanFieldLatencyFirstResp: 1 * time.Second.Microseconds(),
		},
		DurationMicros: 5 * time.Second.Microseconds(),
	}

	// 测试从Input字段提取数据
	result, err := span.ExtractByJsonpath(ctx, "Input", "name")
	assert.NoError(t, err)
	assert.Equal(t, result, "test")

	result, err = span.ExtractByJsonpath(ctx, "Input", "data.value")
	assert.NoError(t, err)
	assert.Equal(t, result, "123")

	result, err = span.ExtractByJsonpath(ctx, "Input", "data.nested.key")
	assert.NoError(t, err)
	assert.Equal(t, result, "hello")

	// 测试从Output字段提取数据
	result, err = span.ExtractByJsonpath(ctx, "Output", "result")
	assert.NoError(t, err)
	assert.Equal(t, result, "success")

	result, err = span.ExtractByJsonpath(ctx, "Output", "score")
	assert.NoError(t, err)
	// Float precision may vary, so we check if it starts with "0.95"
	assert.True(t, strings.HasPrefix(result, "0.95"))

	result, err = span.ExtractByJsonpath(ctx, "Output", "details.message")
	assert.NoError(t, err)
	assert.Equal(t, result, "completed")

	// 测试从Tags字段提取数据
	result, err = span.ExtractByJsonpath(ctx, "Tags.tag1", "custom")
	assert.NoError(t, err)
	assert.Equal(t, result, "value")

	result, err = span.ExtractByJsonpath(ctx, "Tags.count", "")
	assert.NoError(t, err)
	assert.Equal(t, result, "42")

	// 测试空jsonpath的处理
	result, err = span.ExtractByJsonpath(ctx, "Input", "")
	assert.NoError(t, err)
	assert.Equal(t, result, span.Input)

	result, err = span.ExtractByJsonpath(ctx, "Output", "")
	assert.NoError(t, err)
	assert.Equal(t, result, span.Output)

	// 测试不支持的key类型
	result, err = span.ExtractByJsonpath(ctx, "UnsupportedKey", "path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported mapping key")
	assert.Equal(t, result, "")

	// 测试空数据的处理
	emptySpan := &Span{
		Input:  "",
		Output: "",
	}
	result, err = emptySpan.ExtractByJsonpath(ctx, "Input", "name")
	assert.NoError(t, err)
	assert.Equal(t, result, "")

	result, err = emptySpan.ExtractByJsonpath(ctx, "Output", "result")
	assert.NoError(t, err)
	assert.Equal(t, result, "")

	// 测试无效JSON的处理
	invalidJsonSpan := &Span{
		Input: `{"invalid": json}`,
	}
	result, err = invalidJsonSpan.ExtractByJsonpath(ctx, "Input", "invalid")
	assert.Error(t, err)
	assert.Equal(t, result, "")

	// 测试不存在的JSON路径
	result, err = span.ExtractByJsonpath(ctx, "Input", "nonexistent.path")
	assert.NoError(t, err)
	assert.Equal(t, result, "")

	// 测试Tags字段不存在的情况
	result, err = span.ExtractByJsonpath(ctx, "Tags.nonexistent", "path")
	assert.NoError(t, err)
	assert.Equal(t, result, "")

	// 测试duration_micros
	result, err = span.ExtractByJsonpath(ctx, "Tags.duration", "")
	assert.NoError(t, err)
	assert.Equal(t, strconv.FormatInt(5*time.Second.Milliseconds(), 10), result)

	// 测试latency_first_resp
	result, err = span.ExtractByJsonpath(ctx, "Tags.latency_first_resp", "")
	assert.NoError(t, err)
	assert.Equal(t, strconv.FormatInt(1*time.Second.Milliseconds(), 10), result)
}

// TestGetFieldValue_SystemTags tests the GetFieldValue method with system tags
func TestGetFieldValue_SystemTags(t *testing.T) {
	t.Parallel()
	span := &Span{
		SystemTagsString: map[string]string{
			"system_tag1": "system_value1",
		},
		SystemTagsLong: map[string]int64{
			"system_tag2": 123,
		},
		SystemTagsDouble: map[string]float64{
			"system_tag3": 3.14,
		},
		TagsString: map[string]string{
			"user_tag1": "user_value1",
		},
		TagsLong: map[string]int64{
			"user_tag2": 456,
		},
		TagsDouble: map[string]float64{
			"user_tag3": 2.71,
		},
		TagsBool: map[string]bool{
			"user_tag4": true,
		},
		TagsByte: map[string]string{
			"user_tag5": "byte_value",
		},
	}

	tests := []struct {
		name      string
		fieldName string
		isSystem  bool
		isCustom  bool
		want      interface{}
	}{
		// System tags tests
		{
			name:      "get system string tag",
			fieldName: "system_tag1",
			isSystem:  true,
			want:      "system_value1",
		},
		{
			name:      "get system long tag",
			fieldName: "system_tag2",
			isSystem:  true,
			want:      int64(123),
		},
		{
			name:      "get system double tag",
			fieldName: "system_tag3",
			isSystem:  true,
			want:      3.14,
		},
		{
			name:      "get non-existent system tag",
			fieldName: "non_existent",
			isSystem:  true,
			want:      nil,
		},
		// User tags tests
		{
			name:      "get user string tag",
			fieldName: "user_tag1",
			isSystem:  false,
			isCustom:  true,
			want:      "user_value1",
		},
		{
			name:      "get user long tag",
			fieldName: "user_tag2",
			isSystem:  false,
			isCustom:  true,
			want:      int64(456),
		},
		{
			name:      "get user double tag",
			fieldName: "user_tag3",
			isSystem:  false,
			isCustom:  true,
			want:      2.71,
		},
		{
			name:      "get user bool tag",
			fieldName: "user_tag4",
			isSystem:  false,
			isCustom:  true,
			want:      true,
		},
		{
			name:      "get user byte tag",
			fieldName: "user_tag5",
			isSystem:  false,
			isCustom:  true,
			want:      "byte_value",
		},
		{
			name:      "get non-existent user tag",
			fieldName: "non_existent",
			isSystem:  false,
			isCustom:  true,
			want:      nil,
		},
		// System field should not return user tags
		{
			name:      "system field should not return user tag",
			fieldName: "user_tag1",
			isSystem:  true,
			isCustom:  false,
			want:      nil,
		},
		// User field should not return system tags
		{
			name:      "user field should not return system tag",
			fieldName: "system_tag1",
			isSystem:  false,
			isCustom:  false,
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := span.GetFieldValue(tt.fieldName, tt.isSystem, tt.isCustom)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSpan_MergeHistoryContext(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("merge input and output history messages", func(t *testing.T) {
		span := &Span{
			Input: `{"messages":[{"role":"user","content":"cur1"},{"role":"assistant","content":"cur2"}]}`,
		}
		history := []*Span{
			{Input: `{"messages":[{"role":"system","content":"hist_in1"}]}`},
			{Output: `{"choices":[{"role":"assistant","content":"hist_out1"}]}`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["messages"].([]interface{})
		assert.Equal(t, 4, len(msgs))
		first, _ := msgs[0].(map[string]interface{})
		second, _ := msgs[1].(map[string]interface{})
		third, _ := msgs[2].(map[string]interface{})
		fourth, _ := msgs[3].(map[string]interface{})
		assert.Equal(t, "system", first["role"])
		assert.Equal(t, "assistant", second["role"])
		assert.Equal(t, "user", third["role"])
		assert.Equal(t, "assistant", fourth["role"])
	})

	t.Run("merge response api input/output string into wrappers", func(t *testing.T) {
		span := &Span{
			Input: `{"input":"cur_in"}`,
		}
		history := []*Span{
			{Input: `{"input":"hist_in"}`},
			{Output: `{"output":"hist_out"}`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["input"].([]interface{})
		assert.Equal(t, 3, len(msgs))
		first, _ := msgs[0].(map[string]interface{})
		second, _ := msgs[1].(map[string]interface{})
		third, _ := msgs[2].(map[string]interface{})
		assert.Equal(t, "user", first["role"])
		assert.Equal(t, "hist_in", first["content"])
		assert.Equal(t, "assistant", second["role"])
		assert.Equal(t, "hist_out", second["content"])
		assert.Equal(t, "user", third["role"])
		assert.Equal(t, "cur_in", third["content"])
	})

	t.Run("merge response api input/output list into messages", func(t *testing.T) {
		span := &Span{
			Input: `{"messages":[{"role":"user","content":"cur"}]}`,
		}
		history := []*Span{
			{Input: `{"input":[{"role":"user","content":"hist_in"}]}`},
			{Output: `{"output":[{"role":"assistant","content":"hist_out"}]}`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["messages"].([]interface{})
		assert.Equal(t, 3, len(msgs))
		first, _ := msgs[0].(map[string]interface{})
		second, _ := msgs[1].(map[string]interface{})
		third, _ := msgs[2].(map[string]interface{})
		assert.Equal(t, "user", first["role"])
		assert.Equal(t, "assistant", second["role"])
		assert.Equal(t, "user", third["role"])
	})

	t.Run("current messages fallback to input when messages is not array", func(t *testing.T) {
		span := &Span{
			Input: `{"messages":"bad","input":"cur_in"}`,
		}
		history := []*Span{
			{Input: `{"messages":"bad","input":"hist_in"}`},
			{Output: `{"choices":"bad","output":"hist_out"}`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["input"].([]interface{})
		assert.Equal(t, 3, len(msgs))
		first, _ := msgs[0].(map[string]interface{})
		second, _ := msgs[1].(map[string]interface{})
		third, _ := msgs[2].(map[string]interface{})
		assert.Equal(t, "user", first["role"])
		assert.Equal(t, "hist_in", first["content"])
		assert.Equal(t, "assistant", second["role"])
		assert.Equal(t, "hist_out", second["content"])
		assert.Equal(t, "user", third["role"])
		assert.Equal(t, "cur_in", third["content"])
	})

	t.Run("empty current input string merges history to input", func(t *testing.T) {
		span := &Span{Input: `{"input":""}`}
		history := []*Span{
			{Input: `{"input":"hist_in"}`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["input"].([]interface{})
		assert.Equal(t, 1, len(msgs))
		first, _ := msgs[0].(map[string]interface{})
		assert.Equal(t, "user", first["role"])
		assert.Equal(t, "hist_in", first["content"])
	})

	t.Run("no messages and no input merges history to input", func(t *testing.T) {
		span := &Span{Input: `{"foo":"bar"}`}
		history := []*Span{
			{Input: `{"messages":[{"role":"system","content":"h"}]}`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["input"].([]interface{})
		assert.Equal(t, 1, len(msgs))
		first, _ := msgs[0].(map[string]interface{})
		assert.Equal(t, "system", first["role"])
		assert.Equal(t, "h", first["content"])
		assert.Equal(t, "bar", m["foo"])
	})

	t.Run("no history messages keeps input unchanged", func(t *testing.T) {
		orig := `{"messages":[{"role":"user","content":"cur"}]}`
		span := &Span{Input: orig}
		history := []*Span{
			{Input: `{"no_messages":[]}`},
			{Output: `{"info":"x"}`},
		}
		span.MergeHistoryContext(ctx, history)
		assert.Equal(t, orig, span.Input)
	})

	t.Run("invalid current input leaves unchanged", func(t *testing.T) {
		span := &Span{Input: `not-json`}
		history := []*Span{
			{Input: `{"messages":[{"role":"system","content":"h"}]}`},
		}
		span.MergeHistoryContext(ctx, history)
		assert.Equal(t, `not-json`, span.Input)
	})

	t.Run("invalid history json is skipped", func(t *testing.T) {
		span := &Span{Input: `{"messages":[{"role":"user","content":"cur"}]}`}
		history := []*Span{
			{Input: `{"messages":[{"role":"system","content":"h1"}]}`},
			{Output: `{"messages": "not-array"}`},
			{Input: `{"messages":[{"role":"assistant","content":"h2"}]}`},
			{Output: `bad-json`},
		}
		span.MergeHistoryContext(ctx, history)
		var m map[string]interface{}
		_ = json.Unmarshal([]byte(span.Input), &m)
		msgs, _ := m["messages"].([]interface{})
		assert.Equal(t, 3, len(msgs))
	})

	t.Run("empty history does nothing", func(t *testing.T) {
		orig := `{"messages":[{"role":"user","content":"cur"}]}`
		span := &Span{Input: orig}
		span.MergeHistoryContext(ctx, nil)
		assert.Equal(t, orig, span.Input)
	})

	t.Run("helper methods in span.go", func(t *testing.T) {
		span := &Span{
			SpanID:      "0000000000000001",
			TraceID:     "00000000000000000000000000000001",
			WorkspaceID: "1",
			StartTime:   time.Now().UnixMicro(),
			SpanType:    SpanTypeModel,
			SystemTagsString: map[string]string{
				SpanFieldKeyPreviousResponseID: "prev",
				SpanFieldTenant:                "tenant1",
			},
		}
		assert.True(t, span.IsResponseAPISpan())
		assert.Equal(t, "tenant1", span.GetTenant())

		span2 := &Span{SpanType: SpanTypePrompt}
		assert.False(t, span2.IsResponseAPISpan())
		span3 := &Span{SpanType: SpanTypeModel}
		assert.False(t, span3.IsResponseAPISpan())
		span4 := &Span{SpanType: SpanTypeModel, SystemTagsString: map[string]string{SpanFieldKeyPreviousResponseID: ""}}
		assert.False(t, span4.IsResponseAPISpan())
	})

	t.Run("AddAutoEvalAnnotation and SpanList helpers", func(t *testing.T) {
		span := &Span{
			SpanID:      "0000000000000001",
			TraceID:     "00000000000000000000000000000001",
			WorkspaceID: "1",
			StartTime:   time.Now().UnixMicro(),
		}
		anno, err := span.AddAutoEvalAnnotation(1, 2, 3, 0.5, "reason", "user1")
		assert.NoError(t, err)
		assert.NotNil(t, anno)
		assert.Equal(t, AnnotationTypeAutoEvaluate, anno.AnnotationType)
		assert.Equal(t, 1, len(span.Annotations))

		spans := SpanList{
			{SpanType: SpanTypePrompt, StartTime: 2},
			{SpanType: SpanTypeModel, StartTime: 3, TagsLong: map[string]int64{SpanFieldInputTokens: 10, SpanFieldOutputTokens: 20}},
			{SpanType: SpanTypeLLMCall, StartTime: 1, TagsLong: map[string]int64{SpanFieldInputTokens: 1, SpanFieldOutputTokens: 2}},
		}
		in, out, err := spans.Stat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(11), in)
		assert.Equal(t, int64(22), out)

		filtered := spans.FilterSpans(GetModelSpansFilter())
		assert.Equal(t, 2, len(filtered))

		spans.SortByStartTime(false)
		assert.Equal(t, int64(1), spans[0].StartTime)
		spans.SortByStartTime(true)
		assert.Equal(t, int64(3), spans[0].StartTime)

		uniq := SpanList{
			{SpanID: "a", TraceID: "t"},
			{SpanID: "a", TraceID: "t"},
			{SpanID: "b", TraceID: "t"},
		}.Uniq()
		assert.Equal(t, 2, len(uniq))
	})

	t.Run("field and tag helpers", func(t *testing.T) {
		type sample struct {
			Str   string  `json:"str"`
			Bool  bool    `json:"bool"`
			I64   int64   `json:"i64"`
			F64   float64 `json:"f64"`
			Ptr   *string `json:"ptr"`
			Bad   int     `json:"bad"`
			NoTag string
		}

		s := &sample{}
		fields := NewStruct(s).Fields()
		assert.GreaterOrEqual(t, len(fields), 1)

		var ptrField *Field
		var badField *Field
		var noTagField *Field
		for _, f := range fields {
			if f.Name() == "Ptr" {
				ptrField = f
			}
			if f.Name() == "Bad" {
				badField = f
			}
			if f.Name() == "NoTag" {
				noTagField = f
			}
			alias, err := f.TagJson()
			if f.Name() == "Bad" {
				assert.NoError(t, err)
				assert.NotEmpty(t, alias)
			}
		}
		assert.NotNil(t, noTagField)
		_, err := noTagField.TagJson()
		assert.Error(t, err)

		assert.NotNil(t, ptrField)
		assert.Equal(t, reflect.Ptr, ptrField.Kind())
		assert.NoError(t, ptrField.Set("x"))
		assert.NotNil(t, s.Ptr)
		assert.Equal(t, "x", *s.Ptr)
		vt, err := ptrField.ValueType()
		assert.NoError(t, err)
		assert.Equal(t, TagValueTypeString, vt)

		assert.NotNil(t, badField)
		_, err = badField.ValueType()
		assert.Error(t, err)

		assert.Equal(t, "Bool", TagValueTypeBool.String())
		assert.Equal(t, "I64", TagValueTypeInt64.String())
		assert.Equal(t, "F64", TagValueTypeFloat64.String())
		assert.Equal(t, "String", TagValueTypeString.String())
		assert.Equal(t, "<UNSET>", TagValueTypeUnknown.String())
	})
}

// TestSizeofSpans tests the SizeofSpans function
func TestSizeofSpans(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		spans SpanList
		want  int
	}{
		{
			name:  "empty span list",
			spans: SpanList{},
			want:  0,
		},
		{
			name: "single span with basic fields",
			spans: SpanList{
				{
					SpanID:         "test-span-id",
					TraceID:        "test-trace-id",
					StartTime:      1234567890,
					DurationMicros: 1000,
					SpanName:       "test-span",
					Input:          "test input",
					Output:         "test output",
				},
			},
		},
		{
			name: "span with all tag types",
			spans: SpanList{
				{
					SpanID:    "test-span-id",
					TraceID:   "test-trace-id",
					StartTime: 1234567890,
					SystemTagsString: map[string]string{
						"sys_tag1": "sys_value1",
					},
					SystemTagsLong: map[string]int64{
						"sys_tag2": 123,
					},
					SystemTagsDouble: map[string]float64{
						"sys_tag3": 3.14,
					},
					TagsString: map[string]string{
						"tag1": "value1",
					},
					TagsLong: map[string]int64{
						"tag2": 456,
					},
					TagsDouble: map[string]float64{
						"tag3": 2.71,
					},
					TagsBool: map[string]bool{
						"tag4": true,
					},
					TagsByte: map[string]string{
						"tag5": "byte_value",
					},
				},
			},
		},
		{
			name: "span with AttrTos",
			spans: SpanList{
				{
					SpanID:    "test-span-id",
					TraceID:   "test-trace-id",
					StartTime: 1234567890,
					AttrTos: &AttrTos{
						InputDataURL:  "input-url",
						OutputDataURL: "output-url",
						MultimodalData: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
					},
				},
			},
		},
		{
			name: "span with annotations",
			spans: SpanList{
				{
					SpanID:    "test-span-id",
					TraceID:   "test-trace-id",
					StartTime: 1234567890,
					Annotations: []*Annotation{
						{
							ID:             "annotation-id",
							SpanID:         "test-span-id",
							TraceID:        "test-trace-id",
							WorkspaceID:    "workspace-id",
							Key:            "test-key",
							Value:          NewStringValue("test-value"),
							Reasoning:      "test-reasoning",
							CreatedBy:      "user-id",
							UpdatedBy:      "user-id",
							AnnotationType: AnnotationTypeManualFeedback,
							Status:         AnnotationStatusNormal,
						},
						nil, // Test nil annotation handling
					},
				},
			},
		},
		{
			name: "multiple spans",
			spans: SpanList{
				{
					SpanID:    "span1",
					TraceID:   "trace1",
					StartTime: 1234567890,
					SpanName:  "span1",
				},
				{
					SpanID:    "span2",
					TraceID:   "trace2",
					StartTime: 1234567891,
					SpanName:  "span2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SizeofSpans(tt.spans)
			// We can't predict exact size due to internal Go structures,
			// but we can verify it's non-negative and reasonable
			if tt.name == "empty span list" {
				assert.Equal(t, 0, got)
			} else {
				assert.GreaterOrEqual(t, got, 0)
				// For non-empty spans, size should be greater than 0
				if len(tt.spans) > 0 {
					assert.Greater(t, got, 0)
				}
			}
		})
	}
}

// TestSizeOfString tests the SizeOfString function
func TestSizeOfString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    string
		want int
	}{
		{
			name: "empty string",
			s:    "",
			want: 0,
		},
		{
			name: "simple string",
			s:    "hello",
			want: 5,
		},
		{
			name: "string with spaces",
			s:    "hello world",
			want: 11,
		},
		{
			name: "UTF-8 string",
			s:    "你好世界",
			want: 12, // 4 characters * 3 bytes each
		},
		{
			name: "string with special characters",
			s:    "hello@#$%^&*()",
			want: 14,
		},
		{
			name: "long string",
			s:    "this is a very long string that contains many characters",
			want: 56,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SizeOfString(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSpan_GetFieldValue_AllFields tests GetFieldValue for all supported fields
func TestSpan_GetFieldValue_AllFields(t *testing.T) {
	t.Parallel()
	span := &Span{
		StartTime:      1234567890,
		SpanID:         "test-span-id",
		ParentID:       "test-parent-id",
		TraceID:        "test-trace-id",
		DurationMicros: 1000,
		CallType:       "test-call-type",
		PSM:            "test-psm",
		LogID:          "test-log-id",
		WorkspaceID:    "test-workspace-id",
		SpanName:       "test-span-name",
		SpanType:       "test-span-type",
		Method:         "test-method",
		StatusCode:     200,
		Input:          "test-input",
		Output:         "test-output",
		ObjectStorage:  "test-object-storage",
		TagsString: map[string]string{
			"custom_tag": "custom-value",
		},
	}

	tests := []struct {
		name      string
		fieldName string
		isSystem  bool
		isCustom  bool
		want      interface{}
	}{
		{name: "StartTime", fieldName: SpanFieldStartTime, isSystem: false, want: int64(1234567890)},
		{name: "SpanID", fieldName: SpanFieldSpanId, isSystem: false, want: "test-span-id"},
		{name: "ParentID", fieldName: SpanFieldParentID, isSystem: false, want: "test-parent-id"},
		{name: "TraceID", fieldName: SpanFieldTraceId, isSystem: false, want: "test-trace-id"},
		{name: "Duration", fieldName: SpanFieldDuration, isSystem: false, want: int64(1000)},
		{name: "CallType", fieldName: SpanFieldCallType, isSystem: false, want: "test-call-type"},
		{name: "PSM", fieldName: SpanFieldPSM, isSystem: false, want: "test-psm"},
		{name: "LogID", fieldName: SpanFieldLogID, isSystem: false, want: "test-log-id"},
		{name: "WorkspaceID", fieldName: SpanFieldSpaceId, isSystem: false, want: "test-workspace-id"},
		{name: "SpanName", fieldName: SpanFieldSpanName, isSystem: false, want: "test-span-name"},
		{name: "SpanType", fieldName: SpanFieldSpanType, isSystem: false, want: "test-span-type"},
		{name: "Method", fieldName: SpanFieldMethod, isSystem: false, want: "test-method"},
		{name: "StatusCode", fieldName: SpanFieldStatusCode, isSystem: false, want: int32(200)},
		{name: "Status", fieldName: SpanFieldStatus, isSystem: false, want: SpanStatusError},
		{name: "Input", fieldName: SpanFieldInput, isSystem: false, want: "test-input"},
		{name: "Output", fieldName: SpanFieldOutput, isSystem: false, want: "test-output"},
		{name: "ObjectStorage", fieldName: SpanFieldObjectStorage, isSystem: false, want: "test-object-storage"},
		{name: "Custom tag with isCustom", fieldName: "custom_tag", isSystem: false, isCustom: true, want: "custom-value"},
		{name: "Unknown field", fieldName: "unknown_field", isSystem: false, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := span.GetFieldValue(tt.fieldName, tt.isSystem, tt.isCustom)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSpanList_FilterModelSpans tests the FilterModelSpans method
func TestSpanList_FilterModelSpans(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		spans SpanList
		want  int // expected number of spans after filtering
	}{
		{
			name:  "empty span list",
			spans: SpanList{},
			want:  0,
		},
		{
			name: "no model spans",
			spans: SpanList{
				{SpanType: "prompt"},
				{SpanType: "parser"},
			},
			want: 0,
		},
		{
			name: "only LLMCall spans",
			spans: SpanList{
				{SpanType: "LLMCall"},
				{SpanType: "LLMCall"},
			},
			want: 2,
		},
		{
			name: "only model spans",
			spans: SpanList{
				{SpanType: "model"},
				{SpanType: "model"},
			},
			want: 2,
		},
		{
			name: "mixed spans",
			spans: SpanList{
				{SpanType: "LLMCall"},
				{SpanType: "model"},
				{SpanType: "prompt"},
				{SpanType: "parser"},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := GetModelSpansFilter()
			got := 0
			for _, span := range tt.spans {
				if f.Satisfied(span) {
					got++
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
