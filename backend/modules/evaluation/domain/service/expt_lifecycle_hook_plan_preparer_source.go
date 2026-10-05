// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func r07Source(run *entity.HookStoredRun) bool {
	return run.PlanReady && !run.ExecutionStarted && run.State.Gate == entity.HookGateClosed && run.State.Intent.Status == entity.ExptStatus_Terminated && run.State.Intent.Reason == "HOOK_BEFORE_FAILED"
}

func (p *hookPlanPreparer) sourceRun(ctx context.Context, run *entity.HookStoredRun, id int64) (*entity.HookStoredRun, error) {
	key := run.State.Key
	key.RunID = id
	if id <= 0 || id == run.State.Key.RunID {
		return nil, entity.ErrHookStoreConflict
	}
	source, err := p.deps.Runs.GetRun(ctx, key)
	if err != nil {
		return nil, err
	}
	if source == nil || source.State.Key != key || source.Snapshot.ExecutionScope != run.Snapshot.ExecutionScope {
		return nil, entity.ErrHookStoreConflict
	}
	if entity.ValidateHookStorageState(&source.State) != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	return source, nil
}

func (p *hookPlanPreparer) noBaseResults(ctx context.Context, key entity.HookRunKey, scope string) (bool, error) {
	found, err := p.deps.ResultReader.HasExperimentResults(ctx, key, scope)
	return !found && err == nil, err
}

func (p *hookPlanPreparer) checkCopySource(ctx context.Context, run *entity.HookStoredRun, c preparationCursor) error {
	source, err := p.sourceRun(ctx, run, c.SourceID)
	if err != nil {
		return preparationStorageError(err)
	}
	if !r07Source(source) || source.PlanCount != c.SourceCount || source.PlanHash != c.SourceHash {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_CHANGED")
	}
	empty, err := p.noBaseResults(ctx, run.State.Key, run.Snapshot.ExecutionScope)
	if err != nil {
		return preparationStorageError(err)
	}
	if !empty {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_CHANGED")
	}
	return nil
}

func (p *hookPlanPreparer) copySource(ctx context.Context, run *entity.HookStoredRun, c preparationCursor) error {
	if err := p.checkCopySource(ctx, run, c); err != nil {
		return err
	}
	source, err := p.sourceRun(ctx, run, c.SourceID)
	if err != nil {
		return preparationStorageError(err)
	}
	page, err := p.deps.Plans.ReadPlanPage(ctx, entity.HookPlanReadInput{Key: source.State.Key, ExecutionScope: run.Snapshot.ExecutionScope, StartOrdinal: c.SourceDigest.Count, Limit: 100})
	if err != nil {
		return preparationStorageError(err)
	}
	if page == nil || page.RunVersion != source.Version {
		return entity.ErrHookStoreConflict
	}
	if !page.Ready || page.Count != c.SourceCount || page.Hash != c.SourceHash || !validPreparationPage(page, c.SourceDigest.Count, source.State.Key) {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_CHANGED")
	}
	c.SourceDigest, err = entity.AppendHookPlanDigest(c.SourceDigest, page.Items)
	if err != nil {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_CHANGED")
	}
	if !page.HasMore {
		if c.SourceDigest.Count != c.SourceCount || c.SourceDigest.Hash != c.SourceHash {
			return p.fail(ctx, run, "HOOK_PLAN_SOURCE_CHANGED")
		}
		c.Phase = "verify"
	}
	items := make([]entity.HookPlanItem, len(page.Items))
	copy(items, page.Items)
	for i := range items {
		items[i].ID = 0
	}
	return p.persist(ctx, run, c, items)
}
