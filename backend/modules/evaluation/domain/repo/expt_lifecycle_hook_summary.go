// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookSummaryRepo accepts explicit, already-authorized Run keys. Ownership
// validation here does not replace API authorization. A nil summary means legacy;
// any unavailable/corrupt Run fails the batch without partial results.
type IHookSummaryRepo interface {
	MGetSummaries(context.Context, []entity.HookRunKey) (map[entity.HookRunKey]*entity.LifecycleHookRunSummary, error)
}
