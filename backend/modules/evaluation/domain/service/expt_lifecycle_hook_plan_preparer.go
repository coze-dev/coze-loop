// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

type HookPlanPreparerDependencies struct {
	Runs           repo.IHookRepo
	Plans          repo.IHookPlanRepo
	Selector       hook.PlanSelector
	Codec          hook.StorageCodec
	Experiments    repo.IExperimentRepo
	Initialization repo.IHookRunInitializationRepo
	ResultReader   repo.IHookPlanResultReader
	IDs            idgen.IIDGenerator
}

type hookPlanPreparer struct{ deps HookPlanPreparerDependencies }

var (
	ErrHookPlanPreparationFailed = errors.New("hook plan preparation failed")
	ErrHookPlanSourceRetry       = errors.New("hook plan source retry pending")
)

func NewHookPlanPreparer(d HookPlanPreparerDependencies) (hook.PlanPreparer, error) {
	for _, dep := range []any{d.Runs, d.Plans, d.Selector, d.Codec, d.Experiments, d.Initialization, d.ResultReader, d.IDs} {
		if missingManagerHookDependency(dep) {
			return nil, errors.New("missing hook plan preparer dependency")
		}
	}
	return &hookPlanPreparer{deps: d}, nil
}

func (p *hookPlanPreparer) PreparePlan(ctx context.Context, in hook.WorkerRunInput) error {
	if ctx == nil {
		return entity.ErrHookStoreConflict
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	key := in.Candidate.Key
	if (entity.HookPlanReadInput{Key: key, ExecutionScope: in.ExecutionScope, Limit: 1}).Validate() != nil {
		return entity.ErrHookStoreConflict
	}
	run, err := p.deps.Runs.GetRun(ctx, key)
	if err != nil {
		return preparationStorageError(err)
	}
	if run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != in.ExecutionScope || run.Version < 0 {
		return entity.ErrHookStoreConflict
	}
	if entity.ValidateHookStorageState(&run.State) != nil {
		return entity.ErrHookStoreCorrupt
	}
	if run.PlanReady || run.State.Gate == entity.HookGateClosed || run.State.Finalize != entity.HookFinalizeNone || entity.IsExptFinished(run.State.Status) || run.State.Status == entity.ExptStatus_Terminating {
		return nil
	}
	initial, err := p.deps.Initialization.ReadRunInitialization(ctx, key)
	if err != nil {
		return preparationStorageError(err)
	}
	if initial == nil || !initial.Managed || initial.RunLog == nil {
		return entity.ErrHookStoreCorrupt
	}
	if initial.LatestRunID != key.RunID {
		return entity.ErrHookStoreConflict
	}
	log := initial.RunLog
	expt, err := p.deps.Experiments.GetByID(ctx, key.ExperimentID, key.WorkspaceID)
	if err != nil {
		return preparationStorageError(err)
	}
	if log == nil || expt == nil || log.ID != key.RunID || log.ExptRunID != key.RunID || log.SpaceID != key.WorkspaceID || log.ExptID != key.ExperimentID || expt.ID != key.ExperimentID || expt.SpaceID != key.WorkspaceID || expt.LatestRunID != key.RunID || entity.ExptRunMode(log.Mode) != run.Mode || log.CreatedBy != run.CreatedBy {
		return entity.ErrHookStoreConflict
	}
	if entity.IsExptFinished(entity.ExptStatus(log.Status)) || entity.ExptStatus(log.Status) == entity.ExptStatus_Terminating || entity.IsExptFinished(expt.Status) || expt.Status == entity.ExptStatus_Terminating {
		return nil
	}
	boundCandidate := expt.ExptType == entity.ExptType_Offline && entity.HookBoundExecutionMode(run.Mode) && (expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig || entity.HookBoundRetryMode(run.Mode))
	if run.State.Before.Status == entity.HookOperationDisabled && !entity.HookExecutionInitializationRequired(run.State.After.Status != entity.HookOperationDisabled, run.Mode, expt.ExptType, expt.EvalSetSourceType) && !boundCandidate {
		return nil
	}
	snapshot, err := p.deps.Codec.DecodeSnapshot(ctx, key, in.ExecutionScope, run.Snapshot)
	if err != nil {
		return preparationStorageError(err)
	}
	if snapshot == nil {
		return p.fail(ctx, run, "HOOK_PLAN_SEED_INVALID")
	}
	snap := snapshot.Input()
	modes := [...]string{"", "submit", "fail_retry", "append", "retry_all", "retry_items", "trial_run"}
	if snap.Context == nil || snap.Context.Initiator == nil || snap.Config == nil || run.Mode < 1 || int(run.Mode) >= len(modes) || snap.Context.GetRunMode() != modes[run.Mode] || snap.Context.Initiator.GetUserID() != run.CreatedBy || (!managerHookEnabled(snap.Config.Before) && !managerHookEnabled(snap.Config.After)) {
		return p.fail(ctx, run, "HOOK_PLAN_SNAPSHOT_INVALID")
	}
	seed := snap.Selection
	if seed == nil || seed.Validate() != nil {
		return p.fail(ctx, run, "HOOK_PLAN_SEED_INVALID")
	}
	if boundCandidate && snap.Execution == nil && run.State.Before.Status == entity.HookOperationDisabled {
		return nil
	}
	if boundCandidate && snap.Execution != nil {
		if entity.HookBoundRetryMode(run.Mode) && (run.SourceRunID == nil || *run.SourceRunID <= 0) {
			return entity.ErrHookStoreConflict
		}
		binding, err := entity.NewHookExecutionInitializationBinding(run, snapshot)
		if err != nil {
			return p.fail(ctx, run, "HOOK_PLAN_SNAPSHOT_INVALID")
		}
		frozen := binding.Input().Execution
		owned := *expt
		owned.TrialRunItemCount = seed.TrialRunItemCount
		owned.EvalConf = &entity.EvaluationConfiguration{}
		for _, set := range frozen.Sets {
			if frozen.SingleSet {
				owned.EvalSetID, owned.EvalSetVersionID, owned.EvalSetSpaceID = set.EvalSetID, set.EvalSetVersionID, set.SourceSpaceID
				continue
			}
			owned.EvalConf.EvalSetConfigs = append(owned.EvalConf.EvalSetConfigs, &entity.EvalSetConfig{EvalSetID: set.EvalSetID, EvalSetVersionID: set.EvalSetVersionID, SourceSpaceID: set.SourceSpaceID, ItemFilter: set.ItemFilter})
		}
		expt = &owned
	}
	fingerprint, err := entity.HookSelectionConfigFingerprint(expt)
	if err != nil || fingerprint != seed.ConfigFingerprint || seed.TrialRunItemCount != expt.TrialRunItemCount {
		return p.fail(ctx, run, "HOOK_PLAN_CONFIG_CHANGED")
	}
	c, err := readPreparationCursor(run, seed.ConfigFingerprint)
	if err != nil {
		return p.fail(ctx, run, "HOOK_PLAN_CURSOR_INVALID")
	}
	if run.Mode == entity.EvaluationModeRetryItems {
		if c.Batch > len(log.ItemIds) {
			return p.fail(ctx, run, "HOOK_PLAN_BATCH_CHANGED")
		}
		if c.Phase == "verify" && c.Batch < len(log.ItemIds) {
			c.Phase = "select"
			c.Selector = ""
			c.BatchDone = false
			c.Verified = entity.NewHookPlanDigest()
			return p.persist(ctx, run, c, nil)
		}
	}
	if c.Phase == "verify" {
		return p.verify(ctx, run, c)
	}
	if c.Route == "" {
		c.Route = "normal"
		if run.Mode == entity.EvaluationModeFailRetry && run.SourceRunID != nil {
			source, err := p.sourceRun(ctx, run, *run.SourceRunID)
			if err != nil && !errors.Is(err, entity.ErrHookStoreMissing) {
				return preparationStorageError(err)
			}
			if source != nil && r07Source(source) {
				empty, err := p.noBaseResults(ctx, key, run.Snapshot.ExecutionScope)
				if err != nil {
					return preparationStorageError(err)
				}
				if empty {
					c.Route = "copy"
					c.SourceID = *run.SourceRunID
					c.SourceCount = source.PlanCount
					c.SourceHash = source.PlanHash
				}
			}
		}
	}
	if c.Route == "copy" {
		return p.copySource(ctx, run, c)
	}
	if c.BatchDone {
		c.Selector = ""
		c.BatchDone = false
	}
	selection := entity.HookSelectionInput{Key: key, Mode: run.Mode, Experiment: preparationExperiment(expt), HasExplicitItemIDs: seed.HasExplicitItemIDs, Cursor: c.Selector}
	if run.Mode == entity.EvaluationModeRetryItems {
		if c.Batch == len(log.ItemIds) {
			c.Phase = "verify"
			c.Verified = entity.NewHookPlanDigest()
			return p.persist(ctx, run, c, nil)
		}
		selection.ItemIDs = slices.Clone(log.ItemIds[c.Batch].ItemIDs)
		if len(selection.ItemIDs) == 0 {
			c.Batch++
			if c.Batch == len(log.ItemIds) {
				c.Phase = "verify"
			}
			return p.persist(ctx, run, c, nil)
		}
	} else if run.Mode == entity.EvaluationModeTrialRun {
		selection.ItemIDs = log.GetItemIDs()
	}
	page, err := p.selectPlanPage(ctx, selection)
	if err != nil {
		return p.sourceError(ctx, run, c, err)
	}
	if len(page.Items) > 100 || page.NextCursor == "" || !page.Done && page.NextCursor == c.Selector {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_INVALID")
	}
	c.Selector = page.NextCursor
	if page.Done {
		if run.Mode == entity.EvaluationModeRetryItems {
			c.Batch++
			c.BatchDone = true
			if c.Batch == len(log.ItemIds) {
				c.Phase = "verify"
			}
		} else {
			c.Phase = "verify"
		}
	}
	return p.persist(ctx, run, c, page.Items)
}

func (p *hookPlanPreparer) persist(ctx context.Context, run *entity.HookStoredRun, c preparationCursor, items []entity.HookPlanItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	seen := make(map[int64]entity.HookPlanItem, len(items))
	unique := make([]entity.HookPlanItem, 0, len(items))
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if item.ID != 0 || item.SourceSpaceID <= 0 || item.EvalSetID <= 0 || item.EvalSetVersionID < 0 || item.ItemID <= 0 || item.ItemVersionID < 0 {
			return p.fail(ctx, run, "HOOK_PLAN_SOURCE_INVALID")
		}
		if old, ok := seen[item.ItemID]; ok {
			if old != item {
				return p.fail(ctx, run, "HOOK_PLAN_ITEM_CHANGED")
			}
			continue
		}
		seen[item.ItemID] = item
		unique = append(unique, item)
		ids = append(ids, item.ItemID)
	}
	if len(ids) > 0 {
		found, err := p.deps.Plans.MGetPlanItems(ctx, entity.HookPlanLookupInput{Key: run.State.Key, ExecutionScope: run.Snapshot.ExecutionScope, ItemIDs: ids})
		if err != nil {
			return preparationStorageError(err)
		}
		if found == nil || found.RunVersion != run.Version || found.Count != run.PlanCount || found.Ready {
			return entity.ErrHookStoreConflict
		}
		existing := map[int64]bool{}
		for _, item := range found.Items {
			item.ID = 0
			want, ok := seen[item.ItemID]
			if !ok || want != item || existing[item.ItemID] {
				return p.fail(ctx, run, "HOOK_PLAN_ITEM_CHANGED")
			}
			existing[item.ItemID] = true
		}
		fresh := unique[:0]
		for _, item := range unique {
			if !existing[item.ItemID] {
				fresh = append(fresh, item)
			}
		}
		unique = fresh
	}
	if len(unique) > 0 {
		allocated, err := p.deps.IDs.GenMultiIDs(ctx, len(unique))
		if err != nil {
			return preparationStorageError(err)
		}
		if len(allocated) != len(unique) {
			return entity.ErrHookPlanStorage
		}
		for i, id := range allocated {
			unique[i].ID = id
		}
		if (entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: run.State.Key}, Items: unique}).Validate() != nil {
			return entity.ErrHookPlanStorage
		}
		c.Selected, err = entity.AppendHookPlanDigest(c.Selected, unique)
		if err != nil {
			return p.fail(ctx, run, "HOOK_PLAN_DIGEST_INVALID")
		}
	}
	next, err := encodePreparationCursor(c)
	if err != nil {
		return p.fail(ctx, run, "HOOK_PLAN_CURSOR_INVALID")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	guard := entity.HookStoreGuard{Key: run.State.Key, ExpectedVersion: run.Version}
	if len(unique) > 0 {
		_, err = p.deps.Runs.AppendPlanPage(ctx, entity.HookPlanPageInput{HookStoreGuard: guard, StartOrdinal: run.PlanCount, Cursor: run.PlanCursor, NextCursor: next, Items: unique})
	} else {
		_, err = p.deps.Plans.AdvancePlanCursor(ctx, entity.HookAdvancePlanInput{HookStoreGuard: guard, ExecutionScope: run.Snapshot.ExecutionScope, ExpectedCount: run.PlanCount, Cursor: run.PlanCursor, NextCursor: next})
	}
	return preparationStorageError(err)
}

func (p *hookPlanPreparer) verify(ctx context.Context, run *entity.HookStoredRun, c preparationCursor) error {
	if c.Route == "copy" {
		if err := p.checkCopySource(ctx, run, c); err != nil {
			return err
		}
	}
	page, err := p.deps.Plans.ReadPlanPage(ctx, entity.HookPlanReadInput{Key: run.State.Key, ExecutionScope: run.Snapshot.ExecutionScope, StartOrdinal: c.Verified.Count, Limit: 100})
	if err != nil {
		return preparationStorageError(err)
	}
	if page == nil || page.RunVersion != run.Version || page.Count != run.PlanCount || page.Ready {
		return entity.ErrHookStoreConflict
	}
	if !validPreparationPage(page, c.Verified.Count, run.State.Key) {
		return p.fail(ctx, run, "HOOK_PLAN_VERIFY_INVALID")
	}
	c.Verified, err = entity.AppendHookPlanDigest(c.Verified, page.Items)
	if err != nil {
		return p.fail(ctx, run, "HOOK_PLAN_VERIFY_INVALID")
	}
	if page.HasMore {
		return p.persist(ctx, run, c, nil)
	}
	if c.Verified != c.Selected || c.Verified.Count != run.PlanCount || c.Route == "copy" && (c.Verified.Count != c.SourceCount || c.Verified.Hash != c.SourceHash) {
		return p.fail(ctx, run, "HOOK_PLAN_VERIFY_INVALID")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err = p.deps.Runs.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: run.State.Key, ExpectedVersion: run.Version}, Count: c.Verified.Count, Hash: c.Verified.Hash})
	return preparationStorageError(err)
}

func (p *hookPlanPreparer) sourceError(ctx context.Context, run *entity.HookStoredRun, c preparationCursor, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, entity.ErrHookPlanStorage) || errors.Is(err, entity.ErrHookStoreConflict) || errors.Is(err, entity.ErrHookStoreCorrupt) || errors.Is(err, entity.ErrHookStoreMissing) {
		return preparationStorageError(err)
	}
	if errors.Is(err, errSelectionCursor) || errors.Is(err, errSelectionSource) {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_INVALID")
	}
	if c.Retries >= 10 {
		return p.fail(ctx, run, "HOOK_PLAN_SOURCE_EXHAUSTED")
	}
	c.Retries++
	if err := p.persist(ctx, run, c, nil); err != nil {
		return err
	}
	return ErrHookPlanSourceRetry
}

func (p *hookPlanPreparer) fail(ctx context.Context, run *entity.HookStoredRun, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := p.deps.Runs.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: run.State.Key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_SystemTerminated, Reason: reason}})
	if err != nil {
		return preparationStorageError(err)
	}
	return ErrHookPlanPreparationFailed
}

func preparationStorageError(err error) error {
	if err == nil {
		return nil
	}
	for _, safe := range []error{context.Canceled, context.DeadlineExceeded, entity.ErrHookStoreConflict, entity.ErrHookStoreCorrupt, entity.ErrHookStoreMissing} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return entity.ErrHookPlanStorage
}
