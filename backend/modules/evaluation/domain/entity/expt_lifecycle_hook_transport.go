// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

// HookTransportInput comes from the verified claimed snapshot. Remaining is the
// budget at invocation, converted by the caller from its current DB clock anchor.
type HookTransportInput struct {
	WorkspaceID int64
	Config      *HookConfig                      `json:"-"`
	Request     *spi.InvokeExperimentHookRequest `json:"-"`
	Remaining   time.Duration
}

type HookKeyBinding struct {
	WorkspaceID int64
	URL         string `json:"-"`
	Environment HookEnvironment
	Lane        string
}

type HookSigningKey struct {
	KeyID  string
	Secret []byte `json:"-"`
	// Binds this key to the normalized egress policy from the same authorization read.
	PolicyFingerprint string `json:"-"`
}

type HookTransportResult struct {
	Outcome HookOutcome
	// Response is untrusted business data; the Worker must redact before persistence.
	Response   *spi.InvokeExperimentHookResponse `json:"-"`
	RetryAfter string
	// Preserve monotonic time in-process. This is NOT DB CompletedAt: the Worker
	// must map it using its same-source DB/local anchor, including late/cancel fencing.
	LocalCompletedAt time.Time `json:"-"`
}
