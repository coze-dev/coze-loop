// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func hookProgressItemRow(tx *gorm.DB, key entity.HookRunKey, itemID, version int64) *gorm.DB {
	return hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), key).Where("item_id=? AND item_version_id=?", itemID, version)
}

func lockHookProgressItem(tx *gorm.DB, key entity.HookRunKey, itemID, version int64) (*model.ExptItemResultRunLog, error) {
	var current model.ExptItemResultRunLog
	if err := hookProgressItemRow(tx, key, itemID, version).Clauses(clause.Locking{Strength: "UPDATE"}).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrHookStoreMissing
		}
		return nil, err
	}
	return &current, nil
}

func hookProgressItemResulted(item *model.ExptItemResultRunLog) bool {
	return item != nil && item.ResultState != nil && *item.ResultState == int32(entity.ExptItemResultStateResulted)
}

// Only cancellation can repeat archival after admission closes.
func hookProgressNormalFrozen(life *model.ExptLifecycleRun, item *model.ExptItemResultRunLog) bool {
	return !hookCancellation(life) && (life.FinalizeState == 1 || hookProgressItemResulted(item))
}

func (r *hookTurnProgressRepo) WriteItemRun(ctx context.Context, in entity.HookItemRunWriteInput) (terminal bool, err error) {
	if in.Status != entity.ItemRunState_Processing && in.Status != entity.ItemRunState_Success && in.Status != entity.ItemRunState_Fail {
		return false, entity.ErrHookStoreCorrupt
	}
	err = r.itemTransaction(ctx, in.HookRunKey, in.ItemID, in.ItemVersionID, func(tx *gorm.DB, life *model.ExptLifecycleRun) error {
		if life.FinalizeState == 2 {
			return entity.ErrHookStoreConflict
		}
		q := hookProgressItemRow(tx, in.HookRunKey, in.ItemID, in.ItemVersionID)
		current, e := lockHookProgressItem(tx, in.HookRunKey, in.ItemID, in.ItemVersionID)
		if e != nil {
			return e
		}
		terminal = entity.ItemRunState(current.Status) == entity.ItemRunState_Terminal
		var currentErr string
		if current.ErrMsg != nil {
			currentErr = string(*current.ErrMsg)
		}
		effective := !terminal && (current.Status != int32(in.Status) || in.ErrMsg != nil && currentErr != *in.ErrMsg)
		resulted := hookProgressItemResulted(current)
		if hookProgressNormalFrozen(life, current) {
			if effective {
				return entity.ErrHookStoreConflict
			}
			return nil
		}
		if life.FinalizeState == 1 && resulted && !effective {
			return nil
		}
		fields := make(map[string]any)
		if !terminal {
			fields["status"] = int32(in.Status)
			if in.ErrMsg != nil {
				fields["err_msg"] = []byte(*in.ErrMsg)
			}
		}
		if in.Status != entity.ItemRunState_Processing && !resulted || hookCancellation(life) && effective && resulted {
			fields["result_state"] = int32(entity.ExptItemResultStateLogged)
		}
		if len(fields) == 0 {
			return nil
		}
		return q.Where("id=?", current.ID).Updates(fields).Error
	})
	return terminal, err
}
