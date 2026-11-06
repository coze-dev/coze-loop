// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type filterBoundaryIndex struct {
	repo.IExptTurnResultFilterRepo
	plan          *entity.ExptFilterPlan
	snapshotErr   error
	snapshotHook  func()
	saveErr       error
	readErrTarget entity.ExptFilterTarget
	readErr       error
	rows          map[entity.ExptFilterTarget][]*entity.ExptFilterRow
	snapshotIDs   [][]int64
	saveCalls     int
	savedRows     []*entity.ExptTurnResultFilterEntity
	savedPlan     *entity.ExptFilterPlan
	savedOp       entity.ExptFilterOperation
	reads         []entity.ExptFilterTarget
}

func TestRequestFilterCopyDoesNotRewriteCallerConditions(t *testing.T) {
	keyword := "original"
	request := &entity.ExptTurnResultFilterAccelerator{
		MapCond:          &entity.ExptTurnResultFilterMapCond{EvalTargetDataFilters: []*entity.FieldFilter{{Key: "actual_output", Op: "=", Values: []any{"value"}}}},
		ItemSnapshotCond: &entity.ItemSnapshotFilter{ItemKey: &entity.FieldFilter{Key: "item_key", Op: "=", Values: []any{"original"}}},
		KeywordSearch:    &entity.KeywordFilter{Keyword: &keyword, ItemSnapshotFilter: &entity.ItemSnapshotFilter{ItemKey: &entity.FieldFilter{Key: "item_key"}}},
	}
	copy := copyFilterAccelerator(request)
	copy.MapCond.EvalTargetDataFilters = nil
	copy.ItemSnapshotCond.ItemKey = nil
	copy.KeywordSearch.Keyword = nil
	copy.KeywordSearch.ItemSnapshotFilter.ItemKey = nil
	require.Len(t, request.MapCond.EvalTargetDataFilters, 1)
	require.NotNil(t, request.ItemSnapshotCond.ItemKey)
	require.Equal(t, "original", *request.KeywordSearch.Keyword)
	require.NotNil(t, request.KeywordSearch.ItemSnapshotFilter.ItemKey)
}

func (r *filterBoundaryIndex) SnapshotPlan(_ context.Context, ids []int64) (*entity.ExptFilterPlan, error) {
	r.snapshotIDs = append(r.snapshotIDs, append([]int64(nil), ids...))
	if r.snapshotHook != nil {
		r.snapshotHook()
	}
	return r.plan.Clone(), r.snapshotErr
}

func (r *filterBoundaryIndex) SaveIndex(_ context.Context, rows []*entity.ExptTurnResultFilterEntity, plan *entity.ExptFilterPlan, op entity.ExptFilterOperation) error {
	r.saveCalls++
	r.savedRows, r.savedPlan, r.savedOp = rows, plan, op
	return r.saveErr
}

func (r *filterBoundaryIndex) ReadIndex(_ context.Context, target entity.ExptFilterTarget, _ entity.ExptFilterReadQuery) ([]*entity.ExptFilterRow, error) {
	r.reads = append(r.reads, target)
	if target == r.readErrTarget {
		return nil, r.readErr
	}
	return r.rows[target], nil
}

func (r *filterBoundaryIndex) ScanIndexKeys(context.Context, entity.ExptFilterTarget, entity.ExptFilterIndexScanQuery) ([]entity.ExptFilterStoredKey, *entity.ExptFilterTargetCursor, error) {
	return nil, nil, errors.New("unexpected scan")
}

func TestFilterPolicyFailureStopsResultQuery(t *testing.T) {
	fault := errors.New("policy unavailable")
	for _, tc := range []struct {
		name string
		plan *entity.ExptFilterPlan
		err  error
	}{
		{name: "read_error", err: fault},
		{name: "nil_plan"},
		{name: "empty_targets", plan: &entity.ExptFilterPlan{ReadTarget: entity.ExptFilterTargetV2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			index := &filterBoundaryIndex{plan: tc.plan, snapshotErr: tc.err}
			svc := ExptResultServiceImpl{exptTurnResultFilterRepo: index, ExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl), ExptTurnResultRepo: repomocks.NewMockIExptTurnResultRepo(ctrl)}
			base := int64(2)
			rows, states, total, err := svc.ListTurnResult(context.Background(), &entity.MGetExperimentResultParam{SpaceID: 1, ExptIDs: []int64{2}, BaseExptID: &base, UseAccelerator: true}, &entity.Experiment{ID: 2, SpaceID: 1})
			require.Error(t, err)
			if tc.err != nil {
				require.ErrorIs(t, err, fault)
			}
			require.Nil(t, rows)
			require.Nil(t, states)
			require.Zero(t, total)
			require.Equal(t, [][]int64{{2}}, index.snapshotIDs)
			require.Empty(t, index.reads)
			require.Zero(t, index.saveCalls)
			require.Nil(t, index.savedRows)
		})
	}
}

func TestFilterIndexSaveHonorsPinnedPlanAndError(t *testing.T) {
	fault := errors.New("partial index send")
	for _, pinned := range []bool{false, true} {
		t.Run(map[bool]string{false: "capture", true: "pinned"}[pinned], func(t *testing.T) {
			plan := &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}, ExptIDs: []int64{2, 3}, ConfigHash: "frozen"}
			index := &filterBoundaryIndex{plan: plan, saveErr: fault}
			svc := ExptResultServiceImpl{exptTurnResultFilterRepo: index}
			ctx := context.Background()
			if pinned {
				ctx = contexts.WithExptFilterPlan(ctx, plan)
				index.plan = nil
				index.snapshotErr = errors.New("must not reload pinned policy")
			}
			rows := []*entity.ExptTurnResultFilterEntity{{SpaceID: 1, ExptID: 3, ItemID: 4}, {SpaceID: 1, ExptID: 2, ItemID: 3}}
			err := svc.saveCurrentFilterIndex(ctx, rows)
			require.ErrorIs(t, err, fault)
			require.Equal(t, 1, index.saveCalls)
			require.Equal(t, rows, index.savedRows)
			require.Equal(t, entity.ExptFilterCurrent, index.savedOp)
			require.Equal(t, plan, index.savedPlan)
			if pinned {
				require.Empty(t, index.snapshotIDs)
			} else {
				require.Equal(t, [][]int64{{2, 3}}, index.snapshotIDs)
			}
		})
	}
	t.Run("legacy_without_index_extension", func(t *testing.T) {
		legacy := repomocks.NewMockIExptTurnResultFilterRepo(gomock.NewController(t))
		rows := []*entity.ExptTurnResultFilterEntity{{SpaceID: 1, ExptID: 2, ItemID: 3}}
		legacy.EXPECT().Save(gomock.Any(), rows).Return(fault)
		require.ErrorIs(t, (ExptResultServiceImpl{exptTurnResultFilterRepo: legacy}).saveCurrentFilterIndex(context.Background(), rows), fault)
	})
	t.Run("policy_error_prevents_save", func(t *testing.T) {
		index := &filterBoundaryIndex{snapshotErr: fault}
		err := (ExptResultServiceImpl{exptTurnResultFilterRepo: index}).saveCurrentFilterIndex(context.Background(), []*entity.ExptTurnResultFilterEntity{{SpaceID: 1, ExptID: 2, ItemID: 3}})
		require.ErrorIs(t, err, fault)
		require.Zero(t, index.saveCalls)
		require.Nil(t, index.savedRows)
	})
}

func TestCompareFilterRowsRejectsMalformedSnapshots(t *testing.T) {
	now := time.UnixMilli(1767312001000)
	validTime, future := uint64(now.UnixMilli()-1), uint64(now.UnixMilli()+1)
	row := func() *entity.ExptTurnResultFilterEntity {
		return &entity.ExptTurnResultFilterEntity{SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: 7, EvalSetID: 8, EvalSetVersionID: 9, CreatedDate: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC), EvalTargetData: map[string]string{"actual_output": "value"}}
	}
	for _, tc := range []struct {
		name     string
		expected []*entity.ExptTurnResultFilterEntity
		actual   []*entity.ExptFilterRow
		target   entity.ExptFilterTarget
		wantErr  bool
		wantSame bool
	}{
		{"equal_v2", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{{Filter: row(), WriteTimestampMS: &validTime}}, entity.ExptFilterTargetV2, false, true},
		{"legacy_no_timestamp", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{{Filter: row()}}, entity.ExptFilterTargetLegacy, false, true},
		{"nil_source", []*entity.ExptTurnResultFilterEntity{nil}, nil, entity.ExptFilterTargetV2, true, false},
		{"duplicate_source", []*entity.ExptTurnResultFilterEntity{row(), row()}, nil, entity.ExptFilterTargetV2, true, false},
		{"nil_stored", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{nil}, entity.ExptFilterTargetV2, true, false},
		{"nil_stored_filter", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{{}}, entity.ExptFilterTargetV2, true, false},
		{"duplicate_stored", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{{Filter: row(), WriteTimestampMS: &validTime}, {Filter: row(), WriteTimestampMS: &validTime}}, entity.ExptFilterTargetV2, true, false},
		{"missing_timestamp", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{{Filter: row()}}, entity.ExptFilterTargetV2, true, false},
		{"future_timestamp", []*entity.ExptTurnResultFilterEntity{row()}, []*entity.ExptFilterRow{{Filter: row(), WriteTimestampMS: &future}}, entity.ExptFilterTargetV2, true, false},
		{"missing_row", []*entity.ExptTurnResultFilterEntity{row()}, nil, entity.ExptFilterTargetV2, false, false},
		{"unexpected_row", nil, []*entity.ExptFilterRow{{Filter: row(), WriteTimestampMS: &validTime}}, entity.ExptFilterTargetV2, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			same, err := CompareExptFilterRows(tc.expected, tc.actual, tc.target, now)
			require.Equal(t, tc.wantSame, same)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.name == "future_timestamp" {
				require.ErrorIs(t, err, ErrExptFilterFutureTimestamp)
			}
		})
	}
}

func TestCompareFilterIndexReadsAllTargetsAndPreservesFailures(t *testing.T) {
	fault := errors.New("index read failed")
	for _, tc := range []struct {
		name    string
		failAt  entity.ExptFilterTarget
		missing bool
	}{
		{"legacy_failure", entity.ExptFilterTargetLegacy, false},
		{"v2_failure", entity.ExptFilterTargetV2, false},
		{"legacy_mismatch_does_not_hide_v2_error", entity.ExptFilterTargetV2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := &entity.ExptTurnResultFilterEntity{SpaceID: 1, ExptID: 2, ItemID: 3, CreatedDate: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}
			index := &filterBoundaryIndex{plan: &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: entity.ExptFilterTargetV2, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}}, readErrTarget: tc.failAt, readErr: fault, rows: map[entity.ExptFilterTarget][]*entity.ExptFilterRow{entity.ExptFilterTargetLegacy: {{Filter: row}}}}
			if tc.missing {
				index.rows[entity.ExptFilterTargetLegacy] = nil
			}
			same, err := (ExptResultServiceImpl{exptTurnResultFilterRepo: index}).compareFilterIndex(context.Background(), index, 1, 2, 3, []*entity.ExptTurnResultFilterEntity{row})
			require.False(t, same)
			require.ErrorIs(t, err, fault)
			want := []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy}
			if tc.failAt == entity.ExptFilterTargetV2 {
				want = append(want, entity.ExptFilterTargetV2)
			}
			require.Equal(t, want, index.reads)
		})
	}
	t.Run("empty_source_cannot_verify", func(t *testing.T) {
		index := &filterBoundaryIndex{}
		same, err := (ExptResultServiceImpl{exptTurnResultFilterRepo: index}).compareFilterIndex(context.Background(), index, 1, 2, 3, nil)
		require.False(t, same)
		require.Error(t, err)
		require.Empty(t, index.reads)
		require.Empty(t, index.snapshotIDs)
	})
}

func TestFilterItemPagesPreserveScopeAndStopOnFailure(t *testing.T) {
	t.Run("explicit_ids_are_bounded_and_not_mutated", func(t *testing.T) {
		ids := make([]int64, 0, 103)
		for id := int64(102); id > 0; id-- {
			ids = append(ids, id)
		}
		ids = append(ids, 50)
		before := append([]int64(nil), ids...)
		var pages [][]int64
		err := (ExptResultServiceImpl{}).forEachFilterItemPage(context.Background(), 1, 2, ids, func(page []int64) error {
			pages = append(pages, append([]int64(nil), page...))
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, before, ids)
		require.Len(t, pages, 2)
		require.Len(t, pages[0], 100)
		for index, id := range pages[0] {
			require.Equal(t, int64(index+1), id)
		}
		require.Equal(t, []int64{101, 102}, pages[1])
	})
	t.Run("explicit_callback_failure_stops_remaining_pages", func(t *testing.T) {
		ids := make([]int64, 101)
		for index := range ids {
			ids[index] = int64(index + 1)
		}
		fault := errors.New("projection failed")
		calls := 0
		err := (ExptResultServiceImpl{}).forEachFilterItemPage(context.Background(), 1, 2, ids, func([]int64) error {
			calls++
			return fault
		})
		require.ErrorIs(t, err, fault)
		require.Equal(t, 1, calls)
	})
	for _, mode := range []string{"empty", "nil_row", "invalid_item", "callback_failure", "cancel_after_first_page"} {
		t.Run(mode, func(t *testing.T) {
			items := repomocks.NewMockIExptItemResultRepo(gomock.NewController(t))
			svc := ExptResultServiceImpl{ExptItemResultRepo: items}
			rows := []*entity.ExptItemResult{}
			switch mode {
			case "nil_row":
				rows = []*entity.ExptItemResult{nil}
			case "invalid_item":
				rows = []*entity.ExptItemResult{{ItemID: 0}}
			default:
				if mode != "empty" {
					for id := int64(1); id <= 100; id++ {
						rows = append(rows, &entity.ExptItemResult{ItemID: id})
					}
				}
			}
			items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(2), int64(1), entity.NewPage(1, 100), false).DoAndReturn(func(ctx context.Context, _, _ int64, _ entity.Page, _ bool) ([]*entity.ExptItemResult, int64, error) {
				require.True(t, contexts.CtxWriteDB(ctx))
				return rows, int64(len(rows)), nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			fault := errors.New("projection failed")
			calls := 0
			err := svc.forEachFilterItemPage(ctx, 1, 2, nil, func(page []int64) error {
				calls++
				require.Len(t, page, 100)
				if mode == "callback_failure" {
					return fault
				}
				cancel()
				return nil
			})
			switch mode {
			case "empty":
				require.NoError(t, err)
				require.Zero(t, calls)
			case "nil_row", "invalid_item":
				require.Error(t, err)
				require.Zero(t, calls)
			case "callback_failure":
				require.ErrorIs(t, err, fault)
				require.Equal(t, 1, calls)
			case "cancel_after_first_page":
				require.ErrorIs(t, err, context.Canceled)
				require.Equal(t, 1, calls)
			}
		})
	}
}
