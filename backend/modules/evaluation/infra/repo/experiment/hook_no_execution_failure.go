// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Caller holds the active original Run's experiment/lifecycle/item transaction boundary.
func markHookNoExecutionFailure(tx *gorm.DB, key entity.HookRunKey, item *entity.HookTerminationItem, message string) error {
	if item == nil || item.Item == nil || item.Manifest.Key != key || item.Manifest.Validate() != nil || message == "" {
		return entity.ErrHookStoreCorrupt
	}
	current, row, err := readHookArchiveItem(tx, key, item.Manifest.Frozen.ItemID)
	if err != nil {
		return err
	}
	storedIdentity, suppliedIdentity := current.Manifest, item.Manifest
	storedIdentity.NoExecutionFailure, suppliedIdentity.NoExecutionFailure = false, false
	if !reflect.DeepEqual(storedIdentity, suppliedIdentity) || current.Item.ID != item.Item.ID || len(current.Turns) != 0 || len(item.Targets) != 0 || len(item.Evaluators) != 0 {
		return entity.ErrHookStoreConflict
	}
	if err := validateHookNoExecutionEvidence(tx, key, current); err != nil {
		return err
	}
	if current.Manifest.NoExecutionFailure {
		if string(current.Item.ErrMsg) == message {
			return nil
		}
		return entity.ErrHookStoreConflict
	}
	if (current.Item.Status != int32(entity.ItemRunState_Queueing) && current.Item.Status != int32(entity.ItemRunState_Processing)) || current.Item.ResultState == int32(entity.ExptItemResultStateResulted) {
		return entity.ErrHookAdmissionDenied
	}
	manifest := current.Manifest
	manifest.NoExecutionFailure = true
	raw, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	if err := hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), key).Where("id=?", row.ID).UpdateColumn("execution_manifest", raw)); err != nil {
		return err
	}
	return hookOneRow(hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), key).Where("id=? AND status=?", current.Item.ID, current.Item.Status).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Fail), "result_state": int32(entity.ExptItemResultStateLogged), "err_msg": []byte(message)}))
}

func validateHookNoExecutionEvidence(tx *gorm.DB, key entity.HookRunKey, item *entity.HookTerminationItem) error {
	m := item.Manifest
	if len(item.Turns) != 0 {
		return entity.ErrHookStoreCorrupt
	}
	var items []model.ExptItemResult
	var turns []model.ExptTurnResult
	ids := make([]int64, 0, len(m.Turns))
	for _, mt := range m.Turns {
		ids = append(ids, mt.ResultID)
	}
	if err := tx.Unscoped().Where("id=?", m.ItemResultID).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&items).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Where("id IN ?", ids).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&turns).Error; err != nil {
		return err
	}
	if len(items) != 1 || len(turns) != len(m.Turns) {
		return entity.ErrHookStoreCorrupt
	}
	p := items[0]
	if p.ID != m.ItemResultID || p.SpaceID != key.WorkspaceID || p.ExptID != key.ExperimentID || p.ExptRunID != key.RunID || p.ItemID != m.Frozen.ItemID || p.DeletedAt.Valid || p.ItemVersionID != m.Frozen.ItemVersionID || p.ItemIdx == nil || *p.ItemIdx != int32(m.Ordinal) {
		return entity.ErrHookStoreCorrupt
	}
	if p.Status != int32(entity.ItemRunState_Queueing) && p.Status != int32(entity.ItemRunState_Processing) && !(m.NoExecutionFailure && p.Status == int32(entity.ItemRunState_Fail) && bytes.Equal(gptr.Indirect(p.ErrMsg), item.Item.ErrMsg)) {
		return entity.ErrHookStoreCorrupt
	}
	expected := map[int64]entity.HookExecutionTurnManifest{}
	for _, mt := range m.Turns {
		expected[mt.ResultID] = mt
	}
	for _, tr := range turns {
		mt, ok := expected[tr.ID]
		if !ok || tr.SpaceID != key.WorkspaceID || tr.ExptID != key.ExperimentID || tr.ExptRunID != key.RunID || tr.ItemID != m.Frozen.ItemID || tr.DeletedAt.Valid || tr.ItemVersionID != m.Frozen.ItemVersionID || tr.TurnID != mt.TurnID || tr.TurnIdx == nil || *tr.TurnIdx != mt.TurnIdx || tr.TargetResultID != 0 || tr.WeightedScore != nil {
			return entity.ErrHookStoreCorrupt
		}
		if tr.Status != int32(entity.TurnRunState_Queueing) && tr.Status != int32(entity.TurnRunState_Processing) && !(m.NoExecutionFailure && tr.Status == int32(entity.TurnRunState_Fail) && bytes.Equal(gptr.Indirect(tr.ErrMsg), item.Item.ErrMsg)) {
			return entity.ErrHookStoreCorrupt
		}
		delete(expected, tr.ID)
	}
	for _, c := range []struct {
		table any
		count int64
	}{{&model.ExptItemResult{}, 1}, {&model.ExptTurnResult{}, int64(len(m.Turns))}} {
		var n int64
		if err := hookRunScope(tx.Unscoped().Model(c.table), key).Where("item_id=?", m.Frozen.ItemID).Count(&n).Error; err != nil {
			return err
		}
		if n != c.count {
			return entity.ErrHookStoreCorrupt
		}
	}
	return checkHookNoExecutionReferences(tx, key, ids)
}

func checkHookNoExecutionReferences(tx *gorm.DB, key entity.HookRunKey, ids []int64) error {
	// Both predicates have existing leading indexes and retain wrong-tenant/wrong-expt checks.
	for _, bound := range []struct {
		sql string
		id  int64
	}{{"space_id=?", key.WorkspaceID}, {"expt_id=?", key.ExperimentID}} {
		var n int64
		if err := tx.Unscoped().Model(&model.ExptTurnEvaluatorResultRef{}).Where(bound.sql, bound.id).Where("expt_turn_result_id IN ?", ids).Count(&n).Error; err != nil {
			return err
		}
		if n != 0 {
			return entity.ErrHookStoreCorrupt
		}
	}
	return nil
}
