// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func (p *itemHookPorts) InitializeHookTurnRunLogs(_ context.Context, key entity.HookRunKey, itemID, version int64, candidates []*entity.ExptTurnResultRunLog) (bool, []*entity.ExptTurnResultRunLog, error) {
	if p.run == nil || p.run.State.Key != key || p.run.State.Before.Status != entity.HookOperationDisabled || p.run.State.After.Status == entity.HookOperationDisabled || itemID != 4 || version < 0 || candidates != nil {
		return true, nil, entity.ErrHookExecutionUnsupported
	}
	return false, nil, nil
}

// newExecutionHookFixture obtains its binding from newItemHookFixture's after-only Run.
// This double has no frozen manifest; frozen tests use the actual MySQL initializer.
func (p *executionHookProgress) InitializeHookTurnRunLogs(_ context.Context, key entity.HookRunKey, itemID, version int64, candidates []*entity.ExptTurnResultRunLog) (bool, []*entity.ExptTurnResultRunLog, error) {
	if key != (entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}) || itemID != 4 || version < 0 || candidates != nil {
		return true, nil, entity.ErrHookExecutionUnsupported
	}
	return false, nil, nil
}

func TestHookLazyTurnsCompatibilityRejectsFrozenContract(t *testing.T) {
	f := newItemHookFixture(t, true, false)
	require.Equal(t, entity.HookOperationDisabled, f.ports.run.State.Before.Status)
	handled, _, err := f.ports.InitializeHookTurnRunLogs(context.Background(), f.ports.run.State.Key, 4, 0, nil)
	require.NoError(t, err)
	require.False(t, handled)
	f.ports.run.State.Before = entity.HookOperation{ID: "before", Status: entity.HookOperationSucceeded}
	handled, _, err = f.ports.InitializeHookTurnRunLogs(context.Background(), f.ports.run.State.Key, 4, 0, nil)
	require.Error(t, err)
	require.True(t, handled, "a frozen contract cannot silently fall through to the legacy creator")
}
