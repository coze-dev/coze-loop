// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/stretchr/testify/require"
)

func onlinePreparationFixture(t *testing.T) *preparerFixture {
	t.Helper()
	f := newPreparerFixture(t, entity.EvaluationModeAppend)
	f.s.expt.ExptType = entity.ExptType_Online
	r := f.s.runs[30]
	snapshot, err := f.deps.Codec.DecodeSnapshot(context.Background(), r.State.Key, "local", r.Snapshot)
	require.NoError(t, err)
	in := snapshot.Input()
	in.Context.Experiment.Type = gptr.Of("online")
	snapshot, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	r.Snapshot, err = f.deps.Codec.EncodeSnapshot(context.Background(), "key", snapshot)
	require.NoError(t, err)
	return f
}

func TestHookPlanPreparerReviewDrainingMakesProgress(t *testing.T) {
	for _, where := range []string{"pending", "experiment", "run", "log", "all"} {
		t.Run(where, func(t *testing.T) {
			f := onlinePreparationFixture(t)
			if where == "experiment" || where == "all" {
				f.s.expt.Status = entity.ExptStatus_Draining
			}
			if where == "run" || where == "all" {
				f.s.runs[30].State.Status = entity.ExptStatus_Draining
			}
			if where == "log" || where == "all" {
				f.s.logs[30].Status = int64(entity.ExptStatus_Draining)
			}
			for i := 0; i < 3; i++ {
				require.NoError(t, f.step(t))
			}
			require.True(t, f.s.runs[30].PlanReady)
			require.True(t, f.s.runs[30].State.Before.Activated)
			require.Equal(t, entity.HookFinalizeNone, f.s.runs[30].State.Finalize)
			require.False(t, f.s.runs[30].State.After.Activated)
		})
	}
}

func TestHookPlanPreparerReviewSlowSourcePersistsBudget(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	f.selector.selectFn = func(ctx context.Context, _ entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		<-ctx.Done()
		return entity.HookSelectionPage{}, ctx.Err()
	}
	for i := 0; i < 11; i++ {
		p, err := NewHookPlanPreparer(f.deps)
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		err = p.PreparePlan(ctx, f.in)
		parentErr := ctx.Err()
		cancel()
		if i < 10 {
			require.ErrorIs(t, err, ErrHookPlanSourceRetry)
		} else {
			require.ErrorIs(t, err, ErrHookPlanPreparationFailed)
		}
		require.NoError(t, parentErr, "source child budget must leave the parent alive for CAS")
	}
	require.Equal(t, 11, f.selector.calls)
	require.Equal(t, entity.HookFinalizePending, f.s.runs[30].State.Finalize)
	require.Equal(t, "HOOK_PLAN_SOURCE_EXHAUSTED", f.s.runs[30].State.Intent.Reason)
	require.Len(t, f.s.effects, 11)
}

// The old Get reader deliberately returns a lagging replica; the new port reads current primary state.
type reviewPrimaryInitialization struct {
	repo.IHookRunInitializationRepo
	repo.IExptRunLogRepo
	s                          *preparerStore
	stale                      entity.ExptRunLog
	primaryReads, replicaReads int
}

func (r *reviewPrimaryInitialization) Get(context.Context, int64, int64) (*entity.ExptRunLog, error) {
	r.replicaReads++
	out := r.stale
	return &out, nil
}
func (r *reviewPrimaryInitialization) ReadRunInitialization(_ context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	r.primaryReads++
	out := *r.s.logs[key.RunID]
	out.ItemIds = append([]entity.ExptRunLogItems(nil), out.ItemIds...)
	return &entity.HookRunInitialization{RunLog: &out, Managed: true, LatestRunID: r.s.expt.LatestRunID}, nil
}

func TestHookPlanPreparerReviewPrimaryBatchCannotBeLost(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeRetryItems)
	f.appendBatch(1)
	f.selector.selectFn = func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		return entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(in.ItemIDs[0])}, Done: true, NextCursor: "batch-done"}, nil
	}
	require.NoError(t, f.step(t))
	reader := &reviewPrimaryInitialization{s: f.s, stale: *f.s.logs[30]}
	reader.stale.ItemIds = append([]entity.ExptRunLogItems(nil), reader.stale.ItemIds...)
	f.appendBatch(2)
	f.deps.Initialization = reader
	for i := 0; i < 4; i++ {
		require.NoError(t, f.step(t))
	}
	require.True(t, f.s.runs[30].PlanReady)
	require.Equal(t, []int64{1, 2}, persistedItemIDs(f.s.rows[30]))
	require.Positive(t, reader.primaryReads)
	require.Zero(t, reader.replicaReads)
}

func TestHookPlanPreparerReviewR07MustNotUseNilStatusScanner(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
	f.source(t, 1)
	// Nil provider is safe here: the real DAO rejects empty status before any DB access.
	legacy := experimentrepo.NewExptItemResultRepo(experimentmysql.NewExptItemResultDAO(nil))
	_, _, err := legacy.ScanItemResults(context.Background(), 20, 0, 1, nil, 10)
	require.ErrorContains(t, err, "null status")
	f.deps.ResultReader = f.results
	require.NoError(t, f.step(t))
	require.Len(t, f.s.rows[30], 1)
}

func TestHookPlanPreparerReviewLateSourceResults(t *testing.T) {
	for _, kind := range []string{"success", "invalid_source", "storage_error"} {
		t.Run(kind, func(t *testing.T) {
			f := newPreparerFixture(t, entity.EvaluationModeSubmit)
			f.selector.selectFn = func(ctx context.Context, _ entity.HookSelectionInput) (entity.HookSelectionPage, error) {
				<-ctx.Done()
				if kind == "invalid_source" {
					return entity.HookSelectionPage{}, errSelectionSource
				}
				if kind == "storage_error" {
					return entity.HookSelectionPage{}, entity.ErrHookPlanStorage
				}
				return entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "late-done", Done: true}, nil
			}
			p, err := NewHookPlanPreparer(f.deps)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			require.ErrorIs(t, p.PreparePlan(ctx, f.in), ErrHookPlanSourceRetry)
			require.Empty(t, f.s.rows[30])
			require.Equal(t, []string{"advance"}, f.s.effects)
			require.Equal(t, entity.HookFinalizeNone, f.s.runs[30].State.Finalize)
		})
	}
}

func TestHookPlanPreparerReviewSourceCapAndTrueCancellation(t *testing.T) {
	t.Run("three_second_cap", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.selectFn = func(ctx context.Context, _ entity.HookSelectionInput) (entity.HookSelectionPage, error) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok)
			remaining := time.Until(deadline)
			require.Positive(t, remaining)
			require.LessOrEqual(t, remaining, 3*time.Second)
			return entity.HookSelectionPage{NextCursor: "done", Done: true}, nil
		}
		require.NoError(t, f.step(t))
	})
	t.Run("parent_cancel", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f.selector.selectFn = func(child context.Context, _ entity.HookSelectionInput) (entity.HookSelectionPage, error) {
			cancel()
			<-child.Done()
			return entity.HookSelectionPage{}, child.Err()
		}
		p, err := NewHookPlanPreparer(f.deps)
		require.NoError(t, err)
		require.ErrorIs(t, p.PreparePlan(ctx, f.in), context.Canceled)
		require.Empty(t, f.s.effects)
	})
}

type reviewInitializationValue struct {
	repo.IHookRunInitializationRepo
	value *entity.HookRunInitialization
	err   error
}

func (r reviewInitializationValue) ReadRunInitialization(context.Context, entity.HookRunKey) (*entity.HookRunInitialization, error) {
	return r.value, r.err
}

func TestHookPlanPreparerReviewInitializationChecks(t *testing.T) {
	for _, kind := range []string{"nil", "unmanaged", "latest", "wrong_log", "error"} {
		t.Run(kind, func(t *testing.T) {
			f := newPreparerFixture(t, entity.EvaluationModeSubmit)
			log := *f.s.logs[30]
			reader := reviewInitializationValue{value: &entity.HookRunInitialization{RunLog: &log, Managed: true, LatestRunID: 30}}
			switch kind {
			case "nil":
				reader.value = nil
			case "unmanaged":
				reader.value.Managed = false
			case "latest":
				reader.value.LatestRunID = 29
			case "wrong_log":
				log.ExptRunID = 29
			case "error":
				reader.err = errors.New("primary secret")
			}
			f.deps.Initialization = reader
			err := f.step(t)
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
			require.Empty(t, f.s.effects)
			require.Zero(t, f.selector.calls)
		})
	}
}

func TestHookPlanPreparerReviewResultReaderErrorAndExistingSuccess(t *testing.T) {
	t.Run("real_reader_unavailable", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
		f.source(t, 1)
		f.deps.ResultReader = experimentrepo.NewHookPlanResultReader(nil)
		require.ErrorIs(t, f.step(t), entity.ErrHookPlanStorage)
		require.Empty(t, f.s.effects)
		require.Empty(t, f.s.rows[30])
		require.False(t, f.s.runs[30].PlanReady)
	})
	for _, kind := range []string{"item", "turn"} {
		t.Run(kind, func(t *testing.T) {
			f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
			f.source(t, 1)
			f.results.items = kind == "item"
			f.results.turns = kind == "turn"
			f.selector.pages[""] = entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(7)}, NextCursor: "failed-only", Done: true}
			f.complete(t)
			require.Equal(t, []int64{7}, persistedItemIDs(f.s.rows[30]))
			require.Positive(t, f.results.calls)
		})
	}
}

func TestHookPlanPreparerReviewTerminatingStillStops(t *testing.T) {
	for _, where := range []string{"experiment", "log"} {
		t.Run(where, func(t *testing.T) {
			f := onlinePreparationFixture(t)
			if where == "experiment" {
				f.s.expt.Status = entity.ExptStatus_Terminating
			} else {
				f.s.logs[30].Status = int64(entity.ExptStatus_Terminating)
			}
			require.NoError(t, f.step(t))
			require.False(t, f.s.runs[30].PlanReady)
			require.Empty(t, f.s.effects)
			require.Zero(t, f.selector.calls)
		})
	}
}

func TestHookPlanPreparerReviewRejectsNilNewPorts(t *testing.T) {
	for _, kind := range []string{"initialization", "result_reader"} {
		t.Run(kind, func(t *testing.T) {
			f := newPreparerFixture(t, entity.EvaluationModeSubmit)
			if kind == "initialization" {
				f.deps.Initialization = (*reviewPrimaryInitialization)(nil)
			} else {
				f.deps.ResultReader = (*preparerResults)(nil)
			}
			_, err := NewHookPlanPreparer(f.deps)
			require.Error(t, err)
		})
	}
}
