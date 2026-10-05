// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func (r *hookTurnProgressRepo) ApplyHookConsumerControl(ctx context.Context, in entity.HookConsumerControlInput) (out entity.HookConsumerControlResult, err error) {
	if ctx == nil || r == nil || r.executionScope == nil || (entity.HookStoreGuard{Key: in.Key}).Validate() != nil || in.ItemID <= 0 {
		return out, entity.ErrHookStoreCorrupt
	}
	switch in.Action {
	case entity.HookConsumerYield:
		if in.RetryTimes <= 0 {
			return out, entity.ErrHookStoreCorrupt
		}
	case entity.HookConsumerFail:
		if in.ErrorMessage == "" {
			return out, entity.ErrHookStoreCorrupt
		}
	case entity.HookConsumerStartReserved, entity.HookConsumerReservationAbsent:
	default:
		return out, entity.ErrHookStoreCorrupt
	}
	scope, err := r.executionScope(ctx)
	if err != nil {
		return out, err
	}
	if !hookPlanProviderAvailable(&hookRunRepo{provider: r.provider}, ctx) {
		return out, entity.ErrHookExecutionStorage
	}
	var frozen model.ExptLifecycleRun
	if err := hookRunScope(r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}), in.Key).Select("before_enabled", "after_enabled", "execution_scope").First(&frozen).Error; err != nil {
		return out, err
	}
	if frozen.ExecutionScope != scope {
		return out, entity.ErrHookStoreConflict
	}
	if !frozen.BeforeEnabled && !frozen.AfterEnabled {
		return out, nil
	}
	storage := &hookFinalizationRepo{provider: r.provider, binding: r.binding}
	err = storage.itemArchiveTransaction(ctx, in.Key, scope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		var run model.ExptRunLog
		if err := hookRunScope(tx, in.Key).First(&run).Error; err != nil {
			return err
		}
		if !entity.HookExecutionInitializationRequired(life.BeforeEnabled || life.AfterEnabled, entity.ExptRunMode(gptr.Indirect(run.Mode)), entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) && !boundFinalizationRequired(r.binding, life, entity.ExptRunMode(gptr.Indirect(run.Mode))) {
			return nil
		}
		out.Handled = true
		if expt.LatestRunID != in.Key.RunID || life.Gate == 2 || life.FinalizeState != 0 || entity.IsExptFinished(entity.ExptStatus(expt.Status)) || entity.ExptStatus(expt.Status) == entity.ExptStatus_Terminating || entity.IsExptFinished(entity.ExptStatus(gptr.Indirect(run.Status))) || entity.ExptStatus(gptr.Indirect(run.Status)) == entity.ExptStatus_Terminating {
			return nil
		}
		if life.Gate != 1 || !life.ExecutionInitialized || life.PlanState != 1 {
			return entity.ErrHookAdmissionDenied
		}
		current, _, err := storage.readArchiveItem(tx, in.Key, in.ItemID)
		if err != nil {
			return err
		}
		if current.Manifest.Ordinal >= life.PlanCount {
			return entity.ErrHookStoreCorrupt
		}
		if entity.IsItemRunFinished(entity.ItemRunState(current.Item.Status)) || current.Item.ResultState == int32(entity.ExptItemResultStateResulted) {
			return nil
		}
		if current.Item.Status != int32(entity.ItemRunState_Queueing) && current.Item.Status != int32(entity.ItemRunState_Processing) {
			return entity.ErrHookStoreCorrupt
		}
		if in.Action == entity.HookConsumerFail {
			if err := failHookConsumerItem(tx, in, current); err != nil {
				return err
			}
			out.Changed = true
			return nil
		}
		var item model.ExptItemResultRunLog
		if err := hookRunScope(tx, in.Key).Where("id=?", current.Item.ID).First(&item).Error; err != nil {
			return err
		}
		fields := map[string]any{}
		next := entity.ItemRunState_Queueing
		switch in.Action {
		case entity.HookConsumerYield:
			if item.Status != int32(entity.ItemRunState_Processing) || item.RetryTimes >= in.RetryTimes {
				return nil
			}
			fields["retry_times"], fields["err_msg"] = in.RetryTimes, []byte(in.ErrorMessage)
		case entity.HookConsumerStartReserved:
			if item.Status != int32(entity.ItemRunState_Queueing) || item.QuotaReservationState != int32(entity.QuotaReservationStateReserved) {
				out.Proceed = item.Status == int32(entity.ItemRunState_Processing) && item.QuotaReservationState == int32(entity.QuotaReservationStateNone)
				return nil
			}
			next = entity.ItemRunState_Processing
			fields["quota_reservation_state"] = int32(entity.QuotaReservationStateNone)
			out.Proceed = true
		case entity.HookConsumerReservationAbsent:
			if item.Status == int32(entity.ItemRunState_Queueing) && item.QuotaReservationState == int32(entity.QuotaReservationStateReserved) {
				if err := hookOneRow(hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), in.Key).Where("id=? AND status=? AND quota_reservation_state=?", item.ID, item.Status, item.QuotaReservationState).UpdateColumn("quota_reservation_state", int32(entity.QuotaReservationStateNone))); err != nil {
					return err
				}
				out.Changed = true
				return nil
			}
			if item.Status != int32(entity.ItemRunState_Processing) || item.QuotaReservationState != int32(entity.QuotaReservationStateNone) {
				return nil
			}
		}
		fields["status"] = int32(next)
		if err := hookOneRow(hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), in.Key).Where("id=? AND status=? AND quota_reservation_state=?", item.ID, item.Status, item.QuotaReservationState).UpdateColumns(fields)); err != nil {
			return err
		}
		projectionChanged, err := projectHookConsumerState(tx, in.Key, current, next, in.Action != entity.HookConsumerYield)
		if err != nil {
			return err
		}
		out.ProjectionChanged = projectionChanged
		out.Changed = true
		return nil
	})
	if err != nil {
		return entity.HookConsumerControlResult{Handled: out.Handled}, err
	}
	return out, nil
}

func failHookConsumerItem(tx *gorm.DB, in entity.HookConsumerControlInput, current *entity.HookTerminationItem) error {
	if len(current.Turns) == 0 {
		return markHookNoExecutionFailure(tx, in.Key, current, in.ErrorMessage)
	}
	for _, turn := range current.Turns {
		switch turn.Status {
		case entity.TurnRunState_Queueing, entity.TurnRunState_Processing:
			if err := hookOneRow(hookRunScope(tx.Model(&model.ExptTurnResultRunLog{}), in.Key).Where("id=? AND status=?", turn.ID, int32(turn.Status)).UpdateColumns(map[string]any{"status": int32(entity.TurnRunState_Fail), "err_msg": []byte(in.ErrorMessage)})); err != nil {
				return err
			}
		case entity.TurnRunState_Success, entity.TurnRunState_Fail, entity.TurnRunState_Terminal:
		default:
			return entity.ErrHookStoreCorrupt
		}
	}
	return hookOneRow(hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), in.Key).Where("id=? AND status=?", current.Item.ID, current.Item.Status).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateLogged), "err_msg": []byte(in.ErrorMessage)}))
}

func projectHookConsumerState(tx *gorm.DB, key entity.HookRunKey, current *entity.HookTerminationItem, next entity.ItemRunState, turns bool) (changed bool, err error) {
	m := current.Manifest
	var item model.ExptItemResult
	if err := tx.Unscoped().Where("id=?", m.ItemResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
		return false, err
	}
	if item.DeletedAt.Valid || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != m.Frozen.ItemID || item.ItemVersionID != m.Frozen.ItemVersionID || item.ItemIdx == nil || *item.ItemIdx != int32(m.ProjectionOrdinal()) {
		return false, entity.ErrHookStoreCorrupt
	}
	if entity.IsItemRunFinished(entity.ItemRunState(item.Status)) {
		return false, entity.ErrHookStoreConflict
	}
	if turns {
		for _, mt := range m.Turns {
			var turn model.ExptTurnResult
			if err := tx.Unscoped().Where("id=?", mt.ResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&turn).Error; err != nil {
				return false, err
			}
			if turn.DeletedAt.Valid || turn.SpaceID != key.WorkspaceID || turn.ExptID != key.ExperimentID || turn.ExptRunID != key.RunID || turn.ItemID != m.Frozen.ItemID || turn.ItemVersionID != m.Frozen.ItemVersionID || turn.TurnID != mt.TurnID || turn.TurnIdx == nil || *turn.TurnIdx != mt.TurnIdx {
				return false, entity.ErrHookStoreCorrupt
			}
			if turn.Status == int32(entity.TurnRunState_Queueing) || turn.Status == int32(entity.TurnRunState_Processing) {
				status := entity.TurnRunState_Queueing
				if next == entity.ItemRunState_Processing {
					status = entity.TurnRunState_Processing
				}
				if turn.Status != int32(status) {
					if err := hookOneRow(hookRunScope(tx.Model(&model.ExptTurnResult{}), key).Where("id=? AND status=?", turn.ID, turn.Status).UpdateColumn("status", int32(status))); err != nil {
						return false, err
					}
					changed = true
				}
			}
		}
	}
	if item.Status == int32(next) {
		return changed, nil
	}
	columns := map[int32]string{int32(entity.ItemRunState_Queueing): "pending_cnt", int32(entity.ItemRunState_Processing): "processing_cnt"}
	oldColumn, newColumn := columns[item.Status], columns[int32(next)]
	if oldColumn == "" || newColumn == "" {
		return false, entity.ErrHookStoreCorrupt
	}
	if err := hookOneRow(hookRunScope(tx.Model(&model.ExptItemResult{}), key).Where("id=? AND status=?", item.ID, item.Status).UpdateColumn("status", int32(next))); err != nil {
		return false, err
	}
	if err := hookOneRow(tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=? AND "+oldColumn+">0", key.WorkspaceID, key.ExperimentID).UpdateColumns(map[string]any{oldColumn: gorm.Expr(oldColumn + " - 1"), newColumn: gorm.Expr(newColumn + " + 1")})); err != nil {
		return false, err
	}
	return true, nil
}
