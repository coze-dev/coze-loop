// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strconv"
	"testing"
	"time"
)

func TestGranularityToSecond(t *testing.T) {
	tests := []struct {
		name        string
		granularity MetricGranularity
		want        int64
	}{
		{"min", MetricGranularity1Min, 60},
		{"hour", MetricGranularity1Hour, 3600},
		{"day", MetricGranularity1Day, 86400},
		{"week", MetricGranularity1Week, 86400},
		{"default", MetricGranularity("unknown"), 86400},
	}

	for _, tt := range tests {
		if got := GranularityToSecond(tt.granularity); got != tt.want {
			t.Fatalf("%s: want %d, got %d", tt.name, tt.want, got)
		}
	}
}

func TestNewTimeIntervalsMinute(t *testing.T) {
	start := time.Date(2024, time.January, 2, 10, 3, 30, 0, time.UTC).UnixMilli()
	end := time.Date(2024, time.January, 2, 10, 5, 30, 0, time.UTC).UnixMilli()
	got := NewTimeIntervals(start, end, MetricGranularity1Min)
	if len(got) != 3 {
		t.Fatalf("unexpected length: want 3, got %d", len(got))
	}
	first := strconv.FormatInt(time.Date(2024, time.January, 2, 10, 3, 0, 0, time.UTC).UnixMilli(), 10)
	if got[0] != first {
		t.Fatalf("unexpected first element: want %s, got %s", first, got[0])
	}
	interval := GranularityToSecond(MetricGranularity1Min) * 1000
	for i := 1; i < len(got); i++ {
		prev, _ := strconv.ParseInt(got[i-1], 10, 64)
		curr, _ := strconv.ParseInt(got[i], 10, 64)
		if curr-prev != interval {
			t.Fatalf("index %d: want diff %d, got %d", i, interval, curr-prev)
		}
	}
}

func TestNewTimeIntervalsDay(t *testing.T) {
	start := time.Date(2023, time.May, 10, 15, 20, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2023, time.May, 12, 1, 0, 0, 0, time.UTC).UnixMilli()
	got := NewTimeIntervals(start, end, MetricGranularity1Day)
	if len(got) != 3 {
		t.Fatalf("unexpected length: want 3, got %d", len(got))
	}
	loc := time.UnixMilli(start).Location()
	first := strconv.FormatInt(time.Date(2023, time.May, 10, 0, 0, 0, 0, loc).UnixMilli(), 10)
	if got[0] != first {
		t.Fatalf("unexpected first element: want %s, got %s", first, got[0])
	}
	interval := GranularityToSecond(MetricGranularity1Day) * 1000
	for i := 1; i < len(got); i++ {
		prev, _ := strconv.ParseInt(got[i-1], 10, 64)
		curr, _ := strconv.ParseInt(got[i], 10, 64)
		if curr-prev != interval {
			t.Fatalf("index %d: want diff %d, got %d", i, interval, curr-prev)
		}
	}
}

func TestNewTimeIntervalsDefault(t *testing.T) {
	start := time.Date(2022, time.July, 1, 11, 45, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2022, time.July, 2, 9, 0, 0, 0, time.UTC).UnixMilli()
	got := NewTimeIntervals(start, end, MetricGranularity("unexpected"))
	if len(got) != 2 {
		t.Fatalf("unexpected length: want 2, got %d", len(got))
	}
	loc := time.UnixMilli(start).Location()
	first := strconv.FormatInt(time.Date(2022, time.July, 1, 0, 0, 0, 0, loc).UnixMilli(), 10)
	if got[0] != first {
		t.Fatalf("unexpected first element: want %s, got %s", first, got[0])
	}
	interval := GranularityToSecond(MetricGranularity("unexpected")) * 1000
	prev, _ := strconv.ParseInt(got[0], 10, 64)
	curr, _ := strconv.ParseInt(got[1], 10, 64)
	if curr-prev != interval {
		t.Fatalf("unexpected diff: want %d, got %d", interval, curr-prev)
	}
}

func TestMetricFillNull(t *testing.T) {
	if got := (&MetricFillNull{}).Interpolate(); got != "null" {
		t.Fatalf("unexpected interpolate result: %s", got)
	}
}
