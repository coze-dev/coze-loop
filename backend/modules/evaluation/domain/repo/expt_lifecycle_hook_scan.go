// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookScanRepo only discovers candidates. Consumers must recheck ownership,
// activation, deadlines and fencing when claiming; a candidate is not permission to execute.
// Rows may move behind a cursor concurrently; start a new sweep after reaching the tail.
type IHookScanRepo interface {
	ScanDueOperations(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error)
	ScanExpiredOperations(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error)
	ScanPreparingPlans(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error)
	ScanPendingFinalizations(context.Context, entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error)
}
