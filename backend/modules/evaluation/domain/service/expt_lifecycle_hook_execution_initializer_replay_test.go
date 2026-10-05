// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestHookFrozenExecutionInitializerPersistentCancelBeforeWrite(t *testing.T) {
	f := newExecutionTestFixture(1)
	canceled := false
	f.loader.fn = func(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
		p, e := f.load(ctx, in)
		canceled = true
		return p, e
	}
	f.repo.writeFn = func(entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
		require.True(t, canceled)
		return nil, entity.ErrHookAdmissionDenied
	}
	got, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.False(t, got.Initialized)
	require.Len(t, f.repo.reads, 1)
	require.Len(t, f.repo.writes, 1)
	require.Empty(t, f.repo.completes)
	require.Nil(t, f.repo.rows[0].Manifest)
}

func TestHookFrozenExecutionInitializerConflictReadbackCannotSkipMissingRows(t *testing.T) {
	f := newExecutionTestFixture(2)
	f.repo.writeFn = func(entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
		f.repo.rows[0].Manifest = f.manifest(0, 0)
		f.repo.version++
		return nil, entity.ErrHookStoreConflict
	}
	got, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.False(t, got.Initialized)
	require.Len(t, f.repo.reads, 2)
	require.Zero(t, f.repo.reads[1].StartOrdinal)
	require.Len(t, f.ids.counts, 1)
	require.Len(t, f.loader.calls, 1)
	require.Empty(t, f.repo.completes)
}

func TestHookFrozenExecutionInitializerConflictReadbackHonorsCancelGuard(t *testing.T) {
	f := newExecutionTestFixture(1)
	f.repo.writeFn = func(entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
		return nil, entity.ErrHookStoreConflict
	}
	f.repo.readFn = func(in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
		if len(f.repo.reads) > 1 {
			return nil, entity.ErrHookAdmissionDenied
		}
		return f.repo.page(in), nil
	}
	got, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.False(t, got.Initialized)
	require.Len(t, f.repo.reads, 2)
	require.Len(t, f.repo.writes, 1)
	require.Len(t, f.loader.calls, 1)
	require.Len(t, f.ids.counts, 1)
	require.Empty(t, f.repo.completes)
}

func TestHookFrozenExecutionInitializerConcurrentCompletionIsAuthoritative(t *testing.T) {
	f := newExecutionTestFixture(1)
	f.repo.writeFn = func(entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
		f.repo.initialized = true
		f.repo.version = 5
		return nil, entity.ErrHookStoreConflict
	}
	got, err := f.initialize(t, context.Background())
	require.NoError(t, err)
	require.Equal(t, entity.HookExecutionInitializationCompletion{Initialized: true, RunVersion: 5}, got)
	require.Len(t, f.repo.reads, 2)
	require.Len(t, f.repo.writes, 1)
	require.Empty(t, f.repo.completes)
}

func TestHookFrozenExecutionInitializerRejectsAliasedCommittedRecordIDs(t *testing.T) {
	f := newExecutionTestFixture(2)
	f.repo.rows[0].Manifest = f.manifest(0, 0)
	f.repo.rows[1].Manifest = f.manifest(1, 0)
	f.repo.rows[1].Manifest.ItemRunLogID = f.repo.rows[0].Manifest.ItemRunLogID
	got, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.False(t, got.Initialized)
	require.Empty(t, f.loader.calls)
	require.Empty(t, f.ids.counts)
	require.Empty(t, f.repo.completes)
}
