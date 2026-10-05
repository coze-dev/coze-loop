// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// RunID zero resolves Latest once in a primary read-only snapshot.
type IHookFinalizationRepo interface {
	ReadFinalizationSource(context.Context, entity.HookRunKey) (*entity.HookFinalizationSource, error)
	ReadFinalizationStats(context.Context, entity.HookRunKey, string) (*entity.HookFinalizationStats, error)
}
type IHookFinalizationOwnerReader interface {
	ReadFinalizationOwner(context.Context, string) (string, error)
}

// Optional cleanup of initialization scaffolding; never admits or completes initialization.
type IHookPartialInitializationFinalizer interface {
	PreparePartialInitializationTermination(context.Context, entity.HookRunKey, string) (bool, error)
}

// Acceptance persists intent and the conditional Terminating projection atomically.
type IHookTerminationRepo interface {
	AcceptTermination(context.Context, entity.HookFinalizeInput) (entity.HookStoreResult, error)
}
