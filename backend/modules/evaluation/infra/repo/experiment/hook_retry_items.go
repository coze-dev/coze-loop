// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ repo.IHookRetryItemsTailRepo = (*hookRunRepo)(nil)

func (r *hookRunRepo) RecordRetryItemsSourceFailure(ctx context.Context, key entity.HookRunKey, scope, cursor string) (bool, error) {
	exhausted := false
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		s, err := r.retryItemsRun(tx, key, scope)
		if err != nil {
			return err
		}
		if s.view.PlanCursor != cursor {
			return entity.ErrHookStoreConflict
		}
		c, log, err := retryItemsCursor(s)
		if err != nil {
			return err
		}
		if _, pending := c.Page(log.ItemIds); !pending {
			return entity.ErrHookStoreConflict
		}
		if c.Retries >= 10 {
			exhausted = true
			return nil
		}
		c.Retries++
		raw, err := c.Encode()
		if err != nil {
			return err
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		return updateHookLifecycle(tx, key, s.life.Version, now, map[string]any{"plan_cursor": raw})
	})
	return exhausted, err
}

func retryItemsCursor(s *lockedHookRun) (entity.HookRetryItemsCursor, *entity.ExptRunLog, error) {
	log, err := convert.NewExptRunLogConvertor().PO2DO(&s.log)
	if err != nil {
		return entity.HookRetryItemsCursor{}, nil, entity.ErrHookStoreCorrupt
	}
	c, err := entity.DecodeHookRetryItemsCursor(s.view.PlanCursor, s.view.State.Key, log.ItemIds, s.life.PlanCount, s.view.PlanHash)
	return c, log, err
}

func (r *hookRunRepo) retryItemsRun(tx *gorm.DB, key entity.HookRunKey, scope string) (*lockedHookRun, error) {
	if r.executionBinding == nil || r.executionBinding.source.Mode != entity.EvaluationModeRetryItems {
		return nil, entity.ErrHookExecutionUnsupported
	}
	_, s, err := r.loadExecutionRun(tx, key, scope)
	if err != nil {
		return nil, err
	}
	if !s.life.ExecutionInitialized {
		return nil, entity.ErrHookAdmissionDenied
	}
	return s, nil
}

func (r *hookRunRepo) retryItemsSources(tx *gorm.DB, s *lockedHookRun, items []entity.HookPlanItem) (*entity.HookExecutionInitializationPage, error) {
	c, log, err := retryItemsCursor(s)
	if err != nil {
		return nil, err
	}
	ids, pending := c.Page(log.ItemIds)
	if !pending || len(items) > len(ids) {
		return nil, entity.ErrHookStoreConflict
	}
	seen := map[int64]bool{}
	ancestors, err := retryAncestors(tx, s.view.State.Key, r.executionBinding.source.SourceRunID)
	if err != nil {
		return nil, err
	}
	page := &entity.HookExecutionInitializationPage{Count: s.life.PlanCount + int64(len(items)), RunVersion: s.life.Version, BoundSnapshotHash: s.life.SnapshotHash}
	for i, item := range items {
		if (entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: s.view.State.Key}, Items: []entity.HookPlanItem{item}}).Validate() != nil || !slices.Contains(ids, item.ItemID) || seen[item.ItemID] {
			return nil, entity.ErrHookStoreConflict
		}
		seen[item.ItemID] = true
		var exists int64
		if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), s.view.State.Key).Where("item_id=?", item.ItemID).Count(&exists).Error; err != nil {
			return nil, err
		}
		if exists != 0 {
			return nil, entity.ErrHookStoreConflict
		}
		row := retryItemsRow(s.view.State.Key, s.life.PlanCount+int64(i), item)
		m, err := r.retryProjectionSource(tx, row, ancestors)
		if err != nil {
			return nil, err
		}
		entry := entity.HookExecutionInitializationItem{Ordinal: row.Ordinal, Frozen: item, Reuse: m}
		if m != nil && m.ItemRef != nil {
			entry.ItemRefConfigHash = m.ItemRef.ConfigHash
		}
		page.Items = append(page.Items, entry)
	}
	digest, err := entity.AppendHookPlanDigest(c.Published, items)
	if err != nil {
		return nil, err
	}
	page.Hash, page.NextOrdinal = digest.Hash, page.Count
	return page, nil
}

func retryItemsRow(key entity.HookRunKey, ordinal int64, item entity.HookPlanItem) model.ExptLifecycleRunItem {
	return model.ExptLifecycleRunItem{ID: item.ID, SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, Ordinal: ordinal, SourceSpaceID: item.SourceSpaceID, EvalSetID: item.EvalSetID, EvalSetVersionID: item.EvalSetVersionID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID}
}

func (r *hookRunRepo) ReadRetryItemsSources(ctx context.Context, key entity.HookRunKey, scope, cursor string, items []entity.HookPlanItem) (*entity.HookExecutionInitializationPage, error) {
	var page *entity.HookExecutionInitializationPage
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		s, err := r.retryItemsRun(tx, key, scope)
		if err != nil {
			return err
		}
		if s.view.PlanCursor != cursor {
			return entity.ErrHookStoreConflict
		}
		page, err = r.retryItemsSources(tx, s, items)
		return err
	})
	return page, err
}

func (r *hookRunRepo) AppendPreparedRetryItemsPage(ctx context.Context, key entity.HookRunKey, scope, cursor string, manifests []entity.HookExecutionManifest) (bool, error) {
	changed := false
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		s, err := r.retryItemsRun(tx, key, scope)
		if err != nil {
			return err
		}
		c, log, err := retryItemsCursor(s)
		if err != nil {
			return err
		}
		if s.view.PlanCursor != cursor {
			return retryItemsReplay(tx, s, log, c, cursor, manifests)
		}
		items := make([]entity.HookPlanItem, len(manifests))
		for i, m := range manifests {
			if m.Validate() != nil || m.Key != key || m.Ordinal != s.life.PlanCount+int64(i) || m.NoExecutionFailure || m.TurnLogsInitialized == nil || *m.TurnLogsInitialized {
				return entity.ErrHookStoreConflict
			}
			items[i] = m.Frozen
		}
		page, err := r.retryItemsSources(tx, s, items)
		if err != nil {
			return err
		}
		deltas := map[string]int64{}
		for i, m := range manifests {
			if page.Items[i].Reuse != nil {
				var old model.ExptItemResult
				if err := tx.Where("id=?", m.ItemResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&old).Error; err != nil {
					return err
				}
				field := map[entity.ItemRunState]string{entity.ItemRunState_Queueing: "pending_cnt", entity.ItemRunState_Processing: "processing_cnt", entity.ItemRunState_Success: "success_cnt", entity.ItemRunState_Fail: "fail_cnt", entity.ItemRunState_Terminal: "terminated_cnt"}[entity.ItemRunState(old.Status)]
				if field == "" {
					return entity.ErrHookStoreCorrupt
				}
				deltas[field]--
			}
			deltas["pending_cnt"]++
			row := retryItemsRow(key, m.Ordinal, m.Frozen)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			if page.Items[i].Reuse != nil {
				if err := r.writeRetryExecutionItem(tx, row, m, page.Items[i].Reuse); err != nil {
					return err
				}
			} else {
				if err := r.writeExecutionItemRef(tx, m); err != nil {
					return err
				}
				if err := writeExecutionItem(tx, row, m); err != nil {
					return err
				}
			}
		}
		updates := map[string]any{}
		q := tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", key.WorkspaceID, key.ExperimentID)
		for field, delta := range deltas {
			if delta == 0 {
				continue
			}
			updates[field] = gorm.Expr(field+" + ?", delta)
			if delta < 0 {
				q = q.Where(field+">=?", -delta)
			}
		}
		if len(updates) > 0 {
			if err := hookOneRow(q.UpdateColumns(updates)); err != nil {
				return err
			}
		}
		c = c.Advance(log.ItemIds)
		c.Published = entity.HookPlanDigest{Count: page.Count, Hash: page.Hash}
		next, err := c.Encode()
		if err != nil {
			return err
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err := updateHookLifecycle(tx, key, s.life.Version, now, map[string]any{"plan_cursor": next, "plan_count": page.Count, "plan_hash": page.Hash}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed && err == nil, err
}

func retryItemsReplay(tx *gorm.DB, s *lockedHookRun, log *entity.ExptRunLog, current entity.HookRetryItemsCursor, raw string, manifests []entity.HookExecutionManifest) error {
	var expected entity.HookRetryItemsCursor
	if json.Unmarshal([]byte(raw), &expected) != nil {
		return entity.ErrHookStoreConflict
	}
	expected, err := entity.DecodeHookRetryItemsCursor(raw, s.view.State.Key, log.ItemIds, expected.Published.Count, expected.Published.Hash)
	if err != nil {
		return err
	}
	_, pending := expected.Page(log.ItemIds)
	if !pending || current.Phase != "tail" {
		return entity.ErrHookStoreConflict
	}
	next := expected.Advance(log.ItemIds)
	if current.Batch < next.Batch || current.Batch == next.Batch && current.Offset < next.Offset {
		return entity.ErrHookStoreConflict
	}
	for i, m := range manifests {
		if m.Validate() != nil || m.Key != s.view.State.Key || m.Ordinal != expected.Published.Count+int64(i) {
			return entity.ErrHookStoreConflict
		}
		var row model.ExptLifecycleRunItem
		if err := hookRunScope(tx, m.Key).Where("ordinal=? AND item_id=?", m.Ordinal, m.Frozen.ItemID).First(&row).Error; err != nil {
			return err
		}
		actual, err := hookTerminationManifest(m.Key, row)
		if err != nil {
			return err
		}
		actual, m = actual.Clone(), m.Clone()
		// Turn admission may have progressed since the original page receipt.
		for j := range actual.Turns {
			actual.Turns[j].RunLogID = 0
		}
		actual.TurnLogsInitialized = gptr.Of(false)
		if actual.Retry != nil {
			actual.Retry.ReusedTargets = nil
			actual.Retry.ReusedEvaluators = nil
		}
		if !reflect.DeepEqual(actual, m) {
			return entity.ErrHookStoreConflict
		}
	}
	return nil
}
