// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package converter

import (
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestSpanListDO2PO(t *testing.T) {
	tests := []struct {
		name  string
		spans loop_span.SpanList
		want  int
	}{
		{
			name:  "empty span list",
			spans: loop_span.SpanList{},
			want:  0,
		},
		{
			name: "single span",
			spans: loop_span.SpanList{
				{
					TraceID:        "trace1",
					SpanID:         "span1",
					WorkspaceID:    "ws1",
					SpanType:       "http",
					SpanName:       "test-span",
					ParentID:       "parent1",
					StartTime:      time.Now().UnixMicro(),
					DurationMicros: 1000,
					PSM:            "test-psm",
					LogID:          "log1",
					StatusCode:     200,
					Input:          "input data",
					Output:         "output data",
					TagsBool: map[string]bool{
						"success": true,
						"error":   false,
					},
					TagsString: map[string]string{
						"method": "GET",
					},
					TagsLong: map[string]int64{
						"count": 10,
					},
					TagsByte: map[string]string{
						"data": "bytes",
					},
					SystemTagsDouble: map[string]float64{
						"cpu": 0.8,
					},
					SystemTagsLong: map[string]int64{
						"memory": 1024,
					},
					SystemTagsString: map[string]string{
						"host": "localhost",
					},
					Method:        "GET",
					CallType:      "http",
					ObjectStorage: "s3://bucket/path",
				},
			},
			want: 1,
		},
		{
			name: "multiple spans",
			spans: loop_span.SpanList{
				{TraceID: "trace1", SpanID: "span1"},
				{TraceID: "trace2", SpanID: "span2"},
				{TraceID: "trace3", SpanID: "span3"},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpanListDO2PO(tt.spans, loop_span.TTL3d)
			assert.Equal(t, tt.want, len(got))
			if len(tt.spans) > 0 && len(got) > 0 {
				assert.Equal(t, tt.spans[0].TraceID, got[0].TraceID)
				assert.Equal(t, tt.spans[0].SpanID, got[0].SpanID)
			}
		})
	}
}

func TestSpanListPO2DO(t *testing.T) {
	tests := []struct {
		name  string
		spans []*dao.Span
		want  int
	}{
		{
			name:  "empty po span list",
			spans: []*dao.Span{},
			want:  0,
		},
		{
			name: "single po span",
			spans: []*dao.Span{
				{
					TraceID:    "trace1",
					SpanID:     "span1",
					SpaceID:    "ws1",
					SpanType:   "http",
					SpanName:   "test-span",
					ParentID:   "parent1",
					StartTime:  time.Now().UnixMicro(),
					Duration:   1000,
					Psm:        ptr.Of("test-psm"),
					Logid:      ptr.Of("log1"),
					StatusCode: 200,
					Input:      "input data",
					Output:     "output data",
					TagsBool: map[string]uint8{
						"success": 1,
						"error":   0,
					},
					TagsString: map[string]string{
						"method": "GET",
					},
					TagsLong: map[string]int64{
						"count": 10,
					},
					TagsByte: map[string]string{
						"data": "bytes",
					},
					SystemTagsFloat: map[string]float64{
						"cpu": 0.8,
					},
					SystemTagsLong: map[string]int64{
						"memory": 1024,
					},
					SystemTagsString: map[string]string{
						"host": "localhost",
					},
					Method:        ptr.Of("GET"),
					CallType:      ptr.Of("http"),
					ObjectStorage: ptr.Of("s3://bucket/path"),
				},
			},
			want: 1,
		},
		{
			name: "multiple po spans",
			spans: []*dao.Span{
				{TraceID: "trace1", SpanID: "span1"},
				{TraceID: "trace2", SpanID: "span2"},
				{TraceID: "trace3", SpanID: "span3"},
			},
			want: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpanListPO2DO(tt.spans)
			assert.Equal(t, tt.want, len(got))
			if len(tt.spans) > 0 && len(got) > 0 {
				assert.Equal(t, tt.spans[0].TraceID, got[0].TraceID)
				assert.Equal(t, tt.spans[0].SpanID, got[0].SpanID)
			}
		})
	}
}

func TestSpanDO2PO(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		span *loop_span.Span
		ttl  loop_span.TTL
		want *dao.Span
	}{
		{
			name: "convert span with all fields",
			span: &loop_span.Span{
				TraceID:        "trace1",
				SpanID:         "span1",
				WorkspaceID:    "ws1",
				SpanType:       "http",
				SpanName:       "test-span",
				ParentID:       "parent1",
				StartTime:      now.UnixMicro(),
				DurationMicros: 1000,
				PSM:            "test-psm",
				LogID:          "log1",
				StatusCode:     200,
				Input:          "input data",
				Output:         "output data",
				TagsBool: map[string]bool{
					"success": true,
					"error":   false,
				},
				TagsString: map[string]string{
					"method": "GET",
				},
				TagsLong: map[string]int64{
					"count": 10,
				},
				TagsByte: map[string]string{
					"data": "bytes",
				},
				SystemTagsDouble: map[string]float64{
					"cpu": 0.8,
				},
				SystemTagsLong: map[string]int64{
					"memory": 1024,
				},
				SystemTagsString: map[string]string{
					"host": "localhost",
				},
				Method:        "GET",
				CallType:      "http",
				ObjectStorage: "s3://bucket/path",
			},
			ttl: loop_span.TTL3d,
			want: &dao.Span{
				TraceID:    "trace1",
				SpanID:     "span1",
				SpaceID:    "ws1",
				SpanType:   "http",
				SpanName:   "test-span",
				ParentID:   "parent1",
				StartTime:  now.UnixMicro(),
				Duration:   1000,
				Psm:        ptr.Of("test-psm"),
				Logid:      ptr.Of("log1"),
				StatusCode: 200,
				Input:      "input data",
				Output:     "output data",
				TagsBool: map[string]uint8{
					"success": 1,
					"error":   0,
				},
				TagsString: map[string]string{
					"method": "GET",
				},
				TagsLong: map[string]int64{
					"count": 10,
				},
				TagsByte: map[string]string{
					"data": "bytes",
				},
				SystemTagsFloat: map[string]float64{
					"cpu": 0.8,
				},
				SystemTagsLong: map[string]int64{
					"memory": 1024,
				},
				SystemTagsString: map[string]string{
					"host": "localhost",
				},
				Method:        ptr.Of("GET"),
				CallType:      ptr.Of("http"),
				ObjectStorage: ptr.Of("s3://bucket/path"),
			},
		},
		{
			name: "convert span with minimal fields",
			span: &loop_span.Span{
				TraceID: "trace1",
				SpanID:  "span1",
			},
			ttl: loop_span.TTL7d,
			want: &dao.Span{
				TraceID:  "trace1",
				SpanID:   "span1",
				TagsBool: map[string]uint8{},
			},
		},
		{
			name: "convert span with different TTL values",
			span: &loop_span.Span{
				TraceID: "trace1",
				SpanID:  "span1",
			},
			ttl: loop_span.TTL30d,
			want: &dao.Span{
				TraceID:  "trace1",
				SpanID:   "span1",
				TagsBool: map[string]uint8{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpanDO2PO(tt.span, tt.ttl)
			assert.Equal(t, tt.want.TraceID, got.TraceID)
			assert.Equal(t, tt.want.SpanID, got.SpanID)
			assert.Equal(t, tt.want.TagsBool, got.TagsBool)
			if tt.span.Method != "" {
				assert.Equal(t, *tt.want.Method, *got.Method)
			}
			if tt.span.CallType != "" {
				assert.Equal(t, *tt.want.CallType, *got.CallType)
			}
			if tt.span.ObjectStorage != "" {
				assert.Equal(t, *tt.want.ObjectStorage, *got.ObjectStorage)
			}
			// Check TTL-based logic delete date
			switch tt.ttl {
			case loop_span.TTL3d:
				assert.True(t, got.LogicDeleteDate > time.Now().Add(2*24*time.Hour).UnixMicro())
				assert.True(t, got.LogicDeleteDate < time.Now().Add(4*24*time.Hour).UnixMicro())
			case loop_span.TTL7d:
				assert.True(t, got.LogicDeleteDate > time.Now().Add(6*24*time.Hour).UnixMicro())
				assert.True(t, got.LogicDeleteDate < time.Now().Add(8*24*time.Hour).UnixMicro())
			case loop_span.TTL30d:
				assert.True(t, got.LogicDeleteDate > time.Now().Add(29*24*time.Hour).UnixMicro())
				assert.True(t, got.LogicDeleteDate < time.Now().Add(31*24*time.Hour).UnixMicro())
			}
		})
	}
}

func TestSpanPO2DO(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		span *dao.Span
		want *loop_span.Span
	}{
		{
			name: "convert po span with all fields",
			span: &dao.Span{
				TraceID:    "trace1",
				SpanID:     "span1",
				SpaceID:    "ws1",
				SpanType:   "http",
				SpanName:   "test-span",
				ParentID:   "parent1",
				StartTime:  now.UnixMicro(),
				Duration:   1000,
				Psm:        ptr.Of("test-psm"),
				Logid:      ptr.Of("log1"),
				StatusCode: 200,
				Input:      "input data",
				Output:     "output data",
				TagsBool: map[string]uint8{
					"success": 1,
					"error":   0,
				},
				TagsString: map[string]string{
					"method": "GET",
				},
				TagsLong: map[string]int64{
					"count": 10,
				},
				TagsByte: map[string]string{
					"data": "bytes",
				},
				SystemTagsFloat: map[string]float64{
					"cpu": 0.8,
				},
				SystemTagsLong: map[string]int64{
					"memory": 1024,
				},
				SystemTagsString: map[string]string{
					"host": "localhost",
				},
				Method:          ptr.Of("GET"),
				CallType:        ptr.Of("http"),
				ObjectStorage:   ptr.Of("s3://bucket/path"),
				LogicDeleteDate: now.Add(24 * time.Hour).UnixMicro(),
			},
			want: &loop_span.Span{
				TraceID:        "trace1",
				SpanID:         "span1",
				WorkspaceID:    "ws1",
				SpanType:       "http",
				SpanName:       "test-span",
				ParentID:       "parent1",
				StartTime:      now.UnixMicro(),
				DurationMicros: 1000,
				PSM:            "test-psm",
				LogID:          "log1",
				StatusCode:     200,
				Input:          "input data",
				Output:         "output data",
				TagsBool: map[string]bool{
					"success": true,
					"error":   false,
				},
				TagsString: map[string]string{
					"method": "GET",
				},
				TagsLong: map[string]int64{
					"count": 10,
				},
				TagsByte: map[string]string{
					"data": "bytes",
				},
				SystemTagsDouble: map[string]float64{
					"cpu": 0.8,
				},
				SystemTagsLong: map[string]int64{
					"memory": 1024,
				},
				SystemTagsString: map[string]string{
					"host": "localhost",
				},
				TagsDouble:      map[string]float64{},
				Method:          "GET",
				CallType:        "http",
				ObjectStorage:   "s3://bucket/path",
				LogicDeleteTime: now.Add(24 * time.Hour).UnixMicro(),
			},
		},
		{
			name: "convert po span with minimal fields",
			span: &dao.Span{
				TraceID:  "trace1",
				SpanID:   "span1",
				TagsBool: map[string]uint8{},
			},
			want: &loop_span.Span{
				TraceID:          "trace1",
				SpanID:           "span1",
				TagsBool:         map[string]bool{},
				TagsString:       map[string]string{},
				TagsLong:         map[string]int64{},
				TagsByte:         map[string]string{},
				TagsDouble:       map[string]float64{},
				SystemTagsString: map[string]string{},
				SystemTagsLong:   map[string]int64{},
				SystemTagsDouble: map[string]float64{},
			},
		},
		{
			name: "convert nil po span",
			span: nil,
			want: nil,
		},
		{
			name: "convert po span with zero values",
			span: &dao.Span{
				TraceID: "trace1",
				SpanID:  "span1",
				TagsBool: map[string]uint8{
					"zero": 0,
					"one":  1,
				},
			},
			want: &loop_span.Span{
				TraceID: "trace1",
				SpanID:  "span1",
				TagsBool: map[string]bool{
					"zero": false,
					"one":  true,
				},
				TagsString:       map[string]string{},
				TagsLong:         map[string]int64{},
				TagsByte:         map[string]string{},
				TagsDouble:       map[string]float64{},
				SystemTagsString: map[string]string{},
				SystemTagsLong:   map[string]int64{},
				SystemTagsDouble: map[string]float64{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SpanPO2DO(tt.span)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCopyMap(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]interface{}
		want map[string]interface{}
	}{
		{
			name: "copy string map",
			in: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			want: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "copy int map",
			in: map[string]interface{}{
				"key1": 1,
				"key2": 2,
			},
			want: map[string]interface{}{
				"key1": 1,
				"key2": 2,
			},
		},
		{
			name: "copy empty map",
			in:   map[string]interface{}{},
			want: map[string]interface{}{},
		},
		{
			name: "copy nil map",
			in:   nil,
			want: map[string]interface{}{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CopyMap(tt.in)
			assert.Equal(t, tt.want, got)
			// Ensure it's a deep copy
			if tt.in != nil && len(tt.in) > 0 {
				for k := range tt.in {
					got[k] = "modified"
					assert.NotEqual(t, tt.in[k], got[k])
					break
				}
			}
		})
	}
}
