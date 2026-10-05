// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookPlanRepo consumes the authorized RunKey and trusted deployment scope.
// It does not change the legacy append/freeze or experiment repository contracts.
type IHookPlanRepo interface {
	ReadPlanPage(context.Context, entity.HookPlanReadInput) (*entity.HookPlanReadPage, error)
	AdvancePlanCursor(context.Context, entity.HookAdvancePlanInput) (entity.HookStoreResult, error)
	MGetPlanItems(context.Context, entity.HookPlanLookupInput) (*entity.HookPlanLookupResult, error)
}
