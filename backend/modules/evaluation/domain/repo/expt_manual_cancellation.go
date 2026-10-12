// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ManualCancellationRepositories share the transaction that fences experiment.LatestRunID.
type ManualCancellationRepositories struct {
	Source      IHookFinalizationRepo
	Experiments IExperimentRepo
	Runs        IExptRunLogRepo
	Items       IExptItemResultRepo
	Turns       IExptTurnResultRepo
	Stats       IExptStatsRepo
	Templates   IExptTemplateRepo
}

type IManualCancellationFenceRepo interface {
	WithManualCancellationFence(context.Context, entity.HookRunKey, func(context.Context, ManualCancellationRepositories) error) error
}
