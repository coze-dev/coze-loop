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
