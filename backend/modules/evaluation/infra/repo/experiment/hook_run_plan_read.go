// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"
	"reflect"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewHookPlanRepo(provider db.Provider) repo.IHookPlanRepo {
	return &hookRunRepo{provider: provider}
}

func (r *hookRunRepo) ReadPlanPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookPlanReadPage, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if !hookPlanProviderAvailable(r, ctx) {
		return nil, entity.ErrHookPlanStorage
	}
	var out *entity.HookPlanReadPage
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		if _, err := lockHookExperiment(tx, in.Key); err != nil {
			return err
		}
		s, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		if s.life.ExecutionScope != in.ExecutionScope || in.StartOrdinal > s.life.PlanCount {
			return entity.ErrHookStoreConflict
		}
		n := min(int64(in.Limit), s.life.PlanCount-in.StartOrdinal)
		end := in.StartOrdinal + n
		// Check the header's tail without scanning the full prefix under the Run lock.
		if s.life.PlanCount > 0 && (n == 0 || end < s.life.PlanCount) {
			var tail int64
			if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), in.Key).Where("ordinal=?", s.life.PlanCount-1).Count(&tail).Error; err != nil {
				return err
			}
			if tail != 1 {
				return entity.ErrHookStoreCorrupt
			}
		}
		page := &entity.HookPlanReadPage{Items: make([]entity.HookPlanItem, 0, n), NextOrdinal: end, Count: s.life.PlanCount, Hash: s.view.PlanHash, Ready: s.view.PlanReady, RunVersion: s.life.Version, HasMore: end < s.life.PlanCount}
		if n > 0 {
			var count int64
			if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), in.Key).Where("ordinal>=? AND ordinal<?", in.StartOrdinal, end).Count(&count).Error; err != nil {
				return err
			}
			if count != n {
				return entity.ErrHookStoreCorrupt
			}
			var rows []model.ExptLifecycleRunItem
			if err := hookRunScope(tx, in.Key).Where("ordinal>=? AND ordinal<?", in.StartOrdinal, end).Order("ordinal ASC, id ASC").Limit(int(n)).Find(&rows).Error; err != nil {
				return err
			}
			if int64(len(rows)) != n {
				return entity.ErrHookStoreCorrupt
			}
			for i, row := range rows {
				if row.SpaceID != in.Key.WorkspaceID || row.ExptID != in.Key.ExperimentID || row.ExptRunID != in.Key.RunID || row.Ordinal != in.StartOrdinal+int64(i) {
					return entity.ErrHookStoreCorrupt
				}
				page.Items = append(page.Items, entity.HookPlanItem{ID: row.ID, SourceSpaceID: row.SourceSpaceID, EvalSetID: row.EvalSetID, EvalSetVersionID: row.EvalSetVersionID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID})
			}
			if err := (entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: in.Key}, StartOrdinal: in.StartOrdinal, Items: page.Items}).Validate(); err != nil {
				return entity.ErrHookStoreCorrupt
			}
		}
		out = page
		return nil
	}, db.WithMaster())
	if err != nil {
		return nil, hookPlanStoreError(err)
	}
	return out, nil
}

func (r *hookRunRepo) AdvancePlanCursor(ctx context.Context, in entity.HookAdvancePlanInput) (entity.HookStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookStoreResult{}, err
	}
	if !hookPlanProviderAvailable(r, ctx) {
		return entity.HookStoreResult{}, entity.ErrHookPlanStorage
	}
	var out entity.HookStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		expt, err := lockHookExperiment(tx, in.Key)
		if err != nil {
			return err
		}
		s, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		if s.life.ExecutionScope != in.ExecutionScope || !hookRunActive(expt, s) || s.view.PlanReady || s.life.PlanCount != in.ExpectedCount {
			return entity.ErrHookStoreConflict
		}
		// Match the immediately preceding CAS receipt, as for AppendPlanPage.
		if s.life.Version == in.ExpectedVersion+1 && s.view.PlanCursor == in.NextCursor {
			out.Run = s.view
			return nil
		}
		if s.life.Version != in.ExpectedVersion || s.view.PlanCursor != in.Cursor {
			return entity.ErrHookStoreConflict
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err := updateHookLifecycle(tx, in.Key, s.life.Version, now, map[string]any{"plan_cursor": in.NextCursor}); err != nil {
			return err
		}
		s, err = loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out = entity.HookStoreResult{Run: s.view, Changed: true}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookStoreResult{}, hookPlanStoreError(err)
	}
	return out, nil
}

func hookPlanProviderAvailable(r *hookRunRepo, ctx context.Context) bool {
	if r == nil || r.provider == nil || ctx == nil {
		return false
	}
	v := reflect.ValueOf(r.provider)
	return v.Kind() != reflect.Ptr || !v.IsNil()
}

func hookPlanStoreError(err error) error {
	for _, safe := range []error{entity.ErrHookStoreConflict, entity.ErrHookStoreCorrupt, entity.ErrHookStoreMissing} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return entity.ErrHookPlanStorage
}
