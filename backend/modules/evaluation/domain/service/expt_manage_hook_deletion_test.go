// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type deletionLegacySpy struct {
	repo.IExperimentRepo
	expt    *entity.Experiment
	deletes int
}

func (s *deletionLegacySpy) GetByID(context.Context, int64, int64) (*entity.Experiment, error) {
	return s.expt, nil
}
func (s *deletionLegacySpy) MGetByID(context.Context, []int64, int64) ([]*entity.Experiment, error) {
	return []*entity.Experiment{s.expt}, nil
}
func (s *deletionLegacySpy) Delete(context.Context, int64, int64) error    { s.deletes++; return nil }
func (s *deletionLegacySpy) MDelete(context.Context, []int64, int64) error { s.deletes++; return nil }

func deletionManager(t *testing.T, f *finalizationManagerFixture) (*ExptMangerImpl, *deletionLegacySpy) {
	t.Helper()
	source, err := f.deps.Repository.ReadFinalizationSource(context.Background(), f.key)
	require.NoError(t, err)
	spy := &deletionLegacySpy{expt: source.Experiment}
	f.manager.exptRepo = spy
	m, err := NewExptManagerWithHookDeletion(f.manager, exptinfra.NewHookDeletionRepo(f.p))
	require.NoError(t, err)
	return m, spy
}

func TestHookDeletionManagerBothEntriesAndRecovery(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprint(batch), func(t *testing.T) {
			f, _ := activeTerminationFixture(t, entity.ItemRunState_Processing, entity.ItemRunState_Success, entity.ItemRunState_Queueing)
			m, spy := deletionManager(t, f)
			ctx := context.Background()
			if batch {
				require.NoError(t, m.MDelete(ctx, []int64{f.expt}, f.space, nil))
			} else {
				require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
			}
			require.Zero(t, spy.deletes, "managed deletion must not softdelete twice")
			state, err := f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookFinalizePending, state.State.Finalize)
			require.False(t, state.State.After.Activated)
			// Recovery is independent of the old visible Get/MGet path.
			m.exptRepo = nil
			require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			state, err = f.repo.GetRun(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookFinalizeCommitted, state.State.Finalize)
			require.True(t, state.State.After.Activated)
			require.Zero(t, f.notifications, "deleted experiment must not publish a Latest projection")
			var n int64
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).Count(&n).Error)
			require.Zero(t, n)
			require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
		})
	}
}

func TestHookDeletionManagerBeforeLeaseRecovery(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true, "tx")
	claim := claimTerminationBefore(t, f)
	m, _ := deletionManager(t, f)
	ctx := context.Background()
	require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
	late, err := f.repo.CompleteAttempt(ctx, claim)
	require.NoError(t, err)
	require.True(t, late.Effects.LateIgnored, "deleted Run must fence an in-flight before result")
	require.False(t, late.Changed)
	m.exptRepo = nil
	require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	state, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Terminated, state.State.Status)
	require.True(t, state.State.After.Activated)
}

type deletionTemplateSpy struct {
	IExptTemplateManager
	updates [][4]int64
}

func (s *deletionTemplateSpy) UpdateExptInfo(_ context.Context, template, space, expt int64, _ entity.ExptStatus, delta int64, _ *int64) error {
	s.updates = append(s.updates, [4]int64{template, space, expt, delta})
	return nil
}

func TestHookDeletionManagerPersistenceFailureAndTemplateOnce(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true, "tx")
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("expt_template_id", 77).Error)
	m, _ := deletionManager(t, f)
	templates := new(deletionTemplateSpy)
	m.templateManager = templates
	trigger := fmt.Sprintf("hook_delete_manager_%d", f.expt)
	require.NoError(t, f.sql.Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE ON expt_lifecycle_run FOR EACH ROW BEGIN IF NEW.expt_id=%d THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='intent failed'; END IF; END", trigger, f.expt)).Error)
	t.Cleanup(func() { f.sql.Exec("DROP TRIGGER IF EXISTS " + trigger) })
	ctx := context.Background()
	require.ErrorContains(t, m.MDelete(ctx, []int64{f.expt}, f.space, nil), "intent failed")
	require.Empty(t, templates.updates)
	var visible model.Experiment
	require.NoError(t, f.sql.First(&visible, f.expt).Error)
	require.NoError(t, f.sql.Exec("DROP TRIGGER "+trigger).Error)
	require.NoError(t, m.MDelete(ctx, []int64{f.expt, f.expt}, f.space, nil))
	require.NoError(t, m.MDelete(ctx, []int64{f.expt}, f.space, nil))
	require.Equal(t, [][4]int64{{77, f.space, f.expt, -1}}, templates.updates)
}

func TestHookDeletionManagerCleanupFailureRecoverable(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing)
	item := ms[0]
	target := &activeTerminationTarget{fail: true, record: &entity.EvalTargetRecord{ID: finalizationTestIDs.Add(1), SpaceID: f.space, TargetID: 91, TargetVersionID: 92, ExperimentRunID: f.key.RunID, ItemID: item.Frozen.ItemID, ItemVersionID: item.Frozen.ItemVersionID, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), Ext: map[string]string{"sandbox_execute_ids": "[\"original-execute\"]"}}}
	persistActiveTerminationTarget(t, f, target.record)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"target_id": 91, "target_version_id": 92}).Error)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("target_result_id", target.record.ID).Error)
	m, _ := deletionManager(t, f)
	m.evalTargetService = target
	ctx := context.Background()
	require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
	m.exptRepo = nil
	require.ErrorContains(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}), "cleanup uncertain")
	pending, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.False(t, pending.State.After.Activated)
	target.fail = false
	require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, []entity.HookRunKey{f.key, f.key}, target.keys)
	var turn model.ExptTurnResult
	require.NoError(t, f.sql.First(&turn, item.Turns[0].ResultID).Error)
	require.Equal(t, target.record.ID, turn.TargetResultID)
	require.Equal(t, int32(entity.TurnRunState_Terminal), turn.Status)
	require.NoError(t, f.quota.CreateOrUpdate(ctx, f.space, func(quota *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		require.NotContains(t, quota.ExptID2RunTime, f.expt)
		return quota, false, nil
	}, nil))
}

func TestHookDeletionManagerNoHookTemplateAndLegacyConstructor(t *testing.T) {
	f := neverAdmittedFinalizationFixture(t, true, true, "tx")
	for _, table := range []any{&model.ExptLifecycleRunItem{}, &model.ExptLifecycleHookRun{}, &model.ExptLifecycleRun{}} {
		require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(table).Error)
	}
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("expt_template_id", 77).Error)
	m, spy := deletionManager(t, f)
	templates := new(deletionTemplateSpy)
	m.templateManager = templates
	require.NoError(t, m.Delete(context.Background(), f.expt, f.space, nil))
	require.Equal(t, [][4]int64{{77, f.space, f.expt, -1}}, templates.updates)
	require.Zero(t, spy.deletes)
	// The unwrapped manager still invokes the existing repository method.
	require.NoError(t, f.manager.Delete(context.Background(), f.expt, f.space, nil))
	require.Equal(t, 1, spy.deletes)
}

func TestHookDeletionManagerRecoversOlderAndCurrentRun(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Processing)
	ctx := context.Background()
	nextKey := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: finalizationTestIDs.Add(1)}
	_, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: nextKey, ExpectedLatestRunID: f.key.RunID,
		RunLog:   &entity.ExptRunLog{ID: nextKey.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: nextKey.RunID, CreatedBy: "successor", Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing)},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{2}, KeyID: "key", Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ExecutionScope: "local"},
		Before:   &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("next-before-%d", nextKey.RunID), IdempotencyKey: fmt.Sprintf("next-before-%d", nextKey.RunID)},
		After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprintf("next-after-%d", nextKey.RunID), IdempotencyKey: fmt.Sprintf("next-after-%d", nextKey.RunID)}})
	require.NoError(t, err)
	m, _ := deletionManager(t, f)
	require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
	nextPending, err := f.repo.GetRun(ctx, nextKey)
	require.NoError(t, err)
	m.exptRepo = nil
	require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	nextUnchanged, err := f.repo.GetRun(ctx, nextKey)
	require.NoError(t, err)
	require.Equal(t, nextPending, nextUnchanged)
	require.NoError(t, f.quota.CreateOrUpdate(ctx, f.space, func(q *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		require.Contains(t, q.ExptID2RunTime, f.expt)
		return q, false, nil
	}, nil))
	require.NoError(t, m.FinalizeRun(ctx, nextKey, entity.HookTerminalIntent{}))
	for _, key := range []entity.HookRunKey{f.key, nextKey} {
		done, err := f.repo.GetRun(ctx, key)
		require.NoError(t, err)
		require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
		require.True(t, done.State.After.Activated)
	}
}

func TestHookDeletionManagerPreservesNormalPendingDecision(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Success)
	ctx := context.Background()
	current, err := f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Success}
	_, err = f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: current.Version}, Intent: intent})
	require.NoError(t, err)
	m, _ := deletionManager(t, f)
	require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
	m.exptRepo = nil
	require.NoError(t, m.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	current, err = f.repo.GetRun(ctx, f.key)
	require.NoError(t, err)
	require.Equal(t, intent, current.State.Intent)
	require.Equal(t, entity.ExptStatus_Success, current.State.Status)
	require.True(t, current.State.After.Activated)
}
