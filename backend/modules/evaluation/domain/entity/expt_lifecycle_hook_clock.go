// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// HookClockAnchor brackets the DB sample with process-local monotonic readings.
// It must never be serialized or reused in another process.
type HookClockAnchor struct {
	DBTime                  time.Time
	LocalBefore, LocalAfter time.Time
	Precision               time.Duration
}

type HookCompletionWindow struct {
	Earliest, Latest time.Time
}

func (a HookClockAnchor) Window(at time.Time) (HookCompletionWindow, error) {
	if a.DBTime.IsZero() || a.Precision <= 0 || !hookHasMonotonic(a.LocalBefore) || !hookHasMonotonic(a.LocalAfter) || !hookHasMonotonic(at) || a.LocalAfter.Before(a.LocalBefore) || at.Before(a.LocalAfter) {
		return HookCompletionWindow{}, ErrHookStoreCorrupt
	}
	return HookCompletionWindow{
		Earliest: a.DBTime.Add(at.Sub(a.LocalAfter) - a.Precision),
		Latest:   a.DBTime.Add(at.Sub(a.LocalBefore) + a.Precision),
	}, nil
}

// Round(0) strips only monotonic data; comparisons/subtractions above then use
// monotonic readings, never the host's wall clock. DB clock steps invalidate an anchor.
func hookHasMonotonic(t time.Time) bool { return t != t.Round(0) }
