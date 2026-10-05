// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func (f *preparerFixture) complete(t *testing.T) {
	t.Helper()
	for i := 0; i < 30 && !f.s.runs[30].PlanReady; i++ {
		require.NoError(t, f.step(t))
	}
	require.True(t, f.s.runs[30].PlanReady)
}
func (f *preparerFixture) appendBatch(ids ...int64) {
	f.s.logs[30].ItemIds = append(f.s.logs[30].ItemIds, entity.ExptRunLogItems{ItemIDs: ids})
	f.s.runs[30].Version++
}
func persistedItemIDs(rows []entity.HookPlanItem) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ItemID)
	}
	return ids
}

func TestHookPlanPreparerEmptyPagesDedupAndReadback(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
	f.selector.pages = map[string]entity.HookSelectionPage{"": {NextCursor: "empty-filter"}, "empty-filter": {Items: []entity.HookPlanItem{planTestRow(1), planTestRow(1)}, NextCursor: "turn-two"}, "turn-two": {Items: []entity.HookPlanItem{planTestRow(1), planTestRow(2)}, NextCursor: "done", Done: true}}
	require.NoError(t, f.step(t))
	require.Empty(t, f.s.rows[30])
	require.Equal(t, []string{"advance"}, f.s.effects)
	f.complete(t)
	require.Equal(t, []int64{1, 2}, persistedItemIDs(f.s.rows[30]))
	require.Equal(t, 3, f.selector.calls)
	for _, row := range f.s.rows[30] {
		require.Positive(t, row.ID)
	}
	before := f.selector.calls
	require.NoError(t, f.step(t))
	require.Equal(t, before, f.selector.calls)
}

func TestHookPlanPreparerBoundedLargeVerify(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	for page := 0; page < 3; page++ {
		key := ""
		if page > 0 {
			key = fmt.Sprint(page)
		}
		var rows []entity.HookPlanItem
		for i := page * 100; i < min((page+1)*100, 205); i++ {
			rows = append(rows, planTestRow(int64(i+1)))
		}
		f.selector.pages[key] = entity.HookSelectionPage{Items: rows, NextCursor: fmt.Sprint(page + 1), Done: page == 2}
	}
	for i := 0; i < 3; i++ {
		require.NoError(t, f.step(t))
		require.False(t, f.s.runs[30].PlanReady)
	}
	require.Len(t, f.s.rows[30], 205)
	for i := 0; i < 2; i++ {
		require.NoError(t, f.step(t))
		require.False(t, f.s.runs[30].PlanReady)
	}
	require.NoError(t, f.step(t))
	require.True(t, f.s.runs[30].PlanReady)
	require.Equal(t, 3, f.selector.calls)
}

func TestHookPlanPreparerEmptyAppendAndNoBefore(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeAppend)
	f.complete(t)
	require.Empty(t, f.s.rows[30])
	require.Equal(t, entity.NewHookPlanDigest().Hash, f.s.runs[30].PlanHash)
	f = newPreparerFixture(t, entity.EvaluationModeSubmit)
	f.s.runs[30].State.Before.Status = entity.HookOperationDisabled
	f.selector.err = errors.New("must not select")
	require.NoError(t, f.step(t))
	require.Empty(t, f.s.effects)
	require.Zero(t, f.selector.calls)
}

func TestHookPlanPreparerLostReceiptResumesCommittedPage(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	f.selector.pages = map[string]entity.HookSelectionPage{"": {Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "second"}, "second": {Items: []entity.HookPlanItem{planTestRow(2)}, NextCursor: "terminal", Done: true}}
	f.s.lostReceipt = true
	require.ErrorIs(t, f.step(t), entity.ErrHookPlanStorage)
	require.Len(t, f.s.rows[30], 1)
	f.complete(t)
	require.Equal(t, []int64{1, 2}, persistedItemIDs(f.s.rows[30]))
	require.Equal(t, 2, f.selector.calls)
}

func TestHookPlanPreparerCancellationAndTwoWorkerCAS(t *testing.T) {
	t.Run("cancel_before_finish", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		require.NoError(t, f.step(t))
		f.s.beforeWrite = func() {
			change, err := entity.BeginHookFinalize(&f.s.runs[30].State, f.in.Candidate.Key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "cancelled"})
			require.NoError(t, err)
			f.s.runs[30].State = change.State
			f.s.runs[30].Version++
		}
		require.ErrorIs(t, f.step(t), entity.ErrHookStoreConflict)
		require.False(t, f.s.runs[30].PlanReady)
		require.NotContains(t, f.s.effects, "finish")
		require.NoError(t, f.step(t))
	})
	t.Run("two_worker_same_page", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.pages[""] = entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "terminal", Done: true}
		f.s.beforeWrite = func() { require.NoError(t, f.step(t)) }
		require.ErrorIs(t, f.step(t), entity.ErrHookStoreConflict)
		require.Len(t, f.s.rows[30], 1)
		require.Equal(t, []string{"append"}, f.s.effects)
		f.complete(t)
		require.Equal(t, []int64{1}, persistedItemIDs(f.s.rows[30]))
	})
}

func TestHookPlanPreparerRetryBatchesAndVerifyTail(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeRetryItems)
	f.appendBatch(1, 2)
	f.selector.selectFn = func(ctx context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		switch in.ItemIDs[0] {
		case 1:
			require.Equal(t, []int64{1, 2}, in.ItemIDs)
			if in.Cursor == "" {
				return entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "batch0-next"}, nil
			}
			require.Equal(t, "batch0-next", in.Cursor)
			return entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(2)}, NextCursor: "batch0-done", Done: true}, nil
		default:
			require.Empty(t, in.Cursor)
			return entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(in.ItemIDs[0])}, NextCursor: "done", Done: true}, nil
		}
	}
	require.NoError(t, f.step(t))
	f.appendBatch(3)
	require.NoError(t, f.step(t))
	require.Contains(t, f.s.runs[30].PlanCursor, "batch0-done", "the completed selector page receipt must be durable before the next batch")
	require.NoError(t, f.step(t))
	require.Equal(t, []int64{1, 2, 3}, persistedItemIDs(f.s.rows[30]))
	f.appendBatch(4)
	require.NoError(t, f.step(t))
	require.False(t, f.s.runs[30].PlanReady)
	require.NoError(t, f.step(t))
	f.s.beforeWrite = func() { f.appendBatch(5) }
	require.ErrorIs(t, f.step(t), entity.ErrHookStoreConflict)
	require.False(t, f.s.runs[30].PlanReady)
	f.complete(t)
	require.Equal(t, []int64{1, 2, 3, 4, 5}, persistedItemIDs(f.s.rows[30]))
	require.Equal(t, 5, f.selector.calls)
}

func (f *preparerFixture) source(t *testing.T, count int) {
	t.Helper()
	source := *f.s.runs[30]
	source.State.Key.RunID = 29
	source.State.Status = entity.ExptStatus_Terminated
	source.State.Gate = entity.HookGateClosed
	source.State.Finalize = entity.HookFinalizeCommitted
	source.State.Before.Status = entity.HookOperationFailed
	source.State.Intent = entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "HOOK_BEFORE_FAILED"}
	source.PlanReady = true
	source.PlanCount = int64(count)
	digest := entity.NewHookPlanDigest()
	for i := 0; i < count; i++ {
		row := entity.HookPlanItem{ID: int64(100 + i), SourceSpaceID: 99, EvalSetID: 81, EvalSetVersionID: 82, ItemID: int64(100 + i), ItemVersionID: 83}
		f.s.rows[29] = append(f.s.rows[29], row)
		var err error
		digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{row})
		require.NoError(t, err)
	}
	source.PlanHash = digest.Hash
	f.s.runs[29] = &source
	f.s.runs[30].SourceRunID = gptr.Of(int64(29))
}

func TestHookPlanPreparerR07CopiesReadyOriginalPlan(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
	f.source(t, 201)
	f.selector.err = errors.New("must not reselect source")
	f.complete(t)
	require.Len(t, f.s.rows[30], 201)
	require.Zero(t, f.selector.calls)
	require.Equal(t, f.s.runs[29].PlanHash, f.s.runs[30].PlanHash)
	for i, row := range f.s.rows[30] {
		want := f.s.rows[29][i]
		require.NotEqual(t, want.ID, row.ID)
		want.ID = row.ID
		require.Equal(t, want, row)
	}
}

func TestHookPlanPreparerR07DoesNotReplaceNormalFailRetry(t *testing.T) {
	for _, reason := range []string{"not_ready", "different_reason", "started", "success_item", "success_turn", "legacy_source"} {
		t.Run(reason, func(t *testing.T) {
			f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
			f.source(t, 1)
			switch reason {
			case "not_ready":
				f.s.runs[29].PlanReady = false
			case "different_reason":
				f.s.runs[29].State.Intent.Reason = "other"
			case "started":
				f.s.runs[29].ExecutionStarted = true
			case "success_item":
				f.results.items = true
			case "success_turn":
				f.results.turns = true
			case "legacy_source":
				delete(f.s.runs, 29)
			}
			f.selector.pages[""] = entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(7)}, NextCursor: "failed-turn-terminal", Done: true}
			f.complete(t)
			require.Equal(t, []int64{7}, persistedItemIDs(f.s.rows[30]))
			require.Equal(t, 1, f.selector.calls)
		})
	}
}

func TestHookPlanPreparerR07RejectsChangedHashAndLateResults(t *testing.T) {
	t.Run("hash", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
		f.source(t, 1)
		f.s.runs[29].PlanHash = strings.Repeat("0", 64)
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.False(t, f.s.runs[30].PlanReady)
		require.Equal(t, entity.ExptStatus_SystemTerminated, f.s.runs[30].State.Intent.Status)
	})
	t.Run("late_results", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
		f.source(t, 101)
		require.NoError(t, f.step(t))
		f.results.turns = true
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.Len(t, f.s.rows[30], 100)
		require.False(t, f.s.runs[30].PlanReady)
	})
	t.Run("wrong_scope", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
		f.source(t, 1)
		f.s.runs[29].Snapshot.ExecutionScope = "other"
		require.ErrorIs(t, f.step(t), entity.ErrHookStoreConflict)
		require.Empty(t, f.s.effects)
	})
	t.Run("read_unavailable", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeFailRetry)
		f.source(t, 1)
		f.results.err = errors.New("database secret")
		require.ErrorIs(t, f.step(t), entity.ErrHookPlanStorage)
		require.Empty(t, f.s.effects)
		require.Empty(t, f.s.runs[30].PlanCursor)
	})
}

func TestHookPlanPreparerDriftAndStoredMetadataTamper(t *testing.T) {
	t.Run("fingerprint", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.s.expt.EvalSetVersionID++
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.Zero(t, f.selector.calls)
		require.Equal(t, entity.ExptStatus_SystemTerminated, f.s.runs[30].State.Intent.Status)
		require.False(t, f.s.runs[30].State.After.Activated)
	})
	t.Run("verify", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.pages[""] = entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "terminal", Done: true}
		require.NoError(t, f.step(t))
		f.s.rows[30][0].ItemVersionID++
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.False(t, f.s.runs[30].PlanReady)
	})
	t.Run("duplicate_metadata", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		changed := planTestRow(1)
		changed.ItemVersionID++
		f.selector.pages[""] = entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(1), changed}, NextCursor: "terminal", Done: true}
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.Empty(t, f.s.rows[30])
	})
}

func TestHookPlanPreparerErrorsBudgetAndParentCancellation(t *testing.T) {
	t.Run("initial_plus_ten", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.err = errors.New("RPC secret")
		for i := 0; i < 10; i++ {
			err := f.step(t)
			require.ErrorIs(t, err, ErrHookPlanSourceRetry)
			require.NotContains(t, err.Error(), "secret")
			require.Equal(t, entity.HookFinalizeNone, f.s.runs[30].State.Finalize)
		}
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.Equal(t, 11, f.selector.calls)
		require.Len(t, f.s.effects, 11)
		require.Equal(t, "finalize", f.s.effects[10])
		require.False(t, f.s.runs[30].PlanReady)
	})
	t.Run("parent_cancel", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		f.selector.selectFn = func(context.Context, entity.HookSelectionInput) (entity.HookSelectionPage, error) {
			cancel()
			return entity.HookSelectionPage{}, context.Canceled
		}
		p, err := NewHookPlanPreparer(f.deps)
		require.NoError(t, err)
		require.ErrorIs(t, p.PreparePlan(ctx, f.in), context.Canceled)
		require.Empty(t, f.s.effects)
	})
	t.Run("lookup_storage", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.selectFn = func(context.Context, entity.HookSelectionInput) (entity.HookSelectionPage, error) {
			f.s.storageErr = errors.New("DB secret")
			return entity.HookSelectionPage{Items: []entity.HookPlanItem{planTestRow(1)}, NextCursor: "done", Done: true}, nil
		}
		require.ErrorIs(t, f.step(t), entity.ErrHookPlanStorage)
		require.Empty(t, f.s.effects)
		require.Empty(t, f.s.runs[30].PlanCursor)
	})
	t.Run("selector_storage_conflict", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.err = entity.ErrHookStoreConflict
		require.ErrorIs(t, f.step(t), entity.ErrHookStoreConflict)
		require.Empty(t, f.s.effects)
	})
}

func TestHookPlanPreparerRawFingerprintThenHydratesCopy(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	// A stale convenience view must not enter the digest or be passed to the selector.
	f.s.expt.EvalSet = &entity.EvaluationSet{ID: 999, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 999}}
	f.selector.selectFn = func(ctx context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		require.Equal(t, int64(71), in.Experiment.EvalSet.ID)
		require.Equal(t, int64(72), in.Experiment.EvalSet.EvaluationSetVersion.ID)
		require.Equal(t, int64(11), in.Experiment.EvalSet.SpaceID)
		in.Experiment.EvalSet.ID = 111
		in.Experiment.EvalSetVersionID = 112
		return entity.HookSelectionPage{NextCursor: "done", Done: true}, nil
	}
	f.complete(t)
	require.Equal(t, int64(999), f.s.expt.EvalSet.ID)
	require.Equal(t, int64(72), f.s.expt.EvalSetVersionID)
}

func TestHookPlanPreparerRejectsMissingDependencies(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	f.deps.Selector = nil
	_, err := NewHookPlanPreparer(f.deps)
	require.Error(t, err)
	var selector *preparerSelector
	f.deps.Selector = selector
	_, err = NewHookPlanPreparer(f.deps)
	require.Error(t, err)
}

func TestHookPlanPreparerUsesProductionSelector(t *testing.T) {
	f := newPreparerFixture(t, entity.EvaluationModeSubmit)
	items := svcmocks.NewMockEvaluationSetItemService(gomock.NewController(t))
	page := 0
	items.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, in *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
		require.True(t, contexts.CtxWriteDB(ctx))
		require.Equal(t, int64(11), in.SpaceID)
		require.Equal(t, int64(71), in.EvaluationSetID)
		require.Equal(t, int64(72), *in.VersionID)
		require.Equal(t, int32(100), *in.PageSize)
		page++
		if page == 1 {
			require.Nil(t, in.PageToken)
			return []*entity.EvaluationSetItem{{ItemID: 7, ItemVersionID: gptr.Of(int64(70))}}, gptr.Of(int64(2)), nil, gptr.Of("source-next"), nil
		}
		require.Equal(t, "source-next", *in.PageToken)
		return []*entity.EvaluationSetItem{{ItemID: 8, ItemVersionID: gptr.Of(int64(80))}}, gptr.Of(int64(2)), nil, nil, nil
	}).Times(2)
	f.deps.Selector = NewHookPlanSelector(items, nil, nil, nil)
	f.complete(t)
	require.Equal(t, []int64{7, 8}, persistedItemIDs(f.s.rows[30]))
	require.Equal(t, int64(70), f.s.rows[30][0].ItemVersionID)
	require.Equal(t, int64(80), f.s.rows[30][1].ItemVersionID)
}

func TestHookPlanPreparerCopiesFiltersWithoutChangingRaw(t *testing.T) {
	raw := &entity.Experiment{ID: 20, SpaceID: 10, EvalSetID: 71, EvalSetVersionID: 71, EvalConf: &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 71, EvalSetVersionID: 71, ItemFilter: &entity.ExptItemFilter{QueryAndOr: "and", FilterFields: []*entity.ExptItemFilterField{{FieldName: "label", Values: []string{"keep"}}}}}}}}
	before, err := json.Marshal(raw)
	require.NoError(t, err)
	copy := preparationExperiment(raw)
	require.Equal(t, int64(71), copy.EvalSet.EvaluationSetVersion.ID)
	copy.EvalConf.EvalSetConfigs[0].ItemFilter.FilterFields[0].Values[0] = "changed"
	copy.EvalConf.EvalSetConfigs[0].EvalSetID++
	after, err := json.Marshal(raw)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestHookPlanPreparerUnknownStateAndOversizedCursorFailClosed(t *testing.T) {
	t.Run("unknown_state", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.s.runs[30].State.Finalize = ""
		require.ErrorIs(t, f.step(t), entity.ErrHookStoreCorrupt)
		require.Empty(t, f.s.effects)
	})
	t.Run("cursor_limit", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.selector.pages[""] = entity.HookSelectionPage{NextCursor: strings.Repeat("x", 65535), Done: true}
		require.ErrorIs(t, f.step(t), ErrHookPlanPreparationFailed)
		require.False(t, f.s.runs[30].PlanReady)
	})
	t.Run("cross_run", func(t *testing.T) {
		f := newPreparerFixture(t, entity.EvaluationModeSubmit)
		f.s.expt.LatestRunID = 31
		require.ErrorIs(t, f.step(t), entity.ErrHookStoreConflict)
		require.Empty(t, f.s.effects)
	})
}
