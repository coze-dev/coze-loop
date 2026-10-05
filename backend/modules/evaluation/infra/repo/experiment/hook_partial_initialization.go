// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Validate the complete frozen proof before cleanup, then commit at most one page per lock.
func (r *hookFinalizationRepo) PreparePartialInitializationTermination(ctx context.Context, key entity.HookRunKey, scope string) (handled bool, err error) {
	if (entity.HookStoreGuard{Key: key}).Validate() != nil || !hookGateASCII(scope) {
		return false, entity.ErrHookStoreCorrupt
	}
	var count int64
	err = r.read(ctx, func(tx *gorm.DB) error {
		if err := checkBoundFinalizationRun(tx, key, scope, r.binding); err != nil {
			return err
		}
		var life model.ExptLifecycleRun
		if err := hookRunScope(tx, key).First(&life).Error; err != nil {
			return err
		}
		if life.ExecutionScope != scope {
			return entity.ErrHookStoreConflict
		}
		if life.ExecutionInitialized {
			return nil
		}
		handled = true
		if !hookCancellation(&life) || life.FinalizeState != 1 {
			return entity.ErrHookFinalizationUnsettled
		}
		if _, err := readHookNeverAdmittedProof(tx, key, &life, true, r.binding); err != nil {
			return err
		}
		count = life.PlanCount
		return nil
	})
	if err != nil || !handled {
		return handled, err
	}
	for start := int64(0); start < count; start += 100 {
		if err := ctx.Err(); err != nil {
			return true, err
		}
		err = r.itemArchiveRecoveryTransaction(ctx, key, scope, true, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
			if !hookCancellation(life) || life.FinalizeState != 1 || life.Gate != 2 || life.ExecutionInitialized || life.ExecutionStarted || life.PlanCount != count {
				return entity.ErrHookFinalizationUnsettled
			}
			var rows []model.ExptLifecycleRunItem
			end := min(start+100, count)
			if err := hookRunScope(tx, key).Where("ordinal>=? AND ordinal<?", start, end).Order("ordinal").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&rows).Error; err != nil {
				return err
			}
			if int64(len(rows)) != end-start {
				return entity.ErrHookStoreCorrupt
			}
			if err := checkBoundFinalizationPage(tx, key, rows, r.binding, true); err != nil {
				return err
			}
			for i, row := range rows {
				if row.Ordinal != start+int64(i) || row.AdmittedAt != nil {
					return entity.ErrHookStoreCorrupt
				}
				if row.ExecutionManifest == nil {
					continue
				}
				current, err := readHookPartialInitializationItem(tx, key, expt, row, true)
				if err != nil {
					return err
				}
				if current.Item.ResultState == int32(entity.ExptItemResultStateResulted) {
					continue
				}
				current.Item.Status = int32(entity.ItemRunState_Terminal)
				current.Item.ErrMsg = []byte(errno.SerializeErr(errno.NewItemManuallyTerminatedErr()))
				if expt.LatestRunID == key.RunID && !expt.DeletedAt.Valid {
					if err := materializeHookItem(tx, entity.HookItemArchiveInput{Key: key, ExecutionScope: scope, ItemID: row.ItemID, Cancellation: true}, current, true); err != nil {
						return err
					}
				}
				if err := hookOneRow(hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), key).Where("id=?", current.Item.ID).UpdateColumns(map[string]any{"status": current.Item.Status, "err_msg": current.Item.ErrMsg, "result_state": int32(entity.ExptItemResultStateResulted)})); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return true, err
		}
	}
	return true, nil
}

func readHookPartialInitializationItem(tx *gorm.DB, key entity.HookRunKey, expt *model.Experiment, row model.ExptLifecycleRunItem, allowUnsettled bool) (*entity.HookTerminationItem, error) {
	current, _, err := readHookArchiveItemRows(tx, key, row.ItemID, false)
	if err != nil {
		return nil, err
	}
	m := current.Manifest
	if row.AdmittedAt != nil || m.NoExecutionFailure || m.TurnLogsInitialized == nil || *m.TurnLogsInitialized || len(current.Turns) != 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	message := []byte(errno.SerializeErr(errno.NewItemManuallyTerminatedErr()))
	item := current.Item
	settled := item.Status == int32(entity.ItemRunState_Terminal) && item.ResultState == int32(entity.ExptItemResultStateResulted) && bytes.Equal(item.ErrMsg, message)
	initial := item.Status == int32(entity.ItemRunState_Queueing) && item.ResultState == 0 && len(item.ErrMsg) == 0
	if (!initial && !settled) || item.LogID != "" || item.RetryTimes != 0 || item.QuotaReservationState != 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	if !allowUnsettled && !settled {
		return nil, entity.ErrHookFinalizationUnsettled
	}
	var result model.ExptItemResult
	if err := tx.Unscoped().Where("id=?", m.ItemResultID).First(&result).Error; err != nil {
		return nil, err
	}
	if result.DeletedAt.Valid || result.SpaceID != key.WorkspaceID || result.ExptID != key.ExperimentID || result.ExptRunID != key.RunID || result.ItemID != row.ItemID || result.ItemVersionID != row.ItemVersionID || result.ItemIdx == nil || int64(*result.ItemIdx) != m.ProjectionOrdinal() || result.LogID != "" || len(gptr.Indirect(result.Ext)) != 0 && entity.ExptType(expt.ExptType) != entity.ExptType_Online {
		return nil, entity.ErrHookStoreCorrupt
	}
	projected := result.Status == int32(entity.ItemRunState_Terminal) && bytes.Equal(gptr.Indirect(result.ErrMsg), message)
	pristine := result.Status == int32(entity.ItemRunState_Queueing) && len(gptr.Indirect(result.ErrMsg)) == 0
	if (!pristine && !projected) || projected && !settled || expt.LatestRunID == key.RunID && !expt.DeletedAt.Valid && settled != projected {
		return nil, entity.ErrHookStoreCorrupt
	}
	ids := make([]int64, 0, len(m.Turns))
	for _, mt := range m.Turns {
		var tr model.ExptTurnResult
		if err := tx.Unscoped().Where("id=?", mt.ResultID).First(&tr).Error; err != nil {
			return nil, err
		}
		if tr.DeletedAt.Valid || tr.SpaceID != key.WorkspaceID || tr.ExptID != key.ExperimentID || tr.ExptRunID != key.RunID || tr.ItemID != row.ItemID || tr.ItemVersionID != row.ItemVersionID || tr.TurnID != mt.TurnID || tr.TurnIdx == nil || *tr.TurnIdx != mt.TurnIdx || tr.TargetResultID != 0 || tr.TraceID != 0 || tr.LogID != "" || tr.WeightedScore != nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		if projected && (tr.Status != int32(entity.TurnRunState_Terminal) || !bytes.Equal(gptr.Indirect(tr.ErrMsg), message)) || !projected && (tr.Status != int32(entity.TurnRunState_Queueing) || len(gptr.Indirect(tr.ErrMsg)) != 0) {
			return nil, entity.ErrHookStoreCorrupt
		}
		ids = append(ids, mt.ResultID)
	}
	if err := checkHookNoExecutionReferences(tx, key, ids); err != nil {
		return nil, err
	}
	return current, nil
}
