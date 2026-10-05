// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type PlanSelector interface {
	SelectPage(context.Context, entity.HookSelectionInput) (entity.HookSelectionPage, error)
}
