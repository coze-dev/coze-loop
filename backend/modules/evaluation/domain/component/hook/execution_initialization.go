// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ExecutionInitializer interface {
	// Resume from ordinal zero; committed manifests are reused without reloading their source.
	InitializeExecution(context.Context, entity.HookRunKey, string) (entity.HookExecutionInitializationCompletion, error)
}
