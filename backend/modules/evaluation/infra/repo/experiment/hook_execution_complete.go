// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package experiment

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *hookRunRepo) CompleteExecutionInitialization(ctx context.Context, in entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error) {
	if err := in.Validate(); err != nil {
		return entity.HookExecutionInitializationCompletion{}, err
	}
	var out entity.HookExecutionInitializationCompletion
	err := r.executionTransaction(ctx, func(tx *gorm.DB) error {
		e, s, err := r.loadExecutionRun(tx, in.Key, in.ExecutionScope)
		if err != nil {
			return err
		}
		if r.executionBinding != nil {
			if err := r.executionBinding.checkReferenceCount(tx, true); err != nil {
				return err
			}
		}
		if in.PlanHash != s.view.PlanHash || in.ExpectedItemCount != s.life.PlanCount {
			return entity.ErrHookStoreConflict
		}
		if !s.life.ExecutionInitialized && (s.life.Version != in.ExpectedVersion || s.life.ExecutionStarted) {
			return entity.ErrHookStoreConflict
		}
		var ledgerCount int64
		if err = hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), in.Key).Count(&ledgerCount).Error; err != nil {
			return err
		}
		if ledgerCount != in.ExpectedItemCount {
			return entity.ErrHookStoreCorrupt
		}
		digest := entity.NewHookPlanDigest()
		var turns int64
		for start := int64(0); start < s.life.PlanCount; {
			page, _, readErr := r.readExecutionPage(tx, s, start, 100)
			if readErr != nil {
				return readErr
			}
			tuples := make([]entity.HookPlanItem, 0, len(page.Items))
			for _, item := range page.Items {
				if item.Manifest == nil {
					return entity.ErrHookStoreCorrupt
				}
				turns += int64(len(item.Manifest.Turns))
				tuples = append(tuples, item.Frozen)
			}
			digest, err = entity.AppendHookPlanDigest(digest, tuples)
			if err != nil {
				return entity.ErrHookStoreCorrupt
			}
			start = page.NextOrdinal
		}
		if digest.Hash != in.PlanHash || turns != in.ExpectedTurnCount {
			return entity.ErrHookStoreCorrupt
		}
		for _, check := range []struct {
			table any
			count int64
		}{{&model.ExptItemResult{}, in.ExpectedItemCount}, {&model.ExptItemResultRunLog{}, in.ExpectedItemCount}, {&model.ExptTurnResult{}, turns}} {
			var n int64
			if err = hookRunScope(tx.Unscoped().Model(check.table), in.Key).Count(&n).Error; err != nil {
				return err
			}
			if n != check.count {
				return entity.ErrHookStoreCorrupt
			}
		}
		if s.life.ExecutionInitialized {
			out = entity.HookExecutionInitializationCompletion{Initialized: true, RunVersion: s.life.Version}
			return nil
		}
		var startedTurns int64
		if err = hookRunScope(tx.Unscoped().Model(&model.ExptTurnResultRunLog{}), in.Key).Count(&startedTurns).Error; err != nil {
			return err
		}
		if startedTurns != 0 {
			return entity.ErrHookStoreCorrupt
		}
		var stats model.ExptStats
		if err = tx.Where("space_id=? AND expt_id=?", in.Key.WorkspaceID, in.Key.ExperimentID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&stats).Error; err != nil {
			return err
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		draining := s.view.Mode == entity.EvaluationModeAppend && s.view.State.Status == entity.ExptStatus_Draining
		if !draining && s.view.State.Status != entity.ExptStatus_Processing {
			if err = hookOneRow(hookRunScope(tx.Model(&model.ExptRunLog{}), in.Key).Where("id=? AND status=? AND lifecycle_hook_version=1", s.log.ID, *s.log.Status).UpdateColumns(map[string]any{"status": int64(entity.ExptStatus_Processing), "updated_at": now})); err != nil {
				return err
			}
		}
		if !draining && entity.ExptStatus(e.Status) != entity.ExptStatus_Processing {
			if err = hookOneRow(tx.Model(&model.Experiment{}).Where("id=? AND space_id=? AND latest_run_id=? AND status=?", in.Key.ExperimentID, in.Key.WorkspaceID, in.Key.RunID, e.Status).UpdateColumns(map[string]any{"status": int32(entity.ExptStatus_Processing), "updated_at": now})); err != nil {
				return err
			}
		}
		if r.executionBinding != nil && entity.HookBoundRetryMode(r.executionBinding.source.Mode) {
			counts, err := retryProjectionCounts(tx, in.Key)
			if err != nil {
				return err
			}
			if err := hookOneRow(tx.Model(&model.ExptStats{}).Where("id=?", stats.ID).UpdateColumns(map[string]any{"pending_cnt": counts.Pending, "processing_cnt": counts.Processing, "success_cnt": counts.Success, "fail_cnt": counts.Fail, "terminated_cnt": counts.Terminated, "updated_at": now})); err != nil {
				return err
			}
		} else if stats.PendingCnt != int32(in.ExpectedItemCount) || stats.ProcessingCnt != 0 || stats.SuccessCnt != 0 || stats.FailCnt != 0 || stats.TerminatedCnt != 0 {
			if err = hookOneRow(tx.Model(&model.ExptStats{}).Where("id=? AND space_id=? AND expt_id=?", stats.ID, in.Key.WorkspaceID, in.Key.ExperimentID).UpdateColumns(map[string]any{"pending_cnt": int32(in.ExpectedItemCount), "processing_cnt": 0, "success_cnt": 0, "fail_cnt": 0, "terminated_cnt": 0, "updated_at": now})); err != nil {
				return err
			}
		}
		fields := map[string]any{"execution_initialized": true}
		if s.view.Mode == entity.EvaluationModeRetryItems {
			log, err := convert.NewExptRunLogConvertor().PO2DO(&s.log)
			if err != nil {
				return entity.ErrHookStoreCorrupt
			}
			cursor, err := entity.PromoteHookRetryItemsCursor(s.view.PlanCursor, in.Key, log.ItemIds, s.life.PlanCount, s.view.PlanHash)
			if err != nil {
				return err
			}
			fields["plan_cursor"] = cursor
		}
		if err = updateHookLifecycle(tx, in.Key, s.life.Version, now, fields); err != nil {
			return err
		}
		out = entity.HookExecutionInitializationCompletion{Initialized: true, Changed: true, RunVersion: s.life.Version + 1}
		return nil
	})
	if err != nil {
		return entity.HookExecutionInitializationCompletion{}, err
	}
	return out, nil
}
