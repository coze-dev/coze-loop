// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Optional Hook-only persistence; legacy scheduler and DAO interfaces are unchanged.
type IHookSchedulerRepo interface {
	IHookFinalizationRepo
	PersistHookDispatch(context.Context, entity.HookSchedulerDispatchInput) ([]int64, error)
}
