// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookTurnProgressRepo never inserts, deletes or updates another turn.
type IHookTurnProgressRepo interface {
	ReadTurnProgress(context.Context, entity.HookTurnProgressKey) (*entity.ExptTurnResultRunLog, error)
	WriteTurnProgress(context.Context, entity.HookTurnProgressInput) (*entity.ExptTurnResultRunLog, error)
}

// Optional result capabilities keep existing progress-only implementations compatible.
type IHookTurnResultWriteRepo interface {
	WriteTurnResult(context.Context, entity.HookTurnProgressInput) (*entity.ExptTurnResultRunLog, error)
}

type IHookItemRunWriteRepo interface {
	// Returns whether the original item is already Terminal.
	WriteItemRun(context.Context, entity.HookItemRunWriteInput) (bool, error)
}
