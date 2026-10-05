// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func NewHookPlanResultReader(provider db.Provider) repo.IHookPlanResultReader {
	return &hookRunRepo{provider: provider}
}

func (r *hookRunRepo) HasExperimentResults(ctx context.Context, key entity.HookRunKey, executionScope string) (bool, error) {
	if err := (entity.HookStoreGuard{Key: key}).Validate(); err != nil {
		return false, err
	}
	if !hookGateASCII(executionScope) {
		return false, entity.ErrHookStoreConflict
	}
	if !hookPlanProviderAvailable(r, ctx) {
		return false, entity.ErrHookPlanStorage
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	found := false
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		if _, err := lockHookExperiment(tx, key); err != nil {
			return err
		}
		run, err := loadHookRun(tx, key)
		if err != nil {
			return err
		}
		if run.life.ExecutionScope != executionScope {
			return entity.ErrHookStoreConflict
		}
		// Current base results may belong to another Run and any status.
		for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}} {
			var ids []int64
			if err := tx.Model(table).Select("id").Where("space_id=? AND expt_id=?", key.WorkspaceID, key.ExperimentID).Limit(1).Find(&ids).Error; err != nil {
				return err
			}
			if len(ids) > 0 {
				found = true
				break
			}
		}
		return nil
	}, db.WithMaster())
	if err != nil {
		return false, hookPlanStoreError(err)
	}
	return found, nil
}
