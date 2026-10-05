// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"slices"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func writeHookFinalizationStats(tx *gorm.DB, in entity.HookFinalizeInput, latest int64, now time.Time, bindings ...*boundHookExecution) error {
	if in.Stats.Key != in.Key {
		return entity.ErrHookStoreConflict
	}
	actual, err := readHookFinalizationStats(tx, in.Key, in.Stats.ExecutionScope, bindings...)
	if err != nil {
		return err
	}
	if actual.Items != in.Stats.Items || actual.Turns != in.Stats.Turns || actual.NeverAdmitted != in.Stats.NeverAdmitted || actual.ActiveTermination != in.Stats.ActiveTermination || !actual.AllowsTerminalStatus(in.Intent.Status) {
		return entity.ErrHookStoreConflict
	}
	if actual.NeverAdmitted && !slices.Equal(actual.ItemIDs, in.Stats.ItemIDs) {
		return entity.ErrHookStoreConflict
	}
	fields := func(c entity.HookFinalizationCounts) map[string]any {
		return map[string]any{"pending_cnt": c.Pending, "processing_cnt": c.Processing, "success_cnt": c.Success, "fail_cnt": c.Fail, "terminated_cnt": c.Terminated, "updated_at": now}
	}
	runFields := fields(actual.Turns)
	runFields["status_message"] = []byte(in.Intent.Reason)
	if actual.NeverAdmitted {
		runFields["status_message"] = []byte("")
	}
	if in.DisplayMessage != nil {
		runFields["status_message"] = []byte(*in.DisplayMessage)
	}
	// The enclosing transaction already holds the original Run and experiment locks.
	result := hookRunScope(tx.Model(&model.ExptRunLog{}), in.Key).Where("id=? AND lifecycle_hook_version=1", in.Key.RunID).UpdateColumns(runFields)
	if result.Error != nil {
		return result.Error
	}
	if latest != in.Key.RunID {
		return nil
	}
	var count int64
	q := tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", in.Key.WorkspaceID, in.Key.ExperimentID)
	if err := q.Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return entity.ErrHookStoreCorrupt
	}
	projection := actual.Items
	if b := firstFinalizationBinding(bindings); b != nil && entity.HookBoundRetryMode(b.source.Mode) {
		projection, err = retryProjectionCounts(tx, in.Key)
		if err != nil {
			return err
		}
	}
	return tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", in.Key.WorkspaceID, in.Key.ExperimentID).
		Where("EXISTS (SELECT 1 FROM experiment e WHERE e.id=? AND e.space_id=? AND e.latest_run_id=?)", in.Key.ExperimentID, in.Key.WorkspaceID, in.Key.RunID).
		UpdateColumns(fields(projection)).Error
}
