// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

func (r *hookTurnProgressRepo) InitializeHookTurnRunLogs(ctx context.Context, key entity.HookRunKey, itemID, version int64, candidates []*entity.ExptTurnResultRunLog) (handled bool, committed []*entity.ExptTurnResultRunLog, err error) {
	if ctx == nil || r == nil || r.executionScope == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil || itemID <= 0 || version < 0 {
		return false, nil, entity.ErrHookStoreCorrupt
	}
	scope, err := r.executionScope(ctx)
	if err != nil || !hookGateASCII(scope) {
		return false, nil, entity.ErrHookExecutionStorage
	}
	err = (&hookRunRepo{provider: r.provider}).executionTransaction(ctx, func(tx *gorm.DB) error {
		var life model.ExptLifecycleRun
		if err := hookRunScope(tx, key).Select("before_enabled", "after_enabled", "execution_scope").First(&life).Error; err != nil {
			return err
		}
		if life.ExecutionScope != scope {
			return entity.ErrHookStoreConflict
		}
		if !life.BeforeEnabled && !life.AfterEnabled {
			return nil
		}
		expt, err := lockHookExperiment(tx, key)
		if err != nil {
			return err
		}
		run, err := loadHookRun(tx, key)
		if err != nil {
			return err
		}
		if run.life.ExecutionScope != scope {
			return entity.ErrHookStoreConflict
		}
		if r.binding != nil {
			if err := checkBoundFinalizationRun(tx, key, scope, r.binding); err != nil {
				return err
			}
		}
		if !hookExecutionRequired(expt, run) && !boundFinalizationRequired(r.binding, &run.life, run.view.Mode) {
			return nil
		}
		handled = true
		if !hookRunActive(expt, run) || run.life.Gate != 1 || run.life.FinalizeState != 0 || !run.life.ExecutionInitialized || !run.view.PlanReady {
			return entity.ErrHookAdmissionDenied
		}
		current, row, err := readHookArchiveItem(tx, key, itemID)
		if err != nil {
			return err
		}
		if err := checkBoundFinalizationRefs(tx, key, []entity.HookExecutionManifest{current.Manifest}, r.binding, true); err != nil {
			return err
		}
		if row.AdmittedAt == nil || current.Manifest.Frozen.ItemVersionID != version {
			return entity.ErrHookStoreConflict
		}
		if entity.IsItemRunFinished(entity.ItemRunState(current.Item.Status)) {
			return entity.ErrHookAdmissionDenied
		}
		if len(current.Turns) > 0 && current.Manifest.TurnLogsInitialized == nil {
			if err := pinHookTurnRunLogs(tx, row, current.Manifest, current.Turns); err != nil {
				return err
			}
		}
		if candidates == nil {
			committed = current.Turns
			return nil
		}
		if len(candidates) != len(current.Manifest.Turns) {
			return entity.ErrHookStoreCorrupt
		}
		expected := make(map[int64]bool, len(current.Manifest.Turns))
		for _, turn := range current.Manifest.Turns {
			expected[turn.TurnID] = true
		}
		ids := make(map[int64]bool, len(candidates))
		rows := make([]*model.ExptTurnResultRunLog, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate == nil || candidate.ID <= 0 || ids[candidate.ID] || candidate.SpaceID != key.WorkspaceID || candidate.ExptID != key.ExperimentID || candidate.ExptRunID != key.RunID || candidate.ItemID != itemID || candidate.ItemVersionID != version || !expected[candidate.TurnID] || candidate.Status != entity.TurnRunState_Processing || candidate.TargetResultID != 0 || candidate.EvaluatorResultIds != nil {
				return entity.ErrHookStoreCorrupt
			}
			delete(expected, candidate.TurnID)
			ids[candidate.ID] = true
			po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(candidate)
			if err != nil {
				return err
			}
			rows = append(rows, po)
		}
		if len(current.Turns) == 0 {
			// The same parent locks fence cancellation until all genuine PreEval rows commit.
			if err := tx.CreateInBatches(rows, 50).Error; err != nil {
				return err
			}
			if err := pinHookTurnRunLogs(tx, row, current.Manifest, candidates); err != nil {
				return err
			}
			current, _, err = readHookArchiveItem(tx, key, itemID)
			if err != nil {
				return err
			}
		}
		committed = current.Turns
		return nil
	})
	if err != nil {
		return handled, nil, err
	}
	return handled, committed, nil
}

func pinHookTurnRunLogs(tx *gorm.DB, row *model.ExptLifecycleRunItem, m entity.HookExecutionManifest, logs []*entity.ExptTurnResultRunLog) error {
	byTurn := map[int64]int64{}
	for _, log := range logs {
		byTurn[log.TurnID] = log.ID
	}
	m = m.Clone()
	m.TurnLogsInitialized = gptr.Of(true)
	for i := range m.Turns {
		m.Turns[i].RunLogID = byTurn[m.Turns[i].TurnID]
	}
	if m.Validate() != nil {
		return entity.ErrHookStoreCorrupt
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return hookOneRow(hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), m.Key).Where("id=?", row.ID).UpdateColumn("execution_manifest", raw))
}
