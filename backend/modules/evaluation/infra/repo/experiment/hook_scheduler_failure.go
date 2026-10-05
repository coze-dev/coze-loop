// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"reflect"
)

func hookSchedulerFailureActive(tx *gorm.DB, key entity.HookRunKey, expt *model.Experiment, life *model.ExptLifecycleRun, bindings ...*boundHookExecution) (bool, error) {
	if expt.LatestRunID != key.RunID || life.Gate == 2 || life.FinalizeState != 0 || entity.IsExptFinished(entity.ExptStatus(expt.Status)) || entity.ExptStatus(expt.Status) == entity.ExptStatus_Terminating {
		return false, nil
	}
	var log model.ExptRunLog
	if err := hookRunScope(tx, key).First(&log).Error; err != nil {
		return false, err
	}
	if entity.IsExptFinished(entity.ExptStatus(gptr.Indirect(log.Status))) || entity.ExptStatus(gptr.Indirect(log.Status)) == entity.ExptStatus_Terminating {
		return false, nil
	}
	if life.Gate != 1 || !life.ExecutionInitialized || life.PlanState != 1 || !(entity.HookExecutionInitializationRequired(life.BeforeEnabled || life.AfterEnabled, entity.ExptRunMode(gptr.Indirect(log.Mode)), entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) || boundFinalizationRequired(firstFinalizationBinding(bindings), life, entity.ExptRunMode(gptr.Indirect(log.Mode)))) {
		return false, entity.ErrHookExecutionUnsupported
	}
	return true, nil
}

func (r *hookFinalizationRepo) ReadHookSchedulerFailureItem(ctx context.Context, key entity.HookRunKey, scope string, itemID int64) (out *entity.HookTerminationItem, err error) {
	err = r.itemArchiveTransaction(ctx, key, scope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		active, err := hookSchedulerFailureActive(tx, key, expt, life, r.binding)
		if err != nil || !active {
			return err
		}
		item, _, err := r.readArchiveItem(tx, key, itemID)
		if err != nil {
			return err
		}
		if item.Manifest.Ordinal >= life.PlanCount {
			return entity.ErrHookStoreCorrupt
		}
		if item.Item.Status != int32(entity.ItemRunState_Processing) || item.Item.ResultState == int32(entity.ExptItemResultStateResulted) {
			return nil
		}
		out = item
		return nil
	})
	return out, err
}

func (r *hookFinalizationRepo) ApplyHookSchedulerFailure(ctx context.Context, in entity.HookSchedulerFailureInput) (changed bool, err error) {
	if in.Item == nil || in.Item.Item == nil || in.Item.Manifest.Validate() != nil || in.Zombie && (in.ZombieSeconds <= 0 || in.ExpiredBefore.IsZero()) || !in.Zombie && (in.SandboxStatus == "" || len(in.ObservedTargetIDs) == 0) {
		return false, entity.ErrHookStoreCorrupt
	}
	key := in.Item.Manifest.Key
	err = r.itemArchiveTransaction(ctx, key, in.ExecutionScope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		active, err := hookSchedulerFailureActive(tx, key, expt, life, r.binding)
		if err != nil || !active {
			return err
		}
		current, _, err := r.readArchiveItem(tx, key, in.Item.Manifest.Frozen.ItemID)
		if err != nil {
			return err
		}
		if current.Item.Status != int32(entity.ItemRunState_Processing) || current.Item.ResultState == int32(entity.ExptItemResultStateResulted) {
			return nil
		}
		if in.Zombie && !current.Item.UpdatedAt.Before(in.ExpiredBefore) {
			return nil
		}
		if !reflect.DeepEqual(current.Manifest, in.Item.Manifest) || !reflect.DeepEqual(current.Item, in.Item.Item) || !reflect.DeepEqual(current.Turns, in.Item.Turns) {
			return entity.ErrHookStoreConflict
		}
		message := errno.SerializeErr(errno.NewSandboxTerminatedBeforeReportErr(in.SandboxStatus))
		if in.Zombie {
			message = errno.SerializeErr(errno.NewItemZombieTimeoutErr(in.ZombieSeconds, in.Async))
		}
		if len(current.Turns) == 0 {
			if !in.Zombie || len(in.Item.Targets) != 0 || len(in.Item.Evaluators) != 0 {
				return entity.ErrHookStoreCorrupt
			}
			if err := markHookNoExecutionFailure(tx, key, current, message); err != nil {
				return err
			}
			changed = true
			return nil
		}
		qualified, err := closeHookSchedulerFailureRecords(tx, in, current)
		if err != nil || !qualified {
			return err
		}
		var item model.ExptItemResult
		if err := tx.Unscoped().Where("id=?", current.Manifest.ItemResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
			return err
		}
		if item.DeletedAt.Valid || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != current.Manifest.Frozen.ItemID || item.ItemVersionID != current.Manifest.Frozen.ItemVersionID || item.ItemIdx == nil || *item.ItemIdx != int32(current.Manifest.ProjectionOrdinal()) {
			return entity.ErrHookStoreCorrupt
		}
		if entity.IsItemRunFinished(entity.ItemRunState(item.Status)) {
			return entity.ErrHookStoreConflict
		}
		for _, turn := range current.Turns {
			switch turn.Status {
			case entity.TurnRunState_Processing, entity.TurnRunState_Queueing:
				if err := hookRunScope(tx.Model(&model.ExptTurnResultRunLog{}), key).Where("id=? AND status=?", turn.ID, int32(turn.Status)).UpdateColumns(map[string]any{"status": int32(entity.TurnRunState_Fail), "err_msg": []byte(message)}).Error; err != nil {
					return err
				}
			case entity.TurnRunState_Success, entity.TurnRunState_Fail, entity.TurnRunState_Terminal:
			default:
				return entity.ErrHookStoreCorrupt
			}
		}
		if err := hookOneRow(hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), key).Where("id=? AND status=?", current.Item.ID, int32(entity.ItemRunState_Processing)).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateLogged), "err_msg": []byte(message)})); err != nil {
			return err
		}
		// Status/counts remain owned by normal archival, exactly as in the legacy failure path.
		if err := hookRunScope(tx.Model(&model.ExptItemResult{}), key).Where("id=?", item.ID).UpdateColumn("err_msg", []byte(message)).Error; err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return false, err
	}
	return changed, nil
}
