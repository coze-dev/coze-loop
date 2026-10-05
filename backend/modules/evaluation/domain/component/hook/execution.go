// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RequestBuilder is separate to preserve the existing StorageCodec contract.
type RequestBuilder interface {
	BuildClaimedRequest(context.Context, string, *entity.HookStoredRun, *entity.HookAttemptClaim) (*spi.InvokeExperimentHookRequest, string, error)
}

type AttemptReader interface {
	// Observes current ownership, ignoring ExpectedVersion as a read filter.
	// The returned Claim is not a new delivery grant and spends no attempt budget.
	ReadAttempt(context.Context, entity.HookAttemptScope) (entity.HookAttemptStoreResult, error)
}

// CompletionProjector is required and explicitly selects a display-content policy.
// Business services own display safety; raw diagnostics and platform secrets are not display fields.
type CompletionProjector interface {
	Project(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (map[string]string, error)
	ProjectError(context.Context, entity.HookRunKey, entity.HookPhase, *spi.InvokeExperimentHookResponse) (*spi.HookError, error)
}

type AttemptExecutionInput struct {
	Key                         entity.HookRunKey
	ExecutionScope, OperationID string
	Phase                       entity.HookPhase
	Owner                       string
	// A fresh ID from the caller's normal ID allocator; unused if no claim wins.
	AttemptID int64
}

type AttemptExecutionResult struct {
	entity.HookAttemptStoreResult
	RecoveryPending  bool
	CompletionWindow *entity.HookCompletionWindow
}
