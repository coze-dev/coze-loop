// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var _ hookcomponent.RequestBuilder = (*StorageCodec)(nil)

// BuildClaimedRequest consumes repository-returned Run/claim data, never current
// experiment configuration. The Worker must still enforce authority and lease ownership.
func (c *StorageCodec) BuildClaimedRequest(ctx context.Context, scope string, run *entity.HookStoredRun, claim *entity.HookAttemptClaim) (*spi.InvokeExperimentHookRequest, string, error) {
	if run == nil || claim == nil || claim.Token.Run != run.State.Key || entity.ValidateHookStorageState(&run.State) != nil {
		return nil, "", errHookStorageCodec
	}
	snapshot, err := c.DecodeSnapshot(ctx, run.State.Key, scope, run.Snapshot)
	if err != nil {
		return nil, "", err
	}
	in := snapshot.Input()
	modes := [...]string{"", "submit", "fail_retry", "append", "retry_all", "retry_items", "trial_run"}
	if run.CreatedBy != in.Context.Initiator.GetUserID() || run.Mode < 1 || int(run.Mode) >= len(modes) || modes[run.Mode] != in.Context.GetRunMode() {
		return nil, "", errHookStorageCodec
	}
	guard := entity.HookRenewAttemptInput{HookAttemptScope: entity.HookAttemptScope{Key: run.State.Key, OperationID: claim.Token.OperationID, Phase: claim.Token.Phase, ExpectedVersion: claim.Version, ExecutionScope: scope, SnapshotHash: run.Snapshot.Hash}, HookAttemptIdentity: claim.HookAttemptIdentity}
	if guard.Validate() != nil {
		return nil, "", errHookStorageCodec
	}
	operations := make(map[entity.HookPhase]entity.HookStoredOperation, 2)
	for _, op := range run.Operations {
		if _, exists := operations[op.Phase]; exists || (op.Phase != entity.HookPhaseBefore && op.Phase != entity.HookPhaseAfter) {
			return nil, "", errHookStorageCodec
		}
		operations[op.Phase] = op
	}
	for _, phase := range []entity.HookPhase{entity.HookPhaseBefore, entity.HookPhaseAfter} {
		config, state := in.Config.Before, run.State.Before
		if phase == entity.HookPhaseAfter {
			config, state = in.Config.After, run.State.After
		}
		enabled := config != nil && config.Enabled != nil && *config.Enabled
		op, exists := operations[phase]
		if exists != enabled || enabled != (state.Status != entity.HookOperationDisabled) || (exists && (op.OperationID != state.ID || op.ID <= 0)) {
			return nil, "", errHookStorageCodec
		}
	}
	op, exists := operations[claim.Token.Phase]
	state, config, event := run.State.Before, in.Config.Before, spi.HookEventTypeBefore
	if claim.Token.Phase == entity.HookPhaseAfter {
		state, config, event = run.State.After, in.Config.After, spi.HookEventTypeAfter
	}
	if !exists || op.OperationID != claim.Token.OperationID || op.IdempotencyKey != claim.IdempotencyKey || op.Version != claim.Version || state.Status != entity.HookOperationRunning || !state.Activated || state.Attempt != claim.Token.Attempt || state.Generation != claim.Token.Generation || op.OccurredAt == nil || op.ActivatedAt == nil || op.OccurredAt.IsZero() || !op.OccurredAt.Equal(*op.ActivatedAt) || !state.LeaseUntil.Equal(claim.LeaseUntil) || !state.AttemptDeadline.Equal(claim.AttemptDeadline) || !state.Deadline.Equal(claim.Deadline) {
		return nil, "", errHookStorageCodec
	}
	if claim.Token.Phase == entity.HookPhaseBefore {
		if !run.PlanReady || run.State.Gate != entity.HookGateWaiting || run.State.Finalize != entity.HookFinalizeNone || entity.IsExptFinished(run.State.Status) || run.State.Status == entity.ExptStatus_Terminating {
			return nil, "", errHookStorageCodec
		}
	} else {
		if run.State.Finalize != entity.HookFinalizeCommitted || run.TerminalAt == nil || run.TerminalAt.IsZero() {
			return nil, "", errHookStorageCodec
		}
		terminals := map[entity.ExptStatus]string{11: spi.HookTerminalStatusSuccess, 12: spi.HookTerminalStatusFailed, 13: spi.HookTerminalStatusTerminated, 14: spi.HookTerminalStatusSystemTerminated}
		terminal, ok := terminals[run.State.Intent.Status]
		if !ok {
			return nil, "", errHookStorageCodec
		}
		in.Context.TerminalStatus = &terminal
		if run.State.Intent.Reason != "" {
			in.Context.TerminalReason = gptr.Of(run.State.Intent.Reason)
		}
	}
	request := &spi.InvokeExperimentHookRequest{SchemaVersion: gptr.Of("1.0"), EventType: &event, OperationID: gptr.Of(op.OperationID), IdempotencyKey: gptr.Of(op.IdempotencyKey), DeliveryID: gptr.Of(claim.DeliveryID), Attempt: gptr.Of(claim.Token.Attempt), OccurredAt: gptr.Of(op.OccurredAt.UTC().Format(time.RFC3339Nano)), Context: in.Context, Parameters: config.ParametersJSON}
	hash, err := RequestBusinessHash(request)
	if err != nil {
		return nil, "", err
	}
	return request, hash, nil
}

// RequestBusinessHash covers only stable SPI business content; delivery, attempt,
// operation identifiers and transport signatures are not part of this digest.
func RequestBusinessHash(request *spi.InvokeExperimentHookRequest) (string, error) {
	body, err := BuildRequest(request)
	if err != nil {
		return "", err
	}
	var wire map[string]json.RawMessage
	if json.Unmarshal(body, &wire) != nil {
		return "", errHookStorageCodec
	}
	stable := make(map[string]json.RawMessage, 5)
	for _, field := range []string{"schema_version", "event_type", "context", "parameters", "occurred_at"} {
		stable[field] = wire[field]
	}
	encoded, err := json.Marshal(stable)
	if err != nil {
		return "", errHookStorageCodec
	}
	return hookContentHash(encoded), nil
}
