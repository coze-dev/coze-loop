// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"encoding/json"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
)

func TestTrans(t *testing.T) {
	transCfg := SpanTransCfgList{
		&SpanTransConfig{
			SpanFilter: &FilterFields{
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"service_name_a"},
					},
					{
						FieldName: SpanFieldSpanType,
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"span_type_a"},
					},
				},
			},
			TagFilter: &TagFilter{
				KeyBlackList: []string{
					"custom_a",
					"custom_c",
				},
			},
		},
		&SpanTransConfig{
			SpanFilter: &FilterFields{
				FilterFields: []*FilterField{
					{
						FieldName: "service_name",
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"service_name_a"},
					},
					{
						FieldName: SpanFieldSpanType,
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"span_type_b"},
					},
				},
			},
			TagFilter: &TagFilter{
				KeyBlackList: []string{
					"custom_b",
				},
			},
			InputFilter: &InputFilter{
				KeyWhiteList: []string{
					"input_a",
				},
			},
			OutputFilter: &OutputFilter{
				KeyWhiteList: []string{
					"output_b",
				},
			},
		},
	}
	spans := []*Span{
		{
			SpanID:   "aaa",
			TraceID:  "zz",
			ParentID: "zzz",
			SpanType: "span_type_a",
			TagsString: map[string]string{
				"service_name": "service_name_a",
				"custom_a":     "custom_a",
				"custom_b":     "custom_b",
				"custom_c":     "custom_c",
			},
			Input:  `{"input_a": 123, "input_b": 234}`,
			Output: `{"output_a": 123, "output_b": 1234}`,
		},
		{
			SpanID:   "aaaa",
			TraceID:  "zzz",
			ParentID: "zzzz",
			SpanType: "span_type_a",
			TagsString: map[string]string{
				"service_name": "service_name_b",
				"custom_a":     "custom_a",
				"custom_b":     "custom_b",
				"custom_c":     "custom_c",
			},
			Input:  `{"input_a": 123, "input_b": 234}`,
			Output: `{"output_a": 123, "output_b": 1234}`,
		},
		{
			SpanID:   "aaaaa",
			TraceID:  "zzzz",
			ParentID: "zzzzz",
			SpanType: "span_type_b",
			TagsString: map[string]string{
				"service_name": "service_name_b",
				"custom_a":     "custom_a",
				"custom_b":     "custom_b",
				"custom_c":     "custom_c",
			},
			Input:  `{"input_a": 123, "input_b": 234}`,
			Output: `{"output_a": 123, "output_b": 1234}`,
		},
		{
			SpanID:   "aaaaaa",
			TraceID:  "zzzzz",
			ParentID: "zzzzzz",
			SpanType: "span_type_b",
			TagsString: map[string]string{
				"service_name": "service_name_a",
				"custom_a":     "custom_a",
				"custom_b":     "custom_b",
				"custom_c":     "custom_c",
			},
			Input:  `{"input_a": 123, "input_b": 234}`,
			Output: `{"output_a": 123, "output_b": 1234}`,
		},
	}
	spans, err := transCfg.Transform(context.Background(), spans)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 2 {
		t.Fatalf("len(spans) = %d, want 2", len(spans))
	}
	for _, tag := range spans[0].getTags() {
		if tag.Key == "custom_a" {
			t.Errorf("custom_a should not exist")
		}
		if tag.Key == "custom_c" {
			t.Errorf("custom_c should not exist")
		}
	}
	for _, tag := range spans[1].getTags() {
		if tag.Key == "custom_b" {
			t.Errorf("custom_b should not exist")
		}
		if tag.Key == "input" {
			out := make(map[string]any)
			if err := json.Unmarshal([]byte(*tag.Value.VStr), &out); err != nil {
				t.Fatal(err)
			}
			for k := range out {
				if k != "input_a" {
					t.Fatal("only input_a reserved")
				}
			}
		}
		if tag.Key == "output" {
			out := make(map[string]any)
			if err := json.Unmarshal([]byte(*tag.Value.VStr), &out); err != nil {
				t.Fatal(err)
			}
			for k := range out {
				if k != "output_b" {
					t.Fatal("only output_b reserved")
				}
			}
		}
	}

	st := time.Now().UnixMicro()
	tests := []struct {
		InputSpans  SpanList
		OutputSpans SpanList
	}{
		{
			InputSpans: SpanList{
				{
					SpanID:          "A",
					ParentID:        "0",
					LogicDeleteTime: st - time.Minute.Microseconds(),
				},
				{
					SpanID:          "B",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "C",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "D",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "E",
					ParentID:        "B",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
			},
			OutputSpans: SpanList{
				{
					SpanID:          "B",
					ParentID:        "0",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "C",
					ParentID:        "0",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "D",
					ParentID:        "0",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "E",
					ParentID:        "B",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
			},
		},
		{
			InputSpans: SpanList{
				{
					SpanID:          "A",
					ParentID:        "0",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "B",
					ParentID:        "A",
					LogicDeleteTime: st - time.Minute.Microseconds(),
				},
				{
					SpanID:          "C",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "D",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "E",
					ParentID:        "B",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
			},
			OutputSpans: SpanList{
				{
					SpanID:          "A",
					ParentID:        "0",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "C",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "D",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
				{
					SpanID:          "E",
					ParentID:        "A",
					LogicDeleteTime: st + time.Minute.Microseconds(),
				},
			},
		},
		{
			InputSpans: SpanList{
				{
					SpanID:   "A",
					ParentID: "0",
				},
				{
					SpanID:   "C",
					ParentID: "A",
				},
				{
					SpanID:   "D",
					ParentID: "A",
				},
			},
			OutputSpans: SpanList{
				{
					SpanID:   "A",
					ParentID: "0",
				},
				{
					SpanID:   "C",
					ParentID: "A",
				},
				{
					SpanID:   "D",
					ParentID: "A",
				},
			},
		},
	}
	ctx := context.Background()
	for _, tc := range tests {
		var nilCfg SpanTransCfgList
		out, err := nilCfg.Transform(ctx, tc.InputSpans)
		assert.Nil(t, err)
		assert.Equal(t, tc.OutputSpans, out)
	}
}

func TestParentIdRedirect(t *testing.T) {
	spans := []*Span{
		{
			SpanID:   "B",
			ParentID: "A",
			// SpanType: "_delete",
		},
		{
			SpanID:   "C",
			ParentID: "B",
			// SpanType: "_delete",
		},
		{
			SpanID:   "A",
			ParentID: "0",
			SpanType: "_save",
			TagsLong: map[string]int64{
				"custom_a": 12,
				"custom_c": 12,
			},
		},
		{
			SpanID:   "D",
			ParentID: "C",
			SpanType: "_save",
			TagsDouble: map[string]float64{
				"custom_a": 12.1,
				"custom_c": 12.2,
			},
		},
	}
	spans2 := []*Span{
		{
			SpanID:   "D",
			ParentID: "C",
			SpanType: "_save",
			TagsDouble: map[string]float64{
				"custom_a": 12.1,
				"custom_c": 12.2,
			},
		},
		{
			SpanID:   "C",
			ParentID: "B",
			SpanType: "_delete",
		},
		{
			SpanID:   "B",
			ParentID: "A",
			SpanType: "_delete",
		},
		{
			SpanID:   "A",
			ParentID: "",
			SpanType: "_save",
			TagsLong: map[string]int64{
				"custom_a": 12,
				"custom_c": 12,
			},
		},
	}
	transCfg := SpanTransCfgList{
		&SpanTransConfig{
			SpanFilter: &FilterFields{
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldSpanType,
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumEq),
						Values:    []string{"_save"},
					},
				},
			},
			TagFilter: &TagFilter{
				KeyBlackList: []string{
					"custom_a",
					"custom_c",
				},
			},
		},
	}
	spans, err := transCfg.Transform(context.Background(), spans)
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range spans {
		switch span.SpanID {
		case "A":
			if span.ParentID != "0" {
				t.Fatalf("span.ParentID = %s, want 0", span.ParentID)
			}
			if len(span.TagsLong) > 0 {
				t.Fatalf("span.TagsLong = %v, want 0", span.TagsLong)
			}
		case "D":
			if span.ParentID != "A" {
				t.Fatalf("span.ParentID = %s, want A", span.ParentID)
			}
			if len(span.TagsDouble) > 0 {
				t.Fatalf("span.TagsDouble = %v, want 0", span.TagsDouble)
			}
		}
	}

	spans2, err = transCfg.Transform(context.Background(), spans2)
	if err != nil {
		t.Fatal(err)
	}
	for _, span := range spans2 {
		switch span.SpanID {
		case "A":
			if span.ParentID != "" {
				t.Fatalf("span.ParentID = %s, want ''", span.ParentID)
			}
			if len(span.TagsLong) > 0 {
				t.Fatalf("span.TagsLong = %v, want 0", span.TagsLong)
			}
		case "D":
			if span.ParentID != "A" {
				t.Fatalf("span.ParentID = %s, want A", span.ParentID)
			}
			if len(span.TagsDouble) > 0 {
				t.Fatalf("span.TagsDouble = %v, want 0", span.TagsDouble)
			}
		}
	}
}

func TestParentIdRedirectChaos(t *testing.T) {
	transCfg := SpanTransCfgList{
		&SpanTransConfig{
			SpanFilter: &FilterFields{
				FilterFields: []*FilterField{
					{
						FieldName: SpanFieldSpanType,
						FieldType: FieldTypeString,
						QueryType: ptr.Of(QueryTypeEnumIn),
						Values:    []string{"B", "C", "D", "E", "F", "I", "K", "L", "M", "O", "P", "R", "X", "Y"},
					},
				},
			},
		},
	}
	var outSpans []*Span
	for i := 0; i < 100; i++ {
		// 0->A->B->C->D........->Z
		spans := make([]*Span, 0)
		for i := 'A'; i < 'Z'; i++ {
			span := &Span{
				SpanID:   string(i),
				SpanType: string(i),
			}
			if i != 'A' {
				span.ParentID = string(i - 1)
			} else {
				span.ParentID = "0"
			}
			spans = append(spans, span)
		}
		rand.Shuffle(len(spans), func(i, j int) {
			spans[i], spans[j] = spans[j], spans[i]
		})
		out, err := transCfg.Transform(context.Background(), spans)
		if err != nil {
			t.Fatal(err)
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].SpanID < out[j].SpanID
		})
		for i := 0; i < len(out); i++ {
			if i == 0 {
				if out[i].ParentID != "0" {
					t.Fatalf("out[i].ParentID = %s, want 0", out[i].ParentID)
				}
			} else {
				if out[i].ParentID != out[i-1].SpanID {
					t.Fatalf("out[i].ParentID = %s, want %s", out[i].ParentID, out[i-1].SpanID)
				}
			}
		}
		if outSpans == nil {
			outSpans = out
			continue
		}
		// compare, should be the same everytime
		if len(outSpans) != len(out) {
			t.Fatal("len(outSpans) != len(out)")
		}
		for i := range outSpans {
			if outSpans[i].SpanID != out[i].SpanID {
				t.Fatal("outSpans[i].SpanID != out[i].SpanID")
			} else if outSpans[i].ParentID != out[i].ParentID {
				t.Fatal("outSpans[i].ParentID != out[i].ParentID")
			}
		}
	}
}
