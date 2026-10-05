// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"strings"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
)

type coordinatorRuns struct {
	repo.IHookRepo
	run  *entity.HookStoredRun
	read func(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error)
}

func (r *coordinatorRuns) GetRun(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
	if r.read != nil {
		return r.read(ctx, key)
	}
	copy := *r.run
	return &copy, nil
}

type coordinatorPreparer func(context.Context, hook.WorkerRunInput) error

func (f coordinatorPreparer) PreparePlan(ctx context.Context, in hook.WorkerRunInput) error {
	return f(ctx, in)
}

type coordinatorFinalizer func(context.Context, entity.HookRunKey, entity.HookTerminalIntent) error

func (f coordinatorFinalizer) FinalizeRun(ctx context.Context, key entity.HookRunKey, intent entity.HookTerminalIntent) error {
	return f(ctx, key, intent)
}

type coordinatorWake func(context.Context, entity.HookWakeEvent) error

func (f coordinatorWake) PublishWake(ctx context.Context, event entity.HookWakeEvent) error {
	return f(ctx, event)
}

type coordinatorGate func(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error)

func (f coordinatorGate) CanDispatch(ctx context.Context, key entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	return f(ctx, key)
}

type coordinatorCodec struct {
	hook.StorageCodec
	decode func(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error)
}

func (c coordinatorCodec) DecodeSnapshot(ctx context.Context, key entity.HookRunKey, scope string, p entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
	if c.decode != nil {
		return c.decode(ctx, key, scope, p)
	}
	return &entity.HookRunSnapshot{}, nil
}

type coordinatorSchedulePublisher func(context.Context, *entity.ExptScheduleEvent) error

func (f coordinatorSchedulePublisher) PublishHookSchedule(ctx context.Context, event *entity.ExptScheduleEvent) error {
	return f(ctx, event)
}

type coordinatorRetryItemsPublisher struct {
	coordinatorSchedulePublisher
	tail, wakes int
}

func (p *coordinatorRetryItemsPublisher) PrepareRetryItemsTail(context.Context, entity.HookRunKey) (bool, error) {
	p.tail++
	return true, nil
}
func (p *coordinatorRetryItemsPublisher) PublishRetryItemsContinuation(context.Context, entity.HookRunKey) error {
	p.wakes++
	return nil
}

func TestHookWorkerRetryItemsProcessesActiveTail(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.runs.run.Mode = entity.EvaluationModeRetryItems
	f.runs.run.PlanReady = true
	f.runs.run.ExecutionStarted = true
	f.runs.run.State.Gate = entity.HookGateReady
	f.runs.run.State.Before.Status = entity.HookOperationSucceeded
	p := &coordinatorRetryItemsPublisher{}
	f.deps.SchedulePublisher = p
	require.NoError(t, f.worker(t).PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, p.tail)
	require.Equal(t, 1, p.wakes)
	require.Zero(t, f.prepared)
}

func TestHookWorkerRetryItemsPendingUsesStartupQuota(t *testing.T) {
	f := newCoordinatorFixture(t)
	in := coordinatorScheduleSeed(t, f).Input()
	in.Context.RunMode = gptr.Of("retry_items")
	in.Schedule.Mode = entity.EvaluationModeRetryItems
	snapshot, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	f.runs.run.Mode = entity.EvaluationModeRetryItems
	f.deps.Codec = coordinatorCodec{decode: func(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
		return snapshot, nil
	}}
	denied := errors.New("startup quota full")
	quotaErr := denied
	quotaCalls := 0
	var sent []*entity.ExptScheduleEvent
	p := &coordinatorRetryItemsPublisher{coordinatorSchedulePublisher: func(_ context.Context, event *entity.ExptScheduleEvent) error {
		quotaCalls++
		if quotaErr != nil {
			return quotaErr
		}
		sent = append(sent, event)
		return nil
	}}
	f.deps.SchedulePublisher = p
	c := f.worker(t)
	require.ErrorIs(t, c.PreparePlan(context.Background(), f.input()), denied)
	require.Equal(t, 1, quotaCalls)
	require.Empty(t, sent)
	require.Zero(t, p.tail)
	require.Zero(t, p.wakes)
	quotaErr = nil
	require.NoError(t, c.PreparePlan(context.Background(), f.input()))
	require.Equal(t, 2, quotaCalls)
	require.Len(t, sent, 1)
	require.Equal(t, int64(1700000001), sent[0].CreatedAt)
	require.Equal(t, "original-user", sent[0].Session.UserID)
	require.Equal(t, int32(8), sent[0].Session.AppID)
	require.Equal(t, "original", sent[0].Ext["route"])
	require.Zero(t, f.prepared)
	require.Zero(t, f.finalized)
	require.Zero(t, p.wakes)
}

func TestHookWorkerRetryItemsInitializedEmptyPrefixRecoversTail(t *testing.T) {
	f := newCoordinatorFixture(t)
	in := coordinatorScheduleSeed(t, f).Input()
	in.Context.RunMode = gptr.Of("retry_items")
	in.Schedule.Mode = entity.EvaluationModeRetryItems
	snapshot, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	f.deps.Codec = coordinatorCodec{decode: func(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
		return snapshot, nil
	}}
	run := f.runs.run
	run.Mode = entity.EvaluationModeRetryItems
	run.State.Status = entity.ExptStatus_Processing
	run.ExecutionStarted = false
	// Initialization committed an empty prefix; an accepted batch is still unmaterialized.
	digest := entity.NewHookPlanDigest()
	run.PlanCount, run.PlanHash = digest.Count, digest.Hash
	run.PlanCursor, err = (entity.HookRetryItemsCursor{Version: 2, Key: run.State.Key, Fingerprint: strings.Repeat("b", 64), Phase: "tail", Published: digest}).Encode()
	require.NoError(t, err)
	accepted := []entity.ExptRunLogItems{{ItemIDs: []int64{102}}}
	cursor, err := entity.DecodeHookRetryItemsCursor(run.PlanCursor, run.State.Key, accepted, run.PlanCount, run.PlanHash)
	require.NoError(t, err)
	items, pending := cursor.Page(accepted)
	require.True(t, pending)
	require.Equal(t, []int64{102}, items)
	p := &coordinatorRetryItemsPublisher{coordinatorSchedulePublisher: func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("committed initialization must not reenter startup quota")
		return nil
	}}
	f.deps.SchedulePublisher = p
	require.NoError(t, f.worker(t).PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, p.tail, "recover the accepted tail even before the first AdmitItem")
	require.Equal(t, 1, p.wakes, "replace the lost scheduler event")
	require.Zero(t, f.prepared)
	require.Zero(t, f.finalized)
	require.False(t, run.ExecutionStarted)
}

type coordinatorFixture struct {
	deps                HookWorkerCoordinatorDependencies
	runs                *coordinatorRuns
	prepared, finalized int
	wakes               []entity.HookWakeEvent
}

func newCoordinatorFixture(t *testing.T) *coordinatorFixture {
	t.Helper()
	f := &coordinatorFixture{runs: &coordinatorRuns{run: &entity.HookStoredRun{
		State: entity.HookRunState{Key: entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33},
			Status: entity.ExptStatus_Processing, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone,
			Before: entity.HookOperation{ID: "before-original", Status: entity.HookOperationPending},
			After:  entity.HookOperation{ID: "after-original", Status: entity.HookOperationPending}},
		Snapshot: entity.HookProtectedSnapshot{ExecutionScope: "ppe_original"}, Version: 7,
	}}}
	f.deps = HookWorkerCoordinatorDependencies{
		ExecutionScope: "ppe_original", Runs: f.runs,
		Codec: coordinatorCodec{}, SchedulePublisher: coordinatorSchedulePublisher(func(context.Context, *entity.ExptScheduleEvent) error {
			t.Fatal("legacy snapshot must not publish")
			return nil
		}),
		Preparer: coordinatorPreparer(func(ctx context.Context, in hook.WorkerRunInput) error {
			require.Equal(t, entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}, in.Candidate.Key)
			require.Equal(t, "ppe_original", in.ExecutionScope)
			f.prepared++
			return nil
		}),
		Manager: coordinatorFinalizer(func(ctx context.Context, key entity.HookRunKey, intent entity.HookTerminalIntent) error {
			require.Equal(t, entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}, key)
			require.Equal(t, entity.HookTerminalIntent{Status: entity.ExptStatus_SystemTerminated, Reason: "ORIGINAL_REASON"}, intent)
			f.finalized++
			f.runs.run.State.Finalize = entity.HookFinalizeCommitted
			f.runs.run.State.Status = intent.Status
			f.runs.run.State.After.Activated = true
			return nil
		}),
		HookWake: coordinatorWake(func(ctx context.Context, event entity.HookWakeEvent) error {
			f.wakes = append(f.wakes, event)
			return nil
		}),
		Gate: coordinatorGate(func(ctx context.Context, key entity.HookRunKey) (entity.HookAdmissionDecision, error) {
			require.Equal(t, entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}, key)
			return entity.HookAdmissionDecision{Gate: entity.HookGateReady}, nil
		}),
	}
	return f
}

func (f *coordinatorFixture) worker(t *testing.T) *HookWorkerCoordinator {
	t.Helper()
	c, err := NewHookWorkerCoordinator(f.deps)
	require.NoError(t, err)
	return c
}

func (f *coordinatorFixture) input() hook.WorkerRunInput {
	return hook.WorkerRunInput{ExecutionScope: "ppe_original", Candidate: entity.HookRunCandidate{
		Key: entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}, Version: 1}}
}

func (f *coordinatorFixture) pending() {
	f.runs.run.State.Gate = entity.HookGateClosed
	f.runs.run.State.Before.Status = entity.HookOperationFailed
	f.runs.run.State.Finalize = entity.HookFinalizePending
	f.runs.run.State.Intent = entity.HookTerminalIntent{Status: entity.ExptStatus_SystemTerminated, Reason: "ORIGINAL_REASON"}
}

func (f *coordinatorFixture) ready() {
	f.runs.run.PlanReady = true
	f.runs.run.State.Gate = entity.HookGateReady
	f.runs.run.State.Before.Status = entity.HookOperationSucceeded
}

func (f *coordinatorFixture) effects() hook.WorkerEffects {
	return hook.WorkerEffects{ExecutionScope: "ppe_original", Key: f.input().Candidate.Key, OperationID: "before-original"}
}

func TestHookWorkerCoordinatorPreparesExactlyOnePage(t *testing.T) {
	f := newCoordinatorFixture(t)
	c := f.worker(t)
	require.NoError(t, c.PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, f.prepared)
	require.Empty(t, f.wakes)
	require.Zero(t, f.finalized)
}

func TestHookWorkerCoordinatorFinalizeRestoresPersistedIntentAndAfterWake(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.pending()
	c := f.worker(t)
	for range 2 {
		require.NoError(t, c.FinalizeRun(context.Background(), f.input()))
	}
	require.Equal(t, 2, f.finalized)
	require.Equal(t, []entity.HookWakeEvent{
		{Run: entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}, OperationID: "after-original", ExecutionScope: "ppe_original"},
		{Run: entity.HookRunKey{WorkspaceID: 11, ExperimentID: 22, RunID: 33}, OperationID: "after-original", ExecutionScope: "ppe_original"},
	}, f.wakes)
}

func TestHookWorkerCoordinatorCannotInventScheduleContinuation(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.ready()
	effect := f.effects()
	effect.WakeEvaluation = true
	require.ErrorIs(t, f.worker(t).ApplyEffects(context.Background(), effect), ErrHookWorkerScheduleRecoveryUnavailable)
	require.Empty(t, f.wakes)
	require.Zero(t, f.finalized)
}

func TestHookWorkerCoordinatorPreparationWakesCommittedBefore(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.deps.Preparer = coordinatorPreparer(func(context.Context, hook.WorkerRunInput) error {
		f.prepared++
		f.runs.run.PlanReady = true
		f.runs.run.State.Before.Activated = true
		return nil
	})
	require.NoError(t, f.worker(t).PreparePlan(context.Background(), f.input()))
	require.Equal(t, 1, f.prepared)
	require.Equal(t, []entity.HookWakeEvent{{Run: f.input().Candidate.Key, OperationID: "before-original", ExecutionScope: "ppe_original"}}, f.wakes)
}

func TestHookWorkerCoordinatorHintsNeverCreateIntent(t *testing.T) {
	f := newCoordinatorFixture(t)
	effect := f.effects()
	effect.BeginFinalize, effect.ActivateAfter, effect.WakeEvaluation = true, true, true
	require.NoError(t, f.worker(t).ApplyEffects(context.Background(), effect))
	require.NoError(t, f.worker(t).FinalizeRun(context.Background(), f.input()))
	require.Zero(t, f.finalized)
	require.Empty(t, f.wakes)
}

func TestHookWorkerCoordinatorEffectsResumeOriginalFinalize(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.pending()
	effect := f.effects()
	effect.BeginFinalize, effect.WakeEvaluation = true, true
	require.NoError(t, f.worker(t).ApplyEffects(context.Background(), effect))
	require.Equal(t, 1, f.finalized)
	require.Len(t, f.wakes, 1)
}

func TestHookWorkerCoordinatorRejectsUntrustedIdentity(t *testing.T) {
	for _, change := range []string{"scope", "workspace", "experiment", "run", "operation", "stored_scope", "stored_key", "corrupt", "negative_version"} {
		t.Run(change, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			f.ready()
			effect := f.effects()
			effect.WakeEvaluation = true
			switch change {
			case "scope":
				effect.ExecutionScope = "another"
			case "workspace":
				effect.Key.WorkspaceID = 0
			case "experiment":
				effect.Key.ExperimentID = 0
			case "run":
				effect.Key.RunID = 0
			case "operation":
				effect.OperationID = "unrelated"
			case "stored_scope":
				f.runs.run.Snapshot.ExecutionScope = "another"
			case "stored_key":
				f.runs.run.State.Key.RunID++
			case "corrupt":
				f.runs.run.State.After.Activated = true
			case "negative_version":
				f.runs.run.Version = -1
			}
			err := f.worker(t).ApplyEffects(context.Background(), effect)
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrHookWorkerScheduleRecoveryUnavailable)
			require.Zero(t, f.prepared)
			require.Zero(t, f.finalized)
			require.Empty(t, f.wakes)
		})
	}
}

func TestHookWorkerCoordinatorCancelAndReadErrorsStopEffects(t *testing.T) {
	for _, stage := range []string{"cancelled", "read_error", "cancel_on_read"} {
		t.Run(stage, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			f.pending()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := context.Canceled
			if stage == "cancelled" {
				cancel()
			}
			if stage == "read_error" {
				want = errors.New("storage unavailable")
				f.runs.read = func(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) { return nil, want }
			}
			if stage == "cancel_on_read" {
				f.runs.read = func(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) {
					cancel()
					return f.runs.run, nil
				}
			}
			require.ErrorIs(t, f.worker(t).FinalizeRun(ctx, f.input()), want)
			require.Zero(t, f.finalized)
			require.Empty(t, f.wakes)
		})
	}
}

func TestHookWorkerCoordinatorUsesPrimaryReads(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.runs.read = func(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
		require.True(t, contexts.CtxWriteDB(ctx))
		return f.runs.run, nil
	}
	require.NoError(t, f.worker(t).PreparePlan(context.Background(), f.input()))
}

func TestHookWorkerCoordinatorRejectsIncompleteConstruction(t *testing.T) {
	for _, missing := range []string{"runs", "preparer", "manager", "wake", "gate", "scope", "invalid_scope", "typed_nil"} {
		t.Run(missing, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			switch missing {
			case "runs":
				f.deps.Runs = nil
			case "preparer":
				f.deps.Preparer = nil
			case "manager":
				f.deps.Manager = nil
			case "wake":
				f.deps.HookWake = nil
			case "gate":
				f.deps.Gate = nil
			case "scope":
				f.deps.ExecutionScope = ""
			case "invalid_scope":
				f.deps.ExecutionScope = "bad scope"
			case "typed_nil":
				f.deps.Manager = coordinatorFinalizer(nil)
			}
			c, err := NewHookWorkerCoordinator(f.deps)
			require.ErrorIs(t, err, ErrHookWorkerConfiguration)
			require.Nil(t, c)
		})
	}
}

func TestHookWorkerCoordinatorPreparationStopsClosedRuns(t *testing.T) {
	for _, state := range []string{"closed", "terminating", "finalizing", "finished"} {
		t.Run(state, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			switch state {
			case "closed":
				f.runs.run.State.Gate = entity.HookGateClosed
			case "terminating":
				f.runs.run.State.Status = entity.ExptStatus_Terminating
			case "finalizing":
				f.pending()
			case "finished":
				f.pending()
				f.runs.run.State.Status = entity.ExptStatus_SystemTerminated
				f.runs.run.State.Finalize = entity.HookFinalizeCommitted
				f.runs.run.State.After.Activated = true
			}
			require.NoError(t, f.worker(t).PreparePlan(context.Background(), f.input()))
			require.Zero(t, f.prepared)
			require.Zero(t, f.finalized)
			require.Empty(t, f.wakes)
		})
	}
}

func TestHookWorkerCoordinatorWakeFailureCanReplayCommittedRun(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.pending()
	failure := errors.New("publish failed")
	calls := 0
	f.deps.HookWake = coordinatorWake(func(ctx context.Context, event entity.HookWakeEvent) error {
		calls++
		require.Equal(t, "after-original", event.OperationID)
		if calls == 1 {
			return failure
		}
		return nil
	})
	c := f.worker(t)
	require.ErrorIs(t, c.FinalizeRun(context.Background(), f.input()), failure)
	require.Equal(t, entity.HookFinalizeCommitted, f.runs.run.State.Finalize)
	require.NoError(t, c.FinalizeRun(context.Background(), f.input()))
	require.Equal(t, 2, calls)
}

func TestHookWorkerCoordinatorRechecksFinalizationBeforeWake(t *testing.T) {
	for _, change := range []string{"manager_error", "cancelled", "not_committed", "wrong_run", "wrong_intent", "after_done"} {
		t.Run(change, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			f.pending()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.deps.Manager = coordinatorFinalizer(func(ctx context.Context, key entity.HookRunKey, intent entity.HookTerminalIntent) error {
				switch change {
				case "manager_error":
					return errors.New("manager failed")
				case "cancelled":
					cancel()
				case "wrong_run":
					f.runs.run.State.Key.RunID++
				case "wrong_intent":
					f.runs.run.State.Intent.Reason = "NEW_REASON"
				case "after_done":
					f.runs.run.State.Finalize = entity.HookFinalizeCommitted
					f.runs.run.State.Status = intent.Status
					f.runs.run.State.After.Status = entity.HookOperationSucceeded
				}
				return nil
			})
			err := f.worker(t).FinalizeRun(ctx, f.input())
			if change == "after_done" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Empty(t, f.wakes)
		})
	}
}

func TestHookWorkerCoordinatorScheduleHintChecksLiveGate(t *testing.T) {
	for _, gate := range []entity.HookGateState{entity.HookGateClosed, entity.HookGateWaiting, "unknown"} {
		t.Run(string(gate), func(t *testing.T) {
			f := newCoordinatorFixture(t)
			f.ready()
			f.deps.Gate = coordinatorGate(func(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error) {
				return entity.HookAdmissionDecision{Gate: gate}, nil
			})
			effect := f.effects()
			effect.WakeEvaluation = true
			err := f.worker(t).ApplyEffects(context.Background(), effect)
			if gate == "unknown" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, f.wakes)
			require.Zero(t, f.finalized)
		})
	}
}

func TestHookWorkerCoordinatorRejectsInvalidCandidateBeforeIO(t *testing.T) {
	for _, change := range []string{"scope", "key", "version", "nil_context"} {
		t.Run(change, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			in := f.input()
			ctx := context.Background()
			switch change {
			case "scope":
				in.ExecutionScope = "untrusted"
			case "key":
				in.Candidate.Key.RunID = 0
			case "version":
				in.Candidate.Version = -1
			case "nil_context":
				ctx = nil
			}
			f.runs.read = func(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error) {
				t.Fatal("invalid request read storage")
				return nil, nil
			}
			c := f.worker(t)
			require.Error(t, c.PreparePlan(ctx, in))
			require.Error(t, c.FinalizeRun(ctx, in))
		})
	}
}

func TestHookWorkerCoordinatorPreparationDoesNotWakeSupersededRun(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.runs.run.PlanReady = true
	f.runs.run.State.Before.Activated = true
	f.deps.Gate = coordinatorGate(func(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error) {
		return entity.HookAdmissionDecision{Gate: entity.HookGateClosed, Reason: "RUN_OWNERSHIP_MISMATCH"}, nil
	})
	require.NoError(t, f.worker(t).PreparePlan(context.Background(), f.input()))
	require.Zero(t, f.prepared)
	require.Empty(t, f.wakes)
}

func TestHookWorkerCoordinatorPreparationRechecksCancellation(t *testing.T) {
	f := newCoordinatorFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.deps.Preparer = coordinatorPreparer(func(ctx context.Context, in hook.WorkerRunInput) error {
		_, bounded := ctx.Deadline()
		require.True(t, bounded)
		f.runs.run.PlanReady = true
		f.runs.run.State.Before.Activated = true
		cancel()
		return nil
	})
	require.ErrorIs(t, f.worker(t).PreparePlan(ctx, f.input()), context.Canceled)
	require.Empty(t, f.wakes)
}

func TestHookWorkerCoordinatorOnlyCommittedAfterCanWake(t *testing.T) {
	f := newCoordinatorFixture(t)
	f.pending()
	effect := f.effects()
	effect.OperationID = "after-original"
	effect.ActivateAfter = true
	c := f.worker(t)
	require.NoError(t, c.ApplyEffects(context.Background(), effect))
	require.Empty(t, f.wakes)
	f.runs.run.State.Finalize = entity.HookFinalizeCommitted
	f.runs.run.State.Status = entity.ExptStatus_SystemTerminated
	f.runs.run.State.After.Activated = true
	require.NoError(t, c.ApplyEffects(context.Background(), effect))
	require.Len(t, f.wakes, 1)
	require.Equal(t, "after-original", f.wakes[0].OperationID)
	f.runs.run.State.After.Status = entity.HookOperationSucceeded
	f.runs.run.State.After.Activated = false
	require.NoError(t, c.ApplyEffects(context.Background(), effect))
	require.Len(t, f.wakes, 1)
}

func coordinatorScheduleSeed(t *testing.T, f *coordinatorFixture) *entity.HookRunSnapshot {
	t.Helper()
	f.ready()
	f.runs.run.State.Status = entity.ExptStatus_Pending
	f.runs.run.Mode = entity.EvaluationModeSubmit
	f.runs.run.CreatedBy = "original-user"
	key := f.input().Candidate.Key
	s, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "ppe_original", CreatedAt: time.Unix(1700000001, 0),
		Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}, ParametersJSON: gptr.Of("{}")}},
		Context: &spi.HookRunContext{WorkspaceID: gptr.Of("11"), ExperimentID: gptr.Of("22"), RunID: gptr.Of("33"), RunMode: gptr.Of("submit"),
			Initiator: &spi.HookInitiator{UserID: gptr.Of("original-user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("original"), Type: gptr.Of("offline")}, EvalSets: []*spi.HookEvalSetRef{}},
		Schedule: &entity.HookScheduleSeed{Version: 1, Key: key, ExecutionScope: "ppe_original", Mode: entity.EvaluationModeSubmit, CreatedAt: 1700000001, ItemRetryTimes: 4, Session: &entity.Session{UserID: "original-user", AppID: 8}, Ext: map[string]string{entity.RetryYieldExtKey: "true", "route": "original"}}})
	require.NoError(t, err)
	return s
}

func TestHookScheduleCoordinatorPublishesOriginalEventAndRepeatsAfterLoss(t *testing.T) {
	f := newCoordinatorFixture(t)
	snapshot := coordinatorScheduleSeed(t, f)
	f.deps.Codec = coordinatorCodec{decode: func(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
		return snapshot, nil
	}}
	var sent []*entity.ExptScheduleEvent
	failure := errors.New("lost MQ")
	f.deps.SchedulePublisher = coordinatorSchedulePublisher(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		sent = append(sent, event)
		if len(sent) == 1 {
			return failure
		}
		return nil
	})
	c := f.worker(t)
	effect := f.effects()
	effect.WakeEvaluation = true
	require.ErrorIs(t, c.ApplyEffects(context.Background(), effect), failure)
	require.NoError(t, c.PreparePlan(context.Background(), f.input()))
	require.Len(t, sent, 2)
	require.Equal(t, sent[0], sent[1])
	require.Equal(t, int64(1700000001), sent[1].CreatedAt)
	require.Equal(t, 4, sent[1].ItemRetryTimes)
	require.Equal(t, "true", sent[1].Ext[entity.RetryYieldExtKey])
	require.Equal(t, int32(8), sent[1].Session.AppID)
	sent[0].Ext["route"] = "mutated"
	require.Equal(t, "original", snapshot.Input().Schedule.Ext["route"])
}

func TestHookScheduleCoordinatorRechecksAfterDecode(t *testing.T) {
	for _, change := range []string{"cancel", "gate_closed", "new_run", "started", "processing", "creator", "decode_error"} {
		t.Run(change, func(t *testing.T) {
			f := newCoordinatorFixture(t)
			snapshot := coordinatorScheduleSeed(t, f)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f.deps.Codec = coordinatorCodec{decode: func(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error) {
				switch change {
				case "cancel":
					cancel()
				case "gate_closed":
					f.runs.run.State.Gate = entity.HookGateClosed
				case "new_run":
					f.runs.run.State.Key.RunID++
				case "started":
					f.runs.run.ExecutionStarted = true
				case "processing":
					f.runs.run.State.Status = entity.ExptStatus_Processing
				case "creator":
					f.runs.run.CreatedBy = "other"
				case "decode_error":
					return nil, errors.New("decode")
				}
				return snapshot, nil
			}}
			f.deps.SchedulePublisher = coordinatorSchedulePublisher(func(context.Context, *entity.ExptScheduleEvent) error {
				t.Fatal("stale/cancelled seed published")
				return nil
			})
			effect := f.effects()
			effect.WakeEvaluation = true
			err := f.worker(t).ApplyEffects(ctx, effect)
			if change == "gate_closed" || change == "started" || change == "processing" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
