// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
)

func TestDateValidator_CorrectDate(t *testing.T) {
	now := time.Now()
	todayEnd := EndTimeOfDay(now.UnixMilli())
	earliestTime := StartTimeOfDay(now.UnixMilli()) - int64(time.Duration(365*HoursPerDay)*time.Hour/time.Millisecond)

	tests := []struct {
		name          string
		validator     DateValidator
		wantStart     int64
		wantEnd       int64
		wantErr       bool
		expectedError int
	}{
		{
			name: "valid time range within limits",
			validator: DateValidator{
				Start:        now.Add(-1 * time.Hour).UnixMilli(),
				End:          now.UnixMilli(),
				EarliestDays: 365,
			},
			wantStart: now.Add(-1 * time.Hour).UnixMilli(),
			wantEnd:   now.UnixMilli(),
			wantErr:   false,
		},
		{
			name: "start time is zero",
			validator: DateValidator{
				Start:        0,
				End:          now.UnixMilli(),
				EarliestDays: 365,
			},
			wantStart:     0,
			wantEnd:       0,
			wantErr:       true,
			expectedError: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name: "end time is zero",
			validator: DateValidator{
				Start:        now.UnixMilli(),
				End:          0,
				EarliestDays: 365,
			},
			wantStart:     0,
			wantEnd:       0,
			wantErr:       true,
			expectedError: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name: "start time is negative",
			validator: DateValidator{
				Start:        -1,
				End:          now.UnixMilli(),
				EarliestDays: 365,
			},
			wantStart:     0,
			wantEnd:       0,
			wantErr:       true,
			expectedError: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name: "start time greater than end time",
			validator: DateValidator{
				Start:        now.UnixMilli(),
				End:          now.Add(-1 * time.Hour).UnixMilli(),
				EarliestDays: 365,
			},
			wantStart:     0,
			wantEnd:       0,
			wantErr:       true,
			expectedError: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name: "both start and end exceed today",
			validator: DateValidator{
				Start:        todayEnd + 1000,
				End:          todayEnd + 2000,
				EarliestDays: 365,
			},
			wantStart:     0,
			wantEnd:       0,
			wantErr:       true,
			expectedError: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name: "both start and end exceed max days ago",
			validator: DateValidator{
				Start:        earliestTime - 2000,
				End:          earliestTime - 1000,
				EarliestDays: 365,
			},
			wantStart:     0,
			wantEnd:       0,
			wantErr:       true,
			expectedError: obErrorx.CommercialCommonInvalidParamCodeCode,
		},
		{
			name: "start time before earliest, end time valid - should correct start time",
			validator: DateValidator{
				Start:        earliestTime - 1000,
				End:          now.UnixMilli(),
				EarliestDays: 365,
			},
			wantStart: earliestTime,
			wantEnd:   now.UnixMilli(),
			wantErr:   false,
		},
		{
			name: "end time exceeds today, start time valid - should correct end time",
			validator: DateValidator{
				Start:        now.Add(-1 * time.Hour).UnixMilli(),
				End:          todayEnd + 1000,
				EarliestDays: 365,
			},
			wantStart: now.Add(-1 * time.Hour).UnixMilli(),
			wantEnd:   todayEnd,
			wantErr:   false,
		},
		{
			name: "both start and end need correction",
			validator: DateValidator{
				Start:        earliestTime - 1000,
				End:          todayEnd + 1000,
				EarliestDays: 365,
			},
			wantStart: earliestTime,
			wantEnd:   todayEnd,
			wantErr:   false,
		},
		{
			name: "edge case - start equals earliest time",
			validator: DateValidator{
				Start:        earliestTime,
				End:          now.UnixMilli(),
				EarliestDays: 365,
			},
			wantStart: earliestTime,
			wantEnd:   now.UnixMilli(),
			wantErr:   false,
		},
		{
			name: "edge case - end equals latest time",
			validator: DateValidator{
				Start:        now.Add(-1 * time.Hour).UnixMilli(),
				End:          todayEnd,
				EarliestDays: 365,
			},
			wantStart: now.Add(-1 * time.Hour).UnixMilli(),
			wantEnd:   todayEnd,
			wantErr:   false,
		},
		{
			name: "different earliest days setting",
			validator: DateValidator{
				Start:        now.Add(-10 * 24 * time.Hour).UnixMilli(),
				End:          now.UnixMilli(),
				EarliestDays: 7, // only 7 days allowed
			},
			wantStart: StartTimeOfDay(now.UnixMilli()) - int64(time.Duration(7*HoursPerDay)*time.Hour/time.Millisecond),
			wantEnd:   now.UnixMilli(),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd, err := tt.validator.CorrectDate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.expectedError != 0 {
					// Check if error contains the expected error code
					assert.Contains(t, err.Error(), "600904002")
				}
				assert.Equal(t, tt.wantStart, gotStart)
				assert.Equal(t, tt.wantEnd, gotEnd)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantStart, gotStart)
				assert.Equal(t, tt.wantEnd, gotEnd)
			}
		})
	}
}

func TestStartTimeOfDay(t *testing.T) {
	// Test with a specific timestamp
	testTime := time.Date(2025, 1, 15, 14, 30, 45, 123456789, time.Local)
	expected := time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local)

	result := StartTimeOfDay(testTime.UnixMilli())
	assert.Equal(t, expected.UnixMilli(), result)
}

func TestEndTimeOfDay(t *testing.T) {
	// Test with a specific timestamp
	testTime := time.Date(2025, 1, 15, 14, 30, 45, 123456789, time.Local)
	expected := time.Date(2025, 1, 15, 23, 59, 59, 999999999, time.Local)

	result := EndTimeOfDay(testTime.UnixMilli())
	assert.Equal(t, expected.UnixMilli(), result)
}

func TestStartTimeOfDay_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "beginning of day",
			input:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local),
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local),
		},
		{
			name:     "end of day",
			input:    time.Date(2025, 1, 15, 23, 59, 59, 999999999, time.Local),
			expected: time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local),
		},
		{
			name:     "leap year date",
			input:    time.Date(2024, 2, 29, 12, 0, 0, 0, time.Local),
			expected: time.Date(2024, 2, 29, 0, 0, 0, 0, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StartTimeOfDay(tt.input.UnixMilli())
			assert.Equal(t, tt.expected.UnixMilli(), result)
		})
	}
}

func TestEndTimeOfDay_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "beginning of day",
			input:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local),
			expected: time.Date(2025, 1, 15, 23, 59, 59, 999999999, time.Local),
		},
		{
			name:     "end of day",
			input:    time.Date(2025, 1, 15, 23, 59, 59, 999999999, time.Local),
			expected: time.Date(2025, 1, 15, 23, 59, 59, 999999999, time.Local),
		},
		{
			name:     "leap year date",
			input:    time.Date(2024, 2, 29, 12, 0, 0, 0, time.Local),
			expected: time.Date(2024, 2, 29, 23, 59, 59, 999999999, time.Local),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EndTimeOfDay(tt.input.UnixMilli())
			assert.Equal(t, tt.expected.UnixMilli(), result)
		})
	}
}

func TestHoursPerDay_Constant(t *testing.T) {
	assert.Equal(t, 24, HoursPerDay)
}
