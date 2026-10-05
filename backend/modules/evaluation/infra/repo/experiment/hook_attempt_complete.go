// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func (r *hookRunRepo) CompleteAttempt(ctx context.Context, in entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error) {
	if err := in.Validate(); err != nil {
		return entity.HookAttemptStoreResult{}, err
	}
	var out entity.HookAttemptStoreResult
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		// Display fields are authorized storage content, not SQL diagnostic payloads.
		tx = tx.Session(&gorm.Session{Logger: logger.Discard})
		expt, s, op, err := lockHookAttemptScope(tx, in.HookAttemptScope)
		if err != nil {
			return err
		}
		audit, err := lockHookAttemptIdentity(tx, in.HookAttemptIdentity)
		if err != nil {
			return err
		}
		now, err := hookDBNow(tx)
		if err != nil {
			return err
		}
		out.Run = s.view
		// A committed HTTP outcome is immutable, including replay with different content.
		if audit.ResultCategory != nil && *audit.ResultCategory != string(entity.HookTimeoutUncertain) && op.Attempt == in.Token.Attempt && op.LeaseGeneration == in.Token.Generation {
			return nil
		}
		if !hookAttemptAllowed(expt, s, in.Phase) || !hookAttemptCurrent(op, in.HookAttemptIdentity) {
			return markHookAttemptLate(tx, audit, &out)
		}
		if op.Version != in.ExpectedVersion {
			return entity.ErrHookStoreConflict
		}
		change, err := entity.CompleteHookOperation(&s.view.State, entity.HookResultInput{Token: in.Token, Outcome: in.Outcome, Config: in.Config, CompletedAt: in.CompletedAt, CommitAt: now, RetryAfter: in.RetryAfter})
		if err != nil {
			return err
		}
		if change.Effects.LateIgnored {
			return markHookAttemptLate(tx, audit, &out)
		}
		fields := map[string]any{"result_redacted": in.ResultRedacted, "error_code": string(in.Outcome.Code), "error_message": in.ErrorMessage}
		if in.DisplayErrorCode != "" {
			fields["error_code"] = in.DisplayErrorCode
		}
		if in.Outcome.Code == entity.HookSucceeded {
			fields["error_code"], fields["error_message"] = nil, nil
		}
		if err := persistHookAttemptChange(tx, s, op, change, now, fields); err != nil {
			return err
		}
		if err := hookOneRow(tx.Model(&model.ExptLifecycleHookAttempt{}).Where("id=? AND operation_id=? AND attempt=? AND lease_generation=? AND delivery_id=? AND result_category IS NULL", audit.ID, audit.OperationID, audit.Attempt, audit.LeaseGeneration, audit.DeliveryID).UpdateColumns(map[string]any{"finished_at": in.CompletedAt, "http_status": in.Outcome.HTTPStatus, "result_category": string(in.Outcome.Code), "error_redacted": in.ErrorRedacted, "log_id": in.LogID})); err != nil {
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

func markHookAttemptLate(tx *gorm.DB, audit *model.ExptLifecycleHookAttempt, out *entity.HookAttemptStoreResult) error {
	out.Effects.LateIgnored = true
	if audit.LateIgnored {
		return nil
	}
	return hookOneRow(tx.Model(&model.ExptLifecycleHookAttempt{}).Where("id=? AND operation_id=? AND attempt=? AND lease_generation=? AND delivery_id=? AND late_ignored=false", audit.ID, audit.OperationID, audit.Attempt, audit.LeaseGeneration, audit.DeliveryID).UpdateColumn("late_ignored", true))
}

func persistHookAttemptChange(tx *gorm.DB, s *lockedHookRun, op *model.ExptLifecycleHookRun, change entity.HookStateChange, now time.Time, fields map[string]any) error {
	state := change.State.Before
	if op.Phase == string(entity.HookPhaseAfter) {
		state = change.State.After
	}
	fields["status"], fields["lease_owner"], fields["lease_until"] = string(state.Status), nil, nil
	fields["lease_generation"] = state.Generation
	fields["next_attempt_at"] = nil
	if change.Effects.Retry.Retry {
		fields["next_attempt_at"] = now.Add(change.Effects.Retry.Delay)
	}
	if err := updateHookOperation(tx, s.view.State.Key, op, fields); err != nil {
		return err
	}
	if op.Phase == string(entity.HookPhaseAfter) {
		return nil
	}
	gate := 0
	switch change.State.Gate {
	case entity.HookGateReady:
		gate = 1
	case entity.HookGateClosed:
		gate = 2
	}
	lifecycle := map[string]any{"gate": gate}
	if change.Effects.BeginFinalize {
		lifecycle["finalize_state"], lifecycle["terminal_status"], lifecycle["terminal_reason"] = 1, int32(change.State.Intent.Status), change.State.Intent.Reason
		lifecycle["terminal_at"], lifecycle["next_reconcile_at"] = now, now
	}
	return updateHookLifecycle(tx, s.view.State.Key, s.life.Version, now, lifecycle)
}
