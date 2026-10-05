// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"math"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type HookFrozenExecutionInitializerDependencies struct {
	Repository repo.IHookExecutionInitializationRepo
	Loader     hook.PlanPageLoader
	IDs        idgen.IIDGenerator
}

type hookFrozenExecutionInitializer struct {
	deps                  HookFrozenExecutionInitializerDependencies
	boundKey              entity.HookRunKey
	boundMode             entity.ExptRunMode
	boundScope, boundHash string
}

func NewHookFrozenExecutionInitializer(d HookFrozenExecutionInitializerDependencies) (hook.ExecutionInitializer, error) {
	for _, dependency := range []any{d.Repository, d.Loader, d.IDs} {
		if missingManagerHookDependency(dependency) {
			return nil, errors.New("missing frozen execution initializer dependency")
		}
	}
	return &hookFrozenExecutionInitializer{deps: d}, nil
}

func (s *hookFrozenExecutionInitializer) InitializeExecution(ctx context.Context, key entity.HookRunKey, scope string) (entity.HookExecutionInitializationCompletion, error) {
	var empty entity.HookExecutionInitializationCompletion
	in := entity.HookExecutionInitializationReadInput{Key: key, ExecutionScope: scope, Limit: 100}
	if ctx == nil || in.Validate() != nil {
		return empty, entity.ErrHookStoreConflict
	}
	if s.boundHash != "" && (s.boundKey != key || s.boundScope != scope) {
		return empty, entity.ErrHookStoreConflict
	}
	plan := executionInitializationPlan{}
	var turnCount int64
	for {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		page, err := s.deps.Repository.ReadExecutionInitializationPage(ctx, in)
		if err != nil || ctx.Err() != nil {
			return empty, executionInitializationError(ctx, err)
		}
		if s.boundHash != "" && (page == nil || page.BoundSnapshotHash != s.boundHash) {
			return empty, entity.ErrHookStoreCorrupt
		}
		if s.boundMode == entity.EvaluationModeAppend && plan.hash != "" && page.Count > plan.count {
			return empty, entity.ErrHookStoreConflict
		}
		if err = plan.observe(page, in); err != nil {
			return empty, err
		}
		if page.Initialized && page.BoundSnapshotHash == "" {
			return entity.HookExecutionInitializationCompletion{Initialized: true, RunVersion: page.RunVersion}, nil
		}
		if executionPageMissing(page) {
			manifests, err := s.loadExecutionManifests(ctx, in, page)
			if err != nil {
				return empty, err
			}
			receipt, conflicted, err := s.writeExecutionManifests(ctx, in, page, manifests)
			if err != nil {
				return empty, err
			}
			if err = plan.observe(receipt, in); err != nil {
				return empty, err
			}
			if receipt.Initialized && receipt.BoundSnapshotHash == "" {
				return entity.HookExecutionInitializationCompletion{Initialized: true, RunVersion: receipt.RunVersion}, nil
			}
			if err = validateExecutionReceipt(page, receipt); err != nil {
				return empty, err
			}
			if executionPageMissing(receipt) {
				if conflicted {
					return empty, entity.ErrHookStoreConflict
				}
				return empty, entity.ErrHookStoreCorrupt
			}
			page = receipt
		}
		for _, item := range page.Items {
			n := int64(len(item.Manifest.Turns))
			if turnCount > math.MaxInt64-n {
				return empty, entity.ErrHookStoreCorrupt
			}
			turnCount += n
		}
		if !page.HasMore {
			break
		}
		in.StartOrdinal = page.NextOrdinal
	}
	complete := entity.HookExecutionInitializationCompleteInput{
		HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: plan.version},
		ExecutionScope: scope, PlanHash: plan.hash, ExpectedItemCount: plan.count, ExpectedTurnCount: turnCount,
	}
	if err := complete.Validate(); err != nil {
		return empty, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	result, err := s.deps.Repository.CompleteExecutionInitialization(ctx, complete)
	if err != nil || ctx.Err() != nil {
		return empty, executionInitializationError(ctx, err)
	}
	if !result.Initialized || result.RunVersion < plan.version || result.Changed && result.RunVersion == plan.version {
		return empty, entity.ErrHookStoreCorrupt
	}
	return result, nil
}

func (s *hookFrozenExecutionInitializer) writeExecutionManifests(ctx context.Context, in entity.HookExecutionInitializationReadInput, page *entity.HookExecutionInitializationPage, manifests []entity.HookExecutionManifest) (*entity.HookExecutionInitializationPage, bool, error) {
	write := entity.HookExecutionInitializationWriteInput{
		HookStoreGuard: entity.HookStoreGuard{Key: in.Key, ExpectedVersion: page.RunVersion},
		ExecutionScope: in.ExecutionScope, PlanHash: page.Hash, StartOrdinal: in.StartOrdinal, Items: manifests,
	}
	if write.Validate() != nil {
		return nil, false, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	receipt, err := s.deps.Repository.WriteExecutionInitializationPage(ctx, write)
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	if errors.Is(err, entity.ErrHookStoreConflict) {
		// A different manifest may have won. Re-read this page once; never retry a source read or skip a missing item.
		receipt, err = s.deps.Repository.ReadExecutionInitializationPage(ctx, in)
		if err != nil || ctx.Err() != nil {
			return nil, true, executionInitializationError(ctx, err)
		}
		return receipt, true, nil
	}
	if err != nil {
		return nil, false, executionInitializationError(ctx, err)
	}
	return receipt, false, nil
}

func executionInitializationError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, safe := range []error{
		context.Canceled, context.DeadlineExceeded, entity.ErrHookStoreConflict, entity.ErrHookStoreCorrupt,
		entity.ErrHookStoreMissing, entity.ErrHookAdmissionDenied, entity.ErrHookExecutionUnsupported,
		entity.ErrHookExecutionStorage, entity.ErrHookPlanStorage, entity.ErrHookFrozenPlanInvalid,
		entity.ErrHookFrozenPlanNotReady, entity.ErrHookFrozenItemUnavailable, entity.ErrHookFrozenSchemaInvalid, entity.ErrHookFrozenContentUnavailable,
	} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	return entity.ErrHookExecutionStorage
}
