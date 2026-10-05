// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"gorm.io/gorm"
)

var _ hookcomponent.AttemptReader = (*hookRunRepo)(nil)

func hookAttemptDBClock(tx *gorm.DB) (*entity.HookClockAnchor, error) {
	start := time.Now()
	now, err := hookDBNow(tx)
	end := time.Now()
	if err != nil {
		return nil, err
	}
	return &entity.HookClockAnchor{DBTime: now, LocalBefore: start, LocalAfter: end, Precision: time.Millisecond}, nil
}

func (r *hookRunRepo) ReadAttempt(ctx context.Context, in entity.HookAttemptScope) (entity.HookAttemptStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	var out entity.HookAttemptStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		expt, s, op, err := lockHookAttemptScope(tx, in)
		if err != nil {
			return err
		}
		out.Run = s.view
		clock, err := hookAttemptDBClock(tx)
		if err != nil {
			return err
		}
		out.Clock = clock
		if op.Status != string(entity.HookOperationRunning) || !hookAttemptAllowed(expt, s, in.Phase) {
			return nil
		}
		audit, err := lockHookAttemptAudit(tx, op.OperationID, op.Attempt)
		if err != nil {
			return err
		}
		if op.LeaseOwner == nil || op.LeaseUntil == nil || op.AttemptDeadline == nil || op.OperationDeadline == nil || audit.LeaseGeneration != op.LeaseGeneration || audit.ResultCategory != nil || audit.DeliveryID != hookAttemptDelivery(*audit, *op.LeaseOwner) {
			return entity.ErrHookStoreCorrupt
		}
		out.Claim = &entity.HookAttemptClaim{HookAttemptIdentity: entity.HookAttemptIdentity{Token: entity.HookClaimToken{Run: in.Key, OperationID: op.OperationID, Phase: in.Phase, Attempt: op.Attempt, Generation: op.LeaseGeneration}, Owner: *op.LeaseOwner, DeliveryID: audit.DeliveryID}, Version: op.Version, IdempotencyKey: op.IdempotencyKey, StartedAt: audit.StartedAt, LeaseUntil: *op.LeaseUntil, AttemptDeadline: *op.AttemptDeadline, Deadline: *op.OperationDeadline}
		return nil
	}, db.WithMaster())
	if err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	return out, nil
}
