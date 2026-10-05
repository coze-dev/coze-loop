// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IHookExecutionInitializationRepo interface {
	ReadExecutionInitializationPage(context.Context, entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error)
	WriteExecutionInitializationPage(context.Context, entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error)
	CompleteExecutionInitialization(context.Context, entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error)
}
