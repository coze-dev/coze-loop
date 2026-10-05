// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// The witness validates the original DB/monotonic calibration, not HTTP duration.
// Compatible samples assume one unchanged clock offset between their brackets;
// discontinuities hidden within sampling uncertainty cannot be distinguished.
func (e *HookAttemptExecutor) observeCompletion(ctx context.Context, scope entity.HookAttemptScope, anchor entity.HookClockAnchor, local, deadline time.Time) (entity.HookCompletionWindow, entity.HookClockAnchor, error) {
	var empty entity.HookCompletionWindow
	w, err := anchor.Window(local)
	if err != nil || local.After(time.Now()) || !w.Latest.Before(deadline) {
		return empty, anchor, ErrHookExecutionUnavailable
	}
	witness, err := e.reader.ReadAttempt(ctx, scope)
	if err != nil || witness.Clock == nil {
		return empty, anchor, ErrHookExecutionUnavailable
	}
	clock := *witness.Clock
	if _, err := clock.Window(time.Now()); err != nil || clock.LocalBefore.Before(local) {
		return empty, anchor, ErrHookExecutionUnavailable
	}
	predictedStart, err := anchor.Window(clock.LocalBefore)
	if err != nil {
		return empty, anchor, ErrHookExecutionUnavailable
	}
	predictedEnd, err := anchor.Window(clock.LocalAfter)
	if err != nil || clock.DBTime.Add(clock.Precision).Before(predictedStart.Earliest) || clock.DBTime.Add(-clock.Precision).After(predictedEnd.Latest) {
		return empty, anchor, ErrHookExecutionUnavailable
	}
	// Keep the HTTP completion interval unchanged; queue/lock/commit latency
	// advances the witness's local and DB readings together, not CompletedAt.
	w.Latest = w.Latest.Add(time.Millisecond - time.Nanosecond).Truncate(time.Millisecond)
	if !w.Latest.Before(deadline) {
		return empty, clock, ErrHookExecutionUnavailable
	}
	return w, clock, nil
}
