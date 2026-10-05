// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"time"
)

func (r *hookRunRepo) ClaimAttempt(ctx context.Context, in entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	var out entity.HookAttemptStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, s, op, err := lockHookAttemptScope(tx, in.HookAttemptScope)
		if err != nil {
			return err
		}
		if op.Version != in.ExpectedVersion || !hookAttemptAllowed(expt, s, in.Phase) || op.ActivatedAt == nil || op.NextAttemptAt == nil || (op.Status != string(entity.HookOperationPending) && op.Status != string(entity.HookOperationRetryWait)) {
			return entity.ErrHookStoreConflict
		}
		if op.LeaseGeneration == math.MaxInt64 {
			return entity.ErrHookStoreConflict
		}
		// Do not prelock a new audit ID: RR gap locks can deadlock independent claims.
		if op.Attempt > 0 {
			audit, err := lockHookAttemptAudit(tx, op.OperationID, op.Attempt)
			if err != nil {
				return err
			}
			if audit.ResultCategory == nil {
				return entity.ErrHookStoreCorrupt
			}
		}
		clock, err := hookAttemptDBClock(tx)
		if err != nil {
			return err
		}
		now := clock.DBTime
		if now.Before(*op.NextAttemptAt) {
			return entity.ErrHookStoreConflict
		}
		timeout := hookAttemptTimeout(in.Config)
		deadline := gptr.Indirect(op.OperationDeadline)
		if op.Attempt == 0 {
			if op.Status != string(entity.HookOperationPending) || !deadline.IsZero() {
				return entity.ErrHookStoreCorrupt
			}
			deadline, err = entity.HookStageDeadline(now, int32(timeout/time.Second), in.Config.Retry)
			if err != nil {
				return err
			}
		} else if deadline.IsZero() || op.AttemptDeadline == nil {
			return entity.ErrHookStoreCorrupt
		}
		if op.Attempt >= in.Config.Retry.EffectiveMaxRetries()+1 || now.Add(timeout).After(deadline) {
			if op.Status != string(entity.HookOperationRetryWait) {
				return entity.ErrHookStoreCorrupt
			}
			change, err := expiredHookAttemptChange(s, in.Phase, in.Config, now)
			if err != nil {
				return err
			}
			if err := persistHookAttemptChange(tx, s, op, change, now, map[string]any{"error_code": string(entity.HookTimeoutUncertain), "result_redacted": nil, "error_message": nil}); err != nil {
				return err
			}
			s, err = loadHookRun(tx, in.Key)
			if err != nil {
				return err
			}
			out.HookStoreResult = entity.HookStoreResult{Run: s.view, Changed: true, Effects: change.Effects}
			return nil
		}
		attemptDeadline := now.Add(timeout)
		lease := hookAttemptLease(now, attemptDeadline, in.LeaseSeconds)
		audit := model.ExptLifecycleHookAttempt{ID: in.AttemptID, OperationID: op.OperationID, Attempt: op.Attempt + 1, LeaseGeneration: op.LeaseGeneration + 1, StartedAt: now}
		audit.DeliveryID = hookAttemptDelivery(audit, in.Owner)
		fields := map[string]any{"status": string(entity.HookOperationRunning), "attempt": audit.Attempt, "lease_generation": audit.LeaseGeneration, "lease_owner": in.Owner, "lease_until": lease, "attempt_deadline": attemptDeadline, "operation_deadline": deadline, "next_attempt_at": nil, "result_redacted": nil, "error_code": nil, "error_message": nil}
		if err := updateHookOperation(tx, in.Key, op, fields); err != nil {
			return err
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		s, err = loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out = entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: s.view, Changed: true}, Claim: &entity.HookAttemptClaim{HookAttemptIdentity: entity.HookAttemptIdentity{Token: entity.HookClaimToken{Run: in.Key, OperationID: op.OperationID, Phase: in.Phase, Attempt: audit.Attempt, Generation: audit.LeaseGeneration}, Owner: in.Owner, DeliveryID: audit.DeliveryID}, Version: op.Version + 1, IdempotencyKey: op.IdempotencyKey, StartedAt: now, LeaseUntil: lease, AttemptDeadline: attemptDeadline, Deadline: deadline}}
		out.Clock = clock
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	return out, nil
}

func lockHookAttemptScope(tx *gorm.DB, in entity.HookAttemptScope) (*model.Experiment, *lockedHookRun, *model.ExptLifecycleHookRun, error) {
	expt, err := lockHookExperiment(tx, in.Key)
	if err != nil {
		return nil, nil, nil, err
	}
	s, err := loadHookRun(tx, in.Key)
	if err != nil {
		return nil, nil, nil, err
	}
	op := s.before
	if in.Phase == entity.HookPhaseAfter {
		op = s.after
	}
	if op == nil || op.OperationID != in.OperationID || s.life.ExecutionScope != in.ExecutionScope || s.life.SnapshotHash != in.SnapshotHash {
		return nil, nil, nil, entity.ErrHookStoreConflict
	}
	return expt, s, op, nil
}

func hookAttemptAllowed(expt *model.Experiment, s *lockedHookRun, phase entity.HookPhase) bool {
	if phase == entity.HookPhaseAfter {
		return s.view.State.Finalize == entity.HookFinalizeCommitted
	}
	return hookRunActive(expt, s) && s.view.PlanReady
}

func hookAttemptTimeout(config *entity.HookConfig) time.Duration {
	seconds := gptr.Indirect(config.TimeoutSeconds)
	if seconds == 0 {
		seconds = 180
	}
	return time.Duration(seconds) * time.Second
}

func hookAttemptLease(now, deadline time.Time, configured ...*int32) time.Time {
	seconds := entity.HookDefaultLeaseSeconds
	if len(configured) > 0 && configured[0] != nil {
		seconds = *configured[0]
	}
	lease := now.Add(time.Duration(seconds) * time.Second)
	limit := deadline.Add(5 * time.Second)
	if lease.After(limit) {
		return limit
	}
	return lease
}

// Bind historical owner to delivery without storing it in redacted result fields.
func hookAttemptDelivery(a model.ExptLifecycleHookAttempt, owner string) string {
	return fmt.Sprintf("hook_%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d:%d:%s", a.ID, a.OperationID, a.Attempt, a.LeaseGeneration, owner))))
}
func (r *hookRunRepo) RenewAttempt(ctx context.Context, in entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	var out entity.HookAttemptStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, s, op, err := lockHookAttemptScope(tx, in.HookAttemptScope)
		if err != nil {
			return err
		}
		audit, err := lockHookAttemptIdentity(tx, in.HookAttemptIdentity)
		if err != nil {
			return err
		}
		clock, err := hookAttemptDBClock(tx)
		if err != nil {
			return err
		}
		now := clock.DBTime
		if !hookAttemptAllowed(expt, s, in.Phase) || !hookAttemptCurrent(op, in.HookAttemptIdentity) || op.Version != in.ExpectedVersion || audit.ResultCategory != nil || !now.Before(gptr.Indirect(op.LeaseUntil)) {
			return entity.ErrHookStoreConflict
		}
		lease := hookAttemptLease(now, *op.AttemptDeadline, in.LeaseSeconds)
		if !now.Before(lease) {
			return entity.ErrHookStoreConflict
		}
		if err := updateHookOperation(tx, in.Key, op, map[string]any{"lease_until": lease}); err != nil {
			return err
		}
		s, err = loadHookRun(tx, in.Key)
		if err != nil {
			return err
		}
		out = entity.HookAttemptStoreResult{HookStoreResult: entity.HookStoreResult{Run: s.view, Changed: true}, Claim: &entity.HookAttemptClaim{HookAttemptIdentity: in.HookAttemptIdentity, Version: op.Version + 1, IdempotencyKey: op.IdempotencyKey, StartedAt: audit.StartedAt, LeaseUntil: lease, AttemptDeadline: *op.AttemptDeadline, Deadline: *op.OperationDeadline}}
		out.Clock = clock
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	return out, nil
}

func lockHookAttemptIdentity(tx *gorm.DB, in entity.HookAttemptIdentity) (*model.ExptLifecycleHookAttempt, error) {
	audit, err := lockHookAttemptAudit(tx, in.Token.OperationID, in.Token.Attempt)
	if err != nil {
		return nil, err
	}
	if audit.LeaseGeneration != in.Token.Generation || audit.DeliveryID != in.DeliveryID || audit.DeliveryID != hookAttemptDelivery(*audit, in.Owner) {
		return nil, entity.ErrHookStoreConflict
	}
	return audit, nil
}

func lockHookAttemptAudit(tx *gorm.DB, operationID string, attempt int32) (*model.ExptLifecycleHookAttempt, error) {
	var audit model.ExptLifecycleHookAttempt
	err := tx.Where("operation_id=? AND attempt=?", operationID, attempt).Clauses(clause.Locking{Strength: "UPDATE"}).First(&audit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, entity.ErrHookStoreConflict
	}
	return &audit, err
}

func hookAttemptCurrent(op *model.ExptLifecycleHookRun, in entity.HookAttemptIdentity) bool {
	return op.Status == string(entity.HookOperationRunning) && op.Attempt == in.Token.Attempt && op.LeaseGeneration == in.Token.Generation && gptr.Indirect(op.LeaseOwner) == in.Owner
}
