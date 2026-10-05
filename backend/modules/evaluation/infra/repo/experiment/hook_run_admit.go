// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *hookRunRepo) AdmitItem(ctx context.Context, in entity.HookAdmitItemInput) (entity.HookAdmitItemResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookAdmitItemResult{}, err
	}
	var out entity.HookAdmitItemResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, err := lockHookExperiment(tx, in.Key)
		if err != nil {
			return err
		}
		s, err := loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		if !hookRunActive(expt, s) || !s.view.PlanReady || s.view.State.Gate != entity.HookGateReady {
			return entity.ErrHookAdmissionDenied
		}
		if r.executionBinding != nil {
			if err := checkBoundFinalizationRun(tx, in.Key, s.life.ExecutionScope, r.executionBinding); err != nil {
				return err
			}
		}
		if (hookExecutionRequired(expt, s) || r.executionBinding != nil || hookMultiSetInitializationRequired(s.life.BeforeEnabled || s.life.AfterEnabled, s.view.Mode, entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType))) && !s.life.ExecutionInitialized {
			return entity.ErrHookAdmissionDenied
		}
		if s.life.Version != in.ExpectedVersion {
			return entity.ErrHookStoreConflict
		}
		var item model.ExptLifecycleRunItem
		// The ledger may grow after initial plan freeze; no initial ordinal/count bound applies.
		if err := hookRunScope(tx, in.Key).Where("item_id=?", in.ItemID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return entity.ErrHookAdmissionDenied
			}
			return err
		}
		if r.executionBinding != nil {
			if item.ExecutionManifest == nil {
				return entity.ErrHookAdmissionDenied
			}
			if err := checkBoundFinalizationPage(tx, in.Key, []model.ExptLifecycleRunItem{item}, r.executionBinding, true); err != nil {
				return err
			}
		}
		if item.AdmittedAt != nil {
			out = entity.HookAdmitItemResult{Admitted: true, AdmittedAt: *item.AdmittedAt, Version: s.life.Version}
			return nil
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if err := hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), in.Key).Where("id=? AND item_id=? AND admitted_at IS NULL", item.ID, in.ItemID).UpdateColumn("admitted_at", now)); err != nil {
			return err
		}
		if err := updateHookLifecycle(tx, in.Key, s.life.Version, now, map[string]any{"execution_started": true}); err != nil {
			return err
		}
		out = entity.HookAdmitItemResult{Admitted: true, NewlyAdmitted: true, AdmittedAt: now, Version: s.life.Version + 1}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookAdmitItemResult{}, err
	}
	return out, nil
}
