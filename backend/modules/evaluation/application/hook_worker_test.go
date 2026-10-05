// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

type workerConfig struct {
	mu    sync.Mutex
	value entity.HookRuntimeConfig
	err   error
}

type workerConfigFunc func(context.Context) (entity.HookRuntimeConfig, error)

func (f workerConfigFunc) GetRuntimeConfig(ctx context.Context) (entity.HookRuntimeConfig, error) {
	return f(ctx)
}

func TestHookWorkerRecoveryDuplicateAcrossStreamsIsOncePerRound(t *testing.T) {
	d := workerDeps()
	cfg := d.Config
	finished := make(chan struct{}, 10)
	var calls atomic.Int32
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		calls.Add(1)
		finished <- struct{}{}
		return hook.AttemptExecutionResult{RecoveryPending: true}, nil
	})
	var reads int
	d.Config = workerConfigFunc(func(ctx context.Context) (entity.HookRuntimeConfig, error) {
		reads++
		if reads > 2 {
			select {
			case <-finished:
			case <-ctx.Done():
				return entity.HookRuntimeConfig{}, ctx.Err()
			}
		}
		return cfg.GetRuntimeConfig(ctx)
	})
	d.ScanRepo = &workerScan{operations: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	w.runRound(s)
	s.jobs.Wait()
	require.Equal(t, int32(1), calls.Load(), "same recovery candidate may appear in stale due/retry/expired pages")
}

func (c *workerConfig) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, c.err
}
func (c *workerConfig) update(f func(*entity.HookRuntimeConfig)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f(&c.value)
}

type workerClock struct {
	calls atomic.Int64
	err   error
}

func (c *workerClock) Now(context.Context) (time.Time, error) {
	return time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC).Add(time.Duration(c.calls.Add(1)) * time.Second), c.err
}

type workerIDs struct {
	idgen.IIDGenerator
	next atomic.Int64
	err  error
}

func (i *workerIDs) GenID(context.Context) (int64, error) { return i.next.Add(1), i.err }

type workerExecute func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error)

func (f workerExecute) Execute(c context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
	return f(c, in)
}

type workerObservation struct {
	mu     sync.Mutex
	events []hook.WorkerEvent
}

func (o *workerObservation) Observe(e hook.WorkerEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, e)
}
func (o *workerObservation) count(code hook.WorkerEventCode) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := 0
	for _, e := range o.events {
		if e.Code == code {
			n++
		}
	}
	return n
}

type workerCoord struct {
	prepare, finalize func(context.Context, hook.WorkerRunInput) error
	effects           func(context.Context, hook.WorkerEffects) error
}

func (c *workerCoord) PreparePlan(ctx context.Context, in hook.WorkerRunInput) error {
	if c.prepare != nil {
		return c.prepare(ctx, in)
	}
	return nil
}
func (c *workerCoord) FinalizeRun(ctx context.Context, in hook.WorkerRunInput) error {
	if c.finalize != nil {
		return c.finalize(ctx, in)
	}
	return nil
}
func (c *workerCoord) ApplyEffects(ctx context.Context, in hook.WorkerEffects) error {
	if c.effects != nil {
		return c.effects(ctx, in)
	}
	return nil
}

type workerScan struct {
	operations func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error)
	runs       func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error)
}

func (s *workerScan) ScanDueOperations(_ context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	return s.op(entity.HookScanDue, in)
}
func (s *workerScan) ScanExpiredOperations(_ context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	return s.op(entity.HookScanExpired, in)
}
func (s *workerScan) op(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	if s.operations != nil {
		return s.operations(k, in)
	}
	return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
}
func (s *workerScan) ScanPreparingPlans(_ context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	return s.run(entity.HookScanPreparing, in)
}
func (s *workerScan) ScanPendingFinalizations(_ context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	return s.run(entity.HookScanFinalize, in)
}
func (s *workerScan) run(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	if s.runs != nil {
		return s.runs(k, in)
	}
	return entity.HookScanPage[entity.HookRunCandidate]{}, nil
}
func workerCandidate(space, id int64) entity.HookOperationCandidate {
	return entity.HookOperationCandidate{Key: entity.HookRunKey{WorkspaceID: space, ExperimentID: space + 100, RunID: id}, ID: id, OperationID: fmt.Sprint("op-", id), Phase: entity.HookPhaseBefore}
}
func workerDeps() HookWorkerDependencies {
	return HookWorkerDependencies{ExecutionScope: "worker-local", Owner: "worker-process", ScanRepo: &workerScan{}, Clock: &workerClock{}, IDs: &workerIDs{}, Config: &workerConfig{value: entity.HookRuntimeConfig{WorkerEnabled: true, WorkerConcurrency: 8, WorkspaceConcurrency: 2, ScanIntervalSeconds: 1, ScanBatchSize: 100}}, Executor: workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		return hook.AttemptExecutionResult{}, nil
	}), Coordinator: &workerCoord{}, Observer: &workerObservation{}}
}
func workerRoundFixture(t *testing.T, d HookWorkerDependencies) (*HookWorker, *hookWorkerSession) {
	t.Helper()
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	s := newHookWorkerSession(context.Background())
	w.session = s
	t.Cleanup(func() { s.cancel(); s.jobs.Wait() })
	return w, s
}

// Losing the raw page tail when every row is filtered would keep the cold space unreachable.
func TestHookWorkerHotPageAdvancesRawCursorAndFixedClock(t *testing.T) {
	d := workerDeps()
	d.Config.(*workerConfig).value.WorkspaceConcurrency = 1
	var queries []entity.HookScanInput
	var mu sync.Mutex
	var sent []int64
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		mu.Lock()
		sent = append(sent, in.Key.WorkspaceID)
		mu.Unlock()
		<-ctx.Done()
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		require.NoError(t, in.Validate(k))
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		queries = append(queries, in)
		if in.Cursor != nil {
			require.Equal(t, int64(100), in.Cursor.ID)
			return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(2, 101)}}, nil
		}
		rows := make([]entity.HookOperationCandidate, 100)
		for i := range rows {
			rows[i] = workerCandidate(1, int64(i+1))
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows, HasMore: true, NextCursor: &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: in.Now, ID: 100}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	w.runRound(s)
	w.runRound(s)
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(sent) == 2 }, time.Second, time.Millisecond)
	mu.Lock()
	require.ElementsMatch(t, []int64{1, 2}, sent)
	mu.Unlock()
	require.Equal(t, queries[0].Now, queries[1].Now)
	w.runRound(s)
	require.True(t, queries[2].Now.After(queries[1].Now))
}

// Even process-wide saturation must not pin any category's cursor at the hot page.
func TestHookWorkerFullPoolAndEmptyFilteredPagesStillAdvance(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 1
	cfg.value.WorkspaceConcurrency = 1
	var got []entity.HookScanInput
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		got = append(got, in)
		if in.Cursor != nil {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{HasMore: true, NextCursor: &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: in.Now, ID: 200}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	s.active["busy"] = 1
	s.spaces[1] = 1
	w.runRound(s)
	w.runRound(s)
	require.Len(t, got, 2)
	require.Equal(t, int64(200), got[1].Cursor.ID)
	require.Equal(t, got[0].Now, got[1].Now)
}

func TestHookWorkerFiveClassesFairWithSingleSlot(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 1
	cfg.value.WorkspaceConcurrency = 1
	entered := make(chan string, 10)
	release := make(chan struct{})
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		entered <- in.OperationID
		select {
		case <-release:
		case <-ctx.Done():
		}
		return hook.AttemptExecutionResult{}, nil
	})
	d.Coordinator = &workerCoord{prepare: func(ctx context.Context, in hook.WorkerRunInput) error {
		entered <- "preparing"
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}, finalize: func(ctx context.Context, in hook.WorkerRunInput) error {
		entered <- "finalize"
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}}
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		id := int64(1)
		if in.Status == entity.HookOperationRetryWait {
			id = 2
		}
		if k == entity.HookScanExpired {
			id = 3
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(id, id)}}, nil
	}, runs: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
		return entity.HookScanPage[entity.HookRunCandidate]{Candidates: []entity.HookRunCandidate{{Key: workerCandidate(9, 9).Key}}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	var seen []string
	for i := 0; i < 5; i++ {
		w.runRound(s)
		select {
		case v := <-entered:
			seen = append(seen, v)
		case <-time.After(time.Second):
			t.Fatal("starved class")
		}
		release <- struct{}{}
		s.jobs.Wait()
	}
	require.ElementsMatch(t, []string{"op-1", "op-2", "op-3", "preparing", "finalize"}, seen)
}

func TestHookWorkerLimitsAndSameOperationDedup(t *testing.T) {
	d := workerDeps()
	var active, maxActive atomic.Int32
	var mu sync.Mutex
	spaces := map[int64]int{}
	d.ScanRepo = &workerScan{operations: func(_ entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		var rows []entity.HookOperationCandidate
		for i := int64(1); i <= 40; i++ {
			rows = append(rows, workerCandidate((i-1)/4+1, i))
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows}, nil
	}}
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		a := active.Add(1)
		for old := maxActive.Load(); a > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, a) {
				break
			}
		}
		mu.Lock()
		spaces[in.Key.WorkspaceID]++
		mu.Unlock()
		<-ctx.Done()
		active.Add(-1)
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	w, s := workerRoundFixture(t, d)
	w.runRound(s)
	w.runRound(s)
	require.Eventually(t, func() bool { return active.Load() == 8 }, time.Second, time.Millisecond)
	require.Equal(t, int32(8), maxActive.Load())
	mu.Lock()
	for _, n := range spaces {
		require.LessOrEqual(t, n, 2)
	}
	mu.Unlock()
	w.mu.Lock()
	require.Len(t, s.active, 8)
	w.mu.Unlock()
}

func TestHookWorkerConfigFailurePauseAndAdmissionDisabledDrain(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	var sends atomic.Int32
	d.ScanRepo = &workerScan{operations: func(_ entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		sends.Add(1)
		return hook.AttemptExecutionResult{}, nil
	})
	w, s := workerRoundFixture(t, d)
	cfg.err = errors.New("credentials=secret")
	w.runRound(s)
	require.Zero(t, sends.Load())
	require.Positive(t, d.Observer.(*workerObservation).count(hook.WorkerConfigFailed))
	cfg.err = nil
	cfg.value.WorkerEnabled = false
	w.runRound(s)
	require.Zero(t, sends.Load())
	cfg.value.WorkerEnabled = true
	cfg.value.AdmissionEnabled = false
	w.runRound(s)
	s.jobs.Wait()
	require.Positive(t, sends.Load())
}

func TestHookWorkerRejectsUnsafeConfigAndMissingDependencies(t *testing.T) {
	for _, change := range []func(*HookWorkerDependencies){func(d *HookWorkerDependencies) { d.Coordinator = nil }, func(d *HookWorkerDependencies) { var p *workerClock; d.Clock = p }, func(d *HookWorkerDependencies) { d.ExecutionScope = "" }, func(d *HookWorkerDependencies) { d.Owner = "\nforged" }} {
		d := workerDeps()
		change(&d)
		_, err := NewHookWorker(d)
		require.Error(t, err)
	}
	for _, v := range []int32{0, -1, 129, 2147483647} {
		d := workerDeps()
		d.Config.(*workerConfig).value.WorkerConcurrency = v
		w, s := workerRoundFixture(t, d)
		w.runRound(s)
		require.Positive(t, d.Observer.(*workerObservation).count(hook.WorkerConfigFailed))
		require.Empty(t, s.active)
	}
	for _, v := range []int32{0, 33, 2147483647} {
		d := workerDeps()
		d.Config.(*workerConfig).value.WorkerConcurrency = 128
		d.Config.(*workerConfig).value.WorkspaceConcurrency = v
		w, s := workerRoundFixture(t, d)
		w.runRound(s)
		require.Positive(t, d.Observer.(*workerObservation).count(hook.WorkerConfigFailed))
	}
}

func TestHookWorkerRunStopRaceRestartAndWakeScope(t *testing.T) {
	d := workerDeps()
	entered := make(chan struct{}, 10)
	var ended atomic.Int32
	d.ScanRepo = &workerScan{operations: func(_ entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		entered <- struct{}{}
		<-ctx.Done()
		ended.Add(1)
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	require.NoError(t, w.Stop(context.Background()))
	event := entity.HookWakeEvent{Run: workerCandidate(1, 1).Key, OperationID: "op-1", ExecutionScope: "worker-local"}
	for round := 0; round < 2; round++ {
		runDone := make(chan error, 1)
		go func() { runDone <- w.Run(context.Background()) }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("did not start")
		}
		require.ErrorIs(t, w.Run(context.Background()), ErrHookWorkerRunning)
		forged := event
		forged.ExecutionScope = "prod"
		require.Error(t, w.Wake(context.Background(), forged))
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					_ = w.Wake(context.Background(), event)
				}
			}()
		}
		wg.Wait()
		var stoppers sync.WaitGroup
		for i := 0; i < 8; i++ {
			stoppers.Add(1)
			go func() { defer stoppers.Done(); _ = w.Stop(context.Background()) }()
		}
		stoppers.Wait()
		require.NoError(t, <-runDone)
		require.Equal(t, int32(round+1), ended.Load())
		require.ErrorIs(t, w.Wake(context.Background(), event), ErrHookWorkerStopped)
	}
}

func TestHookWorkerRecoveryPendingDoesNotSpinOnWake(t *testing.T) {
	d := workerDeps()
	var calls atomic.Int32
	entered := make(chan struct{}, 1)
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanExpired {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		return hook.AttemptExecutionResult{RecoveryPending: true}, nil
	})
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- w.Run(context.Background()) }()
	t.Cleanup(func() { _ = w.Stop(context.Background()); <-done })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("no recovery")
	}
	for i := 0; i < 1000; i++ {
		require.NoError(t, w.Wake(context.Background(), entity.HookWakeEvent{Run: workerCandidate(1, 1).Key, OperationID: "op-1", ExecutionScope: "worker-local"}))
	}
	require.Never(t, func() bool { return calls.Load() > 1 }, 50*time.Millisecond, time.Millisecond)
}

func TestHookWorkerErrorsAndBoundedEffectDelivery(t *testing.T) {
	d := workerDeps()
	var effects atomic.Int32
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, errors.New("raw scan secret")
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1), workerCandidate(2, 2)}}, nil
	}, runs: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
		return entity.HookScanPage[entity.HookRunCandidate]{Candidates: []entity.HookRunCandidate{{Key: workerCandidate(3, 3).Key}}}, nil
	}}
	d.Executor = workerExecute(func(_ context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		if in.OperationID == "op-2" {
			return hook.AttemptExecutionResult{}, errors.New("raw execute secret")
		}
		return hook.AttemptExecutionResult{HookAttemptStoreResult: entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Changed: true, Run: &entity.HookStoredRun{State: entity.HookRunState{Key: in.Key, Gate: entity.HookGateReady}}, Effects: entity.HookStateEffects{BeginFinalize: true}}}}, nil
	})
	d.Coordinator = &workerCoord{prepare: func(context.Context, hook.WorkerRunInput) error { return errors.New("raw plan secret") }, finalize: func(context.Context, hook.WorkerRunInput) error { return errors.New("raw finalize secret") }, effects: func(context.Context, hook.WorkerEffects) error {
		effects.Add(1)
		return errors.New("raw effect secret")
	}}
	w, s := workerRoundFixture(t, d)
	w.runRound(s)
	s.jobs.Wait()
	require.Equal(t, int32(3), effects.Load())
	obs := d.Observer.(*workerObservation)
	require.Positive(t, obs.count(hook.WorkerScanFailed))
	require.Positive(t, obs.count(hook.WorkerExecutionFailed))
	require.Positive(t, obs.count(hook.WorkerCoordinationFailed))
	require.Equal(t, 1, obs.count(hook.WorkerEffectsExhausted))
	require.NotContains(t, fmt.Sprint(obs.events), "raw")
	require.NotContains(t, fmt.Sprint(obs.events), "secret")
}
