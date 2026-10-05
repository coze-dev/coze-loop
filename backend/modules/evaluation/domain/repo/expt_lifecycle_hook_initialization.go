// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Separate from the legacy repository contracts and their generated mocks.
type IHookRunInitializationRepo interface {
	ReadRunInitialization(context.Context, entity.HookRunKey) (*entity.HookRunInitialization, error)
	CreateRunWithoutHooks(context.Context, *entity.ExptRunLog, int64, string) (bool, error)
	AppendHookRunItems(context.Context, entity.HookAppendRunItemsInput) error
}
