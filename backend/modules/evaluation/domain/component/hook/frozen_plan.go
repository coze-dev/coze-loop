// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type PlanPageLoader interface {
	LoadPage(context.Context, entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error)
}

// FrozenContentResolver may only complete content belonging to the supplied frozen tuple.
type FrozenContentResolver interface {
	Resolve(context.Context, entity.HookPlanItem, int64, string, *entity.Content) (*entity.Content, error)
}
