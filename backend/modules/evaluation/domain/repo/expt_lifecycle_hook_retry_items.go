// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IHookRetryItemsTailRepo interface {
	ReadRetryItemsSources(context.Context, entity.HookRunKey, string, string, []entity.HookPlanItem) (*entity.HookExecutionInitializationPage, error)
	AppendPreparedRetryItemsPage(context.Context, entity.HookRunKey, string, string, []entity.HookExecutionManifest) (bool, error)
	RecordRetryItemsSourceFailure(context.Context, entity.HookRunKey, string, string) (bool, error)
}
