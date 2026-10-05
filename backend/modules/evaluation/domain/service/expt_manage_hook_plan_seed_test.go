// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type seedStarterProbe interface {
	LogRunWithPlanSeed(context.Context, int64, int64, entity.ExptRunMode, int64, string, *entity.Session) error
}

func TestHookPlanSeedPersistsBeforeMQ(t *testing.T) {
	f := newHookManagerFixture(t, managerEnabledConfig(true, false))
	f.expt.TrialRunItemCount = 1
	f.expectLock(entity.EvaluationModeTrialRun)
	f.expectRead(0, nil)
	f.expectConfig()
	f.base.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(f.expt, nil)
	f.base.idgenerator.(*idmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(100), nil)
	var err error
	if starter, ok := f.manager.(seedStarterProbe); ok {
		err = starter.LogRunWithPlanSeed(context.Background(), 20, 30, entity.EvaluationModeTrialRun, 10, "[71,72]", &entity.Session{UserID: "run-user"})
	} else {
		err = f.manager.LogRun(context.Background(), 20, 30, entity.EvaluationModeTrialRun, 10, nil, &entity.Session{UserID: "run-user"})
	}
	require.NoError(t, err)
	// No Run()/MQ call: the real creation repository must already have received the selection.
	require.Equal(t, []int64{71, 72}, f.repo.input.RunLog.GetItemIDs())
	var payload struct {
		Selection *struct {
			Version            int   `json:"version"`
			TrialRunItemCount  int64 `json:"trial_run_item_count"`
			HasExplicitItemIDs bool  `json:"has_explicit_item_ids"`
		} `json:"selection"`
	}
	require.NoError(t, json.Unmarshal(f.repo.input.Snapshot.Cipher, &payload))
	require.NotNil(t, payload.Selection)
	require.Equal(t, 1, payload.Selection.Version)
	require.Equal(t, int64(1), payload.Selection.TrialRunItemCount)
	require.True(t, payload.Selection.HasExplicitItemIDs)
	// Recreate the read side after the write, without retaining request/MQ state.
	written := f.repo.input
	items, err := json.Marshal(written.RunLog.ItemIds)
	require.NoError(t, err)
	f.expectRead(30, sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "created_by", "mode", "status", "item_ids", "lifecycle_hook_version"}).AddRow(30, 10, 20, 30, "run-user", 6, 2, items, 1))
	read, err := f.deps.Initialization.ReadRunInitialization(context.Background(), written.Key)
	require.NoError(t, err)
	m := f.mock
	m.ExpectBegin()
	m.ExpectQuery("SELECT .*FROM .experiment.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status"}).AddRow(20, 10, 30, 2))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "gate", "before_enabled", "after_enabled", "snapshot_cipher", "snapshot_key_id", "snapshot_hash", "execution_scope"}).AddRow(10, 20, 30, 0, true, false, written.Snapshot.Cipher, written.Snapshot.KeyID, written.Snapshot.Hash, "local"))
	m.ExpectQuery("SELECT .*FROM .expt_run_log.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status", "mode", "lifecycle_hook_version", "created_by", "item_ids"}).AddRow(30, 10, 20, 30, 2, 6, 1, "run-user", items))
	m.ExpectQuery("SELECT .*FROM .expt_lifecycle_hook_run.*FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id", "operation_id", "idempotency_key", "phase", "status", "execution_scope"}).AddRow(written.Before.ID, written.Before.OperationID, written.Before.IdempotencyKey, "before", "pending", "local"))
	m.ExpectCommit()
	stored, err := f.repo.IHookRepo.GetRun(context.Background(), written.Key)
	require.NoError(t, err)
	reloaded, err := hookinfra.NewStorageCodec(new(managerProtector)).DecodeSnapshot(context.Background(), written.Key, "local", stored.Snapshot)
	require.NoError(t, err)
	require.Equal(t, []int64{71, 72}, read.RunLog.GetItemIDs())
	require.Equal(t, int64(1), reloaded.Input().Selection.TrialRunItemCount)
}

type seedConfigRepo struct {
	repo.IHookConfigRepo
	config *entity.LifecycleHookConf
}

func (r seedConfigRepo) GetConfig(context.Context, hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error) {
	return &entity.HookConfigRecord{Revision: "revision", Config: r.config}, nil
}

type seedExperimentRepo struct {
	repo.IExperimentRepo
	expt *entity.Experiment
}

func (r seedExperimentRepo) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	return r.expt, nil
}

func seedManagerFixture(t *testing.T, before, after bool, count int64) (*managerLockFixture, *entity.Experiment) {
	t.Helper()
	f, _ := newManagerCompatFixture(t, before || after)
	expt := &entity.Experiment{ID: 20, SpaceID: 10, Name: "seed", ExptType: entity.ExptType_Offline, TrialRunItemCount: count}
	e := f.manager.(*ExptMangerImpl)
	e.exptRepo = seedExperimentRepo{expt: expt}
	e.hooks.Configs = seedConfigRepo{config: managerEnabledConfig(before, after)}
	e.hooks.Runs = f.store
	return f, expt
}

func TestHookPlanSeedTrialSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		count     int64
		explicit  bool
		items     []int64
		fail      bool
	}{
		{"absent", "", 2, false, nil, false}, {"empty_array", "[]", 2, true, nil, false}, {"null", "null", 2, true, nil, false},
		{"selected", "[71,72]", 1, true, []int64{71, 72}, false}, {"invalid", "{private", 2, false, nil, true},
		{"zero_ignores", "{private", 0, false, nil, false}, {"negative_ignores", "{private", -1, false, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := seedManagerFixture(t, true, false, tc.count)
			err := f.manager.(IHookRunPlanStarter).LogRunWithPlanSeed(context.Background(), 20, 30, entity.EvaluationModeTrialRun, 10, tc.raw, &entity.Session{UserID: "user"})
			if tc.fail {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "private")
				require.Empty(t, f.store.logs)
				require.Empty(t, f.locks.owner(f.key))
				return
			}
			require.NoError(t, err)
			run := f.store.runs[30]
			s, err := f.manager.(*ExptMangerImpl).hooks.Codec.DecodeSnapshot(context.Background(), run.State.Key, "local", run.Snapshot)
			require.NoError(t, err)
			require.Equal(t, tc.count, s.Input().Selection.TrialRunItemCount)
			require.Equal(t, tc.explicit, s.Input().Selection.HasExplicitItemIDs)
			require.Equal(t, tc.items, f.store.logs[30].GetItemIDs())
		})
	}
}

func TestHookPlanSeedIgnoresUnusedInput(t *testing.T) {
	for _, tc := range []struct {
		before, after, legacy bool
		mode                  entity.ExptRunMode
	}{{false, false, false, entity.EvaluationModeTrialRun}, {false, true, false, entity.EvaluationModeAppend}, {true, false, false, entity.EvaluationModeSubmit}, {false, false, true, entity.EvaluationModeTrialRun}} {
		t.Run(fmt.Sprint(tc), func(t *testing.T) {
			f, _ := seedManagerFixture(t, tc.before, tc.after, 3)
			var m IExptManager = f.manager
			if tc.legacy {
				m = f.base
			}
			err := m.(IHookRunPlanStarter).LogRunWithPlanSeed(context.Background(), 20, 30, tc.mode, 10, "{private", &entity.Session{UserID: "user"})
			require.NoError(t, err)
			require.Empty(t, f.store.logs[30].GetItemIDs())
			if tc.before || tc.after {
				run := f.store.runs[30]
				s, err := f.manager.(*ExptMangerImpl).hooks.Codec.DecodeSnapshot(context.Background(), run.State.Key, "local", run.Snapshot)
				require.NoError(t, err)
				if !tc.before {
					require.Nil(t, s.Input().Selection)
				} else {
					require.False(t, s.Input().Selection.HasExplicitItemIDs)
				}
			}
		})
	}
}

func TestHookPlanSeedReplayPinsSelection(t *testing.T) {
	f, expt := seedManagerFixture(t, true, false, 1)
	starter := f.manager.(IHookRunPlanStarter)
	call := func(raw, user string) error {
		return starter.LogRunWithPlanSeed(context.Background(), 20, 30, entity.EvaluationModeTrialRun, 10, raw, &entity.Session{UserID: user})
	}
	require.NoError(t, call("[71,72]", "user"))
	stored := f.store.runs[30]
	hash := stored.Snapshot.Hash
	owner := f.locks.owner(f.key)
	expt.TrialRunItemCount = 99
	expt.Name = "changed"
	f.identity.fail = true
	require.NoError(t, call("[71,72]", "user"))
	require.Error(t, call("[72]", "user"))
	require.Error(t, call("[71,72]", "different-user"))
	require.Equal(t, hash, f.store.runs[30].Snapshot.Hash)
	require.Equal(t, owner, f.locks.owner(f.key))
	require.Equal(t, 1, f.store.writes)
	require.Equal(t, []int64{71, 72}, f.store.logs[30].GetItemIDs())
	require.Equal(t, "user", f.store.logs[30].CreatedBy)
}

func TestHookPlanSeedReplayExplicitEmptyAndIgnoredInput(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		before                 bool
		count                  int64
		initial, same, changed string
		rejectChanged          bool
	}{
		{"explicit_empty", true, 2, "[]", "null", "", true},
		{"zero_count", true, 0, "{ignored", "another invalid", "", false},
		{"after_only", false, 2, "[]", "null", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := seedManagerFixture(t, tc.before, !tc.before, tc.count)
			m := f.manager.(IHookRunPlanStarter)
			call := func(raw string) error {
				return m.LogRunWithPlanSeed(context.Background(), 20, 30, entity.EvaluationModeTrialRun, 10, raw, &entity.Session{UserID: "user"})
			}
			require.NoError(t, call(tc.initial))
			hash := f.store.runs[30].Snapshot.Hash
			require.NoError(t, call(tc.same))
			err := call(tc.changed)
			if tc.rejectChanged {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, hash, f.store.runs[30].Snapshot.Hash)
			require.Equal(t, 1, f.store.writes)
		})
	}
}
