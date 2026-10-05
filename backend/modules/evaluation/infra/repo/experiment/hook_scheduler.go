// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *hookFinalizationRepo) PersistHookDispatch(ctx context.Context, in entity.HookSchedulerDispatchInput) (changed []int64, err error) {
	seen := make(map[int64]bool, len(in.ItemIDs))
	for _, id := range in.ItemIDs {
		if id <= 0 || seen[id] {
			return nil, entity.ErrHookStoreCorrupt
		}
		seen[id] = true
	}
	// The same experiment -> lifecycle lock boundary arbitrates cancellation and Latest changes.
	err = r.itemArchiveTransaction(ctx, in.Key, in.ExecutionScope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		if expt.LatestRunID != in.Key.RunID || life.Gate == 2 || life.FinalizeState != 0 || entity.IsExptFinished(entity.ExptStatus(expt.Status)) || entity.ExptStatus(expt.Status) == entity.ExptStatus_Terminating {
			return nil
		}
		if life.Gate != 1 || life.PlanState != 1 || !life.ExecutionInitialized {
			return entity.ErrHookAdmissionDenied
		}
		var run model.ExptRunLog
		if err := hookRunScope(tx, in.Key).First(&run).Error; err != nil {
			return err
		}
		if !entity.HookExecutionInitializationRequired(life.BeforeEnabled || life.AfterEnabled, entity.ExptRunMode(gptr.Indirect(run.Mode)), entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) && !boundFinalizationRequired(r.binding, life, entity.ExptRunMode(gptr.Indirect(run.Mode))) {
			return entity.ErrHookExecutionUnsupported
		}
		if entity.IsExptFinished(entity.ExptStatus(gptr.Indirect(run.Status))) || entity.ExptStatus(gptr.Indirect(run.Status)) == entity.ExptStatus_Terminating {
			return nil
		}
		var promoted int64
		for _, id := range in.ItemIDs {
			updated, err := persistHookDispatchItem(tx, in.Key, id, life.PlanCount, r.binding)
			if err != nil {
				return err
			}
			if updated {
				promoted++
				changed = append(changed, id)
			}
		}
		if promoted == 0 {
			return nil
		}
		return hookOneRow(tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=? AND pending_cnt>=?", in.Key.WorkspaceID, in.Key.ExperimentID, promoted).
			UpdateColumns(map[string]any{"pending_cnt": gorm.Expr("pending_cnt - ?", promoted), "processing_cnt": gorm.Expr("processing_cnt + ?", promoted)}))
	})
	if err != nil {
		return nil, err
	}
	return changed, nil
}

func persistHookDispatchItem(tx *gorm.DB, key entity.HookRunKey, itemID, planCount int64, bindings ...*boundHookExecution) (bool, error) {
	var ledger model.ExptLifecycleRunItem
	if err := hookRunScope(tx, key).Where("item_id=?", itemID).First(&ledger).Error; err != nil {
		return false, err
	}
	m, err := hookTerminationManifest(key, ledger)
	if err != nil {
		return false, err
	}
	if m.Ordinal >= planCount {
		return false, entity.ErrHookStoreCorrupt
	}
	if err := checkBoundFinalizationRefs(tx, key, []entity.HookExecutionManifest{m}, firstFinalizationBinding(bindings), true); err != nil {
		return false, err
	}
	var logs []model.ExptItemResultRunLog
	if err := hookRunScope(tx.Unscoped(), key).Where("item_id=?", itemID).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&logs).Error; err != nil {
		return false, err
	}
	if len(logs) != 1 || logs[0].ID != m.ItemRunLogID || logs[0].DeletedAt.Valid || logs[0].ItemVersionID != m.Frozen.ItemVersionID {
		return false, entity.ErrHookStoreCorrupt
	}
	// MQ may finish an item before the publishing tick resumes.
	if entity.IsItemRunFinished(entity.ItemRunState(logs[0].Status)) {
		return false, nil
	}
	if logs[0].Status != int32(entity.ItemRunState_Queueing) && logs[0].Status != int32(entity.ItemRunState_Processing) {
		return false, entity.ErrHookStoreCorrupt
	}
	var item model.ExptItemResult
	if err := tx.Unscoped().Where("id=?", m.ItemResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
		return false, err
	}
	if item.DeletedAt.Valid || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != itemID || item.ItemVersionID != m.Frozen.ItemVersionID || item.ItemIdx == nil || *item.ItemIdx != int32(m.ProjectionOrdinal()) {
		return false, entity.ErrHookStoreCorrupt
	}
	if entity.IsItemRunFinished(entity.ItemRunState(item.Status)) {
		return false, nil
	}
	if item.Status != int32(entity.ItemRunState_Queueing) && item.Status != int32(entity.ItemRunState_Processing) {
		return false, entity.ErrHookStoreCorrupt
	}
	for _, mt := range m.Turns {
		var turn model.ExptTurnResult
		if err := tx.Unscoped().Where("id=?", mt.ResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&turn).Error; err != nil {
			return false, err
		}
		if turn.DeletedAt.Valid || turn.SpaceID != key.WorkspaceID || turn.ExptID != key.ExperimentID || turn.ExptRunID != key.RunID || turn.ItemID != itemID || turn.ItemVersionID != m.Frozen.ItemVersionID || turn.TurnID != mt.TurnID || turn.TurnIdx == nil || *turn.TurnIdx != mt.TurnIdx {
			return false, entity.ErrHookStoreCorrupt
		}
		if entity.TurnRunState(turn.Status) == entity.TurnRunState_Queueing {
			if err := hookRunScope(tx.Model(&model.ExptTurnResult{}), key).Where("id=?", mt.ResultID).UpdateColumn("status", int32(entity.TurnRunState_Processing)).Error; err != nil {
				return false, err
			}
		}
	}
	if logs[0].Status == int32(entity.ItemRunState_Queueing) {
		if err := hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), key).Where("id=?", m.ItemRunLogID).UpdateColumn("status", int32(entity.ItemRunState_Processing)).Error; err != nil {
			return false, err
		}
	}
	if item.Status == int32(entity.ItemRunState_Queueing) {
		if err := hookRunScope(tx.Model(&model.ExptItemResult{}), key).Where("id=?", m.ItemResultID).UpdateColumn("status", int32(entity.ItemRunState_Processing)).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
