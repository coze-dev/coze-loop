// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Source I/O gets less than the parent budget so retry accounting can still commit.
func (p *hookPlanPreparer) selectPlanPage(ctx context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
	budget := 3 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return entity.HookSelectionPage{}, context.DeadlineExceeded
		}
		budget = min(budget, remaining/2)
	}
	sourceCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	deadline, _ := sourceCtx.Deadline()
	page, err := p.deps.Selector.SelectPage(sourceCtx, in)
	completedAt := time.Now()
	// Even a late nil or deterministic error must not outrun the source completion window.
	if !completedAt.Before(deadline) {
		return entity.HookSelectionPage{}, context.DeadlineExceeded
	}
	return page, err
}
