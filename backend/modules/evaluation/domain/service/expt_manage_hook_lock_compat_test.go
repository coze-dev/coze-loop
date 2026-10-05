// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type managerCompatStore struct {
	*managerLockStore
	enabled     bool
	online      bool
	legacySaves int
}

func (s *managerCompatStore) ReadRunInitialization(ctx context.Context, k entity.HookRunKey) (*entity.HookRunInitialization, error) {
	out, err := s.managerLockStore.ReadRunInitialization(ctx, k)
	if out != nil {
		out.HooksEnabled = s.enabled
	}
	return out, err
}
func (s *managerCompatStore) CreateRunWithoutHooks(_ context.Context, l *entity.ExptRunLog, latest int64, revision string) (bool, error) {
	s.writes++
	if s.onWrite != nil {
		s.onWrite()
	}
	if s.writeErr != nil && !s.commitOnError {
		return false, s.writeErr
	}
	if s.enabled || s.latest != latest || revision != "revision" {
		return false, entity.ErrHookStoreConflict
	}
	v := *l
	s.logs[l.ExptRunID] = &v
	s.latest = l.ExptRunID
	return true, s.writeErr
}
func (s *managerCompatStore) CreateRunWithHooks(ctx context.Context, in entity.HookCreateRunInput) (entity.HookStoreResult, error) {
	out, err := s.managerLockStore.CreateRunWithHooks(ctx, in)
	if err == nil {
		out.Run.State.Status = entity.ExptStatus_Pending
		out.Run.State.Gate = entity.HookGateWaiting
		out.Run.State.Finalize = entity.HookFinalizeNone
		out.Run.State.Before = entity.HookOperation{ID: in.Before.OperationID, Status: entity.HookOperationPending}
		out.Run.State.After = entity.HookOperation{Status: entity.HookOperationDisabled}
	}
	return out, err
}
func (s *managerCompatStore) AppendHookRunItems(_ context.Context, in entity.HookAppendRunItemsInput) error {
	if s.appendErr != nil {
		return s.appendErr
	}
	l, r := s.logs[in.Key.RunID], s.runs[in.Key.RunID]
	if l == nil || r == nil || r.State.Key != in.Key || s.latest != in.Key.RunID || r.Snapshot.ExecutionScope != in.ExecutionScope || r.Version != in.ExpectedVersion {
		return entity.ErrHookStoreConflict
	}
	if r.State.Gate == entity.HookGateClosed || r.State.Finalize != entity.HookFinalizeNone || (r.State.Status != entity.ExptStatus_Pending && r.State.Status != entity.ExptStatus_Processing) {
		return entity.ErrHookAdmissionDenied
	}
	if err := l.AppendItemIDs(in.ItemIDs); err != nil {
		return err
	}
	s.appends++
	r.Version++
	return nil
}

type managerCompatLogs struct {
	repo.IExptRunLogRepo
	s *managerCompatStore
}

func (r managerCompatLogs) Get(_ context.Context, exptID, runID int64) (*entity.ExptRunLog, error) {
	v := r.s.logs[runID]
	if v == nil || v.ExptID != exptID {
		return nil, entity.ErrHookStoreMissing
	}
	copy := *v
	copy.ItemIds = append([]entity.ExptRunLogItems(nil), v.ItemIds...)
	return &copy, nil
}
func (r managerCompatLogs) Create(_ context.Context, v *entity.ExptRunLog) error {
	copy := *v
	r.s.logs[v.ExptRunID] = &copy
	r.s.writes++
	return nil
}
func (r managerCompatLogs) Save(_ context.Context, v *entity.ExptRunLog) error {
	copy := *v
	r.s.logs[v.ExptRunID] = &copy
	r.s.legacySaves++
	return nil
}

type managerCompatExpts struct {
	repo.IExperimentRepo
	s *managerCompatStore
}

func (r managerCompatExpts) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	kind := entity.ExptType_Offline
	if r.s.online {
		kind = entity.ExptType_Online
	}
	return &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: r.s.latest, Name: "lock-boundary", ExptType: kind, EvalSetID: 71, EvalSetVersionID: 71, EvalConf: &entity.EvaluationConfiguration{}}, nil
}

func (r managerCompatExpts) Update(_ context.Context, v *entity.Experiment) error {
	if v.LatestRunID > 0 {
		r.s.latest = v.LatestRunID
	}
	return nil
}

func newManagerCompatFixture(t *testing.T, enabled bool) (*managerLockFixture, *managerCompatStore) {
	f := newManagerLockFixture(t)
	s := &managerCompatStore{managerLockStore: f.store, enabled: enabled}
	e := f.manager.(*ExptMangerImpl)
	e.hooks.Initialization = s
	e.hooks.Runs = s
	f.base.runLogRepo = managerCompatLogs{s: s}
	e.runLogRepo = managerCompatLogs{s: s}
	f.base.exptRepo = managerCompatExpts{s: s}
	e.exptRepo = f.base.exptRepo
	return f, s
}
func TestHookManagerLockCompatRejectsOtherRunModes(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeAppend, entity.EvaluationModeTrialRun} {
		for _, enabled := range []bool{false, true} {
			for _, aware := range []bool{false, true} {
				t.Run(fmt.Sprintf("mode=%d/enabled=%t/aware=%t", mode, enabled, aware), func(t *testing.T) {
					f, s := newManagerCompatFixture(t, enabled)
					s.online = mode == entity.EvaluationModeAppend
					var first IExptManager = f.base
					if aware {
						first = f.manager
					}
					require.NoError(t, first.LogRun(context.Background(), 20, 30, mode, 10, []int64{71}, &entity.Session{UserID: "user"}))
					owner := f.locks.owner(f.key)
					require.NotEmpty(t, owner)
					require.EqualValues(t, mode, s.logs[30].Mode)
					run, reused, err := f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{72}, &entity.Session{UserID: "retry-user"})
					t.Logf("mode=%d aware=%t owner=%s returnedRun=%d reused=%t error=%v items=%v appends=%d", mode, aware, owner, run, reused, err, s.logs[30].GetItemIDs(), s.appends)
					require.Equal(t, owner, f.locks.owner(f.key))
					require.Error(t, err, "RetryItems must preserve the old exclusion of an active non-RetryItems Run")
					require.Equal(t, []int64{71}, s.logs[30].GetItemIDs())
					require.Zero(t, s.appends)
				})
			}
		}
	}
}
func TestHookManagerLockCompatNoHookCrossConstructors(t *testing.T) {
	for _, policy := range []struct {
		name   string
		config *entity.LifecycleHookConf
	}{{name: "nil"}, {name: "disabled", config: &entity.LifecycleHookConf{Before: &entity.HookConfig{Enabled: gptr.Of(false)}, After: &entity.HookConfig{Enabled: gptr.Of(false)}}}} {
		for _, pair := range []struct {
			name                    string
			firstAware, secondAware bool
		}{
			{"legacy_to_legacy", false, false}, {"aware_to_aware", true, true}, {"legacy_to_aware", false, true}, {"aware_to_legacy", true, false},
		} {
			t.Run(policy.name+"/"+pair.name, func(t *testing.T) {
				projection := newHookManagerFixture(t, policy.config)
				projection.expectRead(0, nil)
				initial, err := projection.deps.Initialization.ReadRunInitialization(context.Background(), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30})
				require.NoError(t, err)
				require.False(t, initial.HooksEnabled, "production codec and repository must identify the raw no-Hook configuration")
				f, s := newManagerCompatFixture(t, initial.HooksEnabled)
				var first, second IExptManager = f.base, f.base
				if pair.firstAware {
					first = f.manager
				}
				if pair.secondAware {
					second = f.manager
				}
				run, reused, err := first.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "user"})
				require.NoError(t, err)
				require.False(t, reused)
				owner := f.locks.owner(f.key)
				require.NotEmpty(t, owner)
				if pair.firstAware {
					require.NoError(t, first.LogRun(context.Background(), 20, run, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "user"}))
					require.Equal(t, owner, f.locks.owner(f.key), "same-Run replay must not replace the owner")
					require.Len(t, f.locks.owners, 1)
				}
				_, _, err = second.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{71}, &entity.Session{UserID: "user"})
				require.Error(t, err, "duplicate items must fail without dropping the active owner")
				require.Equal(t, []int64{71}, s.logs[run].GetItemIDs())
				require.Equal(t, owner, f.locks.owner(f.key))
				got, reused, err := second.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{72}, &entity.Session{UserID: "user"})
				t.Logf("policy=%s pair=%s owner=%s initialRun=%d got=%d reused=%t error=%v items=%v acquisitions=%d", policy.name, pair.name, owner, run, got, reused, err, s.logs[run].GetItemIDs(), len(f.locks.owners))
				require.Equal(t, owner, f.locks.owner(f.key))
				require.Len(t, f.locks.owners, 1)
				require.NoError(t, err, "a legacy constructor can reuse an active no-Hook RetryItems run only if its lock owner stays readable")
				require.True(t, reused)
				require.Equal(t, run, got)
				require.Equal(t, []int64{71, 72}, s.logs[run].GetItemIDs())
				require.Empty(t, s.runs)
				require.Equal(t, strconv.FormatInt(run, 10), owner)
				require.Zero(t, f.locks.forceDeletes)
			})
		}
	}
}

func TestHookManagerLockCompatEnabledRetryAndLegacyIsolation(t *testing.T) {
	f, s := newManagerCompatFixture(t, true)
	run, reused, err := f.call(context.Background(), true)
	require.NoError(t, err)
	require.False(t, reused)
	owner := f.locks.owner(f.key)
	require.Contains(t, owner, fmt.Sprintf("hook_run:%d:", run))
	_, _, err = f.base.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{72}, &entity.Session{UserID: "user"})
	require.Error(t, err, "legacy reader must not bypass a managed Run through its numeric path")
	require.Equal(t, []int64{71}, s.logs[run].GetItemIDs())
	require.Equal(t, owner, f.locks.owner(f.key))
	got, reused, err := f.manager.LogRetryItemsRun(context.Background(), 20, entity.EvaluationModeRetryItems, 10, []int64{72}, &entity.Session{UserID: "user"})
	require.NoError(t, err)
	require.True(t, reused)
	require.Equal(t, run, got)
	require.Equal(t, []int64{71, 72}, s.logs[run].GetItemIDs())
	require.Equal(t, owner, f.locks.owner(f.key))
	require.Equal(t, 1, s.appends)
	require.Len(t, f.locks.owners, 1)
}

func TestHookManagerLockCompatNoHookWriteFailure(t *testing.T) {
	for _, scenario := range []string{"rollback", "committed", "unknown", "successor", "enabled_after_read"} {
		t.Run(scenario, func(t *testing.T) {
			f, s := newManagerCompatFixture(t, false)
			s.writeErr = errors.New("write receipt unavailable")
			if scenario == "committed" {
				s.commitOnError = true
			}
			if scenario == "unknown" {
				s.confirmErr = errors.New("primary unavailable")
			}
			if scenario == "successor" {
				s.onWrite = func() { f.locks.replace(f.key, "900") }
			}
			if scenario == "enabled_after_read" {
				s.writeErr = nil
				s.onWrite = func() { s.enabled = true }
			}
			_, _, err := f.call(context.Background(), true)
			require.Error(t, err)
			require.Equal(t, "101", f.locks.owners[0], "the allocated new RunID is the unique no-Hook owner")
			switch scenario {
			case "rollback", "enabled_after_read":
				require.Empty(t, f.locks.owner(f.key))
				require.Empty(t, s.logs)
				s.writeErr, s.onWrite, s.enabled = nil, nil, false
				run, reused, err := f.call(context.Background(), true)
				require.NoError(t, err)
				require.False(t, reused)
				require.Equal(t, int64(102), run)
				require.Equal(t, "102", f.locks.owner(f.key))
				require.Equal(t, []int64{71}, s.logs[run].GetItemIDs())
			case "committed":
				require.NotNil(t, s.logs[101])
				require.Equal(t, "101", f.locks.owner(f.key))
			case "unknown":
				require.Empty(t, s.logs)
				require.Equal(t, "101", f.locks.owner(f.key))
			case "successor":
				require.Equal(t, "900", f.locks.owner(f.key))
			}
			require.Zero(t, f.locks.forceDeletes)
		})
	}
}
