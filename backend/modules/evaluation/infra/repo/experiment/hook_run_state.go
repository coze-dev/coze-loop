// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"errors"
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type lockedHookRun struct {
	life          model.ExptLifecycleRun
	log           model.ExptRunLog
	before, after *model.ExptLifecycleHookRun
	view          *entity.HookStoredRun
}

func hookRunScope(tx *gorm.DB, key entity.HookRunKey) *gorm.DB {
	return tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", key.WorkspaceID, key.ExperimentID, key.RunID)
}

func lockHookExperiment(tx *gorm.DB, key entity.HookRunKey) (*model.Experiment, error) {
	var row model.Experiment
	err := tx.Unscoped().Select("id", "space_id", "latest_run_id", "status", "deleted_at", "expt_type", "eval_set_source_type").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND space_id=?", key.ExperimentID, key.WorkspaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrHookStoreMissing
	}
	return &row, err
}

func loadHookRun(tx *gorm.DB, key entity.HookRunKey) (*lockedHookRun, error) {
	s := new(lockedHookRun)
	if err := hookRunScope(tx, key).Clauses(clause.Locking{Strength: "UPDATE"}).First(&s.life).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrHookStoreMissing
		}
		return nil, err
	}
	if !s.life.BeforeEnabled && !s.life.AfterEnabled {
		return nil, entity.ErrHookStoreCorrupt
	}
	if err := hookRunScope(tx.Unscoped(), key).Clauses(clause.Locking{Strength: "UPDATE"}).First(&s.log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrHookStoreCorrupt
		}
		return nil, err
	}
	if s.log.ID != key.RunID || s.log.DeletedAt.Valid || gptr.Indirect(s.log.LifecycleHookVersion) != 1 || s.log.Status == nil || s.log.Mode == nil || *s.log.Mode < 1 || *s.log.Mode > 6 || s.log.CreatedBy == "" {
		return nil, entity.ErrHookStoreCorrupt
	}
	var operations []model.ExptLifecycleHookRun
	if err := hookRunScope(tx, key).Order("phase ASC").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&operations).Error; err != nil {
		return nil, err
	}
	if len(operations) == 0 || len(operations) > 2 || s.life.Version < 0 || s.life.PlanCount < 0 || s.life.PlanState < 0 || s.life.PlanState > 1 || len(s.life.SnapshotCipher) == 0 || s.life.SnapshotKeyID == "" || len(s.life.SnapshotHash) != 64 || s.life.ExecutionScope == "" {
		return nil, entity.ErrHookStoreCorrupt
	}
	v := &entity.HookStoredRun{State: entity.HookRunState{Key: key, Status: entity.ExptStatus(*s.log.Status), Before: entity.HookOperation{Status: entity.HookOperationDisabled}, After: entity.HookOperation{Status: entity.HookOperationDisabled}},
		Version: s.life.Version, SourceRunID: s.life.SourceRunID, Snapshot: entity.HookProtectedSnapshot{Cipher: append([]byte(nil), s.life.SnapshotCipher...), KeyID: s.life.SnapshotKeyID, Hash: s.life.SnapshotHash, ExecutionScope: s.life.ExecutionScope},
		CreatedBy: s.log.CreatedBy, Mode: entity.ExptRunMode(*s.log.Mode), PlanReady: s.life.PlanState == 1, PlanCount: s.life.PlanCount, PlanCursor: gptr.Indirect(s.life.PlanCursor), PlanHash: gptr.Indirect(s.life.PlanHash), ExecutionStarted: s.life.ExecutionStarted, TerminalAt: s.life.TerminalAt, NextReconcileAt: s.life.NextReconcileAt}
	if s.log.StatusMessage != nil {
		v.DisplayMessage = gptr.Of(string(*s.log.StatusMessage))
	}
	v.ExecutionInitialized = s.life.ExecutionInitialized
	switch s.life.Gate {
	case 0:
		v.State.Gate = entity.HookGateWaiting
	case 1:
		v.State.Gate = entity.HookGateReady
	case 2:
		v.State.Gate = entity.HookGateClosed
	default:
		return nil, entity.ErrHookStoreCorrupt
	}
	switch s.life.FinalizeState {
	case 0:
		v.State.Finalize = entity.HookFinalizeNone
	case 1:
		v.State.Finalize = entity.HookFinalizePending
	case 2:
		v.State.Finalize = entity.HookFinalizeCommitted
	default:
		return nil, entity.ErrHookStoreCorrupt
	}
	if s.life.TerminalStatus != nil {
		v.State.Intent = entity.HookTerminalIntent{Status: entity.ExptStatus(*s.life.TerminalStatus), Reason: gptr.Indirect(s.life.TerminalReason)}
	}
	if (v.PlanReady && len(v.PlanHash) != 64) || (v.State.Finalize != entity.HookFinalizeNone && v.TerminalAt == nil) {
		return nil, entity.ErrHookStoreCorrupt
	}
	for i := range operations {
		op := &operations[i]
		if op.ExecutionScope != s.life.ExecutionScope || op.Version < 0 || op.IdempotencyKey == "" || op.Status == string(entity.HookOperationDisabled) {
			return nil, entity.ErrHookStoreCorrupt
		}
		status := entity.HookOperationStatus(op.Status)
		active := op.ActivatedAt != nil && (status == entity.HookOperationPending || status == entity.HookOperationRunning || status == entity.HookOperationRetryWait)
		state := entity.HookOperation{ID: op.OperationID, Status: status, Activated: active, Attempt: op.Attempt, Generation: op.LeaseGeneration, LeaseUntil: gptr.Indirect(op.LeaseUntil), AttemptDeadline: gptr.Indirect(op.AttemptDeadline), Deadline: gptr.Indirect(op.OperationDeadline)}
		switch entity.HookPhase(op.Phase) {
		case entity.HookPhaseBefore:
			s.before = op
			v.State.Before = state
		case entity.HookPhaseAfter:
			s.after = op
			v.State.After = state
		default:
			return nil, entity.ErrHookStoreCorrupt
		}
		v.Operations = append(v.Operations, entity.HookStoredOperation{HookOperationSeed: entity.HookOperationSeed{ID: op.ID, OperationID: op.OperationID, IdempotencyKey: op.IdempotencyKey}, Phase: entity.HookPhase(op.Phase), Version: op.Version, ActivatedAt: op.ActivatedAt, OccurredAt: op.OccurredAt, NextAttemptAt: op.NextAttemptAt})
	}
	if (s.before != nil) != s.life.BeforeEnabled || (s.after != nil) != s.life.AfterEnabled {
		return nil, entity.ErrHookStoreCorrupt
	}
	if v.State.Gate == entity.HookGateWaiting && s.before == nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	if err := entity.ValidateHookStorageState(&v.State); err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	s.view = v
	return s, nil
}

func equalHookSource(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// Invoke only after all rows needed by the operation have been locked.
func hookDBNow(tx *gorm.DB) (time.Time, error) {
	var now time.Time
	err := tx.Raw("SELECT CURRENT_TIMESTAMP(3)").Row().Scan(&now)
	return now, err
}

func hookOneRow(tx *gorm.DB) error {
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected != 1 {
		return entity.ErrHookStoreConflict
	}
	return nil
}
