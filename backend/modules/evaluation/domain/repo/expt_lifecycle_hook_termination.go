// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Optional private surfaces keep legacy manager/result/repository implementations compatible.
type IHookActiveTerminationRepo interface {
	ReadTerminationPage(context.Context, entity.HookPlanReadInput) (*entity.HookExecutionInitializationPage, error)
	PrepareTerminationItem(context.Context, entity.HookRunKey, string, int64) (*entity.HookTerminationItem, error)
	ArchiveHookItem(context.Context, entity.HookItemArchiveInput) ([]*entity.ExptTurnEvaluatorResultRef, error)
}

type IHookItemArchiveRepo interface {
	IHookFinalizationRepo
	ReadHookArchiveItem(context.Context, entity.HookRunKey, string, int64) (*entity.HookTerminationItem, error)
	ArchiveHookItem(context.Context, entity.HookItemArchiveInput) ([]*entity.ExptTurnEvaluatorResultRef, error)
}

type IHookTerminationRecordRepo interface {
	CloseHookExecutionRecords(context.Context, string, *entity.HookTerminationItem) error
}
