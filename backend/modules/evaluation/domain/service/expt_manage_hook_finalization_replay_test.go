// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/dao"
)

type finalizationQuotaFault struct {
	repo.QuotaRepo
	fail bool
}

func (q *finalizationQuotaFault) CreateOrUpdate(ctx context.Context, s int64, fn func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), u *entity.Session) error {
	if q.fail {
		return errors.New("quota unavailable")
	}
	return q.QuotaRepo.CreateOrUpdate(ctx, s, fn, u)
}

type finalizationRunFault struct {
	repo.IHookRepo
	failCommit, loseReceipt bool
	conflicts               int
}

func (r *finalizationRunFault) BeginFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	if r.conflicts > 0 {
		r.conflicts--
		return entity.HookStoreResult{}, entity.ErrHookStoreConflict
	}
	return r.IHookRepo.BeginFinalize(ctx, in)
}

func (r *finalizationRunFault) CommitFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	if r.failCommit {
		return entity.HookStoreResult{}, errors.New("commit unavailable")
	}
	out, err := r.IHookRepo.CommitFinalize(ctx, in)
	if err == nil && r.loseReceipt {
		r.loseReceipt = false
		return entity.HookStoreResult{}, errors.New("receipt lost")
	}
	return out, err
}

type finalizationOwnerFault struct {
	repo.IHookFinalizationOwnerReader
	fail      bool
	afterRead func(string)
}

func (r *finalizationOwnerFault) ReadFinalizationOwner(ctx context.Context, key string) (string, error) {
	if r.fail {
		return "", errors.New("owner read failed")
	}
	owner, err := r.IHookFinalizationOwnerReader.ReadFinalizationOwner(ctx, key)
	if err == nil && r.afterRead != nil {
		r.afterRead(key)
	}
	return owner, err
}

type finalizationSourceProbe struct {
	repo.IHookFinalizationRepo
	afterRead func(entity.HookRunKey)
	keys      []entity.HookRunKey
}

func (r *finalizationSourceProbe) ReadFinalizationSource(ctx context.Context, key entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	r.keys = append(r.keys, key)
	out, err := r.IHookFinalizationRepo.ReadFinalizationSource(ctx, key)
	if err == nil && r.afterRead != nil {
		r.afterRead(key)
	}
	return out, err
}

func finalizationRead(t *testing.T, f *finalizationManagerFixture) *entity.HookStoredRun {
	t.Helper()
	s, err := f.repo.GetRun(context.Background(), f.key)
	require.NoError(t, err)
	return s
}

func finalizationRecreate(t *testing.T, f *finalizationManagerFixture) {
	t.Helper()
	var err error
	f.manager, err = NewExptManagerWithHookFinalization(f.base, f.deps)
	require.NoError(t, err)
}

func TestHookFinalizationManagerRequiredFailuresPending(t *testing.T) {
	for _, failure := range []string{"quota", "commit", "stats-write"} {
		t.Run(failure, func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			q := &finalizationQuotaFault{QuotaRepo: f.quota, fail: failure == "quota"}
			f.base.(*ExptMangerImpl).quotaRepo = q
			r := &finalizationRunFault{IHookRepo: f.repo, failCommit: failure == "commit"}
			f.deps.Runs = r
			trigger := fmt.Sprintf("hook_final_stats_%d", f.expt)
			if failure == "stats-write" {
				require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON expt_stats FOR EACH ROW BEGIN IF NEW.expt_id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='stats write failed'; END IF; END", trigger, f.expt)).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Exec("DROP TRIGGER IF EXISTS "+trigger).Error) })
			}
			finalizationRecreate(t, f)
			require.Error(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithCID("cached")))
			s := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizePending, s.State.Finalize)
			require.False(t, s.State.After.Activated)
			require.Equal(t, entity.ExptStatus_Processing, s.State.Status)
			require.Zero(t, f.notifications)
			q.fail = false
			r.failCommit = false
			if failure == "stats-write" {
				require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error)
			}
			finalizationRecreate(t, f)
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			s = finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, s.State.Finalize)
			require.True(t, s.State.After.Activated)
		})
	}
}

func TestHookFinalizationManagerCommittedUnlockRecovery(t *testing.T) {
	for _, kind := range []string{"owner-read", "lost-commit-receipt", "replace-owner"} {
		t.Run(kind, func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			owner := &finalizationOwnerFault{IHookFinalizationOwnerReader: f.deps.Owners, fail: kind == "owner-read"}
			f.deps.Owners = owner
			runs := &finalizationRunFault{IHookRepo: f.repo, loseReceipt: kind == "lost-commit-receipt"}
			f.deps.Runs = runs
			successor := "hook_run:999999:abcdef0123456789abcdef0123456789"
			if kind == "replace-owner" {
				owner.afterRead = func(key string) { require.NoError(t, f.redis.Set(ctx, key, successor, time.Hour).Err()) }
			}
			finalizationRecreate(t, f)
			err := f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil)
			if kind == "replace-owner" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			s := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, s.State.Finalize)
			require.True(t, s.State.After.Activated)
			owner.fail = false
			owner.afterRead = nil
			finalizationRecreate(t, f)
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, s, finalizationRead(t, f))
			value, readErr := f.redis.Get(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)).Result()
			if kind == "replace-owner" {
				require.NoError(t, readErr)
				require.Equal(t, successor, value)
			} else {
				require.Error(t, readErr)
			}
		})
	}
}

func TestHookFinalizationManagerNilFixationAndNewQuota(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	next := finalizationTestIDs.Add(1)
	oldKey := f.key
	source := &finalizationSourceProbe{IHookFinalizationRepo: f.deps.Repository}
	source.afterRead = func(key entity.HookRunKey) {
		if key.RunID != 0 {
			return
		}
		_, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next},
			ExpectedLatestRunID: oldKey.RunID, RunLog: &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"},
			Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"},
			After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
		require.NoError(t, err)
		require.NoError(t, f.quota.CreateOrUpdate(ctx, f.space, func(q *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
			q.ExptID2RunTime[f.expt] = 456
			return q, true, nil
		}, nil))
		require.NoError(t, f.redis.Set(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt), fmt.Sprintf("hook_run:%d:abcdef0123456789abcdef0123456789", next), time.Hour).Err())
		require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("pending_cnt", 99).Error)
	}
	f.deps.Repository = source
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, nil, f.space, nil, entity.WithCompleteInterval(time.Millisecond)))
	require.Equal(t, int64(0), source.keys[0].RunID)
	for _, k := range source.keys[1:] {
		require.Equal(t, oldKey, k)
	}
	require.Equal(t, entity.HookFinalizeCommitted, finalizationRead(t, f).State.Finalize)
	fresh, err := f.repo.GetRun(ctx, entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next})
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizeNone, fresh.State.Finalize)
	q, err := dao.NewQuotaDAO(f.redis).GetQuotaSpaceExpt(ctx, f.space)
	require.NoError(t, err)
	require.Equal(t, int64(456), q.ExptID2RunTime[f.expt])
	var st model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&st).Error)
	require.Equal(t, int32(99), st.PendingCnt)
	require.Zero(t, f.notifications)
}

func TestHookFinalizationManagerFailedIntentAndCAS(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", 3).Error)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", 2).Error)
	require.NoError(t, f.sql.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", 3).Error)
	require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", 2).Error)
	f.deps.Runs = &finalizationRunFault{IHookRepo: f.repo, conflicts: 2}
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, gptr.Of(f.key.RunID), f.space, nil, entity.WithStatusMessage("normal failure")))
	s := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Failed, s.State.Status)
	require.Empty(t, s.State.Intent.Reason)
	require.True(t, s.State.After.Activated)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Equal(t, s, finalizationRead(t, f))
	require.Equal(t, 1, f.notifications)
	require.ErrorIs(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Success}), entity.ErrHookStoreConflict)
	require.Equal(t, s, finalizationRead(t, f))
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Equal(t, "normal failure", string(gptr.Indirect(expt.StatusMessage)), "Latest reason must commit with original Run intent")
}

func TestHookFinalizationManagerGuards(t *testing.T) {
	for _, kind := range []string{"missing", "marker", "scope", "unsettled", "missing-turn", "online", "append", "uninitialized"} {
		t.Run(kind, func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			key := f.key
			switch kind {
			case "missing":
				key.RunID++
			case "marker":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("lifecycle_hook_version", 2).Error)
			case "scope":
				f.deps.ExecutionScope = "another"
				finalizationRecreate(t, f)
			case "unsettled":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("expt_run_id=?", key.RunID).UpdateColumn("status", 1).Error)
			case "missing-turn":
				require.NoError(t, f.sql.Where("expt_run_id=?", key.RunID).Delete(&model.ExptTurnResultRunLog{}).Error)
			case "online":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("expt_type", 2).Error)
			case "append":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("mode", 3).Error)
			case "uninitialized":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("expt_run_id=?", key.RunID).UpdateColumn("execution_initialized", false).Error)
			}
			require.Error(t, f.manager.CompleteRun(ctx, key.ExperimentID, key.RunID, key.WorkspaceID, nil))
			var count int64
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("expt_run_id=? AND phase='after' AND activated_at IS NOT NULL", f.key.RunID).Count(&count).Error)
			require.Zero(t, count)
		})
	}
}
