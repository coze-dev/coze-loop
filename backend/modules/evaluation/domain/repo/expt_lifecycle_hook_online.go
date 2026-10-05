// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IHookOnlineRepo interface {
	PrepareOnlinePlan(context.Context, entity.HookRunKey, string) error
	AppendOnlinePage(context.Context, entity.HookRunKey, string, []entity.HookExecutionManifest, map[string]string) (bool, error)
	DrainOnlineRun(context.Context, entity.HookRunKey, string) error
}
