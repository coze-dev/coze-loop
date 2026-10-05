// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type hookLegacyInitializationKey struct{}

// WithLegacyOnlyHookInitialization binds a fallback decision to this request's Run.
func WithLegacyOnlyHookInitialization(ctx context.Context, key entity.HookRunKey) context.Context {
	return context.WithValue(ctx, hookLegacyInitializationKey{}, key)
}

func legacyHookInitialization(ctx context.Context) (entity.HookRunKey, bool) {
	if ctx == nil {
		return entity.HookRunKey{}, false
	}
	key, ok := ctx.Value(hookLegacyInitializationKey{}).(entity.HookRunKey)
	return key, ok
}

func validateLegacyHookInitialization(ctx context.Context, key entity.HookRunKey, initial *entity.HookRunInitialization) error {
	wanted, onlyLegacy := legacyHookInitialization(ctx)
	if onlyLegacy && (wanted != key || initial == nil || initial.Managed || initial.HooksEnabled) {
		return entity.ErrHookStoreConflict
	}
	return nil
}

// LogRunWithLegacyHookGuard lets a disabled router use the existing no-Hook creation CAS.
func (e *ExptMangerImpl) LogRunWithLegacyHookGuard(ctx context.Context, initialization repo.IHookRunInitializationRepo, exptID, runID int64, mode entity.ExptRunMode, spaceID int64, itemIDs []int64, rawItemIDs *string, session *entity.Session) error {
	if e == nil {
		return entity.ErrHookConfigStorage
	}
	wanted, onlyLegacy := legacyHookInitialization(ctx)
	key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID}
	if onlyLegacy && wanted != key {
		return entity.ErrHookStoreConflict
	}
	if !onlyLegacy || e.hooks != nil {
		if rawItemIDs != nil {
			return e.LogRunWithPlanSeed(ctx, exptID, runID, mode, spaceID, *rawItemIDs, session)
		}
		return e.LogRun(ctx, exptID, runID, mode, spaceID, itemIDs, session)
	}
	if missingManagerHookDependency(initialization) {
		return entity.ErrHookConfigStorage
	}
	scoped := *e
	// Only the legacy branch is reachable: the request guard rejects managed/enabled reads.
	scoped.hooks = &ExptManagerHookDependencies{Initialization: initialization}
	return scoped.logHookRun(ctx, exptID, runID, mode, spaceID, itemIDs, session, rawItemIDs)
}
