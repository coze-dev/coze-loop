// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type selectionTotalSource struct {
	EvaluationSetItemService
	t          *testing.T
	totals     [2]*int64
	emptyFirst bool
	calls      int
}

func (s *selectionTotalSource) ListEvaluationSetItems(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
	s.t.Helper()
	require.Equal(s.t, int64(8), p.SpaceID)
	require.Equal(s.t, int64(10), p.EvaluationSetID)
	require.Nil(s.t, p.VersionID)
	require.Equal(s.t, int32(100), gptr.Indirect(p.PageSize))
	s.calls++
	require.LessOrEqual(s.t, s.calls, 2)
	if s.calls == 1 {
		require.Empty(s.t, gptr.Indirect(p.PageToken))
		var items []*entity.EvaluationSetItem
		if !s.emptyFirst {
			for id := int64(200); id >= 101; id-- {
				items = append(items, &entity.EvaluationSetItem{ItemID: id})
			}
		}
		return items, s.totals[0], nil, gptr.Of("after-first"), nil
	}
	require.Equal(s.t, "after-first", gptr.Indirect(p.PageToken))
	return []*entity.EvaluationSetItem{{ItemID: 100}}, s.totals[1], nil, nil, nil
}

func selectionTotalInput(mode entity.ExptRunMode, multi bool, count int64) entity.HookSelectionInput {
	in := selectionInput(mode)
	in.Experiment.EvalSet.EvaluationSetVersion.ID = 10
	in.Experiment.TrialRunItemCount = count
	if multi {
		in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
		in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 10, SourceSpaceID: 8}}}
	}
	return in
}

func TestHookSelectionTotalCannotTruncateFiniteSource(t *testing.T) {
	for _, mode := range []struct {
		name  string
		mode  entity.ExptRunMode
		multi bool
	}{
		{"submit", entity.EvaluationModeSubmit, false},
		{"trial101", entity.EvaluationModeTrialRun, false},
		{"multiset", entity.EvaluationModeSubmit, true},
		{"retry_all", entity.EvaluationModeRetryAll, false},
	} {
		for _, totals := range []struct {
			name   string
			values [2]*int64
		}{
			{"stale100", [2]*int64{gptr.Of(int64(100)), gptr.Of(int64(101))}},
			{"exact101", [2]*int64{gptr.Of(int64(101)), gptr.Of(int64(101))}},
			{"nil", [2]*int64{nil, nil}},
			{"zero", [2]*int64{gptr.Of(int64(0)), gptr.Of(int64(0))}},
			{"changing", [2]*int64{gptr.Of(int64(1)), gptr.Of(int64(999))}},
			{"oversized_then_zero", [2]*int64{gptr.Of(int64(999)), gptr.Of(int64(0))}},
		} {
			t.Run(mode.name+"/"+totals.name, func(t *testing.T) {
				source := &selectionTotalSource{t: t, totals: totals.values}
				in := selectionTotalInput(mode.mode, mode.multi, 101)
				seen := make(map[int64]bool)
				for page := 1; page <= 2; page++ {
					out, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
					require.NoError(t, err)
					require.Equal(t, page == 2, out.Done, "cached count is not the source end")
					for _, item := range out.Items {
						require.Zero(t, item.ID)
						require.GreaterOrEqual(t, item.ItemID, int64(100))
						require.LessOrEqual(t, item.ItemID, int64(200))
						require.False(t, seen[item.ItemID])
						seen[item.ItemID] = true
					}
					in.Cursor = out.NextCursor
				}
				require.Len(t, seen, 101)
				require.True(t, seen[100])
				require.True(t, seen[200])
				require.Equal(t, 2, source.calls)
				terminal, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
				require.NoError(t, err)
				require.True(t, terminal.Done)
				require.Empty(t, terminal.Items)
				require.Equal(t, 2, source.calls)
			})
		}
	}
}

func TestHookSelectionTotalEmptySourcePageWithNextStillAdvances(t *testing.T) {
	for _, mode := range []struct {
		name  string
		mode  entity.ExptRunMode
		multi bool
	}{
		{"submit", entity.EvaluationModeSubmit, false}, {"trial", entity.EvaluationModeTrialRun, false}, {"multiset", entity.EvaluationModeSubmit, true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			source := &selectionTotalSource{t: t, emptyFirst: true, totals: [2]*int64{gptr.Of(int64(0)), nil}}
			in := selectionTotalInput(mode.mode, mode.multi, 101)
			first, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.Empty(t, first.Items)
			require.False(t, first.Done)
			in.Cursor = first.NextCursor
			last, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.True(t, last.Done)
			require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, ItemID: 100}}, last.Items)
			require.Equal(t, 2, source.calls)
		})
	}
}

func TestHookSelectionTotalTrialRequestedCountStillStops(t *testing.T) {
	for _, total := range []*int64{nil, gptr.Of(int64(0)), gptr.Of(int64(100)), gptr.Of(int64(999))} {
		source := &selectionTotalSource{t: t, totals: [2]*int64{total, nil}}
		in := selectionTotalInput(entity.EvaluationModeTrialRun, false, 100)
		page, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
		require.NoError(t, err)
		require.True(t, page.Done)
		require.Len(t, page.Items, 100)
		require.Equal(t, 1, source.calls)
	}
}

func TestHookSelectionTotalCancelledResumeDoesNotRead(t *testing.T) {
	source := &selectionTotalSource{t: t, totals: [2]*int64{gptr.Of(int64(101)), nil}}
	in := selectionTotalInput(entity.EvaluationModeSubmit, false, 0)
	first, err := NewHookPlanSelector(source, nil, nil, nil).SelectPage(context.Background(), in)
	require.NoError(t, err)
	in.Cursor = first.NextCursor
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = NewHookPlanSelector(source, nil, nil, nil).SelectPage(ctx, in)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, source.calls)
}
