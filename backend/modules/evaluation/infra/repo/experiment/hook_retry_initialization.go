// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func retryAncestors(tx *gorm.DB, key entity.HookRunKey, source int64) (map[int64]bool, error) {
	seen := map[int64]bool{}
	for source > 0 {
		if source == key.RunID || seen[source] {
			return nil, entity.ErrHookStoreCorrupt
		}
		var log model.ExptRunLog
		if err := tx.Unscoped().Where("id=? AND space_id=? AND expt_id=?", source, key.WorkspaceID, key.ExperimentID).First(&log).Error; err != nil {
			return nil, err
		}
		if log.DeletedAt.Valid || log.ExptRunID != source || log.Status == nil || !entity.IsExptFinished(entity.ExptStatus(*log.Status)) {
			return nil, entity.ErrHookStoreConflict
		}
		seen[source] = true
		if gptr.Indirect(log.LifecycleHookVersion) == 0 {
			break
		}
		var life model.ExptLifecycleRun
		if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", key.WorkspaceID, key.ExperimentID, source).First(&life).Error; err != nil {
			return nil, err
		}
		if life.FinalizeState != 2 {
			return nil, entity.ErrHookFinalizationUnsettled
		}
		source = gptr.Indirect(life.SourceRunID)
	}
	if len(seen) == 0 {
		return nil, entity.ErrHookStoreConflict
	}
	return seen, nil
}

func (r *hookRunRepo) readRetryExecutionPage(tx *gorm.DB, s *lockedHookRun, start int64, limit int) (*entity.HookExecutionInitializationPage, []model.ExptLifecycleRunItem, error) {
	if start < 0 || start > s.life.PlanCount || limit < 1 || limit > 100 {
		return nil, nil, entity.ErrHookStoreConflict
	}
	b := r.executionBinding
	key := s.view.State.Key
	ancestors, err := retryAncestors(tx, key, b.source.SourceRunID)
	if err != nil {
		return nil, nil, err
	}
	end := start + min(int64(limit), s.life.PlanCount-start)
	var rows []model.ExptLifecycleRunItem
	if err := hookRunScope(tx, key).Where("ordinal>=? AND ordinal<?", start, end).Order("ordinal").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	if int64(len(rows)) != end-start {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	page := &entity.HookExecutionInitializationPage{Count: s.life.PlanCount, Hash: s.view.PlanHash, RunVersion: s.life.Version, Initialized: s.life.ExecutionInitialized, NextOrdinal: end, HasMore: end < s.life.PlanCount, BoundSnapshotHash: b.source.SnapshotHash}
	for i, row := range rows {
		if row.Ordinal != start+int64(i) {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		item := entity.HookExecutionInitializationItem{Ordinal: row.Ordinal, Frozen: executionTuple(row)}
		if row.ExecutionManifest != nil {
			m, err := hookTerminationManifest(key, row)
			if err != nil {
				return nil, nil, err
			}
			item.Manifest = &m
			if err := verifyExecutionRecords(tx, key, item, false); err != nil {
				return nil, nil, err
			}
			if err := checkBoundFinalizationRefs(tx, key, []entity.HookExecutionManifest{m}, b, true); err != nil {
				return nil, nil, err
			}
			if m.ItemRef != nil {
				item.ItemRefConfigHash = m.ItemRef.ConfigHash
			}
		} else {
			if s.life.ExecutionInitialized {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
			prototype, err := r.retryProjectionSource(tx, row, ancestors)
			if err != nil {
				return nil, nil, err
			}
			item.Reuse = prototype
			if b.source.Execution.SingleSet {
				item.ItemRefConfigHash = ""
			} else if prototype != nil {
				item.ItemRefConfigHash = prototype.ItemRef.ConfigHash
			} else {
				cfg, err := b.config(item.Frozen)
				if err != nil {
					return nil, nil, err
				}
				item.ItemRefConfigHash = cfg.hash
			}
		}
		page.Items = append(page.Items, item)
	}
	return page, rows, nil
}

func (r *hookRunRepo) retryProjectionSource(tx *gorm.DB, row model.ExptLifecycleRunItem, ancestors map[int64]bool) (*entity.HookExecutionManifest, error) {
	b := r.executionBinding
	key := b.source.Key
	var item model.ExptItemResult
	err := tx.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", key.WorkspaceID, key.ExperimentID, row.ItemID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !(b.source.Execution.SingleSet && b.source.Mode == entity.EvaluationModeRetryAll) {
			var source model.ExptLifecycleRun
			if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", key.WorkspaceID, key.ExperimentID, b.source.SourceRunID).First(&source).Error; err != nil {
				return nil, err
			}
			if source.ExecutionStarted || gptr.Indirect(source.TerminalReason) != "HOOK_BEFORE_FAILED" {
				return nil, entity.ErrHookStoreCorrupt
			}
		}
		if err := verifyExecutionRecords(tx, key, entity.HookExecutionInitializationItem{Ordinal: row.Ordinal, Frozen: executionTuple(row)}, true); err != nil {
			return nil, err
		}
		var refs int64
		if err := tx.Unscoped().Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=? AND item_id=?", key.WorkspaceID, key.ExperimentID, row.ItemID).Count(&refs).Error; err != nil {
			return nil, err
		}
		if refs != 0 {
			return nil, entity.ErrHookStoreCorrupt
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if item.DeletedAt.Valid || !ancestors[item.ExptRunID] || item.ItemVersionID != row.ItemVersionID || item.ItemIdx == nil || *item.ItemIdx < 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	var ref model.ExptItemRef
	raw := []byte("{}")
	hash := ""
	if !b.source.Execution.SingleSet {
		if err := tx.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", key.WorkspaceID, key.ExperimentID, row.ItemID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&ref).Error; err != nil {
			return nil, err
		}
		if ref.DeletedAt.Valid || ref.ItemVersionID != row.ItemVersionID || ref.EvalSetID != row.EvalSetID || ref.EvalSetVersionID != row.EvalSetVersionID || ref.OrderIdx != *item.ItemIdx || len(gptr.Indirect(ref.ItemConfig)) == 0 {
			return nil, entity.ErrHookStoreCorrupt
		}
		raw = append([]byte(nil), (*ref.ItemConfig)...)
		sum := sha256.Sum256(raw)
		hash = hex.EncodeToString(sum[:])
	}
	var owner model.ExptRunLog
	if err := tx.Where("id=?", item.ExptRunID).First(&owner).Error; err != nil {
		return nil, err
	}
	if gptr.Indirect(owner.LifecycleHookVersion) == 1 {
		var origin model.ExptLifecycleRunItem
		oldKey := key
		oldKey.RunID = item.ExptRunID
		if err := hookRunScope(tx, oldKey).Where("item_id=?", row.ItemID).First(&origin).Error; err != nil {
			return nil, err
		}
		m, err := hookTerminationManifest(oldKey, origin)
		if err != nil {
			return nil, err
		}
		if m.ItemResultID != item.ID || (!b.source.Execution.SingleSet && (m.ItemRef == nil || m.ItemRef.ID != ref.ID || m.ItemRef.ConfigHash != hash)) || (b.source.Execution.SingleSet && m.ItemRef != nil) {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	var turns []model.ExptTurnResult
	if err := tx.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", key.WorkspaceID, key.ExperimentID, row.ItemID).Order("turn_idx,turn_id").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&turns).Error; err != nil {
		return nil, err
	}
	if len(turns) == 0 {
		return nil, entity.ErrHookStoreCorrupt
	}
	m := &entity.HookExecutionManifest{Version: 1, Key: key, Ordinal: row.Ordinal, Frozen: executionTuple(row), ItemResultID: item.ID, ItemRef: &entity.HookExecutionItemRef{ID: ref.ID, ConfigHash: hash}, TurnLogsInitialized: gptr.Of(false), Retry: &entity.HookExecutionRetrySource{RunID: item.ExptRunID, Ordinal: int64(*item.ItemIdx), ItemConfig: raw}}
	if b.source.Execution.SingleSet {
		m.ItemRef = nil
	}
	for i, tr := range turns {
		if tr.DeletedAt.Valid || !ancestors[tr.ExptRunID] || tr.ItemVersionID != row.ItemVersionID || tr.TurnIdx == nil || *tr.TurnIdx != int32(i) {
			return nil, entity.ErrHookStoreCorrupt
		}
		m.Turns = append(m.Turns, entity.HookExecutionTurnManifest{TurnID: tr.TurnID, TurnIdx: int32(i), ResultID: tr.ID})
		source := entity.HookExecutionRetryTurn{RunID: tr.ExptRunID, TurnID: tr.TurnID, ResultID: tr.ID, TargetResultID: tr.TargetResultID}
		var refs []model.ExptTurnEvaluatorResultRef
		if err := tx.Where("space_id=? AND expt_id=? AND expt_turn_result_id=?", key.WorkspaceID, key.ExperimentID, tr.ID).Order("id").Find(&refs).Error; err != nil {
			return nil, err
		}
		for _, ref := range refs {
			source.Refs = append(source.Refs, &entity.ExptTurnEvaluatorResultRef{ID: ref.ID, SpaceID: ref.SpaceID, ExptID: ref.ExptID, ExptTurnResultID: ref.ExptTurnResultID, EvaluatorVersionID: ref.EvaluatorVersionID, EvaluatorResultID: ref.EvaluatorResultID, Alias: ref.Alias_, InlineKey: ref.InlineKey, SourceType: ref.SourceType})
		}
		m.Retry.Turns = append(m.Retry.Turns, source)
	}
	return m, nil
}

func (r *hookRunRepo) writeRetryExecutionItem(tx *gorm.DB, row model.ExptLifecycleRunItem, m entity.HookExecutionManifest, source *entity.HookExecutionManifest) error {
	if m.Validate() != nil || source == nil || m.ItemRunLogID <= 0 {
		return entity.ErrHookStoreCorrupt
	}
	want := source.Clone()
	want.ItemRunLogID = m.ItemRunLogID
	if !reflect.DeepEqual(want, m) {
		return entity.ErrHookStoreConflict
	}
	key := m.Key
	if err := hookOneRow(tx.Model(&model.ExptItemResult{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=?", m.ItemResultID, key.WorkspaceID, key.ExperimentID, m.Retry.RunID).UpdateColumns(map[string]any{"expt_run_id": key.RunID, "status": int32(entity.ItemRunState_Queueing), "log_id": "", "err_msg": nil, "ext": nil})); err != nil {
		return err
	}
	for _, tr := range m.Retry.Turns {
		if err := hookOneRow(tx.Model(&model.ExptTurnResult{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=?", tr.ResultID, key.WorkspaceID, key.ExperimentID, tr.RunID).UpdateColumns(map[string]any{"expt_run_id": key.RunID, "status": int32(entity.TurnRunState_Queueing), "target_result_id": 0, "weighted_score": nil, "log_id": "", "err_msg": nil})); err != nil {
			return err
		}
		if err := tx.Unscoped().Where("space_id=? AND expt_id=? AND expt_turn_result_id=?", key.WorkspaceID, key.ExperimentID, tr.ResultID).Delete(&model.ExptTurnEvaluatorResultRef{}).Error; err != nil {
			return err
		}
	}
	log := model.ExptItemResultRunLog{ID: m.ItemRunLogID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, Status: int32(entity.ItemRunState_Queueing)}
	if err := tx.Create(&log).Error; err != nil {
		return err
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), key).Where("id=? AND execution_manifest IS NULL", row.ID).UpdateColumn("execution_manifest", raw))
}

func (b *boundHookExecution) manifestConfig(m entity.HookExecutionManifest) (boundExecutionConfig, error) {
	cfg, err := b.config(m.Frozen)
	if err != nil {
		return cfg, err
	}
	if m.Retry != nil {
		if b.source.Execution.SingleSet {
			return cfg, nil
		}
		if !entity.HookBoundRetryMode(b.source.Mode) || m.ItemRef == nil {
			return cfg, entity.ErrHookStoreCorrupt
		}
		cfg.raw = append([]byte(nil), m.Retry.ItemConfig...)
		sum := sha256.Sum256(cfg.raw)
		cfg.hash = hex.EncodeToString(sum[:])
		if cfg.hash != m.ItemRef.ConfigHash {
			return cfg, entity.ErrHookStoreCorrupt
		}
	}
	return cfg, nil
}

func decodeRetryItemConfig(raw []byte) (*entity.ExptItemConfig, error) {
	var out *entity.ExptItemConfig
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if decoder.Decode(&out) != nil || out == nil || decoder.Decode(new(any)) != io.EOF {
		return nil, entity.ErrHookStoreCorrupt
	}
	return out, nil
}

func retryProjectionCounts(tx *gorm.DB, key entity.HookRunKey) (entity.HookFinalizationCounts, error) {
	var groups []struct {
		Status int32
		N      int64
	}
	var counts entity.HookFinalizationCounts
	if err := tx.Model(&model.ExptItemResult{}).Select("status, COUNT(*) AS n").Where("space_id=? AND expt_id=?", key.WorkspaceID, key.ExperimentID).Group("status").Find(&groups).Error; err != nil {
		return counts, err
	}
	for _, g := range groups {
		if g.N < 0 || g.N > 2147483647 {
			return counts, entity.ErrHookStoreCorrupt
		}
		switch entity.ItemRunState(g.Status) {
		case entity.ItemRunState_Queueing:
			counts.Pending = int32(g.N)
		case entity.ItemRunState_Processing:
			counts.Processing = int32(g.N)
		case entity.ItemRunState_Success:
			counts.Success = int32(g.N)
		case entity.ItemRunState_Fail:
			counts.Fail = int32(g.N)
		case entity.ItemRunState_Terminal:
			counts.Terminated = int32(g.N)
		default:
			return counts, entity.ErrHookStoreCorrupt
		}
	}
	return counts, nil
}
