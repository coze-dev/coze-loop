// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func TestHookOutcomePriority(t *testing.T) {
	for _, tc := range []struct {
		local     HookOutcomeCode
		http      int
		status    string
		retryable bool
		code      HookOutcomeCode
		wantRetry bool
	}{
		{HookSecurityError, 503, "failed", true, HookSecurityError, false},
		{HookTransportError, 0, "", false, HookTransportError, true},
		{HookTimeoutUncertain, 200, "succeeded", false, HookTimeoutUncertain, true},
		{"", 503, "failed", false, HookHTTPError, true},
		{HookResponseTooLarge, 429, "", false, HookHTTPError, true},
		{HookProtocolError, 408, "", false, HookHTTPError, true},
		{"", 403, "failed", true, HookHTTPError, false},
		{"", 409, "failed", true, HookHTTPError, false},
		{"", 301, "failed", true, HookHTTPRedirectError, false},
		{"", 202, "succeeded", false, HookResultNotFinal, false},
		{"", 204, "succeeded", false, HookProtocolError, false},
		{"", 0, "succeeded", false, HookProtocolError, false},
		{"", 600, "failed", true, HookProtocolError, false},
		{HookResponseTooLarge, 200, "succeeded", true, HookResponseTooLarge, false},
		{HookProtocolError, 200, "succeeded", true, HookProtocolError, false},
		{"", 200, "succeeded", true, HookSucceeded, false},
		{"", 200, "failed", true, HookFailed, true},
		{"", 200, "failed", false, HookFailed, false},
		{"", 200, "pending", true, HookProtocolError, false},
	} {
		got := ClassifyHookOutcome(tc.local, tc.http, tc.status, tc.retryable)
		require.Equal(t, tc.code, got.Code)
		require.Equal(t, tc.wantRetry, got.Retryable)
	}
	for status := 100; status < 600; status++ {
		got := ClassifyHookOutcome("", status, "", true)
		require.Equal(t, status == 408 || status == 429 || status >= 500, got.Retryable, "HTTP %d", status)
	}
}

func TestHookRetryDelay(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for i, seconds := range []int{5, 15, 30, 60, 60, 60, 60, 60, 60, 60, 60} {
		require.Equal(t, time.Duration(seconds)*time.Second, HookRetryDelay(int32(i+1), 0, "", now))
	}
	for _, tc := range []struct {
		header  string
		status  int
		seconds int
	}{
		{"40", 429, 40}, {"1", 408, 5}, {"100", 503, 60},
		{strings.Repeat("9", 1000), 500, 60}, {"00060", 599, 60},
		{"-1", 503, 5}, {"+20", 503, 5}, {"1.5", 503, 5}, {"bad", 503, 5},
		{"60", 200, 5}, {"60", 403, 5}, {"60", 202, 5},
		{"Fri, 02 Jan 2026 03:04:45 GMT", 429, 40},
		{"Fri, 02 Jan 2026 03:03:00 GMT", 429, 5},
		{"Fri, 02 Jan 2026 05:04:45 GMT", 429, 60},
	} {
		require.Equal(t, time.Duration(tc.seconds)*time.Second, HookRetryDelay(1, tc.status, tc.header, now), tc.header)
	}
}

func TestHookRetryBudgetAndOriginalDeadline(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	deadline, err := HookStageDeadline(start, 0, nil)
	require.NoError(t, err)
	require.Equal(t, start.Add(7*time.Minute), deadline)
	maxRetry := &HookRetryConf{MaxRetries: gptr.Of(int32(10))}
	maxDeadline, err := HookStageDeadline(start, 1200, maxRetry)
	require.NoError(t, err)
	require.Equal(t, start.Add(230*time.Minute), maxDeadline)
	for _, tc := range []struct {
		timeout int32
		retry   *HookRetryConf
	}{
		{-1, nil}, {1201, nil}, {180, &HookRetryConf{MaxRetries: gptr.Of(int32(11))}},
		{180, &HookRetryConf{Enabled: gptr.Of(false), MaxRetries: gptr.Of(int32(0))}},
	} {
		_, err := HookStageDeadline(start, tc.timeout, tc.retry)
		require.Error(t, err)
	}
	for attempt := int32(1); attempt <= 11; attempt++ {
		decision, err := DecideHookRetry(HookRetryInput{
			Outcome: ClassifyHookOutcome("", 503, "failed", false), Retry: maxRetry,
			Attempt: attempt, TimeoutSeconds: 1200, Now: start, Deadline: maxDeadline,
		})
		require.NoError(t, err)
		require.Equal(t, attempt < 11, decision.Retry)
		if decision.Retry {
			require.Equal(t, attempt+1, decision.NextAttempt)
		}
	}
	in := HookRetryInput{Outcome: ClassifyHookOutcome("", 503, "", false), Attempt: 1, Now: start, Deadline: deadline}
	decision, err := DecideHookRetry(in)
	require.NoError(t, err)
	require.True(t, decision.Retry)
	in.Now = deadline.Add(-185 * time.Second)
	decision, err = DecideHookRetry(in)
	require.NoError(t, err)
	require.True(t, decision.Retry)
	in.Now = in.Now.Add(time.Nanosecond)
	decision, err = DecideHookRetry(in)
	require.NoError(t, err)
	require.False(t, decision.Retry)
	require.Equal(t, start.Add(7*time.Minute), in.Deadline)
	in.Now = start
	in.Retry = &HookRetryConf{Enabled: gptr.Of(false)}
	decision, err = DecideHookRetry(in)
	require.NoError(t, err)
	require.False(t, decision.Retry)
	in.Retry = nil
	in.Outcome = HookOutcome{Code: HookSucceeded, Retryable: true}
	decision, err = DecideHookRetry(in)
	require.NoError(t, err)
	require.False(t, decision.Retry)
	for _, attempt := range []int32{0, 12} {
		in.Attempt = attempt
		_, err := DecideHookRetry(in)
		require.Error(t, err)
	}
}
