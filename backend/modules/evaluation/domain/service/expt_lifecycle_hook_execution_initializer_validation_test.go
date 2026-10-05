// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"math"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

func TestHookFrozenExecutionInitializerConstructorAndInput(t *testing.T) {
	f := newExecutionTestFixture(0)
	for _, bad := range []repo.IHookExecutionInitializationRepo{nil, (*executionTestRepo)(nil)} {
		s, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: bad, Loader: f.loader, IDs: f.ids})
		require.Error(t, err)
		require.Nil(t, s)
	}
	for _, bad := range []hook.PlanPageLoader{nil, (*executionTestLoader)(nil)} {
		s, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: f.repo, Loader: bad, IDs: f.ids})
		require.Error(t, err)
		require.Nil(t, s)
	}
	for _, bad := range []idgen.IIDGenerator{nil, (*executionTestIDs)(nil)} {
		s, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: f.repo, Loader: f.loader, IDs: bad})
		require.Error(t, err)
		require.Nil(t, s)
	}
	s, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: f.repo, Loader: f.loader, IDs: f.ids})
	require.NoError(t, err)
	for _, key := range []entity.HookRunKey{{}, {WorkspaceID: 7, ExperimentID: 11}, {WorkspaceID: -1, ExperimentID: 11, RunID: 13}} {
		out, err := s.InitializeExecution(context.Background(), key, executionTestScope)
		require.Error(t, err)
		require.False(t, out.Initialized)
	}
	out, err := s.InitializeExecution(nil, executionTestKey, executionTestScope)
	require.Error(t, err)
	require.False(t, out.Initialized)
	out, err = s.InitializeExecution(context.Background(), executionTestKey, "")
	require.Error(t, err)
	require.False(t, out.Initialized)
	require.Empty(t, f.repo.reads)
}

func TestHookFrozenExecutionInitializerRejectsInvalidReadPage(t *testing.T) {
	cases := map[string]func(*entity.HookExecutionInitializationPage){
		"count_negative":   func(p *entity.HookExecutionInitializationPage) { p.Count = -1 },
		"count_overflow":   func(p *entity.HookExecutionInitializationPage) { p.Count = math.MaxInt32 + 1 },
		"hash":             func(p *entity.HookExecutionInitializationPage) { p.Hash = "not-a-hash" },
		"version":          func(p *entity.HookExecutionInitializationPage) { p.RunVersion = -1 },
		"next":             func(p *entity.HookExecutionInitializationPage) { p.NextOrdinal = 0 },
		"more":             func(p *entity.HookExecutionInitializationPage) { p.HasMore = true },
		"empty":            func(p *entity.HookExecutionInitializationPage) { p.Items = nil },
		"ordinal":          func(p *entity.HookExecutionInitializationPage) { p.Items[0].Ordinal = 1 },
		"tuple":            func(p *entity.HookExecutionInitializationPage) { p.Items[0].Frozen.SourceSpaceID = 0 },
		"manifest_key":     func(p *entity.HookExecutionInitializationPage) { p.Items[0].Manifest.Key.RunID++ },
		"manifest_tuple":   func(p *entity.HookExecutionInitializationPage) { p.Items[0].Manifest.Frozen.ItemVersionID++ },
		"manifest_ordinal": func(p *entity.HookExecutionInitializationPage) { p.Items[0].Manifest.Ordinal++ },
		"manifest_turn":    func(p *entity.HookExecutionInitializationPage) { p.Items[0].Manifest.Turns[0].TurnID = -1 },
		"manifest_result_alias": func(p *entity.HookExecutionInitializationPage) {
			p.Items[0].Manifest.ItemResultID = p.Items[0].Frozen.ItemID
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newExecutionTestFixture(1)
			f.repo.rows[0].Manifest = f.manifest(0, 0)
			f.repo.readFn = func(in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
				p := f.repo.page(in)
				mutate(p)
				return p, nil
			}
			out, err := f.initialize(t, context.Background())
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.False(t, out.Initialized)
			require.Empty(t, f.loader.calls)
			require.Empty(t, f.ids.counts)
			require.Empty(t, f.repo.completes)
		})
	}
	t.Run("nil_page", func(t *testing.T) {
		f := newExecutionTestFixture(1)
		f.repo.readFn = func(entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
			return nil, nil
		}
		out, err := f.initialize(t, context.Background())
		require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		require.False(t, out.Initialized)
	})
}

func TestHookFrozenExecutionInitializerRejectsInvalidLoadedMetadata(t *testing.T) {
	cases := map[string]func(*entity.HookLoadedPlanPage){
		"count": func(p *entity.HookLoadedPlanPage) { p.Count++ }, "hash": func(p *entity.HookLoadedPlanPage) { p.Hash = "b" + p.Hash[1:] },
		"version": func(p *entity.HookLoadedPlanPage) { p.RunVersion = -1 }, "stale_version": func(p *entity.HookLoadedPlanPage) { p.RunVersion-- },
		"next": func(p *entity.HookLoadedPlanPage) { p.NextOrdinal++ }, "more": func(p *entity.HookLoadedPlanPage) { p.HasMore = true },
		"empty": func(p *entity.HookLoadedPlanPage) { p.Items = nil }, "ordinal": func(p *entity.HookLoadedPlanPage) { p.Items[0].Ordinal++ },
		"frozen": func(p *entity.HookLoadedPlanPage) { p.Items[0].Frozen.ItemVersionID++ }, "item_nil": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item = nil },
		"space": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.SpaceID = executionTestKey.WorkspaceID },
		"set":   func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.EvaluationSetID++ }, "item": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.ItemID++ },
		"item_version": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.ItemVersionID = gptr.Of(int64(99)) }, "version_nil": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.ItemVersionID = nil },
		"turn_empty": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.Turns = nil }, "turn_nil": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.Turns[0] = nil },
		"turn_negative": func(p *entity.HookLoadedPlanPage) { p.Items[0].Item.Turns[0].ID = -1 },
		"turn_duplicate": func(p *entity.HookLoadedPlanPage) {
			p.Items[0].Item.Turns = append(p.Items[0].Item.Turns, p.Items[0].Item.Turns[0])
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newExecutionTestFixture(1)
			f.loader.fn = func(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
				p, e := f.load(ctx, in)
				mutate(p)
				return p, e
			}
			out, err := f.initialize(t, context.Background())
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.False(t, out.Initialized)
			require.Empty(t, f.ids.counts)
			require.Empty(t, f.repo.writes)
			require.Empty(t, f.repo.completes)
		})
	}
	t.Run("nil_page", func(t *testing.T) {
		f := newExecutionTestFixture(1)
		f.loader.fn = func(context.Context, entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) { return nil, nil }
		out, err := f.initialize(t, context.Background())
		require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		require.False(t, out.Initialized)
	})
}

func TestHookFrozenExecutionInitializerRejectsInvalidIDAllocation(t *testing.T) {
	for name, ids := range map[string][]int64{"nil": nil, "short": {50000, 50001}, "extra": {50000, 50001, 50002, 50003}, "zero": {0, 50001, 50002}, "negative": {-1, 50001, 50002}, "duplicate": {50000, 50000, 50002}, "logical_item": {3000, 50001, 50002}, "ledger": {1000, 50001, 50002}} {
		t.Run(name, func(t *testing.T) {
			f := newExecutionTestFixture(1)
			f.ids.fn = func(context.Context, int) ([]int64, error) { return ids, nil }
			out, err := f.initialize(t, context.Background())
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.False(t, out.Initialized)
			require.Empty(t, f.repo.writes)
			require.Empty(t, f.repo.completes)
		})
	}
}

func TestHookFrozenExecutionInitializerRejectsBadCommitReceipt(t *testing.T) {
	for _, kind := range []string{"nil", "count", "hash", "version", "next", "missing", "changed_known"} {
		t.Run(kind, func(t *testing.T) {
			f := newExecutionTestFixture(2)
			f.repo.rows[0].Manifest = f.manifest(0, 0)
			f.repo.writeFn = func(in entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
				if kind == "nil" {
					return nil, nil
				}
				f.repo.rows[1].Manifest = f.manifest(1, 0)
				f.repo.version = 4
				p := f.repo.page(entity.HookExecutionInitializationReadInput{Limit: 2})
				switch kind {
				case "count":
					p.Count++
				case "hash":
					p.Hash = "bad"
				case "version":
					p.RunVersion = 2
				case "next":
					p.NextOrdinal = 1
				case "missing":
					p.Items[1].Manifest = nil
				case "changed_known":
					p.Items[0].Manifest.ItemResultID += 5
				}
				return p, nil
			}
			out, err := f.initialize(t, context.Background())
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.False(t, out.Initialized)
			require.Empty(t, f.repo.completes)
		})
	}
}

func TestHookFrozenExecutionInitializerOnlyCompletionBitMeansDone(t *testing.T) {
	for _, result := range []entity.HookExecutionInitializationCompletion{{RunVersion: 4}, {Changed: true, RunVersion: 4}, {Initialized: true, RunVersion: 2}, {Initialized: true, Changed: true, RunVersion: 3}} {
		f := newExecutionTestFixture(0)
		f.repo.completeFn = func(entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error) {
			return result, nil
		}
		got, err := f.initialize(t, context.Background())
		require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
		require.False(t, got.Initialized)
	}
	f := newExecutionTestFixture(1)
	f.repo.initialized = true
	got, err := f.initialize(t, context.Background())
	require.NoError(t, err)
	require.Equal(t, entity.HookExecutionInitializationCompletion{Initialized: true, RunVersion: 3}, got)
	require.Empty(t, f.loader.calls)
	require.Empty(t, f.ids.counts)
	require.Empty(t, f.repo.writes)
	require.Empty(t, f.repo.completes)
}

func TestHookFrozenExecutionInitializerDetectsPlanDriftAcrossPages(t *testing.T) {
	f := newExecutionTestFixture(101)
	for i := range f.repo.rows {
		f.repo.rows[i].Manifest = f.manifest(i, 0)
	}
	f.repo.readFn = func(in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
		p := f.repo.page(in)
		if in.StartOrdinal > 0 {
			p.Hash = "b" + p.Hash[1:]
		}
		return p, nil
	}
	out, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.False(t, out.Initialized)
	require.Empty(t, f.repo.completes)
	require.Empty(t, f.loader.calls)
}

func TestHookFrozenExecutionInitializerRejectsExhaustedVersionBeforeSource(t *testing.T) {
	f := newExecutionTestFixture(1)
	f.repo.version = math.MaxInt64
	got, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
	require.False(t, got.Initialized)
	require.Empty(t, f.loader.calls, "an uninitialized version that cannot be incremented must fail before reading mutable source")
	require.Empty(t, f.ids.counts)
	f.repo.initialized = true
	got, err = f.initialize(t, context.Background())
	require.NoError(t, err)
	require.True(t, got.Initialized)
	require.Empty(t, f.repo.writes)
}
