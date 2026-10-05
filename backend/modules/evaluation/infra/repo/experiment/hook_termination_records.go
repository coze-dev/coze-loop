// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Only exact records reachable from the frozen original turn logs are closed; output is preserved.
func (r *hookFinalizationRepo) CloseHookExecutionRecords(ctx context.Context, scope string, item *entity.HookTerminationItem) error {
	if item == nil || item.Manifest.Validate() != nil {
		return entity.ErrHookStoreCorrupt
	}
	key := item.Manifest.Key
	return r.itemArchiveRecoveryTransaction(ctx, key, scope, true, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		if !hookCancellation(life) || life.Gate != 2 {
			return entity.ErrHookAdmissionDenied
		}
		var targetConfig model.Experiment
		if err := tx.Unscoped().Select("target_id", "target_version_id", "target_space_id").Where("id=? AND space_id=?", key.ExperimentID, key.WorkspaceID).First(&targetConfig).Error; err != nil {
			return err
		}
		if r.binding != nil {
			target := r.binding.source.Execution.Target
			targetConfig.TargetID, targetConfig.TargetVersionID, targetConfig.TargetSpaceID = target.ID, target.VersionID, target.SourceSpaceID
		}
		current, _, err := r.readArchiveItem(tx, key, item.Manifest.Frozen.ItemID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current.Manifest, item.Manifest) || !reflect.DeepEqual(current.Turns, item.Turns) {
			return entity.ErrHookStoreConflict
		}
		targets := map[int64]*entity.ExptTurnResultRunLog{}
		for _, tr := range current.Turns {
			if tr.TargetResultID > 0 {
				targets[tr.TargetResultID] = tr
			}
		}
		if len(targets) != len(item.Targets) {
			return fmt.Errorf("hook termination target completeness: %w", entity.ErrHookStoreCorrupt)
		}
		for _, record := range item.Targets {
			if record == nil || targets[record.ID] == nil {
				return entity.ErrHookStoreCorrupt
			}
			tr := targets[record.ID]
			delete(targets, record.ID)
			var row tm.TargetRecord
			if err := tx.Unscoped().Where("id=?", record.ID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row).Error; err != nil {
				return err
			}
			targetSpace := targetConfig.TargetSpaceID
			if targetSpace == 0 {
				targetSpace = key.WorkspaceID
			}
			if row.DeletedAt.Valid || row.SpaceID != record.SpaceID || row.SpaceID != targetSpace || row.ExperimentRunID != current.Manifest.TargetRecordRun(row.ID) || row.ItemID != current.Manifest.Frozen.ItemID || row.ItemVersionID != current.Manifest.Frozen.ItemVersionID || row.TurnID != tr.TurnID || row.TargetID != targetConfig.TargetID || row.TargetVersionID != targetConfig.TargetVersionID {
				return fmt.Errorf("hook termination target identity: %w", entity.ErrHookStoreCorrupt)
			}
			if row.ExperimentRunID != key.RunID && row.Status != int32(entity.EvalTargetRunStatusSuccess) {
				return entity.ErrHookStoreCorrupt
			}
			if row.Status == int32(entity.EvalTargetRunStatusAsyncInvoking) || row.Status == int32(entity.EvalTargetRunStatusUnknown) {
				output, err := hookTerminationErrorOutput(row.OutputData, "EvalTargetRunError", &entity.EvalTargetRunError{Code: int32(errno.AsyncEvalTargetTerminatedCode), Message: "async eval target terminated: experiment cancelled"})
				if err != nil {
					return err
				}
				if err := tx.Model(&tm.TargetRecord{}).Where("id=? AND space_id=? AND experiment_run_id=? AND status=?", row.ID, row.SpaceID, key.RunID, row.Status).UpdateColumns(map[string]any{"status": int32(entity.EvalTargetRunStatusFail), "output_data": output}).Error; err != nil {
					return err
				}
			} else if row.Status != int32(entity.EvalTargetRunStatusSuccess) && row.Status != int32(entity.EvalTargetRunStatusFail) {
				return entity.ErrHookStoreCorrupt
			}
		}
		expected, err := expectedHookArchiveRefs(current)
		if err != nil {
			return err
		}
		byRecord := map[int64]hookArchiveRefKey{}
		for k, id := range expected {
			if _, ok := byRecord[id]; ok {
				return entity.ErrHookStoreCorrupt
			}
			byRecord[id] = k
		}
		turnIDs := map[int64]int64{}
		targetIDs := map[int64]int64{}
		for _, mt := range current.Manifest.Turns {
			turnIDs[mt.ResultID] = mt.TurnID
		}
		for _, tr := range current.Turns {
			targetIDs[tr.TurnID] = tr.TargetResultID
		}
		if len(byRecord) != len(item.Evaluators) {
			return entity.ErrHookStoreCorrupt
		}
		for _, record := range item.Evaluators {
			if record == nil {
				return entity.ErrHookStoreCorrupt
			}
			ref, ok := byRecord[record.ID]
			if !ok {
				return entity.ErrHookStoreCorrupt
			}
			delete(byRecord, record.ID)
			var row em.EvaluatorRecord
			if err := tx.Unscoped().Where("id=?", record.ID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row).Error; err != nil {
				return err
			}
			if row.DeletedAt.Valid || row.SpaceID != record.SpaceID || gptr.Indirect(row.ExperimentID) != key.ExperimentID || row.ExperimentRunID != current.Manifest.EvaluatorRecordRun(row.ID) || row.ItemID != current.Manifest.Frozen.ItemID || row.ItemVersionID != current.Manifest.Frozen.ItemVersionID || row.TurnID != turnIDs[ref.turn] || row.EvaluatorVersionID != ref.ref.version || row.Alias_ != ref.ref.alias || row.InlineKey != ref.ref.inline || row.SourceType != int32(record.SourceType) || row.TargetRecordID != record.TargetRecordID {
				return entity.ErrHookStoreCorrupt
			}
			if ref.ref.inline != "" && row.TargetRecordID != targetIDs[row.TurnID] {
				return entity.ErrHookStoreCorrupt
			}
			if row.ExperimentRunID != key.RunID && row.Status != int32(entity.EvaluatorRunStatusSuccess) {
				return entity.ErrHookStoreCorrupt
			}
			if row.Status == int32(entity.EvaluatorRunStatusAsyncInvoking) || row.Status == int32(entity.EvaluatorRunStatusUnknown) {
				output, err := hookTerminationErrorOutput(row.OutputData, "evaluator_run_error", &entity.EvaluatorRunError{Code: int32(errno.ItemManuallyTerminatedCode), Message: "experiment terminated"})
				if err != nil {
					return err
				}
				if err := tx.Model(&em.EvaluatorRecord{}).Where("id=? AND space_id=? AND experiment_run_id=? AND status=?", row.ID, row.SpaceID, key.RunID, row.Status).UpdateColumns(map[string]any{"status": int32(entity.EvaluatorRunStatusFail), "output_data": output}).Error; err != nil {
					return err
				}
			} else if row.Status != int32(entity.EvaluatorRunStatusSuccess) && row.Status != int32(entity.EvaluatorRunStatusFail) && row.Status != int32(entity.EvaluatorRunStatusSkipped) {
				return entity.ErrHookStoreCorrupt
			}
		}
		return nil
	})
}

func hookTerminationErrorOutput(raw *[]byte, key string, value any) ([]byte, error) {
	fields := map[string]json.RawMessage{}
	if raw != nil && len(*raw) > 0 && string(*raw) != "null" {
		if err := json.Unmarshal(*raw, &fields); err != nil {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	errorJSON, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	fields[key] = errorJSON
	return json.Marshal(fields)
}
