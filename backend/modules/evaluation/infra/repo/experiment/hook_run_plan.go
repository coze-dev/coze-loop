// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"math"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

func (r *hookRunRepo) AppendPlanPage(ctx context.Context, in entity.HookPlanPageInput) (entity.HookStoreResult, error) {
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
		if !hookRunActive(expt, s) || s.view.PlanReady {
			return entity.ErrHookStoreConflict
		}
		// An exact replay of the last page acknowledges existing rows without rewinding the cursor.
		if s.life.Version == in.ExpectedVersion+1 && s.life.PlanCount == in.StartOrdinal+int64(len(in.Items)) && s.view.PlanCursor == in.NextCursor {
			matches, err := hookPageMatches(tx, in)
			if err != nil {
				return err
			}
			if !matches {
				return entity.ErrHookStoreConflict
			}
			out.Run = s.view
			return nil
		}
		if s.life.Version != in.ExpectedVersion || s.life.PlanCount != in.StartOrdinal || s.view.PlanCursor != in.Cursor {
			return entity.ErrHookStoreConflict
		}
		rows := make([]model.ExptLifecycleRunItem, 0, len(in.Items))
		for i, item := range in.Items {
			rows = append(rows, model.ExptLifecycleRunItem{ID: item.ID, SpaceID: in.Key.WorkspaceID, ExptID: in.Key.ExperimentID, ExptRunID: in.Key.RunID, Ordinal: in.StartOrdinal + int64(i), SourceSpaceID: item.SourceSpaceID, EvalSetID: item.EvalSetID, EvalSetVersionID: item.EvalSetVersionID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID})
		}
		if result := tx.Create(&rows); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != int64(len(rows)) {
			return entity.ErrHookStoreConflict
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err := updateHookLifecycle(tx, in.Key, s.life.Version, now, map[string]any{"plan_count": s.life.PlanCount + int64(len(rows)), "plan_cursor": in.NextCursor}); err != nil {
			return err
		}
		s, err = loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out = entity.HookStoreResult{Run: s.view, Changed: true}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookStoreResult{}, err
	}
	return out, nil
}

func (r *hookRunRepo) FinishPlan(ctx context.Context, in entity.HookFinishPlanInput) (entity.HookStoreResult, error) {
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
		if s.view.PlanReady {
			if s.life.PlanCount != in.Count || s.view.PlanHash != in.Hash {
				return entity.ErrHookStoreConflict
			}
			out.Run = s.view
			return nil
		}
		if !hookRunActive(expt, s) || s.life.Version != in.ExpectedVersion || s.life.PlanCount != in.Count {
			return entity.ErrHookStoreConflict
		}
		var count int64
		if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), in.Key).Count(&count).Error; err != nil {
			return err
		}
		if count != in.Count {
			return entity.ErrHookStoreCorrupt
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if s.before != nil {
			if s.before.Status != string(entity.HookOperationPending) || s.before.ActivatedAt != nil {
				return entity.ErrHookStoreCorrupt
			}
			if err := updateHookOperation(tx, in.Key, s.before, map[string]any{"activated_at": now, "occurred_at": now, "next_attempt_at": now}); err != nil {
				return err
			}
		}
		if err := updateHookLifecycle(tx, in.Key, s.life.Version, now, map[string]any{"plan_state": 1, "plan_hash": in.Hash}); err != nil {
			return err
		}
		s, err = loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out = entity.HookStoreResult{Run: s.view, Changed: true}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookStoreResult{}, err
	}
	return out, nil
}

func hookPageMatches(tx *gorm.DB, in entity.HookPlanPageInput) (bool, error) {
	var rows []model.ExptLifecycleRunItem
	err := hookRunScope(tx, in.Key).Where("ordinal>=? AND ordinal<?", in.StartOrdinal, in.StartOrdinal+int64(len(in.Items))).Order("ordinal ASC, id ASC").Find(&rows).Error
	if err != nil || len(rows) != len(in.Items) {
		return false, err
	}
	for i, row := range rows {
		item := in.Items[i]
		if row.Ordinal != in.StartOrdinal+int64(i) || row.SourceSpaceID != item.SourceSpaceID || row.EvalSetID != item.EvalSetID || row.EvalSetVersionID != item.EvalSetVersionID || row.ItemID != item.ItemID || row.ItemVersionID != item.ItemVersionID {
			return false, nil
		}
	}
	return true, nil
}

func hookRunActive(expt *model.Experiment, s *lockedHookRun) bool {
	return !expt.DeletedAt.Valid && expt.LatestRunID == s.life.ExptRunID && s.view.State.Finalize == entity.HookFinalizeNone && s.view.State.Gate != entity.HookGateClosed && !entity.IsExptFinished(s.view.State.Status) && s.view.State.Status != entity.ExptStatus_Terminating
}

func updateHookLifecycle(tx *gorm.DB, key entity.HookRunKey, version int64, now time.Time, fields map[string]any) error {
	if version < 0 || version == math.MaxInt64 {
		return entity.ErrHookStoreConflict
	}
	fields["version"], fields["updated_at"] = version+1, now
	return hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleRun{}), key).Where("version=?", version).UpdateColumns(fields))
}

func updateHookOperation(tx *gorm.DB, key entity.HookRunKey, op *model.ExptLifecycleHookRun, fields map[string]any) error {
	if op.Version < 0 || op.Version == math.MaxInt64 {
		return entity.ErrHookStoreConflict
	}
	fields["version"] = op.Version + 1
	fields["updated_at"] = gorm.Expr("CURRENT_TIMESTAMP(3)")
	return hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleHookRun{}), key).Where("id=? AND operation_id=? AND phase=? AND version=? AND attempt=? AND lease_generation=?", op.ID, op.OperationID, op.Phase, op.Version, op.Attempt, op.LeaseGeneration).UpdateColumns(fields))
}
