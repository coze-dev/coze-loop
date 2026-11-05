// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

func TestExptFilterProjectionJSONSizeMatchesEncodingJSON(t *testing.T) {
	values := []string{"", `quote" backslash\ /`, "\b\f\n\r\t\x00\x1f", "<>&</script>", "中文🙂é\u2028\u2029", string([]byte{0xff, 0xfe, 'a', 0xc0, 0xaf}), "\ufffd", strings.Repeat("x<中\n", 1<<16)}
	allBytes := make([]byte, 256)
	for i := range allBytes {
		allBytes[i] = byte(i)
	}
	values = append(values, string(allBytes))
	for i, value := range values {
		score := math.Copysign(0, -1)
		row := &ExptTurnResultFilterEntity{SpaceID: 1, ExptID: 2, ItemID: 3, ItemVersionID: 4, ItemIdx: 5, TurnID: 0, Status: ItemRunState_Success, EvalSetID: 6, EvalSetVersionID: 7, EvaluatorWeightedScore: &score, CreatedDate: time.Date(2025, 1, 2, 3, 4, 5, 123456789, time.FixedZone("test", 8*60*60)), EvalTargetData: map[string]string{value: value, "actual_output": ""}, EvaluatorScore: map[string]float64{"tiny": 1e-9, "huge": 1e30, "negative": -0.125}, AnnotationFloat: map[string]float64{"zero": 0}, AnnotationBool: map[string]bool{"false": false, "true": true}, AnnotationString: map[string]string{value: value}, EvalTargetMetrics: map[string]int64{"min": math.MinInt64, "max": math.MaxInt64}}
		encoded, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		size, err := ExptFilterProjectionJSONSize(row)
		if err != nil {
			t.Fatal(err)
		}
		if size != int64(len(encoded)) {
			t.Fatalf("case %d: got %d want %d", i, size, len(encoded))
		}
		if row.EvalTargetData[value] != value {
			t.Fatal("counting mutated source maps")
		}
	}
	for _, row := range []*ExptTurnResultFilterEntity{{}, {EvalTargetData: map[string]string{}, EvaluatorScore: map[string]float64{}, AnnotationFloat: map[string]float64{}, AnnotationBool: map[string]bool{}, AnnotationString: map[string]string{}, EvalTargetMetrics: map[string]int64{}}} {
		encoded, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		size, err := ExptFilterProjectionJSONSize(row)
		if err != nil || size != int64(len(encoded)) {
			t.Fatalf("nil/empty maps: size=%d want=%d err=%v", size, len(encoded), err)
		}
	}
}
func TestExptFilterProjectionJSONSizeRejectsUnencodableScalars(t *testing.T) {
	if _, err := ExptFilterProjectionJSONSize(nil); err == nil {
		t.Fatal("nil projection accepted")
	}
	for _, row := range []*ExptTurnResultFilterEntity{{EvaluatorScore: map[string]float64{"nan": math.NaN()}}, {AnnotationFloat: map[string]float64{"inf": math.Inf(1)}}, {CreatedDate: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}} {
		if _, err := json.Marshal(row); err == nil {
			t.Fatal("invalid fixture")
		}
		if _, err := ExptFilterProjectionJSONSize(row); err == nil {
			t.Fatal("unencodable row accepted")
		}
	}
}
func TestExptFilterProjectionJSONSizeDoesNotAllocateLargeEncodedValues(t *testing.T) {
	row := &ExptTurnResultFilterEntity{EvalTargetData: map[string]string{"actual_output": strings.Repeat("<", 4<<20)}}
	benchmark := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := ExptFilterProjectionJSONSize(row); err != nil {
				b.Fatal(err)
			}
		}
	})
	if got := benchmark.AllocedBytesPerOp(); got > 64<<10 {
		t.Fatalf("size calculation allocated payload-sized buffers: %d bytes/op", got)
	}
}
