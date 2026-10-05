// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookRepo is independent of the legacy experiment repository and transport.
type IHookRepo interface {
	CreateRunWithHooks(context.Context, entity.HookCreateRunInput) (entity.HookStoreResult, error)
	AppendPlanPage(context.Context, entity.HookPlanPageInput) (entity.HookStoreResult, error)
	FinishPlan(context.Context, entity.HookFinishPlanInput) (entity.HookStoreResult, error)
	GetRun(context.Context, entity.HookRunKey) (*entity.HookStoredRun, error)
	BeginFinalize(context.Context, entity.HookFinalizeInput) (entity.HookStoreResult, error)
	CommitFinalize(context.Context, entity.HookFinalizeInput) (entity.HookStoreResult, error)
	AdmitItem(context.Context, entity.HookAdmitItemInput) (entity.HookAdmitItemResult, error)
	ClaimAttempt(context.Context, entity.HookClaimAttemptInput) (entity.HookAttemptStoreResult, error)
	RenewAttempt(context.Context, entity.HookRenewAttemptInput) (entity.HookAttemptStoreResult, error)
	CompleteAttempt(context.Context, entity.HookCompleteAttemptInput) (entity.HookAttemptStoreResult, error)
	RecoverExpiredAttempt(context.Context, entity.HookRecoverExpiredAttemptInput) (entity.HookAttemptStoreResult, error)
}
