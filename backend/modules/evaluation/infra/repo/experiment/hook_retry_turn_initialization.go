// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *hookTurnProgressRepo) InitializeHookRetryTurnRunLogs(ctx context.Context, key entity.HookRunKey, itemID, version int64, candidates []*entity.ExptTurnResultRunLog) (manifest *entity.HookExecutionManifest, committed []*entity.ExptTurnResultRunLog, err error) {
	if r == nil || r.binding == nil || !entity.HookBoundRetryMode(r.binding.source.Mode) {
		return nil, nil, entity.ErrHookExecutionUnsupported
	}
	scope, err := r.executionScope(ctx)
	if err != nil {
		return nil, nil, err
	}
	storage := &hookFinalizationRepo{provider: r.provider, binding: r.binding}
	err = storage.itemArchiveTransaction(ctx, key, scope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		if expt.LatestRunID != key.RunID || life.Gate != 1 || life.FinalizeState != 0 || !life.ExecutionInitialized {
			return entity.ErrHookAdmissionDenied
		}
		item, row, err := storage.readArchiveItem(tx, key, itemID)
		if err != nil {
			return err
		}
		m := item.Manifest
		if row.AdmittedAt == nil || m.Frozen.ItemVersionID != version || m.Retry == nil || entity.IsItemRunFinished(entity.ItemRunState(item.Item.Status)) {
			return entity.ErrHookAdmissionDenied
		}
		if len(item.Turns) > 0 || candidates == nil {
			copy := m.Clone()
			manifest = &copy
			committed = item.Turns
			return nil
		}
		if len(candidates) != len(m.Turns) {
			return entity.ErrHookStoreCorrupt
		}
		ancestors, err := retryAncestors(tx, key, r.binding.source.SourceRunID)
		if err != nil {
			return err
		}
		m.Retry.ReusedTargets = map[int64]int64{}
		m.Retry.ReusedEvaluators = map[int64]int64{}
		byTurn := map[int64]entity.HookExecutionRetryTurn{}
		candidateByTurn := map[int64]*entity.ExptTurnResultRunLog{}
		for _, source := range m.Retry.Turns {
			byTurn[source.TurnID] = source
		}
		ids := map[int64]bool{}
		for _, candidate := range candidates {
			if candidate == nil || candidate.ID <= 0 || ids[candidate.ID] || candidate.SpaceID != key.WorkspaceID || candidate.ExptID != key.ExperimentID || candidate.ExptRunID != key.RunID || candidate.ItemID != itemID || candidate.ItemVersionID != version || candidate.Status != entity.TurnRunState_Processing {
				return entity.ErrHookStoreCorrupt
			}
			source, ok := byTurn[candidate.TurnID]
			if !ok || candidateByTurn[candidate.TurnID] != nil {
				return entity.ErrHookStoreCorrupt
			}
			ids[candidate.ID] = true
			candidateByTurn[candidate.TurnID] = candidate
			if candidate.TargetResultID > 0 {
				if r.binding.source.Mode != entity.EvaluationModeFailRetry || candidate.TargetResultID != source.TargetResultID {
					return entity.ErrHookStoreCorrupt
				}
				var target tm.TargetRecord
				if err := tx.Unscoped().Where("id=?", candidate.TargetResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&target).Error; err != nil {
					return err
				}
				frozen := r.binding.source.Execution.Target
				space := frozen.SourceSpaceID
				if space == 0 {
					space = key.WorkspaceID
				}
				if target.DeletedAt.Valid || !ancestors[target.ExperimentRunID] || target.SpaceID != space || target.TargetID != frozen.ID || target.TargetVersionID != frozen.VersionID || target.ItemID != itemID || target.ItemVersionID != version || target.TurnID != candidate.TurnID || target.Status != int32(entity.EvalTargetRunStatusSuccess) {
					return entity.ErrHookStoreCorrupt
				}
				m.Retry.ReusedTargets[target.ID] = target.ExperimentRunID
			}
		}
		expected, err := expectedHookArchiveRefs(&entity.HookTerminationItem{Manifest: m, Item: item.Item, Turns: candidates})
		if err != nil {
			return err
		}
		byResult := map[int64]entity.HookExecutionRetryTurn{}
		for _, source := range m.Retry.Turns {
			byResult[source.ResultID] = source
		}
		for keyRef, id := range expected {
			if r.binding.source.Mode != entity.EvaluationModeFailRetry {
				return entity.ErrHookStoreCorrupt
			}
			source := byResult[keyRef.turn]
			matched := false
			for _, ref := range source.Refs {
				if ref != nil && ref.EvaluatorResultID == id && ref.EvaluatorVersionID == keyRef.ref.version && ref.Alias == keyRef.ref.alias && ref.InlineKey == keyRef.ref.inline {
					matched = true
					break
				}
			}
			if !matched {
				return entity.ErrHookStoreCorrupt
			}
			var record em.EvaluatorRecord
			if err := tx.Unscoped().Where("id=?", id).Clauses(clause.Locking{Strength: "UPDATE"}).First(&record).Error; err != nil {
				return err
			}
			if record.DeletedAt.Valid || !ancestors[record.ExperimentRunID] || gptr.Indirect(record.ExperimentID) != key.ExperimentID || record.ItemID != itemID || record.ItemVersionID != version || record.TurnID != source.TurnID || record.EvaluatorVersionID != keyRef.ref.version || record.Alias_ != keyRef.ref.alias || record.InlineKey != keyRef.ref.inline || record.Status != int32(entity.EvaluatorRunStatusSuccess) {
				return entity.ErrHookStoreCorrupt
			}
			if keyRef.ref.inline != "" && (record.SourceType != int32(entity.EvaluatorRecordSourceTypeInline) || record.TargetRecordID != candidateByTurn[source.TurnID].TargetResultID) {
				return entity.ErrHookStoreCorrupt
			}
			if keyRef.ref.inline == "" && record.TargetRecordID != 0 {
				return entity.ErrHookStoreCorrupt
			}
			m.Retry.ReusedEvaluators[id] = record.ExperimentRunID
		}
		rows := make([]*model.ExptTurnResultRunLog, 0, len(candidates))
		for _, candidate := range candidates {
			po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(candidate)
			if err != nil {
				return err
			}
			rows = append(rows, po)
		}
		if err := tx.CreateInBatches(rows, 50).Error; err != nil {
			return err
		}
		if err := pinHookTurnRunLogs(tx, row, m, candidates); err != nil {
			return err
		}
		current, _, err := storage.readArchiveItem(tx, key, itemID)
		if err != nil {
			return err
		}
		copy := current.Manifest.Clone()
		manifest = &copy
		committed = current.Turns
		return nil
	})
	return manifest, committed, err
}
