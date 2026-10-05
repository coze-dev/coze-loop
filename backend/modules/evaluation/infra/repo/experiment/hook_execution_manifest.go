// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package experiment

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func executionTuple(row model.ExptLifecycleRunItem) entity.HookPlanItem {
	return entity.HookPlanItem{ID: row.ID, SourceSpaceID: row.SourceSpaceID, EvalSetID: row.EvalSetID, EvalSetVersionID: row.EvalSetVersionID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID}
}

func readExecutionPage(tx *gorm.DB, s *lockedHookRun, start int64, limit int, allowInputExt ...bool) (*entity.HookExecutionInitializationPage, []model.ExptLifecycleRunItem, error) {
	if start < 0 || start > s.life.PlanCount {
		return nil, nil, entity.ErrHookStoreConflict
	}
	end := start + min(int64(limit), s.life.PlanCount-start)
	var rows []model.ExptLifecycleRunItem
	if err := hookRunScope(tx, s.view.State.Key).Where("ordinal>=? AND ordinal<?", start, end).Order("ordinal ASC,id ASC").Limit(limit + 1).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	if int64(len(rows)) != end-start {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	page := &entity.HookExecutionInitializationPage{Count: s.life.PlanCount, Hash: s.view.PlanHash, RunVersion: s.life.Version, Initialized: s.life.ExecutionInitialized, NextOrdinal: end, HasMore: end < s.life.PlanCount, Items: make([]entity.HookExecutionInitializationItem, 0, len(rows))}
	for i, row := range rows {
		if row.Ordinal != start+int64(i) {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		item := entity.HookExecutionInitializationItem{Ordinal: row.Ordinal, Frozen: executionTuple(row)}
		if (entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: s.view.State.Key}, StartOrdinal: row.Ordinal, Items: []entity.HookPlanItem{item.Frozen}}).Validate() != nil || (s.life.ExecutionInitialized && row.ExecutionManifest == nil) {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		if row.ExecutionManifest != nil {
			raw := *row.ExecutionManifest
			if len(raw) == 0 || len(raw) > 16777215 {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
			var manifest entity.HookExecutionManifest
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&manifest) != nil || decoder.Decode(new(any)) != io.EOF || manifest.Validate() != nil || manifest.Key != s.view.State.Key || manifest.Ordinal != row.Ordinal || manifest.Frozen != item.Frozen {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
			item.Manifest = &manifest
		}
		page.Items = append(page.Items, item)
	}
	if err := verifyExecutionPageRecords(tx, s.view.State.Key, page.Items, !s.life.ExecutionInitialized, allowInputExt...); err != nil {
		return nil, nil, err
	}
	for _, item := range page.Items {
		if item.Manifest != nil && item.Manifest.NoExecutionFailure {
			proof, _, err := readHookArchiveItemRows(tx, s.view.State.Key, item.Frozen.ItemID, false)
			if err != nil {
				return nil, nil, err
			}
			if !hookNoExecutionFailure(proof) {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
			if err := validateHookNoExecutionEvidence(tx, s.view.State.Key, proof); err != nil {
				return nil, nil, err
			}
		}
	}
	return page, rows, nil
}

func verifyExecutionRecords(tx *gorm.DB, key entity.HookRunKey, item entity.HookExecutionInitializationItem, initial bool) error {
	return verifyExecutionPageRecords(tx, key, []entity.HookExecutionInitializationItem{item}, initial)
}

func verifyExecutionPageRecords(tx *gorm.DB, key entity.HookRunKey, page []entity.HookExecutionInitializationItem, initial bool, allowInputExt ...bool) error {
	if len(page) == 0 {
		return nil
	}
	itemIDs := make([]int64, 0, len(page))
	for _, item := range page {
		itemIDs = append(itemIDs, item.Frozen.ItemID)
	}
	var items []model.ExptItemResult
	var logs []model.ExptItemResultRunLog
	var turns []model.ExptTurnResult
	base := func(table any) *gorm.DB {
		// Item/turn keys span runs; retain foreign-run and soft-deleted occupants for validation.
		return tx.Unscoped().Model(table).Where("space_id=? AND expt_id=? AND item_id IN ?", key.WorkspaceID, key.ExperimentID, itemIDs).Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := base(&model.ExptItemResult{}).Find(&items).Error; err != nil {
		return err
	}
	if err := base(&model.ExptItemResultRunLog{}).Where("expt_run_id=?", key.RunID).Find(&logs).Error; err != nil {
		return err
	}
	if err := base(&model.ExptTurnResult{}).Order("item_id ASC,turn_idx ASC,turn_id ASC").Find(&turns).Error; err != nil {
		return err
	}
	itemsByID := make(map[int64][]model.ExptItemResult, len(page))
	logsByID := make(map[int64][]model.ExptItemResultRunLog, len(page))
	turnsByID := make(map[int64][]model.ExptTurnResult, len(page))
	for _, item := range items {
		itemsByID[item.ItemID] = append(itemsByID[item.ItemID], item)
	}
	for _, log := range logs {
		logsByID[log.ItemID] = append(logsByID[log.ItemID], log)
	}
	for _, turn := range turns {
		turnsByID[turn.ItemID] = append(turnsByID[turn.ItemID], turn)
	}
	for _, item := range page {
		id := item.Frozen.ItemID
		if err := validateExecutionRecords(key, item, initial, itemsByID[id], logsByID[id], turnsByID[id], allowInputExt...); err != nil {
			return err
		}
	}
	return nil
}

func validateExecutionRecords(key entity.HookRunKey, item entity.HookExecutionInitializationItem, initial bool, items []model.ExptItemResult, logs []model.ExptItemResultRunLog, turns []model.ExptTurnResult, allowInputExt ...bool) error {
	m := item.Manifest
	if m == nil {
		if len(items)+len(logs)+len(turns) != 0 {
			return entity.ErrHookStoreCorrupt
		}
		return nil
	}
	if len(items) != 1 || len(logs) != 1 || len(turns) != len(m.Turns) {
		return entity.ErrHookStoreCorrupt
	}
	result, log := items[0], logs[0]
	if result.ID != m.ItemResultID || result.ExptRunID != key.RunID || result.ItemVersionID != item.Frozen.ItemVersionID || result.ItemIdx == nil || int64(*result.ItemIdx) != m.ProjectionOrdinal() || result.DeletedAt.Valid ||
		log.ID != m.ItemRunLogID || log.ItemVersionID != item.Frozen.ItemVersionID || log.DeletedAt.Valid {
		return entity.ErrHookStoreCorrupt
	}
	if initial && (result.Status != int32(entity.ItemRunState_Queueing) || log.Status != int32(entity.ItemRunState_Queueing) || log.QuotaReservationState != 0 || log.RetryTimes != 0 || gptr.Indirect(log.ResultState) != 0 || len(gptr.Indirect(result.ErrMsg)) != 0 || len(gptr.Indirect(log.ErrMsg)) != 0 || result.LogID != "" || log.LogID != "" || len(gptr.Indirect(result.Ext)) != 0 && !(len(allowInputExt) > 0 && allowInputExt[0])) {
		return entity.ErrHookStoreCorrupt
	}
	for i, turn := range turns {
		want := m.Turns[i]
		if turn.ID != want.ResultID || turn.ExptRunID != key.RunID || turn.ItemVersionID != item.Frozen.ItemVersionID || turn.TurnID != want.TurnID || turn.TurnIdx == nil || *turn.TurnIdx != want.TurnIdx || turn.DeletedAt.Valid {
			return entity.ErrHookStoreCorrupt
		}
		if initial && (turn.Status != int32(entity.TurnRunState_Queueing) || turn.TargetResultID != 0 || turn.TraceID != 0 || turn.LogID != "" || turn.WeightedScore != nil || len(gptr.Indirect(turn.ErrMsg)) != 0) {
			return entity.ErrHookStoreCorrupt
		}
	}
	return nil
}

func writeExecutionItem(tx *gorm.DB, row model.ExptLifecycleRunItem, m entity.HookExecutionManifest) error {
	if m.NoExecutionFailure || m.TurnLogsInitialized != nil && *m.TurnLogsInitialized {
		return entity.ErrHookStoreCorrupt
	}
	m.TurnLogsInitialized = gptr.Of(false)
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(raw) > 16777215 {
		return entity.ErrHookStoreCorrupt
	}
	item := model.ExptItemResult{ID: m.ItemResultID, SpaceID: m.Key.WorkspaceID, ExptID: m.Key.ExperimentID, ExptRunID: m.Key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, ItemIdx: gptr.Of(int32(m.Ordinal)), Status: int32(entity.ItemRunState_Queueing)}
	log := model.ExptItemResultRunLog{ID: m.ItemRunLogID, SpaceID: m.Key.WorkspaceID, ExptID: m.Key.ExperimentID, ExptRunID: m.Key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, Status: int32(entity.ItemRunState_Queueing)}
	turns := make([]model.ExptTurnResult, 0, len(m.Turns))
	for _, turn := range m.Turns {
		turns = append(turns, model.ExptTurnResult{ID: turn.ResultID, SpaceID: m.Key.WorkspaceID, ExptID: m.Key.ExperimentID, ExptRunID: m.Key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: turn.TurnID, TurnIdx: gptr.Of(turn.TurnIdx), Status: int32(entity.TurnRunState_Queueing)})
	}
	for _, value := range []any{&item, &log, &turns} {
		if err = tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(value, 50).Error; err != nil {
			return err
		}
	}
	if err = verifyExecutionRecords(tx, m.Key, entity.HookExecutionInitializationItem{Ordinal: m.Ordinal, Frozen: m.Frozen, Manifest: &m}, true); err != nil {
		return err
	}
	return hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), m.Key).Where("id=? AND ordinal=? AND execution_manifest IS NULL", row.ID, row.Ordinal).UpdateColumn("execution_manifest", raw))
}
