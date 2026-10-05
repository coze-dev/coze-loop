// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *hookFinalizationRepo) ProjectHookSchedulerStartup(ctx context.Context, key entity.HookRunKey, scope string) (active bool, err error) {
	err = r.itemArchiveTransaction(ctx, key, scope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		eligible, err := hookSchedulerFailureActive(tx, key, expt, life, r.binding)
		if err != nil || !eligible {
			return err
		}
		if r.binding != nil && r.binding.source.Mode == entity.EvaluationModeAppend && entity.ExptStatus(expt.Status) == entity.ExptStatus_Draining {
			active = true
			return nil
		}
		if entity.ExptStatus(expt.Status) != entity.ExptStatus_Processing {
			return entity.ErrHookStoreConflict
		}
		active = true
		// The parent lock precedes lifecycle/template locks; read its association with a current read.
		var parent model.Experiment
		if err := tx.Select("expt_template_id").Where("id=?", key.ExperimentID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&parent).Error; err != nil {
			return err
		}
		if parent.ExptTemplateID == 0 {
			return nil
		}
		var template model.ExptTemplate
		if err := tx.Where("id=? AND space_id=?", parent.ExptTemplateID, key.WorkspaceID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&template).Error; err != nil {
			return err
		}
		var info entity.ExptInfo
		if raw := gptr.Indirect(template.ExptInfo); len(raw) != 0 {
			if err := json.Unmarshal(raw, &info); err != nil {
				return err
			}
		}
		// A different experiment has already claimed the template's latest projection.
		if info.LatestExptID != 0 && info.LatestExptID != key.ExperimentID {
			return nil
		}
		if info.LatestExptID == key.ExperimentID && info.LatestExptStatus == entity.ExptStatus_Processing && info.CronActivate == template.CronActivate {
			return nil
		}
		info.LatestExptID = key.ExperimentID
		info.LatestExptStatus = entity.ExptStatus_Processing
		info.CronActivate = template.CronActivate
		raw, err := json.Marshal(info)
		if err != nil {
			return err
		}
		return hookOneRow(tx.Model(&model.ExptTemplate{}).Where("id=? AND space_id=?", template.ID, key.WorkspaceID).Update("expt_info", raw))
	})
	return active, err
}
