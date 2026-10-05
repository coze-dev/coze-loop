// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookWorkerOverflowFixedSetMatrix(t *testing.T) {
	type scenario struct{ total, batch, full int }
	var cases []scenario
	for _, total := range []int{99, 100, 101, 199, 200, 201, 202, 203, 301} {
		for _, full := range []int{0, 1, 2, 4} {
			cases = append(cases, scenario{total, 100, full})
		}
	}
	cases = append(cases, scenario{503, 100, 2}, scenario{1001, 100, 2}, scenario{2002, 100, 4}, scenario{202, 37, 5}, scenario{301, 17, 2}, scenario{21, 10, 2})
	for _, tc := range cases {
		t.Run(fmt.Sprintf("n%d_page%d_full%d", tc.total, tc.batch, tc.full), func(t *testing.T) {
			d := workerDeps()
			cfg := d.Config.(*workerConfig)
			cfg.value.WorkerConcurrency = 1
			cfg.value.WorkspaceConcurrency = 1
			cfg.value.ScanBatchSize = int32(tc.batch)
			entered := make(chan int64, 1)
			release := make(chan struct{})
			d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
				entered <- in.Key.WorkspaceID
				select {
				case <-release:
					return hook.AttemptExecutionResult{}, errors.New("durable eligible row")
				case <-ctx.Done():
					return hook.AttemptExecutionResult{}, ctx.Err()
				}
			})
			var queries []entity.HookScanInput
			anchors := map[time.Time]bool{}
			due := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
			d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
				if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
					return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
				}
				require.NoError(t, in.Validate(k))
				require.Equal(t, "worker-local", in.ExecutionScope)
				queries = append(queries, in)
				start := int64(1)
				if in.Cursor != nil {
					start = in.Cursor.ID + 1
					require.True(t, anchors[in.Now])
					require.Equal(t, in.Cursor.Now, in.Now)
				} else {
					anchors[in.Now] = true
				}
				end := start + int64(in.Limit) - 1
				if end > int64(tc.total) {
					end = int64(tc.total)
				}
				out := entity.HookScanPage[entity.HookOperationCandidate]{HasMore: end < int64(tc.total)}
				for id := start; id <= end; id++ {
					row := workerCandidate(id, id)
					row.DueAt = due
					out.Candidates = append(out.Candidates, row)
				}
				if out.HasMore {
					out.NextCursor = &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: due, ID: end}
				}
				return out, nil
			}}
			w, s := workerRoundFixture(t, d)
			normalRounds := 0
			runRound := func() {
				before := len(queries)
				prior := s.streams[0]
				w.runRound(s)
				normalRounds++
				require.GreaterOrEqual(t, len(queries)-before, 1)
				require.LessOrEqual(t, len(queries)-before, 2)
				normal := queries[before]
				require.Equal(t, prior.cursor, normal.Cursor)
				if !prior.now.IsZero() {
					require.Equal(t, prior.now, normal.Now)
				}
				end := int64(tc.batch)
				if prior.cursor != nil {
					end += prior.cursor.ID
				}
				if end >= int64(tc.total) {
					require.Nil(t, s.streams[0].cursor)
				} else {
					require.Equal(t, end, s.streams[0].cursor.ID)
				}
				require.LessOrEqual(t, len(s.queued), 500)
				for i, q := range s.pending {
					require.LessOrEqual(t, len(q), 100)
					require.LessOrEqual(t, cap(q), 100)
					if r := s.overflow[i]; r != nil {
						require.LessOrEqual(t, len(r.page), 100)
						require.LessOrEqual(t, cap(r.page), 100)
					}
				}
			}
			seen := map[int64]bool{}
			jobs := 0
			for ; jobs < tc.total*3+tc.total/tc.batch+1 && len(seen) < tc.total; jobs++ {
				runRound()
				select {
				case id := <-entered:
					seen[id] = true
				case <-time.After(time.Second):
					t.Fatal("idle slot not filled")
				}
				for n := 0; n < tc.full; n++ {
					runRound()
				}
				release <- struct{}{}
				s.jobs.Wait()
			}
			require.Len(t, seen, tc.total, "all fixed workspaces, not only resident or last row")
			t.Logf("workspaces=%d jobs=%d normal_rounds=%d total_queries=%d", len(seen), jobs, normalRounds, len(queries))
		})
	}
}

func TestHookWorkerOverflowPauseErrorShrinkAndResume(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 2
	cfg.value.WorkspaceConcurrency = 1
	rows := []entity.HookOperationCandidate{workerCandidate(1, 1), workerCandidate(1, 2), workerCandidate(1, 3)}
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows}, nil
	}}
	entered := make(chan int64, 1)
	release := make(chan struct{})
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		entered <- in.Key.RunID
		select {
		case <-release:
			return hook.AttemptExecutionResult{}, nil
		case <-ctx.Done():
			return hook.AttemptExecutionResult{}, ctx.Err()
		}
	})
	w, s := workerRoundFixture(t, d)
	s.active["hold1"] = 10
	s.active["hold2"] = 11
	w.runRound(s)
	require.Len(t, s.pending[0], 2)
	require.Len(t, s.overflow[0].page, 1)
	rows = nil
	r := s.overflow[0]
	saved := r.page[0]
	cfg.err = errors.New("config failure")
	w.runRound(s)
	require.Equal(t, saved, s.overflow[0].page[0])
	cfg.err = nil
	cfg.value.WorkerEnabled = false
	w.runRound(s)
	require.Same(t, r, s.overflow[0])
	cfg.value.WorkerEnabled = true
	cfg.value.WorkerConcurrency = 1
	cfg.value.AdmissionEnabled = false
	delete(s.active, "hold1")
	w.runRound(s)
	require.Empty(t, entered)
	require.Same(t, r, s.overflow[0])
	delete(s.active, "hold2")
	for want := int64(1); want <= 3; want++ {
		w.runRound(s)
		select {
		case got := <-entered:
			require.Equal(t, want, got)
		case <-time.After(time.Second):
			t.Fatal("overflow candidate lost during pause")
		}
		release <- struct{}{}
		s.jobs.Wait()
	}
	require.Nil(t, s.overflow[0])
	require.Empty(t, s.queued)
}

func TestHookWorkerOverflowReplayErrorKeepsCursor(t *testing.T) {
	d := workerDeps()
	at := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	fail := true
	cursor := &entity.HookScanCursor{Kind: entity.HookScanDue, ExecutionScope: "worker-local", Status: "pending", Now: at, At: at.Add(-time.Second), ID: 200}
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		require.NoError(t, in.Validate(k))
		require.Equal(t, cursor, in.Cursor)
		require.Equal(t, at, in.Now)
		if fail {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, errors.New("private DB error")
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(201, 201)}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	r := &hookWorkerOverflow{stream: hookWorkerStream{kind: entity.HookScanDue, status: entity.HookOperationPending, now: at, cursor: cursor}, page: make([]hookWorkerTask, 0, 100)}
	s.overflow[0] = r
	w.resumeOverflow(s, 0, 100)
	require.Same(t, r, s.overflow[0])
	require.Equal(t, cursor, r.stream.cursor)
	require.False(t, r.tail)
	require.Empty(t, r.page)
	require.Equal(t, 1, d.Observer.(*workerObservation).count(hook.WorkerScanFailed))
	fail = false
	w.resumeOverflow(s, 0, 100)
	require.Nil(t, s.overflow[0])
	require.Len(t, s.pending[0], 1)
	require.Equal(t, int64(201), s.pending[0][0].operation.Key.WorkspaceID)
}

func TestHookWorkerOverflowFiveClassesFiniteSet(t *testing.T) {
	d := workerDeps()
	d.Config.(*workerConfig).value.WorkerConcurrency = 1
	d.Config.(*workerConfig).value.WorkspaceConcurrency = 1
	entered := make(chan int64, 1)
	release := make(chan struct{})
	wait := func(ctx context.Context, id int64) error {
		entered <- id
		select {
		case <-release:
			return errors.New("durable work")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		return hook.AttemptExecutionResult{}, wait(ctx, in.Key.WorkspaceID)
	})
	d.Coordinator = &workerCoord{prepare: func(ctx context.Context, in hook.WorkerRunInput) error {
		return wait(ctx, in.Candidate.Key.WorkspaceID)
	}, finalize: func(ctx context.Context, in hook.WorkerRunInput) error {
		return wait(ctx, in.Candidate.Key.WorkspaceID)
	}}
	due := time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		require.NoError(t, in.Validate(k))
		base := int64(0)
		status := string(in.Status)
		if in.Status == entity.HookOperationRetryWait {
			base = 10000
		}
		if k == entity.HookScanExpired {
			base = 20000
			status = "running"
		}
		start := base + 1
		if in.Cursor != nil {
			start = in.Cursor.ID + 1
		}
		end := start + int64(in.Limit) - 1
		if end > base+202 {
			end = base + 202
		}
		out := entity.HookScanPage[entity.HookOperationCandidate]{HasMore: end < base+202}
		for id := start; id <= end; id++ {
			row := workerCandidate(id, id)
			row.DueAt = due
			out.Candidates = append(out.Candidates, row)
		}
		if out.HasMore {
			out.NextCursor = &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: status, Now: in.Now, At: due, ID: end}
		}
		return out, nil
	}, runs: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
		require.NoError(t, in.Validate(k))
		base, status, at := int64(30000), "preparing", time.Time{}
		if k == entity.HookScanFinalize {
			base, status, at = 40000, "pending", due
		}
		start := base + 1
		if in.Cursor != nil {
			start = in.Cursor.WorkspaceID + 1
		}
		end := start + int64(in.Limit) - 1
		if end > base+202 {
			end = base + 202
		}
		out := entity.HookScanPage[entity.HookRunCandidate]{HasMore: end < base+202}
		for id := start; id <= end; id++ {
			out.Candidates = append(out.Candidates, entity.HookRunCandidate{Key: workerCandidate(id, id).Key, ReconcileAt: at})
		}
		if out.HasMore {
			out.NextCursor = &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: status, Now: in.Now, At: at, WorkspaceID: end, RunID: end}
		}
		return out, nil
	}}
	w, s := workerRoundFixture(t, d)
	seen := map[int64]bool{}
	for jobs := 0; jobs < 3030 && len(seen) < 1010; jobs++ {
		w.runRound(s)
		select {
		case id := <-entered:
			seen[id] = true
		case <-time.After(time.Second):
			t.Fatal("available slot idle")
		}
		w.runRound(s)
		w.runRound(s)
		release <- struct{}{}
		s.jobs.Wait()
		require.LessOrEqual(t, len(s.queued), 500)
		for _, r := range s.overflow {
			if r != nil {
				require.LessOrEqual(t, len(r.page), 100)
				require.LessOrEqual(t, cap(r.page), 100)
			}
		}
	}
	require.Len(t, seen, 1010)
}

func TestHookWorkerOverflowStopClearsRecoveryGeneration(t *testing.T) {
	d := workerDeps()
	d.Config.(*workerConfig).value.WorkerConcurrency = 1
	d.Config.(*workerConfig).value.WorkspaceConcurrency = 1
	var empty atomic.Bool
	var calls atomic.Int32
	started := make(chan struct{}, 1)
	scannedEmpty := make(chan struct{}, 1)
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		if empty.Load() {
			select {
			case scannedEmpty <- struct{}{}:
			default:
			}
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		var rows []entity.HookOperationCandidate
		for id := int64(1); id <= 100; id++ {
			rows = append(rows, workerCandidate(1, id))
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows}, nil
	}}
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	for generation := 0; generation < 2; generation++ {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(func() { cancel(); _ = w.Stop(context.Background()) })
		done := make(chan error, 1)
		go func() { done <- w.Run(ctx) }()
		if generation == 0 {
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("not started")
			}
		} else {
			select {
			case <-scannedEmpty:
			case <-time.After(time.Second):
				t.Fatal("no restart scan")
			}
			require.Never(t, func() bool { return calls.Load() > 1 }, 50*time.Millisecond, time.Millisecond)
		}
		w.mu.Lock()
		s := w.session
		w.mu.Unlock()
		cancel()
		require.NoError(t, w.Stop(context.Background()))
		require.NoError(t, <-done)
		if generation == 0 {
			require.NotNil(t, s.overflow[0])
			require.Len(t, s.overflow[0].page, 98)
			empty.Store(true)
		} else {
			for _, r := range s.overflow {
				require.Nil(t, r)
			}
		}
	}
}

// Expectation written before execution: with a fixed finite set of eligible
// workspaces, the last workspace must eventually execute even if the 100-entry queue
// is saturated. Repeated scanning of that workspace is not sufficient progress.
func TestHookWorkerOverflowColdSpaceEventuallyExecutes(t *testing.T) {
	for _, scenario := range []struct {
		total int64
		full  int
	}{{201, 2}, {202, 0}, {202, 2}} {
		t.Run(fmt.Sprintf("spaces_%d_full_rounds_%d", scenario.total, scenario.full), func(t *testing.T) {
			total, saturatedRounds := scenario.total, scenario.full
			d := workerDeps()
			cfg := d.Config.(*workerConfig)
			cfg.value.WorkerConcurrency = 1
			cfg.value.WorkspaceConcurrency = 1
			cfg.value.ScanBatchSize = 100
			entered := make(chan int64, 1)
			release := make(chan struct{})
			d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
				entered <- in.Key.WorkspaceID
				select {
				case <-release:
					return hook.AttemptExecutionResult{}, errors.New("bounded transient failure leaves durable candidate eligible")
				case <-ctx.Done():
					return hook.AttemptExecutionResult{}, ctx.Err()
				}
			})
			coldScans := 0
			d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
				if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
					return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
				}
				if err := in.Validate(k); err != nil {
					t.Fatal(err)
				}
				start := int64(1)
				if in.Cursor != nil {
					start = in.Cursor.ID + 1
				}
				end := start + int64(in.Limit) - 1
				if end > total {
					end = total
				}
				var rows []entity.HookOperationCandidate
				for id := start; id <= end; id++ {
					row := workerCandidate(id, id)
					row.DueAt = in.Now.Add(-time.Second)
					rows = append(rows, row)
					if id == total {
						coldScans++
					}
				}
				out := entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows, HasMore: end < total}
				if out.HasMore {
					out.NextCursor = &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: in.Now.Add(-time.Second), ID: end}
				}
				return out, nil
			}}
			w, s := workerRoundFixture(t, d)
			seen := map[int64]int{}
			for cycle := 0; cycle < 603; cycle++ {
				w.runRound(s)
				select {
				case space := <-entered:
					seen[space]++
				case <-time.After(time.Second):
					t.Fatal("idle slot was not filled")
				}
				for n := 0; n < saturatedRounds; n++ {
					w.runRound(s)
				}
				release <- struct{}{}
				s.jobs.Wait()
				if len(s.queued) > 500 {
					t.Fatal("queue map exceeded fixed bound")
				}
			}
			if seen[total] == 0 {
				t.Fatalf("workspace %d starved: cold scans=%d, completed jobs=603, distinct executed=%d; want at least one execution", total, coldScans, len(seen))
			}
		})
	}
}

func TestHookWorkerFairnessReleasePeriodMatrix(t *testing.T) {
	for _, rounds := range []int{0, 1, 2, 4, 5, 9, 14} {
		t.Run(fmt.Sprint(rounds), func(t *testing.T) { workerFairnessCategoryFairness(t, rounds) })
	}
}

func TestHookWorkerFairnessStopDoesNotCarryQueuedWorkToRestart(t *testing.T) {
	d := workerDeps()
	d.Config.(*workerConfig).value.WorkerConcurrency = 1
	d.Config.(*workerConfig).value.WorkspaceConcurrency = 1
	var empty atomic.Bool
	var calls atomic.Int32
	started := make(chan struct{}, 2)
	scannedEmpty := make(chan struct{}, 5)
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		if empty.Load() {
			select {
			case scannedEmpty <- struct{}{}:
			default:
			}
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1), workerCandidate(2, 2)}}, nil
	}}
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		calls.Add(1)
		started <- struct{}{}
		<-ctx.Done()
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	for generation := 0; generation < 2; generation++ {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(func() { cancel(); _ = w.Stop(context.Background()) })
		ran := make(chan error, 1)
		go func() { ran <- w.Run(ctx) }()
		if generation == 0 {
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("first generation did not start")
			}
			empty.Store(true)
		} else {
			select {
			case <-scannedEmpty:
			case <-time.After(time.Second):
				t.Fatal("restart did not scan")
			}
			require.Never(t, func() bool { return calls.Load() > 1 }, 50*time.Millisecond, time.Millisecond)
		}
		cancel()
		require.NoError(t, w.Stop(context.Background()))
		require.NoError(t, <-ran)
	}
	require.Equal(t, int32(1), calls.Load())
}

func TestHookWorkerFairnessPageAndReleaseMatrix(t *testing.T) {
	for _, pages := range []int{2, 3, 5, 7, 33, 101} {
		for _, period := range []int{0, 1, 2, 4, 6} {
			t.Run(fmt.Sprintf("pages_%d_full_%d", pages, period), func(t *testing.T) {
				d := workerDeps()
				cfg := d.Config.(*workerConfig)
				cfg.value.WorkerConcurrency = 1
				cfg.value.WorkspaceConcurrency = 1
				cfg.value.ScanBatchSize = 1
				entered := make(chan int64, 1)
				release := make(chan struct{})
				d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
					entered <- in.Key.WorkspaceID
					select {
					case <-release:
						return hook.AttemptExecutionResult{}, errors.New("durable candidate remains")
					case <-ctx.Done():
						return hook.AttemptExecutionResult{}, ctx.Err()
					}
				})
				var queries, replayQueries int
				var chainNow time.Time
				var lastNormalCursor int64
				anchors := map[time.Time]bool{}
				d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
					if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
						return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
					}
					require.NoError(t, in.Validate(k))
					page := 1
					if in.Cursor == nil {
						queries++
						if !chainNow.IsZero() {
							require.True(t, in.Now.After(chainNow))
						}
						chainNow = in.Now
						anchors[in.Now] = true
						lastNormalCursor = 0
					} else {
						if in.Now.Equal(chainNow) && in.Cursor.ID > lastNormalCursor {
							queries++
							require.Equal(t, chainNow, in.Now)
							lastNormalCursor = in.Cursor.ID
						} else {
							// A replay forks a previously observed chain, never its clock.
							replayQueries++
							require.True(t, anchors[in.Now])
							require.Equal(t, in.Cursor.Now, in.Now)
						}
						page = int(in.Cursor.ID) + 1
					}
					out := entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(int64(page), int64(page))}, HasMore: page < pages}
					if out.HasMore {
						out.NextCursor = &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: in.Now, ID: int64(page)}
					}
					return out, nil
				}}
				w, s := workerRoundFixture(t, d)
				seen := map[int64]bool{}
				for cycle := 0; cycle < pages*3; cycle++ {
					w.runRound(s)
					select {
					case space := <-entered:
						seen[space] = true
					case <-time.After(time.Second):
						t.Fatal("free slot not filled")
					}
					for n := 0; n < period; n++ {
						w.runRound(s)
					}
					release <- struct{}{}
					s.jobs.Wait()
				}
				require.Equal(t, pages*3*(period+1), queries, "queue saturation must not stop raw scans")
				require.LessOrEqual(t, replayQueries, queries, "at most one replay page per normal scan")
				require.Len(t, seen, pages, "finite resident spaces must execute, not merely be scanned")
			})
		}
	}
}

func TestHookWorkerFairnessFullQueueBoundsAndOtherClasses(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 1
	cfg.value.WorkspaceConcurrency = 1
	entered := make(chan string, 1)
	release := make(chan struct{})
	wait := func(ctx context.Context, kind string) error {
		entered <- kind
		select {
		case <-release:
			return errors.New("durable")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		return hook.AttemptExecutionResult{}, wait(ctx, in.OperationID)
	})
	d.Coordinator = &workerCoord{prepare: func(ctx context.Context, in hook.WorkerRunInput) error { return wait(ctx, "preparing") }, finalize: func(ctx context.Context, in hook.WorkerRunInput) error { return wait(ctx, "finalize") }}
	var batch int64
	var scans int
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		scans++
		class := int64(0)
		if in.Status == entity.HookOperationRetryWait {
			class = 1
		}
		if k == entity.HookScanExpired {
			class = 2
		}
		rows := make([]entity.HookOperationCandidate, 100)
		for i := range rows {
			id := class*100000 + batch*100 + int64(i) + 1
			rows[i] = workerCandidate(id, id)
			rows[i].OperationID = fmt.Sprintf("class%d-%d", class, id)
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows}, nil
	}, runs: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
		scans++
		rows := make([]entity.HookRunCandidate, 100)
		for i := range rows {
			id := 300000 + batch*100 + int64(i) + 1
			rows[i] = entity.HookRunCandidate{Key: workerCandidate(id, id).Key}
		}
		return entity.HookScanPage[entity.HookRunCandidate]{Candidates: rows}, nil
	}}
	w, s := workerRoundFixture(t, d)
	s.active["busy"] = 1
	s.spaces[1] = 1
	for batch = 0; batch < 50; batch++ {
		w.runRound(s)
		require.Len(t, s.queued, 500)
		for _, q := range s.pending {
			require.Len(t, q, 100)
			require.Equal(t, 100, cap(q))
		}
		require.Zero(t, s.nextDispatch, "full scans must not spend an execution turn")
	}
	require.Equal(t, 250, scans)
	delete(s.active, "busy")
	delete(s.spaces, 1)
	var firstFive []string
	for cycle := 0; cycle < 5; cycle++ {
		w.runRound(s)
		select {
		case kind := <-entered:
			firstFive = append(firstFive, kind)
		case <-time.After(time.Second):
			t.Fatal("queue blocked all classes")
		}
		release <- struct{}{}
		s.jobs.Wait()
	}
	require.Equal(t, []string{"class0-1", "class1-100001", "class2-200001", "preparing", "finalize"}, firstFive)
}

func TestHookWorkerFairnessHotSpaceQueueCannotBlockColdLaterPage(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 2
	cfg.value.WorkspaceConcurrency = 1
	entered := make(chan int64, 8)
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		entered <- in.Key.WorkspaceID
		<-ctx.Done()
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	var hotPage int64
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		hotPage++
		rows := make([]entity.HookOperationCandidate, 100)
		for i := range rows {
			rows[i] = workerCandidate(1, hotPage*100+int64(i))
		}
		if hotPage == 20 {
			rows = []entity.HookOperationCandidate{workerCandidate(2, 9999)}
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: rows, HasMore: true, NextCursor: &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: in.Now, ID: hotPage}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	for i := 0; i < 20; i++ {
		w.runRound(s)
		require.LessOrEqual(t, len(s.pending[0]), 2)
	}
	got := []int64{}
	for i := 0; i < 2; i++ {
		select {
		case id := <-entered:
			got = append(got, id)
		case <-time.After(time.Second):
			t.Fatal("cold space did not obtain spare slot")
		}
	}
	require.ElementsMatch(t, []int64{1, 2}, got)
}

func TestHookWorkerFairnessPendingSurvivesConfigFailurePauseAndShrink(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 2
	cfg.value.WorkspaceConcurrency = 1
	candidates := []entity.HookOperationCandidate{workerCandidate(1, 1), workerCandidate(2, 2), workerCandidate(3, 3)}
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: candidates}, nil
	}}
	entered := make(chan int64, 3)
	release := make(chan struct{})
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		entered <- in.Key.RunID
		select {
		case <-release:
			return hook.AttemptExecutionResult{}, nil
		case <-ctx.Done():
			return hook.AttemptExecutionResult{}, ctx.Err()
		}
	})
	w, s := workerRoundFixture(t, d)
	s.active["busy1"] = 100
	s.active["busy2"] = 101
	w.runRound(s)
	require.Len(t, s.queued, 3)
	candidates = nil
	cfg.err = errors.New("config unavailable")
	w.runRound(s)
	require.Len(t, s.queued, 3)
	require.Empty(t, entered)
	cfg.err = nil
	cfg.update(func(c *entity.HookRuntimeConfig) { c.WorkerEnabled = false })
	w.runRound(s)
	require.Len(t, s.queued, 3)
	cfg.update(func(c *entity.HookRuntimeConfig) {
		c.WorkerEnabled = true
		c.AdmissionEnabled = false
		c.WorkerConcurrency = 1
	})
	delete(s.active, "busy1")
	w.runRound(s)
	require.Empty(t, entered)
	require.Len(t, s.queued, 3)
	delete(s.active, "busy2")
	for expected := int64(1); expected <= 3; expected++ {
		w.runRound(s)
		select {
		case id := <-entered:
			require.Equal(t, expected, id)
		case <-time.After(time.Second):
			t.Fatal("lost pending candidate")
		}
		release <- struct{}{}
		s.jobs.Wait()
	}
	require.Empty(t, s.queued)
}

// Expectation: a continuously eligible category must get a slot after five
// completed jobs, even when each job overlaps four saturated scan rounds.
// An execution error deliberately leaves the same durable candidate eligible.
func TestHookWorkerFairnessCategoryFairnessAcrossSaturatedRounds(t *testing.T) {
	workerFairnessCategoryFairness(t, 4)
}

func TestHookWorkerFairnessCategoryFairnessWithoutSaturationControl(t *testing.T) {
	workerFairnessCategoryFairness(t, 0)
}

func workerFairnessCategoryFairness(t *testing.T, saturatedRounds int) {
	t.Helper()
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 1
	cfg.value.WorkspaceConcurrency = 1
	entered := make(chan string, 32)
	release := make(chan struct{})
	wait := func(ctx context.Context, kind string) error {
		entered <- kind
		select {
		case <-release:
			return errors.New("transient execution failure; candidate remains eligible")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		return hook.AttemptExecutionResult{}, wait(ctx, in.OperationID)
	})
	d.Coordinator = &workerCoord{
		prepare:  func(ctx context.Context, _ hook.WorkerRunInput) error { return wait(ctx, "preparing") },
		finalize: func(ctx context.Context, _ hook.WorkerRunInput) error { return wait(ctx, "finalize") },
	}
	d.ScanRepo = &workerScan{
		operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
			id := int64(1)
			if in.Status == entity.HookOperationRetryWait {
				id = 2
			}
			if k == entity.HookScanExpired {
				id = 3
			}
			return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(id, id)}}, nil
		},
		runs: func(k entity.HookScanKind, _ entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
			id := int64(4)
			if k == entity.HookScanFinalize {
				id = 5
			}
			return entity.HookScanPage[entity.HookRunCandidate]{Candidates: []entity.HookRunCandidate{{Key: workerCandidate(id, id).Key}}}, nil
		},
	}
	w, s := workerRoundFixture(t, d)
	seen := map[string]int{}
	for cycle := 0; cycle < 10; cycle++ {
		w.runRound(s)
		select {
		case kind := <-entered:
			seen[kind]++
		case <-time.After(time.Second):
			t.Fatal("idle slot was not filled")
		}
		for fullRound := 0; fullRound < saturatedRounds; fullRound++ {
			w.runRound(s)
		}
		release <- struct{}{}
		s.jobs.Wait()
	}
	if len(seen) != 5 {
		t.Fatalf("eligible categories starved across 10 completed jobs / %d scans: got %v, want all five", 10*(saturatedRounds+1), seen)
	}
}

// Expectation: advancing a raw cursor while saturated must not make the cold
// second page unreachable forever when the first-page task takes two rounds.
func TestHookWorkerFairnessColdPageFairnessAcrossSaturatedRounds(t *testing.T) {
	workerFairnessColdPageFairness(t, true)
}

func TestHookWorkerFairnessColdPageFairnessWithoutSaturationControl(t *testing.T) {
	workerFairnessColdPageFairness(t, false)
}

func workerFairnessColdPageFairness(t *testing.T, saturatedScan bool) {
	t.Helper()
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	cfg.value.WorkerConcurrency = 1
	cfg.value.WorkspaceConcurrency = 1
	cfg.value.ScanBatchSize = 1
	entered := make(chan int64, 32)
	release := make(chan struct{})
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		entered <- in.Key.WorkspaceID
		select {
		case <-release:
			return hook.AttemptExecutionResult{}, errors.New("transient failure keeps first row eligible")
		case <-ctx.Done():
			return hook.AttemptExecutionResult{}, ctx.Err()
		}
	})
	var coldScans int
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanDue || in.Status != entity.HookOperationPending {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		if in.Cursor != nil {
			coldScans++
			return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(2, 2)}}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{
			Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}, HasMore: true,
			NextCursor: &entity.HookScanCursor{Kind: k, ExecutionScope: in.ExecutionScope, Status: string(in.Status), Now: in.Now, At: in.Now, ID: 1},
		}, nil
	}}
	w, s := workerRoundFixture(t, d)
	seen := map[int64]int{}
	for cycle := 0; cycle < 10; cycle++ {
		w.runRound(s)
		select {
		case space := <-entered:
			seen[space]++
		case <-time.After(time.Second):
			t.Fatal("idle slot was not filled")
		}
		if saturatedScan {
			w.runRound(s)
		}
		release <- struct{}{}
		s.jobs.Wait()
	}
	if seen[2] == 0 {
		t.Fatalf("cold page scanned %d times but never dispatched: got %v, want workspace 2 progress", coldScans, seen)
	}
}

func TestHookWorkerWakeAcceleratesPollWithoutUnboundedScans(t *testing.T) {
	d := workerDeps()
	d.Config.(*workerConfig).value.ScanIntervalSeconds = 10
	executions := make(chan struct{}, 10)
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k != entity.HookScanExpired {
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		executions <- struct{}{}
		return hook.AttemptExecutionResult{RecoveryPending: true}, nil
	})
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- w.Run(context.Background()) }()
	t.Cleanup(func() { _ = w.Stop(context.Background()); <-done })
	select {
	case <-executions:
	case <-time.After(time.Second):
		t.Fatal("no initial scan")
	}
	event := entity.HookWakeEvent{Run: workerCandidate(1, 1).Key, OperationID: "op-1", ExecutionScope: "worker-local"}
	for i := 0; i < 1000; i++ {
		require.NoError(t, w.Wake(context.Background(), event))
	}
	require.Never(t, func() bool { return len(executions) > 0 }, 50*time.Millisecond, time.Millisecond)
	select {
	case <-executions:
	case <-time.After(2 * time.Second):
		t.Fatal("wake hint did not advance inspection before 10s poll")
	}
	require.Empty(t, executions)
}

func TestHookWorkerConfigFailureBetweenCandidatesStopsDispatch(t *testing.T) {
	d := workerDeps()
	base := d.Config
	var reads int
	var calls atomic.Int32
	d.Config = workerConfigFunc(func(ctx context.Context) (entity.HookRuntimeConfig, error) {
		reads++
		if reads >= 3 {
			return entity.HookRuntimeConfig{}, errors.New("raw secret")
		}
		return base.GetRuntimeConfig(ctx)
	})
	d.ScanRepo = &workerScan{operations: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1), workerCandidate(2, 2)}}, nil
	}}
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		calls.Add(1)
		return hook.AttemptExecutionResult{}, nil
	})
	w, s := workerRoundFixture(t, d)
	w.runRound(s)
	s.jobs.Wait()
	require.Equal(t, int32(1), calls.Load())
	require.Equal(t, 1, d.Observer.(*workerObservation).count(hook.WorkerConfigFailed))
}

func TestHookWorkerClockFailureNeverScansOrFallsBackToHost(t *testing.T) {
	for _, clock := range []hook.WorkerClock{&workerClock{err: errors.New("raw clock secret")}, workerBadClock{}} {
		d := workerDeps()
		d.Clock = clock
		var queries int
		d.ScanRepo = &workerScan{operations: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
			queries++
			return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
		}, runs: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
			queries++
			return entity.HookScanPage[entity.HookRunCandidate]{}, nil
		}}
		w, s := workerRoundFixture(t, d)
		w.runRound(s)
		require.Zero(t, queries)
		require.Equal(t, 5, d.Observer.(*workerObservation).count(hook.WorkerClockFailed))
	}
}

type workerBadClock struct{}

func (workerBadClock) Now(context.Context) (time.Time, error) { return time.Time{}, nil }

func TestHookWorkerEffectsRetrySuccessAndStopAbandonment(t *testing.T) {
	for _, stop := range []bool{false, true} {
		t.Run(map[bool]string{false: "retry_then_success", true: "cancel_delivery"}[stop], func(t *testing.T) {
			d := workerDeps()
			var calls atomic.Int32
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			d.Coordinator = &workerCoord{effects: func(context.Context, hook.WorkerEffects) error {
				n := calls.Add(1)
				if stop {
					cancel()
				}
				if n < 2 {
					return errors.New("raw effect")
				}
				return nil
			}}
			w, err := NewHookWorker(d)
			require.NoError(t, err)
			w.deliverEffects(ctx, entity.HookScanDue, hook.WorkerEffects{Key: workerCandidate(1, 1).Key, WakeEvaluation: true})
			obs := d.Observer.(*workerObservation)
			if stop {
				require.Equal(t, int32(1), calls.Load())
				require.Equal(t, 1, obs.count(hook.WorkerEffectsAbandoned))
			} else {
				require.Equal(t, int32(2), calls.Load())
				require.Zero(t, obs.count(hook.WorkerEffectsExhausted))
			}
		})
	}
}

func TestHookWorkerStopTimeoutKeepsGenerationOwned(t *testing.T) {
	d := workerDeps()
	entered := make(chan struct{})
	release := make(chan struct{})
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		close(entered)
		<-release
		return hook.AttemptExecutionResult{}, nil
	})
	d.ScanRepo = &workerScan{operations: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- w.Run(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("did not launch")
	}
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, w.Stop(stopped), context.Canceled)
	require.ErrorIs(t, w.Run(context.Background()), ErrHookWorkerRunning)
	close(release)
	require.NoError(t, w.Stop(context.Background()))
	require.NoError(t, <-done)
}

func TestHookWorkerPauseAndShrinkDoNotCancelExistingWork(t *testing.T) {
	d := workerDeps()
	cfg := d.Config.(*workerConfig)
	var running atomic.Int32
	d.Executor = workerExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		running.Add(1)
		<-ctx.Done()
		running.Add(-1)
		return hook.AttemptExecutionResult{}, ctx.Err()
	})
	d.ScanRepo = &workerScan{operations: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1), workerCandidate(2, 2)}}, nil
	}}
	w, s := workerRoundFixture(t, d)
	w.runRound(s)
	require.Eventually(t, func() bool { return running.Load() == 2 }, time.Second, time.Millisecond)
	cfg.update(func(c *entity.HookRuntimeConfig) { c.WorkerEnabled = false })
	w.runRound(s)
	require.Equal(t, int32(2), running.Load())
	cfg.update(func(c *entity.HookRuntimeConfig) {
		c.WorkerEnabled = true
		c.WorkerConcurrency = 1
		c.WorkspaceConcurrency = 1
	})
	w.runRound(s)
	require.Equal(t, int32(2), running.Load())
}

func TestHookWorkerMissingIDsAndInvalidEffectsAreObservable(t *testing.T) {
	for _, stage := range []string{"id", "nil_run", "wrong_run"} {
		t.Run(stage, func(t *testing.T) {
			d := workerDeps()
			var calls atomic.Int32
			d.ScanRepo = &workerScan{operations: func(entity.HookScanKind, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
				return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
			}}
			if stage == "id" {
				d.IDs = &workerIDs{err: errors.New("raw id secret")}
			}
			d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
				calls.Add(1)
				out := hook.AttemptExecutionResult{HookAttemptStoreResult: entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Changed: true}}}
				if stage == "wrong_run" {
					out.Run = &entity.HookStoredRun{State: entity.HookRunState{Key: workerCandidate(2, 2).Key}}
				}
				return out, nil
			})
			w, s := workerRoundFixture(t, d)
			w.runRound(s)
			s.jobs.Wait()
			if stage == "id" {
				require.Zero(t, calls.Load())
				require.Equal(t, 1, d.Observer.(*workerObservation).count(hook.WorkerIDFailed))
			} else {
				require.Equal(t, 1, d.Observer.(*workerObservation).count(hook.WorkerExecutionFailed))
			}
		})
	}
}

func TestHookWorkerForgedUserContextCannotChooseScope(t *testing.T) {
	d := workerDeps()
	queried := make(chan string, 5)
	d.ScanRepo = &workerScan{operations: func(_ entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		queried <- in.ExecutionScope
		return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
	}}
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	type scopeKey string
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), scopeKey("execution_scope"), "attacker-scope"))
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	select {
	case scope := <-queried:
		require.Equal(t, "worker-local", scope)
	case <-time.After(time.Second):
		t.Fatal("no scan")
	}
	require.NoError(t, w.Stop(context.Background()))
	require.NoError(t, <-done)
}

func TestHookWorkerConcurrentStartStopDoesNotLeakGeneration(t *testing.T) {
	w, err := NewHookWorker(workerDeps())
	require.NoError(t, err)
	for i := 0; i < 30; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		start := make(chan struct{})
		ran := make(chan error, 1)
		stopped := make(chan error, 1)
		go func() { <-start; ran <- w.Run(ctx) }()
		go func() { <-start; stopped <- w.Stop(context.Background()) }()
		close(start)
		require.NoError(t, <-stopped)
		cancel()
		select {
		case err := <-ran:
			require.True(t, err == nil || errors.Is(err, context.Canceled))
		case <-time.After(time.Second):
			t.Fatal("Run generation leaked across Stop/start race")
		}
		require.NoError(t, w.Stop(context.Background()))
	}
}

func TestHookWorkerStopDuringScanPreventsReturnedCandidateDispatch(t *testing.T) {
	d := workerDeps()
	entered := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	d.ScanRepo = &workerScan{operations: func(k entity.HookScanKind, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
		if k == entity.HookScanDue && in.Status == entity.HookOperationPending {
			close(entered)
			<-release
		}
		return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{workerCandidate(1, 1)}}, nil
	}}
	d.Executor = workerExecute(func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
		calls.Add(1)
		return hook.AttemptExecutionResult{}, nil
	})
	w, err := NewHookWorker(d)
	require.NoError(t, err)
	ran := make(chan error, 1)
	go func() { ran <- w.Run(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("scan did not start")
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, w.Stop(stopCtx), context.Canceled)
	close(release)
	require.NoError(t, <-ran)
	require.Zero(t, calls.Load())
}
