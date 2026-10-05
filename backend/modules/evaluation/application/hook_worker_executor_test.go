// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	app "github.com/coze-dev/coze-loop/backend/modules/evaluation/application"
	hook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
)

type queuedComposedScan struct {
	composedScan
	rows []entity.HookOperationCandidate
}

func (s queuedComposedScan) ScanDueOperations(_ context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	if in.Status != entity.HookOperationPending {
		return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
	}
	return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: s.rows}, nil
}

type queuedComposedConfig struct{ composedRuntime }

func (queuedComposedConfig) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	return entity.HookRuntimeConfig{WorkerEnabled: true, WorkerConcurrency: 1, WorkspaceConcurrency: 1, ScanBatchSize: 100, ScanIntervalSeconds: 1}, nil
}

type queuedComposedExecute func(context.Context, hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error)

func (f queuedComposedExecute) Execute(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
	return f(ctx, in)
}

func TestHookWorkerFairnessQueuedCandidateRechecksRealExecutor(t *testing.T) {
	for _, change := range []string{"new_run", "completed_elsewhere", "running_not_expired", "cancelled", "overflow_new_run", "overflow_completed_elsewhere", "overflow_running_not_expired", "overflow_cancelled"} {
		t.Run(change, func(t *testing.T) {
			stateChange := strings.TrimPrefix(change, "overflow_")
			key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
			created := time.Now().UTC().Truncate(time.Millisecond)
			block, err := aes.NewCipher(make([]byte, 32))
			require.NoError(t, err)
			aead, err := cipher.NewGCM(block)
			require.NoError(t, err)
			codec := hookinfra.NewStorageCodec(executorProtector{aead})
			snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "worker-composed", CreatedAt: created, Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), TimeoutSeconds: gptr.Of(int32(1)), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}, Context: &spi.HookRunContext{WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of("2"), RunID: gptr.Of("3"), RunMode: gptr.Of("submit"), Initiator: &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("queued"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{}}})
			require.NoError(t, err)
			protected, err := codec.EncodeSnapshot(context.Background(), "test-key", snapshot)
			require.NoError(t, err)
			store := &composedWorkerStore{run: entity.HookStoredRun{State: entity.HookRunState{Key: key, Status: entity.ExptStatus_Processing, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{ID: "before", Status: entity.HookOperationPending, Activated: true}, After: entity.HookOperation{Status: entity.HookOperationDisabled}}, Snapshot: protected, CreatedBy: "user", Mode: 1, PlanReady: true, Operations: []entity.HookStoredOperation{{HookOperationSeed: entity.HookOperationSeed{ID: 1, OperationID: "before", IdempotencyKey: "stable-key"}, Phase: entity.HookPhaseBefore, ActivatedAt: &created, OccurredAt: &created}}}}
			var sends atomic.Int32
			real, err := service.NewHookAttemptExecutor(store, codec, codec, executorTransport(func(context.Context, entity.HookTransportInput) entity.HookTransportResult {
				sends.Add(1)
				return executorSuccess()
			}), executorProjection(executorSafeProjection))
			require.NoError(t, err)
			held := make(chan struct{})
			release := make(chan struct{})
			outcomes := make(chan hook.AttemptExecutionResult, 10)
			wrapped := queuedComposedExecute(func(ctx context.Context, in hook.AttemptExecutionInput) (hook.AttemptExecutionResult, error) {
				if in.OperationID == "filler" {
					return hook.AttemptExecutionResult{}, nil
				}
				if in.OperationID == "blocker" {
					select {
					case held <- struct{}{}:
					case <-ctx.Done():
						return hook.AttemptExecutionResult{}, ctx.Err()
					}
					select {
					case <-release:
						return hook.AttemptExecutionResult{}, nil
					case <-ctx.Done():
						return hook.AttemptExecutionResult{}, ctx.Err()
					}
				}
				out, err := real.Execute(ctx, in)
				outcomes <- out
				return out, err
			})
			scan := queuedComposedScan{composedScan: composedScan{key: key}, rows: []entity.HookOperationCandidate{{Key: key, OperationID: "blocker", Phase: entity.HookPhaseBefore, ID: 2}, {Key: key, OperationID: "before", Phase: entity.HookPhaseBefore, ID: 1}}}
			if strings.HasPrefix(change, "overflow_") {
				scan.rows = append(scan.rows[:1], entity.HookOperationCandidate{Key: key, OperationID: "filler", Phase: entity.HookPhaseBefore, ID: 3}, scan.rows[1])
			}
			w, err := app.NewHookWorker(app.HookWorkerDependencies{ExecutionScope: "worker-composed", Owner: "process", ScanRepo: scan, Clock: composedRuntime{}, Config: queuedComposedConfig{}, Executor: wrapped, IDs: &composedIDs{}, Coordinator: composedCoord{delivered: make(chan hook.WorkerEffects, 4)}, Observer: composedObserver{events: make(chan hook.WorkerEvent, 32)}})
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() { done <- w.Run(context.Background()) }()
			t.Cleanup(func() { require.NoError(t, w.Stop(context.Background())); require.NoError(t, <-done) })
			select {
			case <-held:
			case <-time.After(time.Second):
				t.Fatal("first job did not reserve the only slot")
			}
			store.mu.Lock()
			switch stateChange {
			case "new_run":
				store.run.State.Key.RunID = 4
			case "completed_elsewhere":
				store.run.State.Before.Status = entity.HookOperationSucceeded
				store.run.State.Before.Activated = false
				store.run.State.Gate = entity.HookGateReady
			case "running_not_expired":
				store.run.State.Before.Status = entity.HookOperationRunning
			case "cancelled":
				store.run.State.Before.Status = entity.HookOperationFailed
				store.run.State.Before.Activated = false
				store.run.State.Gate = entity.HookGateClosed
			}
			store.mu.Unlock()
			close(release)
			require.NoError(t, w.Wake(context.Background(), entity.HookWakeEvent{Run: key, OperationID: "before", ExecutionScope: "worker-composed"}))
			select {
			case out := <-outcomes:
				require.Equal(t, stateChange == "running_not_expired", out.RecoveryPending)
			case <-time.After(3 * time.Second):
				t.Fatal("retained candidate was not retried through the executor")
			}
			require.NoError(t, w.Stop(context.Background()))
			store.mu.Lock()
			defer store.mu.Unlock()
			require.Zero(t, sends.Load())
			require.Zero(t, store.claims)
			require.Zero(t, store.completes)
			if stateChange == "running_not_expired" {
				require.Equal(t, 1, store.recoveries)
			}
		})
	}
}

// Only the external DB boundary is replaced. Worker, Executor, snapshot/request
// codec, completion-window validation and domain result transitions remain real.
type composedWorkerStore struct {
	repo.IHookRepo
	mu                            sync.Mutex
	run                           entity.HookStoredRun
	claim                         *entity.HookAttemptClaim
	denied                        bool
	claims, completes, recoveries int
	projected                     []byte
}

func (s *composedWorkerStore) view() *entity.HookStoredRun {
	v := s.run
	v.Operations = append([]entity.HookStoredOperation(nil), s.run.Operations...)
	return &v
}
func (s *composedWorkerStore) GetRun(_ context.Context, k entity.HookRunKey) (*entity.HookStoredRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if k != s.run.State.Key {
		return nil, entity.ErrHookStoreConflict
	}
	return s.view(), nil
}
func composedClock() *entity.HookClockAnchor {
	start := time.Now()
	dbNow := time.Now().UTC().Truncate(time.Millisecond)
	return &entity.HookClockAnchor{DBTime: dbNow, LocalBefore: start, LocalAfter: time.Now(), Precision: time.Millisecond}
}
func (s *composedWorkerStore) ClaimAttempt(_ context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claims++
	if in.Validate() != nil || in.Key != s.run.State.Key || in.ExecutionScope != s.run.Snapshot.ExecutionScope || s.denied {
		return entity.HookAttemptStoreResult{}, entity.ErrHookStoreConflict
	}
	clock := composedClock()
	now := clock.DBTime
	if s.run.State.Before.Status != entity.HookOperationPending {
		return entity.HookAttemptStoreResult{}, entity.ErrHookStoreConflict
	}
	s.run.State.Before = entity.HookOperation{ID: "before", Status: entity.HookOperationRunning, Activated: true, Attempt: 1, Generation: 1, LeaseUntil: now.Add(6 * time.Second), AttemptDeadline: now.Add(time.Second), Deadline: now.Add(time.Second)}
	s.run.Operations[0].Version++
	s.claim = &entity.HookAttemptClaim{HookAttemptIdentity: entity.HookAttemptIdentity{Token: entity.HookClaimToken{Run: in.Key, OperationID: "before", Phase: entity.HookPhaseBefore, Attempt: 1, Generation: 1}, Owner: in.Owner, DeliveryID: "delivery-1"}, Version: s.run.Operations[0].Version, IdempotencyKey: "stable-key", StartedAt: now, LeaseUntil: s.run.State.Before.LeaseUntil, AttemptDeadline: s.run.State.Before.AttemptDeadline, Deadline: s.run.State.Before.Deadline}
	return entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: s.view(), Changed: true}, Clock: clock, Claim: s.claim}, nil
}
func (s *composedWorkerStore) ReadAttempt(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: s.view()}, Clock: composedClock(), Claim: s.claim}, nil
}
func (s *composedWorkerStore) CompleteAttempt(_ context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completes++
	if err := in.Validate(); err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	changed, err := entity.CompleteHookOperation(&s.run.State, entity.HookResultInput{Token: in.Token, Outcome: in.Outcome, Config: in.Config, CompletedAt: in.CompletedAt, CommitAt: time.Now().UTC(), RetryAfter: in.RetryAfter})
	if err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	s.run.State = changed.State
	s.projected = append([]byte(nil), in.ResultRedacted...)
	return entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: s.view(), Changed: changed.Changed, Effects: changed.Effects}}, nil
}
func (s *composedWorkerStore) RecoverExpiredAttempt(context.Context, entity.HookRecoverExpiredAttemptInput) (entity.HookAttemptStoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recoveries++
	return entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: s.view()}}, nil
}

type composedScan struct {
	repo.IHookScanRepo
	key entity.HookRunKey
}

func (s composedScan) ScanDueOperations(_ context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	if in.Status != entity.HookOperationPending {
		return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
	}
	return entity.HookScanPage[entity.HookOperationCandidate]{Candidates: []entity.HookOperationCandidate{{Key: s.key, OperationID: "before", Phase: entity.HookPhaseBefore, ID: 1}}}, nil
}
func (composedScan) ScanExpiredOperations(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	return entity.HookScanPage[entity.HookOperationCandidate]{}, nil
}
func (composedScan) ScanPreparingPlans(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	return entity.HookScanPage[entity.HookRunCandidate]{}, nil
}
func (composedScan) ScanPendingFinalizations(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	return entity.HookScanPage[entity.HookRunCandidate]{}, nil
}

type composedRuntime struct{}

func (composedRuntime) GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error) {
	return entity.HookRuntimeConfig{WorkerEnabled: true, WorkerConcurrency: 8, WorkspaceConcurrency: 2, ScanIntervalSeconds: 1, ScanBatchSize: 100}, nil
}
func (composedRuntime) Now(context.Context) (time.Time, error) {
	return time.Now().UTC().Truncate(time.Millisecond), nil
}

type composedIDs struct {
	idgen.IIDGenerator
	n atomic.Int64
}

func (i *composedIDs) GenID(context.Context) (int64, error) { return i.n.Add(1), nil }

type composedCoord struct{ delivered chan hook.WorkerEffects }

func (c composedCoord) PreparePlan(context.Context, hook.WorkerRunInput) error {
	return errors.New("unexpected preparing candidate")
}
func (c composedCoord) FinalizeRun(context.Context, hook.WorkerRunInput) error {
	return errors.New("unexpected finalize candidate")
}
func (c composedCoord) ApplyEffects(_ context.Context, e hook.WorkerEffects) error {
	c.delivered <- e
	return nil
}

type composedObserver struct{ events chan hook.WorkerEvent }

func (o composedObserver) Observe(e hook.WorkerEvent) {
	select {
	case o.events <- e:
	default:
	}
}

func TestHookWorkerRealExecutorClaimCompletionAndRecovery(t *testing.T) {
	for _, scenario := range []string{"claim_denied", "success", "uncertain"} {
		t.Run(scenario, func(t *testing.T) {
			key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
			created := time.Now().UTC().Truncate(time.Millisecond)
			block, err := aes.NewCipher(make([]byte, 32))
			require.NoError(t, err)
			aead, err := cipher.NewGCM(block)
			require.NoError(t, err)
			codec := hookinfra.NewStorageCodec(executorProtector{aead})
			snapshot, err := entity.NewHookRunSnapshot(entity.HookRunSnapshotInput{Key: key, ExecutionScope: "worker-composed", CreatedAt: created, Config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(true), TimeoutSeconds: gptr.Of(int32(1)), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}, Context: &spi.HookRunContext{WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of("2"), RunID: gptr.Of("3"), RunMode: gptr.Of("submit"), Initiator: &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")}, Experiment: &spi.HookExperimentRef{Name: gptr.Of("composed"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{}}})
			require.NoError(t, err)
			protected, err := codec.EncodeSnapshot(context.Background(), "test-key", snapshot)
			require.NoError(t, err)
			store := &composedWorkerStore{denied: scenario == "claim_denied", run: entity.HookStoredRun{State: entity.HookRunState{Key: key, Status: entity.ExptStatus_Processing, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{ID: "before", Status: entity.HookOperationPending, Activated: true}, After: entity.HookOperation{Status: entity.HookOperationDisabled}}, Snapshot: protected, CreatedBy: "user", Mode: 1, PlanReady: true, Operations: []entity.HookStoredOperation{{HookOperationSeed: entity.HookOperationSeed{ID: 1, OperationID: "before", IdempotencyKey: "stable-key"}, Phase: entity.HookPhaseBefore, ActivatedAt: &created, OccurredAt: &created}}}}
			var sends atomic.Int32
			executor, err := service.NewHookAttemptExecutor(store, codec, codec, executorTransport(func(_ context.Context, in entity.HookTransportInput) entity.HookTransportResult {
				sends.Add(1)
				require.Equal(t, "before", in.Request.GetOperationID())
				require.Equal(t, "stable-key", in.Request.GetIdempotencyKey())
				require.Positive(t, in.Remaining)
				if scenario == "uncertain" {
					return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain}, LocalCompletedAt: time.Now()}
				}
				return executorSuccess()
			}), executorProjection(executorSafeProjection))
			require.NoError(t, err)
			coordination := composedCoord{delivered: make(chan hook.WorkerEffects, 4)}
			observer := composedObserver{events: make(chan hook.WorkerEvent, 32)}
			worker, err := app.NewHookWorker(app.HookWorkerDependencies{ExecutionScope: "worker-composed", Owner: "process-composed", ScanRepo: composedScan{key: key}, Clock: composedRuntime{}, Config: composedRuntime{}, Executor: executor, IDs: &composedIDs{}, Coordinator: coordination, Observer: observer})
			require.NoError(t, err)
			done := make(chan error, 1)
			go func() { done <- worker.Run(context.Background()) }()
			t.Cleanup(func() { require.NoError(t, worker.Stop(context.Background())); require.NoError(t, <-done) })
			require.Eventually(t, func() bool {
				store.mu.Lock()
				defer store.mu.Unlock()
				if scenario == "claim_denied" {
					return store.claims > 0
				}
				if scenario == "uncertain" {
					return store.recoveries > 0
				}
				return store.completes > 0
			}, time.Second, time.Millisecond)
			for i := 0; i < 100; i++ {
				require.NoError(t, worker.Wake(context.Background(), entity.HookWakeEvent{Run: key, OperationID: "forged-other-operation", ExecutionScope: "worker-composed"}))
			}
			if scenario == "success" {
				select {
				case effect := <-coordination.delivered:
					require.True(t, effect.WakeEvaluation)
					require.Equal(t, key, effect.Key)
				case <-time.After(time.Second):
					t.Fatal("missing completion coordination")
				}
			}
			require.NoError(t, worker.Stop(context.Background()))
			store.mu.Lock()
			defer store.mu.Unlock()
			require.Equal(t, 1, store.claims)
			if scenario == "claim_denied" {
				require.Zero(t, sends.Load())
				require.Zero(t, store.completes)
				require.Empty(t, coordination.delivered)
			} else {
				require.Equal(t, int32(1), sends.Load())
			}
			if scenario == "success" {
				require.Equal(t, entity.HookGateReady, store.run.State.Gate)
				require.JSONEq(t, `{"public":"redacted"}`, string(store.projected))
			}
			if scenario == "uncertain" {
				require.Equal(t, 1, store.recoveries)
				require.Zero(t, store.completes)
				require.Equal(t, entity.HookOperationRunning, store.run.State.Before.Status)
				require.Empty(t, coordination.delivered)
			}
		})
	}
}
