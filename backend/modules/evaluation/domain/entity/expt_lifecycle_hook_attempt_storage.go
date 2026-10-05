// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"time"
	"unicode/utf8"
)

// Config and SnapshotHash are supplied together by a trusted snapshot decoder.
// The repository checks the stored hash; it cannot authenticate the decoded policy.
type HookAttemptScope struct {
	Key                          HookRunKey
	OperationID                  string
	Phase                        HookPhase
	ExpectedVersion              int64
	ExecutionScope, SnapshotHash string
}

type HookAttemptIdentity struct {
	Token             HookClaimToken
	Owner, DeliveryID string
}

type HookClaimAttemptInput struct {
	HookAttemptScope
	Owner     string
	AttemptID int64
	Config    *HookConfig
	// Nil preserves the direct repository caller's original 30-second lease.
	LeaseSeconds *int32
}

type HookRenewAttemptInput struct {
	HookAttemptScope
	HookAttemptIdentity
	LeaseSeconds *int32
}

type HookCompleteAttemptInput struct {
	HookRenewAttemptInput
	Config      *HookConfig
	Outcome     HookOutcome
	CompletedAt time.Time
	// When present, CompletedAt is the conservative upper endpoint, not an exact
	// wall-clock measurement. The executor retains both endpoints across CAS retries.
	CompletionWindow              *HookCompletionWindow `json:"-"`
	RetryAfter                    string
	ResultRedacted, ErrorRedacted []byte
	ErrorMessage, LogID           string
	// DisplayErrorCode never controls retry or the attempt's audit category.
	DisplayErrorCode string
}

type HookRecoverExpiredAttemptInput struct {
	HookAttemptScope
	Config *HookConfig
}

type HookAttemptClaim struct {
	HookAttemptIdentity
	Version                                          int64
	IdempotencyKey                                   string
	StartedAt, LeaseUntil, AttemptDeadline, Deadline time.Time
}

type HookAttemptStoreResult struct {
	HookStoreResult
	Claim *HookAttemptClaim
	Clock *HookClockAnchor `json:"-"`
}

func (in HookAttemptScope) Validate() error {
	if err := (HookStoreGuard{Key: in.Key, ExpectedVersion: in.ExpectedVersion}).Validate(); err != nil {
		return err
	}
	if !hookStorageASCII(in.OperationID, 128) || (in.Phase != HookPhaseBefore && in.Phase != HookPhaseAfter) || !hookStorageASCII(in.ExecutionScope, 128) || !hookStorageHash(in.SnapshotHash) {
		return invalidParam("invalid hook attempt scope")
	}
	return nil
}

func (in HookClaimAttemptInput) Validate() error {
	if err := in.HookAttemptScope.Validate(); err != nil {
		return err
	}
	if in.AttemptID <= 0 || !hookStorageASCII(in.Owner, 128) {
		return invalidParam("invalid hook attempt identity")
	}
	if in.LeaseSeconds != nil && *in.LeaseSeconds <= 0 {
		return invalidParam("invalid hook attempt lease")
	}
	return validateHookAttemptConfig(in.Config, in.Phase)
}

func validateHookAttemptConfig(config *HookConfig, phase HookPhase) error {
	normalized, err := normalizeHookConfig(config, phase == HookPhaseBefore)
	if err != nil {
		return err
	}
	if normalized == nil || !*normalized.Enabled {
		return invalidParam("hook attempt requires an enabled frozen config")
	}
	return nil
}

func (in HookRenewAttemptInput) Validate() error {
	if err := in.HookAttemptScope.Validate(); err != nil {
		return err
	}
	if in.LeaseSeconds != nil && *in.LeaseSeconds <= 0 {
		return invalidParam("invalid hook attempt lease")
	}
	token := in.Token
	if token.Run != in.Key || token.OperationID != in.OperationID || token.Phase != in.Phase || token.Attempt < 1 || token.Attempt > 11 || token.Generation <= 0 || !hookStorageASCII(in.Owner, 128) || !hookStorageASCII(in.DeliveryID, 128) {
		return invalidParam("invalid hook attempt token or owner")
	}
	return nil
}

func (in HookCompleteAttemptInput) Validate() error {
	if err := in.HookRenewAttemptInput.Validate(); err != nil {
		return err
	}
	if err := validateHookAttemptConfig(in.Config, in.Phase); err != nil {
		return err
	}
	if in.CompletedAt.IsZero() || in.Outcome.Code == HookTimeoutUncertain {
		return invalidParam("HTTP completion requires a completion time; recovery is internal")
	}
	if w := in.CompletionWindow; w != nil && (w.Earliest.IsZero() || w.Earliest.After(w.Latest) || !in.CompletedAt.Equal(w.Latest)) {
		return invalidParam("hook completion must retain its original conservative time window")
	}
	if _, err := hookStateOutcome(in.Outcome); err != nil {
		return err
	}
	if in.Outcome.HTTPStatus < 0 || in.Outcome.HTTPStatus > 599 || len(in.ResultRedacted) > 32768 || len(in.ErrorRedacted) > 32768 || !utf8.ValidString(in.ErrorMessage) || len(in.ErrorMessage) > 2048 || (in.LogID != "" && !hookStorageASCII(in.LogID, 128)) || len(in.RetryAfter) > 128 {
		return invalidParam("invalid hook display completion")
	}
	if in.DisplayErrorCode != "" && (in.Outcome.Code != HookFailed || in.Outcome.HTTPStatus != 200 || len(in.ResultRedacted) != 0 || len(in.DisplayErrorCode) > 128 || !utf8.ValidString(in.DisplayErrorCode) || strings.TrimSpace(in.DisplayErrorCode) == "" || strings.TrimSpace(in.ErrorMessage) == "") {
		return invalidParam("invalid hook business error display")
	}
	return nil
}

func (in HookRecoverExpiredAttemptInput) Validate() error {
	if err := in.HookAttemptScope.Validate(); err != nil {
		return err
	}
	return validateHookAttemptConfig(in.Config, in.Phase)
}
