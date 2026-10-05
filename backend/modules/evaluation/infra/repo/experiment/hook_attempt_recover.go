// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"math"
	"time"
)

func (r *hookRunRepo) RecoverExpiredAttempt(ctx context.Context, in entity.HookRecoverExpiredAttemptInput) (entity.HookAttemptStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	var out entity.HookAttemptStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, s, op, err := lockHookAttemptScope(tx, in.HookAttemptScope)
		if err != nil {
			return err
		}
		if op.Version != in.ExpectedVersion || !hookAttemptAllowed(expt, s, in.Phase) {
			return entity.ErrHookStoreConflict
		}
		out.Run = s.view
		if op.Status != string(entity.HookOperationRunning) {
			return nil
		}
		audit, err := lockHookAttemptAudit(tx, op.OperationID, op.Attempt)
		if err != nil {
			return err
		}
		if audit.ResultCategory != nil || audit.LeaseGeneration != op.LeaseGeneration || op.LeaseOwner == nil || audit.DeliveryID != hookAttemptDelivery(*audit, *op.LeaseOwner) {
			return entity.ErrHookStoreCorrupt
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		if now.Before(gptr.Indirect(op.LeaseUntil)) || now.Before(gptr.Indirect(op.AttemptDeadline)) {
			return nil
		}
		change, err := expiredHookAttemptChange(s, in.Phase, in.Config, now)
		if err != nil {
			return err
		}
		if err := persistHookAttemptChange(tx, s, op, change, now, map[string]any{"error_code": string(entity.HookTimeoutUncertain), "result_redacted": nil, "error_message": nil}); err != nil {
			return err
		}
		if err := hookOneRow(tx.Model(&model.ExptLifecycleHookAttempt{}).Where("id=? AND operation_id=? AND attempt=? AND lease_generation=? AND delivery_id=? AND result_category IS NULL", audit.ID, audit.OperationID, audit.Attempt, audit.LeaseGeneration, audit.DeliveryID).UpdateColumns(map[string]any{"result_category": string(entity.HookTimeoutUncertain), "finished_at": nil})); err != nil {
			return err
		}
		s, err = loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out.HookStoreResult = entity.HookStoreResult{Run: s.view, Changed: true, Effects: change.Effects}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	return out, nil
}

// This fence exists only inside the locked transaction; it is never an HTTP lease.
func expiredHookAttemptChange(s *lockedHookRun, phase entity.HookPhase, config *entity.HookConfig, now time.Time) (entity.HookStateChange, error) {
	state := s.view.State
	op := &state.Before
	if phase == entity.HookPhaseAfter {
		op = &state.After
	}
	if op.Generation == math.MaxInt64 {
		return entity.HookStateChange{}, entity.ErrHookStoreConflict
	}
	op.Status, op.Activated = entity.HookOperationRunning, true
	op.Generation++
	op.LeaseUntil = now.Add(time.Second)
	token := entity.HookClaimToken{Run: state.Key, OperationID: op.ID, Phase: phase, Attempt: op.Attempt, Generation: op.Generation}
	return entity.CompleteHookOperation(&state, entity.HookResultInput{Token: token, Config: config, Outcome: entity.HookOutcome{Code: entity.HookTimeoutUncertain}, CommitAt: now})
}
