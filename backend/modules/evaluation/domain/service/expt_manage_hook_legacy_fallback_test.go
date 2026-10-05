// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookLegacyFallbackGuardIsRequestAndRunScoped(t *testing.T) {
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	original := context.Background()
	guarded := WithLegacyOnlyHookInitialization(original, key)
	for _, initial := range []*entity.HookRunInitialization{nil, {Managed: true}, {HooksEnabled: true}} {
		require.ErrorIs(t, validateLegacyHookInitialization(guarded, key, initial), entity.ErrHookStoreConflict)
		require.NoError(t, validateLegacyHookInitialization(original, key, initial), "parent context must remain unchanged")
	}
	require.NoError(t, validateLegacyHookInitialization(guarded, key, &entity.HookRunInitialization{}))
	require.ErrorIs(t, validateLegacyHookInitialization(guarded, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 4}, &entity.HookRunInitialization{}), entity.ErrHookStoreConflict)
}

func TestHookLegacyFallbackMissingInitializationCannotUsePlainWriter(t *testing.T) {
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	ctx := WithLegacyOnlyHookInitialization(context.Background(), key)
	manager := &ExptMangerImpl{}
	require.ErrorIs(t, manager.LogRunWithLegacyHookGuard(ctx, nil, 2, 3, entity.EvaluationModeSubmit, 1, nil, nil, &entity.Session{UserID: "legacy"}), entity.ErrHookConfigStorage)
	require.ErrorIs(t, manager.LogRunWithLegacyHookGuard(ctx, nil, 2, 4, entity.EvaluationModeSubmit, 1, nil, nil, &entity.Session{UserID: "legacy"}), entity.ErrHookStoreConflict)
}
