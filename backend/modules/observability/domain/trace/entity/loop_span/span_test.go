// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpan(t *testing.T) {
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
	assert.Equal(t, span.GetFieldValue(SpanFieldTraceId), "123")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpanId), "456")
	assert.Equal(t, span.GetFieldValue(SpanFieldPSM), "1")
	assert.Equal(t, span.GetFieldValue(SpanFieldLogID), "2")
	assert.Equal(t, span.GetFieldValue(SpanFieldCallType), "custom")
	assert.Equal(t, span.GetFieldValue(SpanFieldDuration), int64(123))
	assert.Equal(t, span.GetFieldValue(SpanFieldStartTime), int64(1234))
	assert.Equal(t, span.GetFieldValue(SpanFieldParentID), "123456")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpaceId), "987")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpanType), "span_type")
	assert.Equal(t, span.GetFieldValue(SpanFieldSpanName), "span_name")
	assert.Equal(t, span.GetFieldValue(SpanFieldInput), "input")
	assert.Equal(t, span.GetFieldValue(SpanFieldOutput), "output")
	assert.Equal(t, span.GetFieldValue(SpanFieldMethod), "method")
	assert.Equal(t, span.GetFieldValue(SpanFieldObjectStorage), "os")
	assert.Equal(t, span.GetFieldValue("tag1"), "1")
	assert.Equal(t, span.GetFieldValue("tag2"), int64(2))
	assert.Equal(t, span.GetFieldValue("tag3"), 3.0)
	assert.Equal(t, span.GetFieldValue("tag4"), true)
	assert.Equal(t, span.GetFieldValue("tag5"), "12")
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
	ctx := context.Background()

	span := &Span{
		Input:  `{"name": "test", "data": {"value": 123, "nested": {"key": "hello"}}}`,
		Output: `{"result": "success", "score": 0.95, "details": {"message": "completed"}}`,
		TagsString: map[string]string{
			"tag1": `{"custom": "value"}`,
		},
		TagsLong: map[string]int64{
			"count": 42,
		},
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
	assert.Equal(t, result, "0.95")

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
}
