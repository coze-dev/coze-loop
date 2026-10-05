// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IHookRetryTurnLogInitializer interface {
	InitializeHookRetryTurnRunLogs(context.Context, entity.HookRunKey, int64, int64, []*entity.ExptTurnResultRunLog) (*entity.HookExecutionManifest, []*entity.ExptTurnResultRunLog, error)
}
