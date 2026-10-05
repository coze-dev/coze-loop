// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Optional Hook startup capability. A true result proves run eligibility even when
// err reports an optional template projection failure; false with err fails closed.
type IHookSchedulerStartupProjectionRepo interface {
	ProjectHookSchedulerStartup(context.Context, entity.HookRunKey, string) (bool, error)
}
