// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

var _ repo.IHookOnlineRepo = (*hookRunRepo)(nil)

func (r *hookRunRepo) onlineRun(tx *gorm.DB, key entity.HookRunKey, scope string) (*model.Experiment, *lockedHookRun, error) {
	e, err := lockHookExperiment(tx, key)
	if err != nil {
		return nil, nil, err
	}
	s, err := loadHookRun(tx, key)
	if err != nil {
		return nil, nil, err
	}
	if s.life.ExecutionScope != scope || s.view.Mode != entity.EvaluationModeAppend || entity.ExptType(e.ExptType) != entity.ExptType_Online {
		return nil, nil, entity.ErrHookStoreConflict
	}
	if r.executionBinding != nil {
		if err := r.executionBinding.checkRun(tx, e, s); err != nil {
			return nil, nil, err
		}
	}
	if !hookRunActive(e, s) {
		return nil, nil, entity.ErrHookAdmissionDenied
	}
	return e, s, nil
}

// Online has no initial source selection; accepted pages publish their own immutable members.
func (r *hookRunRepo) prepareOnlinePlan(tx *gorm.DB, s *lockedHookRun) error {
	if s.view.PlanReady {
		return nil
	}
	key := s.view.State.Key
	if s.life.PlanCount != 0 || s.life.ExecutionInitialized || s.life.ExecutionStarted {
		return entity.ErrHookStoreCorrupt
	}
	var count int64
	if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), key).Count(&count).Error; err != nil {
		return err
	}
	if count != 0 {
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
		if err := updateHookOperation(tx, key, s.before, map[string]any{"activated_at": now, "occurred_at": now, "next_attempt_at": now}); err != nil {
			return err
		}
	}
	return updateHookLifecycle(tx, key, s.life.Version, now, map[string]any{"plan_state": 1, "plan_hash": entity.NewHookPlanDigest().Hash})
}

func (r *hookRunRepo) PrepareOnlinePlan(ctx context.Context, key entity.HookRunKey, scope string) error {
	return r.executionTransaction(ctx, func(tx *gorm.DB) error {
		_, s, err := r.onlineRun(tx, key, scope)
		if err != nil {
			return err
		}
		return r.prepareOnlinePlan(tx, s)
	})
}

func (r *hookRunRepo) AppendOnlinePage(ctx context.Context, key entity.HookRunKey, scope string, manifests []entity.HookExecutionManifest, ext map[string]string) (bool, error) {
	if r.executionBinding == nil || r.executionBinding.source.Mode != entity.EvaluationModeAppend {
		return false, entity.ErrHookExecutionUnsupported
	}
	if len(manifests) > 100 {
		return false, entity.ErrHookStoreCorrupt
	}
	changed := false
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		e, s, err := r.onlineRun(tx, key, scope)
		if err != nil {
			return err
		}
		if (entity.ExptStatus(e.Status) != entity.ExptStatus_Pending && entity.ExptStatus(e.Status) != entity.ExptStatus_Processing) || (s.view.State.Status != entity.ExptStatus_Pending && s.view.State.Status != entity.ExptStatus_Processing) {
			return entity.ErrHookAdmissionDenied
		}
		if err := r.prepareOnlinePlan(tx, s); err != nil {
			return err
		}
		s, err = loadHookRun(tx, key)
		if err != nil {
			return err
		}
		var added []entity.HookPlanItem
		for _, input := range manifests {
			m := input.Clone()
			if m.Validate() != nil || m.Key != key || m.ItemRef != nil || m.Retry != nil || m.NoExecutionFailure || m.TurnLogsInitialized == nil || *m.TurnLogsInitialized {
				return entity.ErrHookStoreCorrupt
			}
			var old model.ExptLifecycleRunItem
			err := hookRunScope(tx, key).Where("item_id=?", m.Frozen.ItemID).First(&old).Error
			if err == nil {
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			m.Ordinal = s.life.PlanCount + int64(len(added))
			row := retryItemsRow(key, m.Ordinal, m.Frozen)
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			if err := writeExecutionItem(tx, row, m); err != nil {
				return err
			}
			if len(ext) > 0 {
				raw, err := json.Marshal(ext)
				if err != nil {
					return entity.ErrHookStoreCorrupt
				}
				if err := hookOneRow(hookRunScope(tx.Model(&model.ExptItemResult{}), key).Where("id=?", m.ItemResultID).UpdateColumn("ext", raw)); err != nil {
					return err
				}
			}
			added = append(added, m.Frozen)
		}
		if len(added) == 0 {
			return nil
		}
		digest, err := entity.AppendHookPlanDigest(entity.HookPlanDigest{Count: s.life.PlanCount, Hash: s.view.PlanHash}, added)
		if err != nil {
			return err
		}
		if err := hookOneRow(tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", key.WorkspaceID, key.ExperimentID).UpdateColumn("pending_cnt", gorm.Expr("pending_cnt + ?", len(added)))); err != nil {
			return err
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err := updateHookLifecycle(tx, key, s.life.Version, now, map[string]any{"plan_count": digest.Count, "plan_hash": digest.Hash}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed && err == nil, err
}

func (r *hookRunRepo) DrainOnlineRun(ctx context.Context, key entity.HookRunKey, scope string) error {
	return r.executionTransaction(ctx, func(tx *gorm.DB) error {
		e, s, err := r.onlineRun(tx, key, scope)
		if err != nil {
			return err
		}
		if err := r.prepareOnlinePlan(tx, s); err != nil {
			return err
		}
		if s.view.State.Status == entity.ExptStatus_Draining && entity.ExptStatus(e.Status) == entity.ExptStatus_Draining {
			return nil
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err := hookOneRow(hookRunScope(tx.Model(&model.ExptRunLog{}), key).Where("id=? AND status=?", key.RunID, gptr.Indirect(s.log.Status)).UpdateColumns(map[string]any{"status": int64(entity.ExptStatus_Draining), "updated_at": now})); err != nil {
			return err
		}
		if err := hookOneRow(tx.Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=?", key.ExperimentID, key.WorkspaceID, key.RunID).UpdateColumns(map[string]any{"status": int32(entity.ExptStatus_Draining), "updated_at": now})); err != nil {
			return err
		}
		s, err = loadHookRun(tx, key)
		if err != nil {
			return err
		}
		return updateHookLifecycle(tx, key, s.life.Version, now, map[string]any{})
	})
}
