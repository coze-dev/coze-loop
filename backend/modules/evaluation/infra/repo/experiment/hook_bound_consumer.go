// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"reflect"
)

func NewBoundHookConsumerRepo(p db.Provider, binding *entity.HookExecutionInitializationBinding) (repo.IHookBoundConsumerRepo, error) {
	base, err := NewBoundHookExecutionInitializationRepo(p, binding)
	if err != nil {
		return nil, err
	}
	return base.(*hookRunRepo), nil
}

func (r *hookRunRepo) ReadBoundConsumerItem(ctx context.Context, key entity.HookRunKey, scope string, itemID int64) (*entity.HookBoundConsumerItem, error) {
	if r == nil || r.executionBinding == nil || itemID <= 0 || (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	var out *entity.HookBoundConsumerItem
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		expt, run, err := r.loadExecutionRun(tx, key, scope)
		if err != nil {
			return err
		}
		if !run.life.ExecutionInitialized {
			return entity.ErrHookAdmissionDenied
		}
		var row model.ExptLifecycleRunItem
		if err := hookRunScope(tx, key).Select("ordinal").Where("item_id=?", itemID).First(&row).Error; err != nil {
			return entity.ErrHookStoreCorrupt
		}
		page, _, err := r.readExecutionPage(tx, run, row.Ordinal, 1)
		if err != nil {
			return err
		}
		if len(page.Items) != 1 || page.Items[0].Frozen.ItemID != itemID || page.Items[0].Manifest == nil {
			return entity.ErrHookStoreCorrupt
		}
		item, _, err := readHookArchiveItemRows(tx, key, itemID, true)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(item.Manifest, *page.Items[0].Manifest) {
			return entity.ErrHookStoreConflict
		}
		if entity.IsItemRunFinished(entity.ItemRunState(item.Item.Status)) {
			return entity.ErrHookAdmissionDenied
		}
		var currentRow model.Experiment
		if err := tx.Where("id=? AND space_id=?", expt.ID, key.WorkspaceID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&currentRow).Error; err != nil {
			return err
		}
		current, err := convert.NewExptConverter().PO2DO(&currentRow, nil)
		if err != nil {
			return entity.ErrHookStoreCorrupt
		}
		config, err := r.executionBinding.manifestConfig(item.Manifest)
		if err != nil {
			return err
		}
		owned := r.executionBinding.binding.Input()
		turns := make(map[int64]*entity.ExptTurnResultRunLog, len(item.Turns))
		for _, turn := range item.Turns {
			turns[turn.TurnID] = turn
		}
		var itemConfig *entity.ExptItemConfig
		if owned.Mode == entity.EvaluationModeAppend {
			itemConfig, err = decodeRetryItemConfig(config.raw)
			if err != nil {
				return err
			}
		} else {
			itemConfig = owned.Execution.Sets[config.index].ItemConfig
		}
		if item.Manifest.Retry != nil && !owned.Execution.SingleSet {
			itemConfig, err = decodeRetryItemConfig(config.raw)
			if err != nil {
				return err
			}
		}
		out = &entity.HookBoundConsumerItem{SnapshotHash: owned.SnapshotHash, PlanHash: page.Hash, PlanCount: page.Count, RunVersion: page.RunVersion, Manifest: item.Manifest.Clone(), ItemConfig: itemConfig, Experiment: current, Result: &entity.ExptItemEvalResult{ItemResultRunLog: item.Item, TurnResultRunLogs: turns}}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
