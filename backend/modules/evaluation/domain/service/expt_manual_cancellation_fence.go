// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type manualCancellationResultBinder interface {
	WithManualCancellationRepositories(repo.ManualCancellationRepositories) ExptResultService
}

func (e *ExptResultServiceImpl) WithManualCancellationRepositories(r repo.ManualCancellationRepositories) ExptResultService {
	copy := *e
	copy.ExperimentRepo, copy.ExptItemResultRepo, copy.ExptTurnResultRepo, copy.ExptStatsRepo = r.Experiments, r.Items, r.Turns, r.Stats
	copy.hookArchive = nil // The fence accepts only marker NULL/0 Runs.
	return &copy
}

func (e *ExptMangerImpl) withManualCancellationFence(ctx context.Context, key entity.HookRunKey, fn func(context.Context, *ExptMangerImpl) error) error {
	if e.finalization == nil {
		return entity.ErrHookFinalizationUnsupported
	}
	fence, ok := e.finalization.Repository.(repo.IManualCancellationFenceRepo)
	if !ok {
		return entity.ErrHookFinalizationUnsupported
	}
	binder, ok := e.exptResultService.(manualCancellationResultBinder)
	if !ok {
		return entity.ErrHookFinalizationUnsupported
	}
	return fence.WithManualCancellationFence(ctx, key, func(ctx context.Context, r repo.ManualCancellationRepositories) error {
		for _, dep := range []any{r.Source, r.Experiments, r.Runs, r.Items, r.Turns, r.Stats} {
			if missingManagerHookDependency(dep) {
				return entity.ErrHookStoreCorrupt
			}
		}
		copy, deps := *e, *e.finalization
		deps.Repository = r.Source
		copy.finalization = &deps
		copy.exptRepo, copy.runLogRepo, copy.itemResultRepo, copy.turnResultRepo, copy.statsRepo = r.Experiments, r.Runs, r.Items, r.Turns, r.Stats
		copy.exptResultService = binder.WithManualCancellationRepositories(r)
		if templates, ok := e.templateManager.(*ExptTemplateManagerImpl); ok {
			if missingManagerHookDependency(r.Templates) {
				return entity.ErrHookStoreCorrupt
			}
			bound := *templates
			bound.templateRepo, bound.exptRepo = r.Templates, r.Experiments
			copy.templateManager = &bound
		}
		return fn(ctx, &copy)
	})
}
