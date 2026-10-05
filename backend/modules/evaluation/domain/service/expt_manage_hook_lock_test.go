// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Stateful SET NX / compare-delete boundary, with no Redis socket or timers.
type managerStateLock struct {
	lock.ILocker
	mu           sync.Mutex
	held         map[string]string
	owners       []string
	forceDeletes int
}

func (l *managerStateLock) BackoffLockWithValue(ctx context.Context, key, value string, _, _ time.Duration) (bool, string, error) {
	if err := ctx.Err(); err != nil {
		return false, "", err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if old := l.held[key]; old != "" {
		return false, old, nil
	}
	l.held[key] = value
	l.owners = append(l.owners, value)
	return true, value, nil
}
func (l *managerStateLock) LockBackoff(ctx context.Context, key string, ttl, wait time.Duration) (bool, error) {
	ok, _, err := l.BackoffLockWithValue(ctx, key, "legacy-holder", ttl, wait)
	return ok, err
}
func (l *managerStateLock) UnlockWithValue(ctx context.Context, key, value string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.held[key] != value {
		return false, nil
	}
	delete(l.held, key)
	return true, nil
}
func (l *managerStateLock) UnlockForce(context.Context, string) (bool, error) {
	l.forceDeletes++
	return false, errors.New("unconditional delete forbidden")
}
func (l *managerStateLock) Exists(_ context.Context, key string) (bool, error) {
	return l.owner(key) != "", nil
}
func (l *managerStateLock) owner(key string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.held[key]
}
func (l *managerStateLock) replace(key, value string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if value == "" {
		delete(l.held, key)
	} else {
		l.held[key] = value
	}
}

type managerLockStore struct {
	repo.IHookRepo
	t                                        *testing.T
	logs                                     map[int64]*entity.ExptRunLog
	runs                                     map[int64]*entity.HookStoredRun
	latest                                   int64
	writes, reads, appends                   int
	writeErr, readErr, confirmErr, appendErr error
	primaryReads                             []bool
	commitOnError                            bool
	onWrite                                  func()
}

func (s *managerLockStore) ReadRunInitialization(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	s.reads++
	s.primaryReads = append(s.primaryReads, contexts.CtxWriteDB(ctx))
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.readErr != nil {
		return nil, s.readErr
	}
	if s.writes > 0 && s.confirmErr != nil {
		return nil, s.confirmErr
	}
	return &entity.HookRunInitialization{LatestRunID: s.latest, ConfigRevision: "revision", HooksEnabled: true, RunLog: s.logs[key.RunID], Managed: s.runs[key.RunID] != nil}, nil
}
func (s *managerLockStore) CreateRunWithoutHooks(context.Context, *entity.ExptRunLog, int64, string) (bool, error) {
	return false, errors.New("unexpected legacy write")
}
func (s *managerLockStore) CreateRunWithHooks(_ context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	s.writes++
	if s.onWrite != nil {
		s.onWrite()
	}
	if s.writeErr != nil && !s.commitOnError {
		return entity.HookStoreResult{}, s.writeErr
	}
	log := *in.RunLog
	s.logs[in.Key.RunID] = &log
	s.latest = in.Key.RunID
	run := &entity.HookStoredRun{State: entity.HookRunState{Key: in.Key}, CreatedBy: log.CreatedBy, Mode: entity.ExptRunMode(log.Mode), Snapshot: in.Snapshot}
	if in.Before != nil {
		run.Operations = append(run.Operations, entity.HookStoredOperation{HookOperationSeed: *in.Before, Phase: entity.HookPhaseBefore})
	}
	s.runs[in.Key.RunID] = run
	return entity.HookStoreResult{Run: run, Changed: true}, s.writeErr
}
func (s *managerLockStore) GetRun(_ context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
	return s.runs[key.RunID], nil
}
func (s *managerLockStore) AppendHookRunItems(context.Context, entity.HookAppendRunItemsInput) error {
	s.appends++
	return s.appendErr
}

type managerLockConfig struct {
	repo.IHookConfigRepo
	err      error
	revision string
}

func (c *managerLockConfig) GetConfig(context.Context, hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	return &entity.HookConfigRecord{Revision: c.revision, Config: managerEnabledConfig(true, false)}, c.err
}

type managerLockIdentity struct {
	fail      bool
	onResolve func()
}

func (i *managerLockIdentity) ResolveInitiator(_ context.Context, id string) (*spi.HookInitiator, error) {
	if i.onResolve != nil {
		i.onResolve()
	}
	if i.fail {
		return nil, errors.New("identity failure")
	}
	return &spi.HookInitiator{UserID: &id, IdentityType: gptr.Of("fornax_user")}, nil
}

type managerLockProtector struct{ fail bool }

func (p *managerLockProtector) Protect(_ context.Context, _ string, value []byte) ([]byte, error) {
	if p.fail {
		return nil, errors.New("encode failure")
	}
	return append([]byte(nil), value...), nil
}
func (*managerLockProtector) Unprotect(_ context.Context, _ string, value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

type managerLockFixture struct {
	base      *ExptMangerImpl
	manager   IExptManager
	locks     *managerStateLock
	store     *managerLockStore
	identity  *managerLockIdentity
	config    *managerLockConfig
	protector *managerLockProtector
	wake      *managerHookWake
	key       string
}

func newManagerLockFixture(t *testing.T) *managerLockFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	base := newTestExptManager(ctrl)
	l := &managerStateLock{held: map[string]string{}}
	base.mutex = l
	base.mtr.(*metricmocks.MockExptMetric).EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	store := &managerLockStore{t: t, logs: map[int64]*entity.ExptRunLog{}, runs: map[int64]*entity.HookStoredRun{}}
	next := int64(100)
	base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) { next++; return next, nil }).AnyTimes()
	base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
		return &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: store.latest, Name: "lock-test", ExptType: entity.ExptType_Offline, EvalSetID: 71, EvalSetVersionID: 71, EvalConf: &entity.EvaluationConfiguration{}}, nil
	}).AnyTimes()
	id := &managerLockIdentity{}
	config := &managerLockConfig{revision: "revision"}
	protector := &managerLockProtector{}
	wake := &managerHookWake{}
	m, err := NewExptManagerWithHooks(base, ExptManagerHookDependencies{Initialization: store, Runs: store, Configs: config, Codec: hookinfra.NewStorageCodec(protector), Identity: id, Runtime: &managerHookRuntime{admission: true}, Wake: wake, ExecutionScope: "local", SnapshotKeyID: "key"})
	require.NoError(t, err)
	return &managerLockFixture{base: base, manager: m, locks: l, store: store, identity: id, config: config, protector: protector, wake: wake, key: base.makeExptMutexLockKey(20)}
}

func TestHookManagerLockWriteBeforeFailureCanRetry(t *testing.T) {
	for _, retryItems := range []bool{false, true} {
		t.Run(map[bool]string{false: "LogRun", true: "LogRetryItemsRun"}[retryItems], func(t *testing.T) {
			f := newManagerLockFixture(t)
			f.identity.fail = true
			call := func() error {
				if retryItems {
					_, _, err := f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "user"})
					return err
				}
				return f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "user"})
			}
			require.Error(t, call())
			require.Zero(t, f.store.writes)
			f.identity.fail = false
			require.NoError(t, call(), "corrected initialization must retry immediately without waiting for the old TTL")
			require.Equal(t, 1, f.store.writes)
			require.NotEmpty(t, f.locks.owner(f.key))
			require.Zero(t, f.locks.forceDeletes)
		})
	}
}

func (f *managerLockFixture) call(ctx context.Context, retry bool) (int64, bool, error) {
	if retry {
		return f.manager.LogRetryItemsRun(ctx, 20, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "user"})
	}
	err := f.manager.LogRun(ctx, 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "user"})
	return 30, false, err
}

func TestHookManagerLockPrewriteFailuresReleaseOnlyNewOwner(t *testing.T) {
	for _, retry := range []bool{false, true} {
		for _, reason := range []string{"read", "config", "revision", "encode"} {
			t.Run(fmt.Sprintf("retry=%v/%s", retry, reason), func(t *testing.T) {
				f := newManagerLockFixture(t)
				switch reason {
				case "read":
					f.store.readErr = errors.New("read failure")
				case "config":
					f.config.err = errors.New("config failure")
				case "revision":
					f.config.revision = "stale"
				case "encode":
					f.protector.fail = true
				}
				_, _, err := f.call(context.Background(), retry)
				require.Error(t, err)
				require.Empty(t, f.locks.owner(f.key))
				require.Zero(t, f.store.writes)
				if reason == "read" {
					require.Empty(t, f.locks.owners)
				}
				f.store.readErr = nil
				f.config.err = nil
				f.config.revision = "revision"
				f.protector.fail = false
				_, _, err = f.call(context.Background(), retry)
				require.NoError(t, err)
				require.Equal(t, 1, f.store.writes)
				require.NotEmpty(t, f.locks.owner(f.key))
				require.Zero(t, f.locks.forceDeletes)
			})
		}
	}
}

func TestHookManagerLockRollbackAndConflictCanRetry(t *testing.T) {
	for _, retry := range []bool{false, true} {
		for _, cause := range []error{entity.ErrHookStoreConflict, errors.New("transaction rolled back")} {
			t.Run(fmt.Sprintf("retry=%v/%v", retry, cause), func(t *testing.T) {
				f := newManagerLockFixture(t)
				f.store.writeErr = cause
				_, _, err := f.call(context.Background(), retry)
				require.Error(t, err)
				require.Empty(t, f.locks.owner(f.key))
				require.Empty(t, f.store.logs)
				require.GreaterOrEqual(t, f.store.reads, 2)
				for _, primary := range f.store.primaryReads {
					require.True(t, primary)
				}
				f.store.writeErr = nil
				_, _, err = f.call(context.Background(), retry)
				require.NoError(t, err)
				require.Len(t, f.locks.owners, 2)
				require.NotEqual(t, f.locks.owners[0], f.locks.owners[1], "owner token must be per acquisition, not just per Run")
				require.Equal(t, 1, len(f.store.logs))
				require.Zero(t, f.locks.forceDeletes)
			})
		}
	}
}

func TestHookManagerLockCommitAndWakeFailureKeepOwner(t *testing.T) {
	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprint(retry), func(t *testing.T) {
			f := newManagerLockFixture(t)
			f.wake.err = errors.New("MQ unavailable")
			run, _, err := f.call(context.Background(), retry)
			require.NoError(t, err)
			require.NotNil(t, f.store.logs[run])
			require.NotEmpty(t, f.wake.events)
			owner := f.locks.owner(f.key)
			require.NotEmpty(t, owner)
			parsed, err := managerHookLockRunID(owner)
			require.NoError(t, err)
			require.Equal(t, run, parsed)
			require.Equal(t, 1, len(f.store.logs))
			require.Zero(t, f.locks.forceDeletes)
		})
	}
}

func TestHookManagerLockUncertainCommitRetainsOrConfirmsAbsence(t *testing.T) {
	for _, retry := range []bool{false, true} {
		for _, scenario := range []string{"committed", "read_failed", "absent", "latest_without_row"} {
			t.Run(fmt.Sprintf("retry=%v/%s", retry, scenario), func(t *testing.T) {
				f := newManagerLockFixture(t)
				f.store.writeErr = errors.New("commit receipt lost")
				if scenario == "committed" {
					f.store.commitOnError = true
				}
				if scenario == "read_failed" {
					f.store.confirmErr = errors.New("primary unavailable")
				}
				if scenario == "latest_without_row" {
					f.store.onWrite = func() {
						if retry {
							f.store.latest = 101
						} else {
							f.store.latest = 30
						}
					}
				}
				_, _, err := f.call(context.Background(), retry)
				require.Error(t, err)
				if scenario == "absent" {
					require.Empty(t, f.locks.owner(f.key))
				} else {
					require.NotEmpty(t, f.locks.owner(f.key))
				}
				require.GreaterOrEqual(t, f.store.reads, 2)
				require.True(t, f.store.primaryReads[len(f.store.primaryReads)-1])
				require.Zero(t, f.locks.forceDeletes)
				if scenario == "committed" {
					owner := f.locks.owner(f.key)
					originalRun := f.store.latest
					f.store.writeErr = nil
					run, reused, err := f.call(context.Background(), retry)
					require.NoError(t, err)
					require.Equal(t, originalRun, run)
					require.Equal(t, retry, reused)
					require.Equal(t, owner, f.locks.owner(f.key))
					require.Equal(t, 1, f.store.writes)
				}
			})
		}
	}
}

func TestHookManagerLockLateFailureCannotDeleteSuccessor(t *testing.T) {
	for _, retry := range []bool{false, true} {
		for _, when := range []string{"prewrite", "write_error"} {
			t.Run(fmt.Sprintf("retry=%v/%s", retry, when), func(t *testing.T) {
				f := newManagerLockFixture(t)
				const successor = "hook_run:30:00000000000000000000000000000001"
				if when == "prewrite" {
					f.identity.fail = true
					f.identity.onResolve = func() { f.locks.replace(f.key, successor) }
				} else {
					f.store.writeErr = errors.New("write failed")
					f.store.onWrite = func() { f.locks.replace(f.key, successor) }
				}
				_, _, err := f.call(context.Background(), retry)
				require.Error(t, err)
				require.Equal(t, successor, f.locks.owner(f.key))
				require.Zero(t, f.locks.forceDeletes)
			})
		}
	}
}

func TestHookManagerLockCancellationUsesBoundedCleanupContext(t *testing.T) {
	for _, when := range []string{"prewrite", "write_error"} {
		t.Run(when, func(t *testing.T) {
			f := newManagerLockFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if when == "prewrite" {
				f.identity.fail = true
				f.identity.onResolve = cancel
			} else {
				f.store.writeErr = context.Canceled
				f.store.onWrite = cancel
			}
			_, _, err := f.call(ctx, false)
			require.Error(t, err)
			require.Empty(t, f.locks.owner(f.key))
			require.Zero(t, f.locks.forceDeletes)
		})
	}
}

func TestHookManagerLockReplayAndReusedFailureNeverRelease(t *testing.T) {
	f := newManagerLockFixture(t)
	run, _, err := f.call(context.Background(), true)
	require.NoError(t, err)
	owner := f.locks.owner(f.key)
	f.store.appendErr = entity.ErrHookAdmissionDenied
	_, _, err = f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{72}, &entity.Session{UserID: "other-user"})
	require.Error(t, err)
	require.Equal(t, 1, f.store.appends)
	require.Equal(t, owner, f.locks.owner(f.key))
	require.Len(t, f.locks.owners, 1)
	err = f.manager.LogRun(context.Background(), 20, run, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "user"})
	require.NoError(t, err)
	require.Equal(t, owner, f.locks.owner(f.key))
	require.Len(t, f.locks.owners, 1)
	err = f.manager.LogRun(context.Background(), 20, run, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "wrong-user"})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Equal(t, owner, f.locks.owner(f.key))
	require.Zero(t, f.locks.forceDeletes)
}

func TestHookManagerLockNilSessionDoesNotAcquire(t *testing.T) {
	f := newManagerLockFixture(t)
	require.Error(t, f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeSubmit, 10, nil, nil))
	_, _, err := f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, nil, nil)
	require.Error(t, err)
	require.Empty(t, f.locks.owners)
	require.Zero(t, f.store.writes)
}
