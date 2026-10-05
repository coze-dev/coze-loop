// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"math"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

func (r *hookRunRepo) BeginFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	return r.finalizeHookRun(ctx, in, false, false)
}

// CommitFinalize confirms external cleanup already completed by the caller.
func (r *hookRunRepo) CommitFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	return r.finalizeHookRun(ctx, in, true, false)
}

func (r *hookRunRepo) AcceptTermination(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	if (in.Intent.Status != entity.ExptStatus_Terminated && in.Intent.Status != entity.ExptStatus_SystemTerminated) || in.Stats != nil {
		return entity.HookStoreResult{}, entity.ErrHookStoreConflict
	}
	return r.finalizeHookRun(ctx, in, false, true)
}

func (r *hookRunRepo) finalizeHookRun(ctx context.Context, in entity.HookFinalizeInput, commit, accepting bool) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	var out entity.HookStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, err := lockHookExperiment(tx, in.Key)
		if err != nil {
			return err
		}
		s, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		if err := checkBoundFinalizationRun(tx, in.Key, s.life.ExecutionScope, r.executionBinding); err != nil {
			return err
		}
		return finalizeLockedHookRun(tx, expt, s, in, commit, accepting, &out, r.executionBinding)
	}, db.WithMaster())
	if err != nil {
		return entity.HookStoreResult{}, err
	}
	return out, nil
}

func finalizeLockedHookRun(tx *gorm.DB, expt *model.Experiment, s *lockedHookRun, in entity.HookFinalizeInput, commit, accepting bool, out *entity.HookStoreResult, bindings ...*boundHookExecution) error {
	var err error
	if s.view.Mode == entity.EvaluationModeAppend && s.view.State.Finalize == entity.HookFinalizeNone && (in.Intent.Status == entity.ExptStatus_Success || in.Intent.Status == entity.ExptStatus_Failed) && s.view.State.Status != entity.ExptStatus_Draining {
		return entity.ErrHookFinalizationUnsettled
	}
	if accepting && (expt.DeletedAt.Valid || s.view.State.Finalize != entity.HookFinalizeCommitted &&
		(entity.IsExptFinished(s.view.State.Status) || expt.LatestRunID == in.Key.RunID && entity.IsExptFinished(entity.ExptStatus(expt.Status)))) {
		return entity.ErrHookStoreConflict
	}
	if in.DisplayMessage != nil && s.view.State.Finalize != entity.HookFinalizeNone &&
		(s.view.DisplayMessage == nil || *s.view.DisplayMessage != *in.DisplayMessage) {
		return entity.ErrHookStoreConflict
	}
	var change entity.HookStateChange
	if commit {
		change, err = entity.CommitHookFinalize(&s.view.State, in.Key, in.Intent)
	} else {
		change, err = entity.BeginHookFinalize(&s.view.State, in.Key, in.Intent)
	}
	if err != nil {
		return err
	}
	markTerminating := accepting && s.view.State.Finalize != entity.HookFinalizeCommitted && s.view.State.Status != entity.ExptStatus_Terminating
	projectTerminating := accepting && s.view.State.Finalize != entity.HookFinalizeCommitted && expt.LatestRunID == in.Key.RunID && expt.Status != int32(entity.ExptStatus_Terminating)
	if !change.Changed && !markTerminating && !projectTerminating {
		out.Run = s.view
		return nil
	}
	if s.life.Version != in.ExpectedVersion {
		return entity.ErrHookStoreConflict
	}
	now, err := hookDBNow(tx)
	if err != nil {
		return err
	}
	fields := map[string]any{"gate": 2}
	if s.view.Mode == entity.EvaluationModeRetryItems {
		cursor, err := retryItemsFinalizeCursor(s.log, s.life, in.Intent)
		if err != nil {
			return err
		}
		fields["plan_cursor"] = cursor
	}
	if commit {
		if in.Stats == nil && !s.life.ExecutionInitialized && hookCancellation(&s.life) {
			var materialized []model.ExptLifecycleRunItem
			if err := hookRunScope(tx.Select("id"), in.Key).Where("execution_manifest IS NOT NULL").Limit(1).Find(&materialized).Error; err != nil {
				return err
			}
			if len(materialized) != 0 {
				return entity.ErrHookFinalizationUnsettled
			}
		}
		if in.Stats != nil {
			latest := expt.LatestRunID
			if expt.DeletedAt.Valid {
				latest = 0
			}
			if err := writeHookFinalizationStats(tx, in, latest, now, bindings...); err != nil {
				return err
			}
		}
		if *s.log.Status != int64(in.Intent.Status) {
			if err := hookOneRow(hookRunScope(tx.Model(&model.ExptRunLog{}), in.Key).Where("id=? AND lifecycle_hook_version=1 AND status=?", s.log.ID, *s.log.Status).UpdateColumns(map[string]any{"status": int64(in.Intent.Status), "updated_at": now})); err != nil {
				return err
			}
		}
		if !expt.DeletedAt.Valid && expt.LatestRunID == in.Key.RunID && (expt.Status != int32(in.Intent.Status) || in.Stats != nil || in.DisplayMessage != nil) {
			projection := map[string]any{"status": int32(in.Intent.Status), "end_at": now, "updated_at": now}
			if in.Stats != nil {
				projection["status_message"] = []byte(in.Intent.Reason)
				if in.Stats.NeverAdmitted {
					projection["status_message"] = []byte("")
				}
			}
			if in.DisplayMessage != nil {
				projection["status_message"] = []byte(*in.DisplayMessage)
			}
			if err := hookOneRow(tx.Unscoped().Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=?", in.Key.ExperimentID, in.Key.WorkspaceID, in.Key.RunID).UpdateColumns(projection)); err != nil {
				return err
			}
			out.LatestProjected = true
		}
		if change.Effects.ActivateAfter {
			if err := updateHookOperation(tx, in.Key, s.after, map[string]any{"activated_at": now, "occurred_at": now, "next_attempt_at": now}); err != nil {
				return err
			}
		}
		fields["finalize_state"], fields["next_reconcile_at"] = 2, nil
	} else if change.Changed {
		if in.DisplayMessage != nil {
			if err := hookRunScope(tx.Model(&model.ExptRunLog{}), in.Key).
				Where("id=? AND lifecycle_hook_version=1", s.log.ID).
				UpdateColumns(map[string]any{"status_message": []byte(*in.DisplayMessage), "updated_at": now}).Error; err != nil {
				return err
			}
		}
		if change.Effects.FenceBefore {
			if s.before.LeaseGeneration == math.MaxInt64 {
				return entity.ErrHookStoreConflict
			}
			if err := updateHookOperation(tx, in.Key, s.before, map[string]any{"status": string(entity.HookOperationFailed), "lease_generation": s.before.LeaseGeneration + 1, "lease_until": nil, "lease_owner": nil, "next_attempt_at": nil}); err != nil {
				return err
			}
		}
		fields["finalize_state"], fields["terminal_status"], fields["terminal_reason"] = 1, int32(in.Intent.Status), in.Intent.Reason
		fields["terminal_at"], fields["next_reconcile_at"] = now, now
	}
	if markTerminating {
		if err := hookOneRow(hookRunScope(tx.Model(&model.ExptRunLog{}), in.Key).
			Where("id=? AND lifecycle_hook_version=1 AND status=?", s.log.ID, *s.log.Status).
			UpdateColumns(map[string]any{"status": int64(entity.ExptStatus_Terminating), "updated_at": now})); err != nil {
			return err
		}
	}
	if projectTerminating {
		if err := hookOneRow(tx.Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=? AND status=?", in.Key.ExperimentID, in.Key.WorkspaceID, in.Key.RunID, expt.Status).
			UpdateColumns(map[string]any{"status": int32(entity.ExptStatus_Terminating), "updated_at": now})); err != nil {
			return err
		}
		out.LatestProjected = true
	}
	if err := updateHookLifecycle(tx, in.Key, s.life.Version, now, fields); err != nil {
		return err
	}
	s, err = loadHookRun(tx, in.Key)
	if err != nil {
		return err
	}
	out.Run, out.Changed, out.Effects = s.view, true, change.Effects
	return nil
}
