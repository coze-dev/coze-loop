// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"errors"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/hints"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func NewHookDeletionRepo(provider db.Provider) repo.IHookDeletionRepo {
	return &hookRunRepo{provider: provider}
}

func (r *hookRunRepo) DeleteExperiments(ctx context.Context, ids []int64, workspaceID int64, scope string) ([]*entity.Experiment, error) {
	if r == nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	return deleteExperimentsWithHookBindings(ctx, r.provider, ids, workspaceID, scope, r.executionBinding, nil)
}

func deleteExperimentsWithHookBindings(ctx context.Context, provider db.Provider, ids []int64, workspaceID int64, scope string, binding *boundHookExecution, prepared *preparedHookDeletion) ([]*entity.Experiment, error) {
	if ctx == nil || provider == nil || workspaceID <= 0 || !hookGateASCII(scope) {
		return nil, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := hookDeletionIDs(ids)
	if err != nil {
		return nil, err
	}
	var deleted []*entity.Experiment
	err = provider.Transaction(ctx, func(tx *gorm.DB) error {
		var parents []*model.Experiment
		for _, id := range ids {
			// Same first lock as Run creation; keep it through enumeration and softdelete.
			var expt model.Experiment
			if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND space_id=?", id, workspaceID).First(&expt).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				}
				return err
			}
			if expt.DeletedAt.Valid {
				continue
			}
			if prepared != nil {
				expected, ok := prepared.parents[id]
				if !ok || expected.typ != expt.ExptType || expected.source != expt.EvalSetSourceType {
					return entity.ErrHookStoreConflict
				}
			}
			parents = append(parents, &expt)
		}
		// Acquire every parent before the first consistent read so later parents cannot use an older snapshot.
		for _, expt := range parents {
			var expected *preparedHookDeletionParent
			if prepared != nil {
				expected = prepared.parents[expt.ID]
			}
			if err := persistHookDeletionWithBindings(tx, expt, scope, expected, binding); err != nil {
				return err
			}
			view, err := convert.NewExptConverter().PO2DO(expt, nil)
			if err != nil {
				return err
			}
			if err := hookOneRow(tx.Where("id=? AND space_id=?", expt.ID, workspaceID).Delete(&model.Experiment{})); err != nil {
				return err
			}
			deleted = append(deleted, view)
		}
		return nil
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func persistHookDeletion(tx *gorm.DB, expt *model.Experiment, scope string, bindings ...*boundHookExecution) error {
	return persistHookDeletionWithBindings(tx, expt, scope, nil, firstFinalizationBinding(bindings))
}

func persistHookDeletionWithBindings(tx *gorm.DB, expt *model.Experiment, scope string, expected *preparedHookDeletionParent, fallback *boundHookExecution) error {
	seen := 0
	for cursor := int64(0); ; {
		var logs []model.ExptRunLog
		// Enumerate markers, not only lifecycle rows: a missing managed ledger must block deletion.
		if err := tx.Unscoped().Select("expt_run_id", "lifecycle_hook_version").Clauses(hints.ForceIndex("uk_expt_run")).Where("space_id=? AND expt_id=? AND expt_run_id>?", expt.SpaceID, expt.ID, cursor).
			Order("expt_run_id").Limit(100).Find(&logs).Error; err != nil {
			return err
		}
		for _, log := range logs {
			if expected != nil && log.ExptRunID <= cursor {
				return entity.ErrHookStoreCorrupt
			}
			cursor = log.ExptRunID
			marker := gptr.Indirect(log.LifecycleHookVersion)
			if expected != nil {
				want, ok := expected.markers[cursor]
				if !ok || want != marker {
					return entity.ErrHookStoreConflict
				}
				seen++
			}
			if marker == 0 {
				continue
			}
			if marker != 1 {
				return entity.ErrHookStoreCorrupt
			}
			key := entity.HookRunKey{WorkspaceID: expt.SpaceID, ExperimentID: expt.ID, RunID: log.ExptRunID}
			s, err := loadHookRun(tx, key)
			if err != nil {
				return err
			}
			if s.life.ExecutionScope != scope {
				return entity.ErrHookStoreConflict
			}
			b := fallback
			if expected != nil {
				entry, ok := expected.runs[key.RunID]
				if !ok {
					return entity.ErrHookStoreConflict
				}
				if err := entry.check(s); err != nil {
					return err
				}
				b = entry.binding
			}
			if s.view.State.Finalize == entity.HookFinalizeCommitted {
				continue
			}
			if err := checkBoundFinalizationRun(tx, key, scope, b); err != nil {
				return err
			}
			if !entity.HookExecutionInitializationRequired(s.life.BeforeEnabled || s.life.AfterEnabled, s.view.Mode, entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) && !boundFinalizationRequired(b, &s.life, s.view.Mode) {
				return entity.ErrHookFinalizationUnsupported
			}
			if !s.life.ExecutionInitialized {
				if s.view.State.Finalize == entity.HookFinalizePending && !hookCancellation(&s.life) {
					return entity.ErrHookFinalizationUnsettled
				}
				// Prove cancellation is recoverable before changing the persisted Gate or intent.
				proof := s.life
				proof.Gate = 2
				if _, err := readHookNeverAdmittedProof(tx, key, &proof, true, b); err != nil {
					return err
				}
			}
			// Pending success/failure/cancellation is immutable and already recoverable.
			if s.view.State.Finalize == entity.HookFinalizePending {
				continue
			}
			if entity.IsExptFinished(s.view.State.Status) {
				return entity.ErrHookStoreCorrupt
			}
			in := entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: s.life.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}}
			if err := finalizeLockedHookRun(tx, expt, s, in, false, true, new(entity.HookStoreResult), b); err != nil {
				return err
			}
		}
		if len(logs) < 100 {
			if expected != nil && seen != len(expected.markers) {
				return entity.ErrHookStoreConflict
			}
			return nil
		}
	}
}

// Only explicit original-Run recovery may see a retained softdeleted parent.
func readHookRecoveryExperiment(tx *gorm.DB, key entity.HookRunKey) (*model.Experiment, error) {
	var expt model.Experiment
	if err := tx.Unscoped().Where("id=? AND space_id=?", key.ExperimentID, key.WorkspaceID).First(&expt).Error; err != nil {
		return nil, err
	}
	if expt.DeletedAt.Valid {
		if err := validateHookDeletedRecovery(tx, key); err != nil {
			return nil, err
		}
	}
	return &expt, nil
}

func validateHookDeletedRecovery(tx *gorm.DB, key entity.HookRunKey) error {
	if (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return entity.ErrHookStoreMissing
	}
	var life model.ExptLifecycleRun
	if err := hookRunScope(tx, key).First(&life).Error; err != nil {
		return err
	}
	var log model.ExptRunLog
	if err := hookRunScope(tx.Unscoped(), key).Where("id=?", key.RunID).First(&log).Error; err != nil {
		return err
	}
	if log.DeletedAt.Valid || gptr.Indirect(log.LifecycleHookVersion) != 1 || log.Status == nil ||
		life.Gate != 2 || (life.FinalizeState != 1 && life.FinalizeState != 2) || life.TerminalAt == nil ||
		!entity.IsExptFinished(entity.ExptStatus(gptr.Indirect(life.TerminalStatus))) || (!life.BeforeEnabled && !life.AfterEnabled) {
		return entity.ErrHookStoreMissing
	}
	return nil
}
