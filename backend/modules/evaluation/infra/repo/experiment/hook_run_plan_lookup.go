// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (r *hookRunRepo) MGetPlanItems(ctx context.Context, in entity.HookPlanLookupInput) (*entity.HookPlanLookupResult, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if !hookPlanProviderAvailable(r, ctx) {
		return nil, entity.ErrHookPlanStorage
	}
	var out *entity.HookPlanLookupResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		if _, err := lockHookExperiment(tx, in.Key); err != nil {
			return err
		}
		s, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		if s.life.ExecutionScope != in.ExecutionScope {
			return entity.ErrHookStoreConflict
		}
		var count int64
		if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), in.Key).Where("item_id IN ?", in.ItemIDs).Count(&count).Error; err != nil {
			return err
		}
		if count < 0 || count > int64(len(in.ItemIDs)) {
			return entity.ErrHookStoreCorrupt
		}
		var rows []model.ExptLifecycleRunItem
		if count > 0 {
			// Do not filter ordinal in SQL: preparing must detect unexpected future rows.
			if err := hookRunScope(tx, in.Key).Where("item_id IN ?", in.ItemIDs).Order("item_id ASC, id ASC").Limit(len(in.ItemIDs)).Find(&rows).Error; err != nil {
				return err
			}
		}
		if int64(len(rows)) != count {
			return entity.ErrHookStoreCorrupt
		}
		requested, seen := make(map[int64]bool, len(in.ItemIDs)), make(map[int64]bool, len(rows))
		for _, id := range in.ItemIDs {
			requested[id] = true
		}
		items := make([]entity.HookPlanItem, 0, len(rows))
		members := make(map[int64]entity.HookPlanItem, len(rows))
		ordinals := make(map[int64]bool, len(rows))
		for _, row := range rows {
			if row.SpaceID != in.Key.WorkspaceID || row.ExptID != in.Key.ExperimentID || row.ExptRunID != in.Key.RunID || !requested[row.ItemID] || seen[row.ItemID] || row.Ordinal < 0 {
				return entity.ErrHookStoreCorrupt
			}
			seen[row.ItemID] = true
			item := entity.HookPlanItem{ID: row.ID, SourceSpaceID: row.SourceSpaceID, EvalSetID: row.EvalSetID, EvalSetVersionID: row.EvalSetVersionID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID}
			items = append(items, item)
			if row.Ordinal >= s.life.PlanCount {
				if !s.view.PlanReady {
					return entity.ErrHookStoreCorrupt
				}
				continue
			}
			if ordinals[row.Ordinal] {
				return entity.ErrHookStoreCorrupt
			}
			ordinals[row.Ordinal] = true
			members[row.ItemID] = item
		}
		if len(items) > 0 {
			if err := (entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, Items: items}).Validate(); err != nil {
				return entity.ErrHookStoreCorrupt
			}
		}
		result := &entity.HookPlanLookupResult{Items: make([]entity.HookPlanItem, 0, len(members)), RunVersion: s.life.Version, Count: s.life.PlanCount, Ready: s.view.PlanReady}
		for _, id := range in.ItemIDs {
			if item, ok := members[id]; ok {
				result.Items = append(result.Items, item)
			}
		}
		out = result
		return nil
	}, db.WithMaster())
	if err != nil {
		return nil, hookPlanStoreError(err)
	}
	return out, nil
}
