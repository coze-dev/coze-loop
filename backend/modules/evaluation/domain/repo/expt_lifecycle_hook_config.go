// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	hookcomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookConfigRepo consumes an authorized owner; callers still enforce resource permissions.
type IHookConfigRepo interface {
	GetConfig(context.Context, hookcomponent.ConfigOwner) (*entity.HookConfigRecord, error)
	// UpdateConfig replaces explicit stages only. Nil/empty configuration is a no-op.
	UpdateConfig(context.Context, hookcomponent.ConfigOwner, entity.HookConfigUpdateInput) (bool, error)
}
