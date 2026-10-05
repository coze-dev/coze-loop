// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Scheduler failure handling is separate from dispatch and cancellation capabilities.
type IHookSchedulerFailureRepo interface {
	ReadHookSchedulerFailureItem(context.Context, entity.HookRunKey, string, int64) (*entity.HookTerminationItem, error)
	ApplyHookSchedulerFailure(context.Context, entity.HookSchedulerFailureInput) (bool, error)
}
