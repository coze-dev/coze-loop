// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"math"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// The closed gate serializes with Hook admission/initialization; flags alone are not proof.
func readHookNeverAdmittedStats(tx *gorm.DB, key entity.HookRunKey, life *model.ExptLifecycleRun) (*entity.HookFinalizationStats, error) {
	return readHookNeverAdmittedProof(tx, key, life, false)
}

func readHookNeverAdmittedProof(tx *gorm.DB, key entity.HookRunKey, life *model.ExptLifecycleRun, allowUnsettled bool, bindings ...*boundHookExecution) (*entity.HookFinalizationStats, error) {
	if life.Gate != 2 || life.PlanState < 0 || life.PlanState > 1 || life.PlanCount < 0 || life.PlanCount > math.MaxInt32 {
		return nil, entity.ErrHookStoreCorrupt
	}
	if (!life.BeforeEnabled && !life.AfterEnabled) || life.ExecutionStarted || life.ExecutionInitialized {
		return nil, entity.ErrHookFinalizationUnsettled
	}
	var expt model.Experiment
	if err := tx.Unscoped().Where("id=? AND space_id=?", key.ExperimentID, key.WorkspaceID).First(&expt).Error; err != nil {
		return nil, err
	}
	var log model.ExptRunLog
	if err := hookRunScope(tx.Unscoped().Select("mode"), key).First(&log).Error; err != nil {
		return nil, err
	}
	if !entity.HookExecutionInitializationRequired(life.BeforeEnabled || life.AfterEnabled, entity.ExptRunMode(gptr.Indirect(log.Mode)), entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) && !boundFinalizationRequired(firstFinalizationBinding(bindings), life, entity.ExptRunMode(gptr.Indirect(log.Mode))) {
		return nil, entity.ErrHookFinalizationUnsettled
	}
	digest := entity.NewHookPlanDigest()
	out := &entity.HookFinalizationStats{Key: key, ExecutionScope: life.ExecutionScope, NeverAdmitted: true}
	var materialized, turns int64
	for start := int64(0); start < life.PlanCount; {
		var page []model.ExptLifecycleRunItem
		if err := hookRunScope(tx, key).Where("ordinal>=?", start).Order("ordinal").Limit(100).Find(&page).Error; err != nil {
			return nil, err
		}
		if len(page) == 0 || int64(len(page)) > life.PlanCount-start {
			return nil, entity.ErrHookStoreCorrupt
		}
		if err := checkBoundFinalizationPage(tx, key, page, firstFinalizationBinding(bindings), false); err != nil {
			return nil, err
		}
		for i, row := range page {
			if row.Ordinal != start+int64(i) {
				return nil, entity.ErrHookStoreCorrupt
			}
			if row.AdmittedAt != nil {
				return nil, entity.ErrHookFinalizationUnsettled
			}
			if row.ExecutionManifest != nil {
				if life.PlanState != 1 {
					return nil, entity.ErrHookStoreCorrupt
				}
				current, err := readHookPartialInitializationItem(tx, key, &expt, row, allowUnsettled)
				if err != nil {
					return nil, err
				}
				materialized++
				turns += int64(len(current.Manifest.Turns))
				if err := addHookFinalizationCount(&out.Items, int32(entity.ItemRunState_Terminal)); err != nil {
					return nil, err
				}
				for range current.Manifest.Turns {
					if err := addHookFinalizationCount(&out.Turns, int32(entity.ItemRunState_Terminal)); err != nil {
						return nil, err
					}
				}
			}
			var err error
			digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{executionTuple(row)})
			if err != nil {
				return nil, err
			}
			out.ItemIDs = append(out.ItemIDs, row.ItemID)
		}
		start += int64(len(page))
	}
	var count int64
	if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), key).Count(&count).Error; err != nil {
		return nil, err
	}
	if count != life.PlanCount || life.PlanState == 1 && digest.Hash != gptr.Indirect(life.PlanHash) {
		return nil, entity.ErrHookStoreCorrupt
	}
	for _, check := range []struct {
		table any
		count int64
	}{
		{&model.ExptItemResultRunLog{}, materialized}, {&model.ExptTurnResultRunLog{}, 0},
		{&model.ExptItemResult{}, materialized}, {&model.ExptTurnResult{}, turns},
	} {
		var n int64
		if err := hookRunScope(tx.Unscoped().Model(check.table), key).Count(&n).Error; err != nil {
			return nil, err
		}
		if n != check.count {
			return nil, entity.ErrHookFinalizationUnsettled
		}
	}
	return out, nil
}
