// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

var executionTestKey = entity.HookRunKey{WorkspaceID: 7, ExperimentID: 11, RunID: 13}

const executionTestScope = "frozen-execution-test"

type executionTestRepo struct {
	rows        []entity.HookExecutionInitializationItem
	version     int64
	initialized bool
	reads       []entity.HookExecutionInitializationReadInput
	writes      []entity.HookExecutionInitializationWriteInput
	completes   []entity.HookExecutionInitializationCompleteInput
	readFn      func(entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error)
	writeFn     func(entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error)
	completeFn  func(entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error)
}

func (r *executionTestRepo) page(in entity.HookExecutionInitializationReadInput) *entity.HookExecutionInitializationPage {
	p := &entity.HookExecutionInitializationPage{Count: int64(len(r.rows)), Hash: strings.Repeat("a", 64), RunVersion: r.version, Initialized: r.initialized}
	if r.initialized {
		return p
	}
	end := min(int64(len(r.rows)), in.StartOrdinal+int64(in.Limit))
	p.NextOrdinal = end
	p.HasMore = end < int64(len(r.rows))
	for _, row := range r.rows[in.StartOrdinal:end] {
		if row.Manifest != nil {
			copy := row.Manifest.Clone()
			row.Manifest = &copy
		}
		p.Items = append(p.Items, row)
	}
	return p
}

func (r *executionTestRepo) ReadExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
	r.reads = append(r.reads, in)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.readFn != nil {
		return r.readFn(in)
	}
	return r.page(in), nil
}
func (r *executionTestRepo) WriteExecutionInitializationPage(ctx context.Context, in entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
	r.writes = append(r.writes, in)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.writeFn != nil {
		return r.writeFn(in)
	}
	if in.ExpectedVersion != r.version {
		return nil, entity.ErrHookStoreConflict
	}
	for i, m := range in.Items {
		if r.rows[int(in.StartOrdinal)+i].Manifest == nil {
			copy := m.Clone()
			r.rows[int(in.StartOrdinal)+i].Manifest = &copy
		}
	}
	r.version++
	return r.page(entity.HookExecutionInitializationReadInput{StartOrdinal: in.StartOrdinal, Limit: int32(len(in.Items))}), nil
}
func (r *executionTestRepo) CompleteExecutionInitialization(ctx context.Context, in entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error) {
	r.completes = append(r.completes, in)
	if err := ctx.Err(); err != nil {
		return entity.HookExecutionInitializationCompletion{}, err
	}
	if r.completeFn != nil {
		return r.completeFn(in)
	}
	if in.ExpectedVersion != r.version {
		return entity.HookExecutionInitializationCompletion{}, entity.ErrHookStoreConflict
	}
	r.initialized = true
	r.version++
	return entity.HookExecutionInitializationCompletion{Initialized: true, Changed: true, RunVersion: r.version}, nil
}

type executionTestIDs struct {
	next   int64
	counts []int
	fn     func(context.Context, int) ([]int64, error)
}

func (*executionTestIDs) GenID(context.Context) (int64, error) {
	return 0, errors.New("unexpected single-ID allocation")
}
func (g *executionTestIDs) GenMultiIDs(ctx context.Context, n int) ([]int64, error) {
	g.counts = append(g.counts, n)
	if g.fn != nil {
		return g.fn(ctx, n)
	}
	out := make([]int64, n)
	for i := range out {
		out[i] = g.next
		g.next++
	}
	return out, nil
}

type executionTestLoader struct {
	calls []entity.HookPlanReadInput
	fn    func(context.Context, entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error)
}

func (l *executionTestLoader) LoadPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
	l.calls = append(l.calls, in)
	return l.fn(ctx, in)
}

type executionTestFixture struct {
	repo   *executionTestRepo
	loader *executionTestLoader
	ids    *executionTestIDs
	turns  map[int64][]int64
}

func newExecutionTestFixture(n int) *executionTestFixture {
	f := &executionTestFixture{repo: &executionTestRepo{version: 3}, loader: &executionTestLoader{}, ids: &executionTestIDs{next: 50000}, turns: map[int64][]int64{}}
	for i := 0; i < n; i++ {
		f.repo.rows = append(f.repo.rows, entity.HookExecutionInitializationItem{Ordinal: int64(i), Frozen: entity.HookPlanItem{ID: 1000 + int64(i), SourceSpaceID: 17, EvalSetID: 23, EvalSetVersionID: 24, ItemID: 3000 + int64(i), ItemVersionID: 9000 + int64(i)}})
	}
	f.loader.fn = f.load
	return f
}
func (f *executionTestFixture) load(_ context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
	end := min(int64(len(f.repo.rows)), in.StartOrdinal+int64(in.Limit))
	p := &entity.HookLoadedPlanPage{Count: int64(len(f.repo.rows)), Hash: strings.Repeat("a", 64), RunVersion: f.repo.version, NextOrdinal: end, HasMore: end < int64(len(f.repo.rows))}
	for _, row := range f.repo.rows[in.StartOrdinal:end] {
		item := &entity.EvaluationSetItem{ID: 6000 + row.Ordinal, SpaceID: row.Frozen.SourceSpaceID, EvaluationSetID: row.Frozen.EvalSetID, SchemaID: 42, ItemID: row.Frozen.ItemID, ItemVersionID: gptr.Of(row.Frozen.ItemVersionID)}
		turns := f.turns[row.Frozen.ItemID]
		if turns == nil {
			turns = []int64{0}
		}
		for _, id := range turns {
			item.Turns = append(item.Turns, &entity.Turn{ID: id, ItemID: row.Frozen.ItemID, EvalSetID: row.Frozen.EvalSetID})
		}
		p.Items = append(p.Items, entity.HookLoadedPlanItem{Ordinal: row.Ordinal, Frozen: row.Frozen, Item: item})
	}
	return p, nil
}
func (f *executionTestFixture) manifest(index int, turns ...int64) *entity.HookExecutionManifest {
	row := f.repo.rows[index]
	base := int64(10000 + index*10)
	m := &entity.HookExecutionManifest{Version: 1, Key: executionTestKey, Ordinal: row.Ordinal, Frozen: row.Frozen, ItemResultID: base, ItemRunLogID: base + 1, TurnLogsInitialized: gptr.Of(false)}
	for i, id := range turns {
		m.Turns = append(m.Turns, entity.HookExecutionTurnManifest{TurnID: id, TurnIdx: int32(i), ResultID: base + 2 + int64(i)})
	}
	return m
}
func (f *executionTestFixture) initialize(t *testing.T, ctx context.Context) (entity.HookExecutionInitializationCompletion, error) {
	t.Helper()
	s, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: f.repo, Loader: f.loader, IDs: f.ids})
	require.NoError(t, err)
	return s.InitializeExecution(ctx, executionTestKey, executionTestScope)
}

func TestHookFrozenExecutionInitializerAllocatesIndependentIDsAndCompletes(t *testing.T) {
	f := newExecutionTestFixture(2)
	f.turns[3000] = []int64{0, 4}
	got, err := f.initialize(t, context.Background())
	require.NoError(t, err)
	require.Equal(t, entity.HookExecutionInitializationCompletion{Initialized: true, Changed: true, RunVersion: 5}, got)
	require.Equal(t, []int{7}, f.ids.counts)
	require.Len(t, f.repo.writes, 1)
	w := f.repo.writes[0]
	require.Equal(t, executionTestKey, w.Key)
	require.Equal(t, executionTestScope, w.ExecutionScope)
	require.Equal(t, int64(3), w.ExpectedVersion)
	require.Equal(t, strings.Repeat("a", 64), w.PlanHash)
	require.Equal(t, []entity.HookExecutionManifest{
		{Version: 1, Key: executionTestKey, Ordinal: 0, Frozen: f.repo.rows[0].Frozen, ItemResultID: 50000, ItemRunLogID: 50001, Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, TurnIdx: 0, ResultID: 50002}, {TurnID: 4, TurnIdx: 1, ResultID: 50003}}, TurnLogsInitialized: gptr.Of(false)},
		{Version: 1, Key: executionTestKey, Ordinal: 1, Frozen: f.repo.rows[1].Frozen, ItemResultID: 50004, ItemRunLogID: 50005, Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, TurnIdx: 0, ResultID: 50006}}, TurnLogsInitialized: gptr.Of(false)},
	}, w.Items)
	require.Equal(t, []entity.HookExecutionInitializationCompleteInput{{HookStoreGuard: entity.HookStoreGuard{Key: executionTestKey, ExpectedVersion: 4}, ExecutionScope: executionTestScope, PlanHash: strings.Repeat("a", 64), ExpectedItemCount: 2, ExpectedTurnCount: 3}}, f.repo.completes)
	got, err = f.initialize(t, context.Background())
	require.NoError(t, err)
	require.True(t, got.Initialized)
	require.False(t, got.Changed)
	require.Len(t, f.loader.calls, 1)
	require.Len(t, f.ids.counts, 1)
	require.Len(t, f.repo.writes, 1)
	require.Len(t, f.repo.completes, 1)
}

func TestHookFrozenExecutionInitializerSkipsCommittedMutableSource(t *testing.T) {
	f := newExecutionTestFixture(5)
	for _, i := range []int{0, 2, 4} {
		f.repo.rows[i].Manifest = f.manifest(i, 0, 9)
	}
	f.loader.fn = func(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
		require.Equal(t, int32(1), in.Limit)
		require.Contains(t, []int64{1, 3}, in.StartOrdinal, "committed source is deleted and must not be read")
		return f.load(ctx, in)
	}
	got, err := f.initialize(t, context.Background())
	require.NoError(t, err)
	require.True(t, got.Initialized)
	require.Equal(t, []entity.HookPlanReadInput{{Key: executionTestKey, ExecutionScope: executionTestScope, StartOrdinal: 1, Limit: 1}, {Key: executionTestKey, ExecutionScope: executionTestScope, StartOrdinal: 3, Limit: 1}}, f.loader.calls)
	require.Equal(t, []int{6}, f.ids.counts)
	require.Len(t, f.repo.writes, 1)
	require.Equal(t, int64(8), f.repo.completes[0].ExpectedTurnCount)
	for _, i := range []int{0, 2, 4} {
		require.Equal(t, *f.manifest(i, 0, 9), f.repo.writes[0].Items[i])
	}
}

func TestHookFrozenExecutionInitializerUsesConcurrentCommittedReceipt(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "returned_receipt", true: "conflict_readback"}[conflict], func(t *testing.T) {
			f := newExecutionTestFixture(1)
			f.repo.writeFn = func(in entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
				require.Len(t, in.Items[0].Turns, 1)
				f.repo.rows[0].Manifest = f.manifest(0, 0, 8)
				f.repo.version = 4
				if conflict {
					return nil, entity.ErrHookStoreConflict
				}
				return f.repo.page(entity.HookExecutionInitializationReadInput{Limit: 1}), nil
			}
			got, err := f.initialize(t, context.Background())
			require.NoError(t, err)
			require.True(t, got.Initialized)
			require.Equal(t, int64(2), f.repo.completes[0].ExpectedTurnCount, "winning original turn set, not this reader's mutable source")
			require.Equal(t, int64(10000), f.repo.rows[0].Manifest.ItemResultID)
			require.Len(t, f.loader.calls, 1)
			require.Len(t, f.ids.counts, 1)
		})
	}
}

func TestHookFrozenExecutionInitializerResumesAfterFirstPageCommit(t *testing.T) {
	f := newExecutionTestFixture(101)
	fail := true
	f.loader.fn = func(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
		if in.StartOrdinal == 100 && fail {
			fail = false
			return nil, entity.ErrHookFrozenContentUnavailable
		}
		return f.load(ctx, in)
	}
	got, err := f.initialize(t, context.Background())
	require.ErrorIs(t, err, entity.ErrHookFrozenContentUnavailable)
	require.False(t, got.Initialized)
	require.Empty(t, f.repo.completes)
	require.NotNil(t, f.repo.rows[99].Manifest)
	require.Nil(t, f.repo.rows[100].Manifest)
	got, err = f.initialize(t, context.Background())
	require.NoError(t, err)
	require.True(t, got.Initialized)
	require.Equal(t, []int{300, 3}, f.ids.counts)
	require.Len(t, f.loader.calls, 3)
	require.Equal(t, int64(0), f.loader.calls[0].StartOrdinal)
	require.Equal(t, int64(100), f.loader.calls[1].StartOrdinal)
	require.Equal(t, int64(100), f.loader.calls[2].StartOrdinal)
	require.Equal(t, int64(101), f.repo.completes[0].ExpectedItemCount)
	require.Equal(t, int64(101), f.repo.completes[0].ExpectedTurnCount)
}

func TestHookFrozenExecutionInitializerEmptyAndCommittedPlans(t *testing.T) {
	for _, n := range []int{0, 2} {
		t.Run(string(rune('0'+n)), func(t *testing.T) {
			f := newExecutionTestFixture(n)
			for i := range f.repo.rows {
				f.repo.rows[i].Manifest = f.manifest(i, 0)
			}
			got, err := f.initialize(t, context.Background())
			require.NoError(t, err)
			require.True(t, got.Initialized)
			require.Empty(t, f.loader.calls)
			require.Empty(t, f.ids.counts)
			require.Empty(t, f.repo.writes)
			require.Equal(t, int64(n), f.repo.completes[0].ExpectedItemCount)
			require.Equal(t, int64(n), f.repo.completes[0].ExpectedTurnCount)
		})
	}
}

func TestHookFrozenExecutionInitializerKeepsVersionZero(t *testing.T) {
	f := newExecutionTestFixture(1)
	f.repo.rows[0].Frozen.ItemVersionID = 0
	f.loader.fn = func(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
		p, err := f.load(ctx, in)
		p.Items[0].Item.ItemVersionID = gptr.Of(int64(999))
		return p, err
	}
	_, err := f.initialize(t, context.Background())
	require.NoError(t, err)
	require.Len(t, f.repo.writes, 1)
	require.Zero(t, f.repo.writes[0].Items[0].Frozen.ItemVersionID)
	require.Zero(t, f.repo.writes[0].Items[0].Turns[0].TurnID)
}

func TestHookFrozenExecutionInitializerCancellationAndPortErrors(t *testing.T) {
	for _, at := range []string{"entry", "source", "ids", "write", "read_error", "source_error", "ids_error", "write_conflict", "unsupported", "complete_error"} {
		t.Run(at, func(t *testing.T) {
			f := newExecutionTestFixture(1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := error(context.Canceled)
			switch at {
			case "entry":
				cancel()
			case "source":
				f.loader.fn = func(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
					p, e := f.load(ctx, in)
					cancel()
					return p, e
				}
			case "ids":
				f.ids.fn = func(context.Context, int) ([]int64, error) { cancel(); return []int64{50000, 50001, 50002}, nil }
			case "write":
				f.repo.writeFn = func(in entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
					cancel()
					return nil, context.Canceled
				}
			case "read_error":
				want = entity.ErrHookExecutionStorage
				f.repo.readFn = func(entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
					return nil, errors.New("private database error")
				}
			case "source_error":
				want = entity.ErrHookFrozenContentUnavailable
				f.loader.fn = func(context.Context, entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) { return nil, want }
			case "ids_error":
				want = entity.ErrHookExecutionStorage
				f.ids.fn = func(context.Context, int) ([]int64, error) { return nil, errors.New("private id error") }
			case "write_conflict":
				want = entity.ErrHookStoreConflict
				f.repo.writeFn = func(entity.HookExecutionInitializationWriteInput) (*entity.HookExecutionInitializationPage, error) {
					return nil, want
				}
			case "unsupported":
				want = entity.ErrHookExecutionUnsupported
				f.repo.readFn = func(entity.HookExecutionInitializationReadInput) (*entity.HookExecutionInitializationPage, error) {
					return nil, want
				}
			case "complete_error":
				want = entity.ErrHookAdmissionDenied
				f.repo.completeFn = func(entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error) {
					return entity.HookExecutionInitializationCompletion{}, want
				}
			}
			got, err := f.initialize(t, ctx)
			require.ErrorIs(t, err, want)
			require.False(t, got.Initialized)
			if at != "complete_error" {
				require.Empty(t, f.repo.completes)
			}
			if at == "entry" {
				require.Empty(t, f.repo.reads)
			}
			if at == "entry" || at == "source" || at == "ids" || at == "read_error" || at == "source_error" || at == "ids_error" || at == "unsupported" {
				require.Empty(t, f.repo.writes)
			}
			if at == "entry" || at == "source" || at == "read_error" || at == "source_error" || at == "unsupported" {
				require.Empty(t, f.ids.counts)
			}
		})
	}
}
