// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExptScheduler_IsEnabled(t *testing.T) {
	tests := []struct {
		name      string
		scheduler *ExptScheduler
		want      bool
	}{
		{
			name:      "nil scheduler should return false",
			scheduler: nil,
			want:      false,
		},
		{
			name: "enable status should return true",
			scheduler: &ExptScheduler{
				Status: SchedulerStatusEnable,
			},
			want: true,
		},
		{
			name: "disable status should return false",
			scheduler: &ExptScheduler{
				Status: SchedulerStatusDisable,
			},
			want: false,
		},
		{
			name: "empty status should return false",
			scheduler: &ExptScheduler{
				Status: "",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scheduler.IsEnabled()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExptScheduler_GetHour(t *testing.T) {
	tests := []struct {
		name      string
		scheduler *ExptScheduler
		want      int
	}{
		{
			name:      "nil scheduler should return 0",
			scheduler: nil,
			want:      0,
		},
		{
			name: "nil TriggerAt should return 0",
			scheduler: &ExptScheduler{
				TriggerAt: nil,
			},
			want: 0,
		},
		{
			name: "should return hour part",
			scheduler: &ExptScheduler{
				TriggerAt: timePtr(time.Date(2025, 1, 1, 14, 30, 0, 0, time.UTC)),
			},
			want: 14,
		},
		{
			name: "midnight should return 0",
			scheduler: &ExptScheduler{
				TriggerAt: timePtr(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
			want: 0,
		},
		{
			name: "last hour should return 23",
			scheduler: &ExptScheduler{
				TriggerAt: timePtr(time.Date(2025, 1, 1, 23, 59, 0, 0, time.UTC)),
			},
			want: 23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scheduler.GetHour()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExptScheduler_GetMinute(t *testing.T) {
	tests := []struct {
		name      string
		scheduler *ExptScheduler
		want      int
	}{
		{
			name:      "nil scheduler should return 0",
			scheduler: nil,
			want:      0,
		},
		{
			name: "nil TriggerAt should return 0",
			scheduler: &ExptScheduler{
				TriggerAt: nil,
			},
			want: 0,
		},
		{
			name: "should return minute part",
			scheduler: &ExptScheduler{
				TriggerAt: timePtr(time.Date(2025, 1, 1, 14, 30, 0, 0, time.UTC)),
			},
			want: 30,
		},
		{
			name: "zero minute should return 0",
			scheduler: &ExptScheduler{
				TriggerAt: timePtr(time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)),
			},
			want: 0,
		},
		{
			name: "last minute should return 59",
			scheduler: &ExptScheduler{
				TriggerAt: timePtr(time.Date(2025, 1, 1, 14, 59, 0, 0, time.UTC)),
			},
			want: 59,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scheduler.GetMinute()
			assert.Equal(t, tt.want, got)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
