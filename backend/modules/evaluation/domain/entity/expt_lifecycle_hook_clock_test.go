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

func TestHookClockWindowIncludesDBReadAndReturnLatency(t *testing.T) {
	start := time.Now()
	db := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	a := HookClockAnchor{DBTime: db, LocalBefore: start, LocalAfter: start.Add(40 * time.Millisecond), Precision: time.Millisecond}
	// A 200ms transaction return plus 100ms HTTP is not 100ms after the DB sample.
	got, err := a.Window(start.Add(340 * time.Millisecond))
	require.NoError(t, err)
	require.Equal(t, db.Add(299*time.Millisecond), got.Earliest)
	require.Equal(t, db.Add(341*time.Millisecond), got.Latest)
	require.False(t, got.Latest.Before(db.Add(330*time.Millisecond)))
}

func TestHookCompletionWindowCannotBeClampedOrInverted(t *testing.T) {
	now := time.Now().UTC()
	in := HookCompleteAttemptInput{HookRenewAttemptInput: HookRenewAttemptInput{HookAttemptScope: HookAttemptScope{Key: HookRunKey{1, 2, 3}, OperationID: "op", Phase: HookPhaseBefore, ExecutionScope: "local", SnapshotHash: strings.Repeat("a", 64)}, HookAttemptIdentity: HookAttemptIdentity{Token: HookClaimToken{Run: HookRunKey{1, 2, 3}, OperationID: "op", Phase: HookPhaseBefore, Attempt: 1, Generation: 1}, Owner: "owner", DeliveryID: "delivery"}}, Config: &HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}, Outcome: HookOutcome{Code: HookSucceeded}, CompletedAt: now, CompletionWindow: &HookCompletionWindow{Earliest: now.Add(-time.Millisecond), Latest: now}}
	require.NoError(t, in.Validate())
	for _, name := range []string{"clamped", "inverted", "zero lower"} {
		t.Run(name, func(t *testing.T) {
			b := in
			w := *in.CompletionWindow
			b.CompletionWindow = &w
			switch name {
			case "clamped":
				b.CompletedAt = now.Add(-time.Second)
			case "inverted":
				w.Earliest = now.Add(time.Second)
			case "zero lower":
				w.Earliest = time.Time{}
			}
			require.Error(t, b.Validate())
		})
	}
}

func TestHookClockRejectsMissingMonotonicOrInvalidInterval(t *testing.T) {
	now := time.Now()
	valid := HookClockAnchor{DBTime: now.Round(0), LocalBefore: now, LocalAfter: now.Add(time.Millisecond), Precision: time.Millisecond}
	for _, name := range []string{"wall sample", "wall completion", "before sample", "reversed", "zero precision", "zero DB"} {
		t.Run(name, func(t *testing.T) {
			a, end := valid, now.Add(time.Second)
			switch name {
			case "wall sample":
				a.LocalBefore = a.LocalBefore.Round(0)
			case "wall completion":
				end = end.Round(0)
			case "before sample":
				end = now
			case "reversed":
				a.LocalAfter = now.Add(-time.Second)
			case "zero precision":
				a.Precision = 0
			case "zero DB":
				a.DBTime = time.Time{}
			}
			_, err := a.Window(end)
			require.Error(t, err)
		})
	}
}
