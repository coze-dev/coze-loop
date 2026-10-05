// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func closeHookSchedulerFailureRecords(tx *gorm.DB, in entity.HookSchedulerFailureInput, current *entity.HookTerminationItem) (bool, error) {
	key := current.Manifest.Key
	var config model.Experiment
	if err := tx.Select("target_id", "target_version_id", "target_space_id").Where("id=? AND space_id=?", key.ExperimentID, key.WorkspaceID).First(&config).Error; err != nil {
		return false, err
	}
	if current.Execution != nil {
		config.TargetID, config.TargetVersionID, config.TargetSpaceID = current.Execution.TargetID, current.Execution.TargetVersionID, current.Execution.TargetSpaceID
	}
	space := config.TargetSpaceID
	if space == 0 {
		space = key.WorkspaceID
	}
	targets := map[int64]*entity.ExptTurnResultRunLog{}
	turnIDs := map[int64]int64{}
	for _, tr := range current.Turns {
		if tr.TargetResultID > 0 {
			targets[tr.TargetResultID] = tr
		}
	}
	for _, mt := range current.Manifest.Turns {
		turnIDs[mt.ResultID] = mt.TurnID
	}
	if len(targets) != len(in.Item.Targets) {
		return false, entity.ErrHookStoreCorrupt
	}
	var targetRows []tm.TargetRecord
	observed := map[int64]bool{}
	for _, id := range in.ObservedTargetIDs {
		if targets[id] == nil || observed[id] {
			return false, entity.ErrHookStoreCorrupt
		}
		observed[id] = true
	}
	qualifies := in.Zombie
	for _, record := range in.Item.Targets {
		if record == nil || targets[record.ID] == nil {
			return false, entity.ErrHookStoreCorrupt
		}
		turn := targets[record.ID]
		delete(targets, record.ID)
		var row tm.TargetRecord
		if err := tx.Unscoped().Where("id=?", record.ID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row).Error; err != nil {
			return false, err
		}
		if row.DeletedAt.Valid || row.SpaceID != space || row.SpaceID != record.SpaceID || row.ExperimentRunID != current.Manifest.TargetRecordRun(row.ID) || row.ItemID != current.Manifest.Frozen.ItemID || row.ItemVersionID != current.Manifest.Frozen.ItemVersionID || row.TurnID != turn.TurnID || row.TargetID != config.TargetID || row.TargetVersionID != config.TargetVersionID {
			return false, entity.ErrHookStoreCorrupt
		}
		if observed[row.ID] && (row.Status == int32(entity.EvalTargetRunStatusAsyncInvoking) || row.Status == int32(entity.EvalTargetRunStatusUnknown)) {
			qualifies = true
		}
		if row.ExperimentRunID != key.RunID && row.Status != int32(entity.EvalTargetRunStatusSuccess) {
			return false, entity.ErrHookStoreCorrupt
		}
		targetRows = append(targetRows, row)
	}
	if !qualifies {
		return false, nil
	}
	expected, err := expectedHookArchiveRefs(current)
	if err != nil {
		return false, err
	}
	byRecord := map[int64]hookArchiveRefKey{}
	for ref, id := range expected {
		if _, ok := byRecord[id]; ok {
			return false, entity.ErrHookStoreCorrupt
		}
		byRecord[id] = ref
	}
	if len(byRecord) != len(in.Item.Evaluators) {
		return false, entity.ErrHookStoreCorrupt
	}
	var evaluatorRows []em.EvaluatorRecord
	for _, record := range in.Item.Evaluators {
		if record == nil {
			return false, entity.ErrHookStoreCorrupt
		}
		ref, ok := byRecord[record.ID]
		if !ok {
			return false, entity.ErrHookStoreCorrupt
		}
		delete(byRecord, record.ID)
		var row em.EvaluatorRecord
		if err := tx.Unscoped().Where("id=?", record.ID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row).Error; err != nil {
			return false, err
		}
		if row.DeletedAt.Valid || row.SpaceID != record.SpaceID || gptr.Indirect(row.ExperimentID) != key.ExperimentID || row.ExperimentRunID != current.Manifest.EvaluatorRecordRun(row.ID) || row.ItemID != current.Manifest.Frozen.ItemID || row.ItemVersionID != current.Manifest.Frozen.ItemVersionID || row.TurnID != turnIDs[ref.turn] || row.EvaluatorVersionID != ref.ref.version || row.Alias_ != ref.ref.alias || row.InlineKey != ref.ref.inline || row.SourceType != int32(record.SourceType) || row.TargetRecordID != record.TargetRecordID {
			return false, entity.ErrHookStoreCorrupt
		}
		evaluatorRows = append(evaluatorRows, row)
		if row.ExperimentRunID != key.RunID && row.Status != int32(entity.EvaluatorRunStatusSuccess) {
			return false, entity.ErrHookStoreCorrupt
		}
	}
	message := fmt.Sprintf("sandbox execute reached terminal state (%s) before result was reported", in.SandboxStatus)
	targetCode, evaluatorCode := int32(errno.SandboxTerminatedBeforeReportCode), int32(errno.SandboxTerminatedBeforeReportCode)
	if in.Zombie {
		message = "async execution terminated: experiment item exceeded zombie timeout"
		targetCode = int32(errno.AsyncEvalTargetZombieTimeoutCode)
		evaluatorCode = int32(errno.AsyncEvaluatorZombieTimeoutCode)
	}
	for _, row := range targetRows {
		switch entity.EvalTargetRunStatus(row.Status) {
		case entity.EvalTargetRunStatusAsyncInvoking, entity.EvalTargetRunStatusUnknown:
			output, err := hookTerminationErrorOutput(row.OutputData, "EvalTargetRunError", &entity.EvalTargetRunError{Code: targetCode, Message: message})
			if err != nil {
				return false, err
			}
			if err := tx.Model(&tm.TargetRecord{}).Where("id=? AND space_id=? AND experiment_run_id=? AND status=?", row.ID, row.SpaceID, key.RunID, row.Status).UpdateColumns(map[string]any{"status": int32(entity.EvalTargetRunStatusFail), "output_data": output}).Error; err != nil {
				return false, err
			}
		case entity.EvalTargetRunStatusSuccess, entity.EvalTargetRunStatusFail:
		default:
			return false, entity.ErrHookStoreCorrupt
		}
	}
	for _, row := range evaluatorRows {
		switch entity.EvaluatorRunStatus(row.Status) {
		case entity.EvaluatorRunStatusAsyncInvoking, entity.EvaluatorRunStatusUnknown:
			output, err := hookTerminationErrorOutput(row.OutputData, "evaluator_run_error", &entity.EvaluatorRunError{Code: evaluatorCode, Message: message})
			if err != nil {
				return false, err
			}
			if err := tx.Model(&em.EvaluatorRecord{}).Where("id=? AND space_id=? AND experiment_run_id=? AND status=?", row.ID, row.SpaceID, key.RunID, row.Status).UpdateColumns(map[string]any{"status": int32(entity.EvaluatorRunStatusFail), "output_data": output}).Error; err != nil {
				return false, err
			}
		case entity.EvaluatorRunStatusSuccess, entity.EvaluatorRunStatusFail, entity.EvaluatorRunStatusSkipped:
		default:
			return false, entity.ErrHookStoreCorrupt
		}
	}
	return true, nil
}
