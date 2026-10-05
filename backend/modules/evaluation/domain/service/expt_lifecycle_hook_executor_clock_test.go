// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type executorClockReader func(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error)

func (f executorClockReader) ReadAttempt(ctx context.Context, in entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
	return f(ctx, in)
}

func TestHookExecutorClockCalibrationKeepsHTTPWindow(t *testing.T) {
	start := time.Now().Add(-10 * time.Second)
	db := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	anchor := entity.HookClockAnchor{DBTime: db, LocalBefore: start, LocalAfter: start.Add(time.Millisecond), Precision: time.Millisecond}
	for _, delay := range []time.Duration{100 * time.Millisecond, 1120 * time.Millisecond, 4 * time.Second} {
		t.Run(delay.String(), func(t *testing.T) {
			witness := entity.HookClockAnchor{DBTime: db.Add(delay), LocalBefore: start.Add(delay), LocalAfter: start.Add(delay + time.Millisecond), Precision: time.Millisecond}
			e := &HookAttemptExecutor{reader: executorClockReader(func(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
				return entity.HookAttemptStoreResult{Clock: &witness}, nil
			})}
			window, _, err := e.observeCompletion(context.Background(), entity.HookAttemptScope{}, anchor, start.Add(20*time.Millisecond), db.Add(time.Second))
			require.NoError(t, err)
			// Hand-derived from the first 1ms sample bracket and 1ms precision.
			require.Equal(t, db.Add(18*time.Millisecond), window.Earliest)
			require.Equal(t, db.Add(21*time.Millisecond), window.Latest)
		})
	}
}

func TestHookExecutorClockCalibrationRejectsDiscontinuity(t *testing.T) {
	start := time.Now().Add(-3 * time.Second)
	db := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	anchor := entity.HookClockAnchor{DBTime: db, LocalBefore: start, LocalAfter: start.Add(time.Millisecond), Precision: time.Millisecond}
	for _, delta := range []time.Duration{2 * time.Second, -50 * time.Millisecond, -time.Second} {
		t.Run(delta.String(), func(t *testing.T) {
			witness := entity.HookClockAnchor{DBTime: db.Add(200*time.Millisecond + delta), LocalBefore: start.Add(200 * time.Millisecond), LocalAfter: start.Add(201 * time.Millisecond), Precision: time.Millisecond}
			e := &HookAttemptExecutor{reader: executorClockReader(func(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
				return entity.HookAttemptStoreResult{Clock: &witness}, nil
			})}
			_, _, err := e.observeCompletion(context.Background(), entity.HookAttemptScope{}, anchor, start.Add(20*time.Millisecond), db.Add(time.Second))
			require.Error(t, err)
		})
	}
}

func TestHookExecutorClockCalibrationDoesNotClampAmbiguousDeadline(t *testing.T) {
	start := time.Now().Add(-3 * time.Second)
	db := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	anchor := entity.HookClockAnchor{DBTime: db, LocalBefore: start, LocalAfter: start.Add(time.Millisecond), Precision: time.Millisecond}
	witness := entity.HookClockAnchor{DBTime: db.Add(time.Second), LocalBefore: start.Add(time.Second), LocalAfter: start.Add(time.Second + time.Millisecond), Precision: time.Millisecond}
	e := &HookAttemptExecutor{reader: executorClockReader(func(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
		return entity.HookAttemptStoreResult{Clock: &witness}, nil
	})}
	w, _, err := e.observeCompletion(context.Background(), entity.HookAttemptScope{}, anchor, start.Add(20*time.Millisecond+time.Nanosecond), db.Add(23*time.Millisecond))
	require.NoError(t, err)
	require.Equal(t, db.Add(22*time.Millisecond), w.Latest)
	_, _, err = e.observeCompletion(context.Background(), entity.HookAttemptScope{}, anchor, start.Add(20*time.Millisecond+time.Nanosecond), db.Add(22*time.Millisecond))
	require.Error(t, err)
}

func TestHookExecutorClockCalibrationValidatesSampleBracket(t *testing.T) {
	start := time.Now().Add(-3 * time.Second)
	db := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	anchor := entity.HookClockAnchor{DBTime: db, LocalBefore: start, LocalAfter: start.Add(time.Millisecond), Precision: time.Millisecond}
	for _, name := range []string{"query_duration", "missing_sample", "wall_only", "zero_precision", "before_completion", "reversed"} {
		t.Run(name, func(t *testing.T) {
			clock := &entity.HookClockAnchor{DBTime: db.Add(1150 * time.Millisecond), LocalBefore: start.Add(1120 * time.Millisecond), LocalAfter: start.Add(1180 * time.Millisecond), Precision: time.Millisecond}
			switch name {
			case "missing_sample":
				clock = nil
			case "wall_only":
				clock.LocalBefore = clock.LocalBefore.Round(0)
			case "zero_precision":
				clock.Precision = 0
			case "before_completion":
				clock.LocalBefore = start.Add(10 * time.Millisecond)
			case "reversed":
				clock.LocalAfter = clock.LocalBefore.Add(-time.Millisecond)
			}
			e := &HookAttemptExecutor{reader: executorClockReader(func(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
				return entity.HookAttemptStoreResult{Clock: clock}, nil
			})}
			w, _, err := e.observeCompletion(context.Background(), entity.HookAttemptScope{}, anchor, start.Add(20*time.Millisecond), db.Add(time.Second))
			if name != "query_duration" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, db.Add(18*time.Millisecond), w.Earliest)
			require.Equal(t, db.Add(21*time.Millisecond), w.Latest)
		})
	}
}
