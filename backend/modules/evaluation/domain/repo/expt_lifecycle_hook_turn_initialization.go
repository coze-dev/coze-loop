// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// A nil candidate list probes authoritative frozen initialization; handled=false means a non-frozen Run.
type IHookTurnLogInitializer interface {
	InitializeHookTurnRunLogs(context.Context, entity.HookRunKey, int64, int64, []*entity.ExptTurnResultRunLog) (handled bool, committed []*entity.ExptTurnResultRunLog, err error)
}
