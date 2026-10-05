// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"time"
)

type retryItemsCandidateLoader interface {
	LoadCandidates(context.Context, entity.HookPlanReadInput, *entity.HookPlanReadPage) (*entity.HookLoadedPlanPage, error)
}

// PrepareRetryItemsTail advances at most one accepted page on an already bound Run.
func (s *ExptSchedulerImpl) PrepareRetryItemsTail(ctx context.Context, key entity.HookRunKey) (bool, error) {
	init := s.hookBoundInitializer
	if ctx == nil || init == nil || s.hookBoundMode != entity.EvaluationModeRetryItems || init.boundKey != key {
		return false, entity.ErrHookExecutionUnsupported
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	run, err := s.hookRuns.GetRun(ctx, key)
	if err != nil {
		return false, err
	}
	if run.State.Gate != entity.HookGateReady || run.State.Finalize != entity.HookFinalizeNone || !run.PlanReady {
		return false, nil
	}
	var version struct {
		Version int `json:"v"`
	}
	if json.Unmarshal([]byte(run.PlanCursor), &version) != nil {
		return false, entity.ErrHookStoreCorrupt
	}
	if version.Version == 1 {
		return false, nil
	}
	source, err := s.hookScheduler.ReadFinalizationSource(ctx, key)
	if err != nil {
		return false, err
	}
	if source == nil || source.RunLog == nil || source.Experiment == nil || !source.Managed || source.Key != key {
		return false, entity.ErrHookStoreCorrupt
	}
	cursor, err := entity.DecodeHookRetryItemsCursor(run.PlanCursor, key, source.RunLog.ItemIds, run.PlanCount, run.PlanHash)
	if err != nil {
		return false, err
	}
	ids, pending := cursor.Page(source.RunLog.ItemIds)
	if !pending {
		return false, nil
	}
	writer, ok := init.deps.Repository.(repo.IHookRetryItemsTailRepo)
	if !ok {
		return false, entity.ErrHookExecutionUnsupported
	}
	factory, ok := s.schedulerModeFactory.(*DefaultSchedulerModeFactory)
	if !ok {
		return false, entity.ErrHookExecutionUnsupported
	}
	selector := NewHookPlanSelector(factory.evaluationSetItemService, factory.exptItemRefRepo, factory.exptTurnResultRepo, factory.exptItemResultRepo)
	preparer := &hookPlanPreparer{deps: HookPlanPreparerDependencies{Selector: selector}}
	selection, err := preparer.selectPlanPage(ctx, entity.HookSelectionInput{Key: key, Mode: entity.EvaluationModeRetryItems, Experiment: source.Experiment, ItemIDs: ids})
	if err != nil {
		return false, s.retryItemsSourceError(ctx, run, writer, err)
	}
	if !selection.Done || len(selection.Items) > 100 {
		return false, entity.ErrHookStoreCorrupt
	}
	var allocated []int64
	if len(selection.Items) > 0 {
		allocated, err = init.deps.IDs.GenMultiIDs(ctx, len(selection.Items))
		if err != nil {
			return false, err
		}
	}
	if len(allocated) != len(selection.Items) {
		return false, entity.ErrHookStoreCorrupt
	}
	for i := range selection.Items {
		selection.Items[i].ID = allocated[i]
	}
	page, err := writer.ReadRetryItemsSources(ctx, key, init.boundScope, run.PlanCursor, selection.Items)
	if err != nil {
		return false, err
	}
	var manifests []entity.HookExecutionManifest
	if len(page.Items) > 0 {
		loader, ok := init.deps.Loader.(retryItemsCandidateLoader)
		if !ok {
			return false, entity.ErrHookExecutionUnsupported
		}
		input := entity.HookPlanReadInput{Key: key, ExecutionScope: init.boundScope, StartOrdinal: run.PlanCount, Limit: int32(len(page.Items))}
		budget := 3 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			budget = min(budget, time.Until(deadline)/2)
		}
		loadCtx, cancel := context.WithTimeout(ctx, budget)
		loaded, err := loader.LoadCandidates(loadCtx, input, &entity.HookPlanReadPage{Items: selection.Items, Count: page.Count, Hash: page.Hash, RunVersion: page.RunVersion, Ready: true, NextOrdinal: page.Count})
		if loadCtx.Err() != nil {
			err = loadCtx.Err()
		}
		cancel()
		if err != nil {
			return false, s.retryItemsSourceError(ctx, run, writer, err)
		}
		if loaded == nil || len(loaded.Items) != len(page.Items) || loaded.Count != page.Count || loaded.Hash != page.Hash || loaded.NextOrdinal != page.Count {
			return false, entity.ErrHookStoreCorrupt
		}
		for i, item := range loaded.Items {
			m, err := executionManifestFromLoaded(key, page.Items[i], item)
			if err != nil {
				return false, err
			}
			manifests = append(manifests, m)
		}
		needed := 0
		for _, m := range manifests {
			if m.ItemResultID == 0 {
				needed++
			}
			if m.ItemRunLogID == 0 {
				needed++
			}
			if m.ItemRef != nil && m.ItemRef.ID == 0 {
				needed++
			}
			for _, tr := range m.Turns {
				if tr.ResultID == 0 {
					needed++
				}
			}
		}
		allocated, err = init.deps.IDs.GenMultiIDs(ctx, needed)
		if err != nil {
			return false, err
		}
		if len(allocated) != needed {
			return false, entity.ErrHookStoreCorrupt
		}
		seen, err := executionPageRecordIDs(page)
		if err != nil {
			return false, err
		}
		for _, id := range allocated {
			if id <= 0 || seen[id] {
				return false, entity.ErrHookStoreCorrupt
			}
			seen[id] = true
		}
		next := 0
		for i := range manifests {
			m := &manifests[i]
			if m.ItemResultID == 0 {
				m.ItemResultID = allocated[next]
				next++
			}
			if m.ItemRunLogID == 0 {
				m.ItemRunLogID = allocated[next]
				next++
			}
			if m.ItemRef != nil && m.ItemRef.ID == 0 {
				m.ItemRef.ID = allocated[next]
				next++
			}
			for j := range m.Turns {
				if m.Turns[j].ResultID == 0 {
					m.Turns[j].ResultID = allocated[next]
					next++
				}
			}
		}
	}
	changed, err := writer.AppendPreparedRetryItemsPage(ctx, key, init.boundScope, run.PlanCursor, manifests)
	if err == nil && changed && len(ids) > 0 {
		_ = s.ResultSvc.UpsertExptTurnResultFilter(ctx, key.WorkspaceID, key.ExperimentID, ids)
	}
	return changed, err
}

func (s *ExptSchedulerImpl) retryItemsSourceError(ctx context.Context, run *entity.HookStoredRun, writer repo.IHookRetryItemsTailRepo, cause error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(cause, entity.ErrHookStoreConflict) || errors.Is(cause, entity.ErrHookExecutionStorage) || errors.Is(cause, entity.ErrHookPlanStorage) {
		return cause
	}
	reason := "HOOK_PLAN_SOURCE_INVALID"
	if !errors.Is(cause, errSelectionCursor) && !errors.Is(cause, errSelectionSource) && !permanentHookInitializationError(cause) {
		exhausted, err := writer.RecordRetryItemsSourceFailure(ctx, run.State.Key, s.hookSchedulerScope, run.PlanCursor)
		if err != nil {
			return err
		}
		if !exhausted {
			return ErrHookPlanSourceRetry
		}
		reason = "HOOK_PLAN_SOURCE_EXHAUSTED"
	}
	_, err := s.hookRuns.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: run.State.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_SystemTerminated, Reason: reason}})
	if err != nil {
		return err
	}
	return ErrHookPlanPreparationFailed
}
