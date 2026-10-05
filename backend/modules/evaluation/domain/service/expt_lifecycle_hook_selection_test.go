// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func selectionInput(mode entity.ExptRunMode) entity.HookSelectionInput {
	return entity.HookSelectionInput{Key: entity.HookRunKey{WorkspaceID: 3, ExperimentID: 1, RunID: 2}, Mode: mode,
		Experiment: &entity.Experiment{ID: 1, SpaceID: 3, EvalSetSpaceID: 8, EvalSet: &entity.EvaluationSet{ID: 10, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 20}}}}
}

func TestHookSelectionSingleAndTrial(t *testing.T) {
	for _, tc := range []struct {
		name           string
		mode           entity.ExptRunMode
		count          int64
		explicit       bool
		ids            []int64
		batch, ordered bool
		size           int32
		want           int
	}{
		{"submit", entity.EvaluationModeSubmit, 0, false, nil, false, false, 100, 2},
		{"retry_all", entity.EvaluationModeRetryAll, 0, false, nil, false, false, 100, 2},
		{"trial_zero", entity.EvaluationModeTrialRun, 0, true, []int64{7}, false, false, 100, 2},
		{"trial_ids_not_limited", entity.EvaluationModeTrialRun, 1, true, []int64{7, 8}, true, false, 100, 2},
		{"trial_empty", entity.EvaluationModeTrialRun, 1, true, []int64{}, false, false, 100, 2},
		{"trial_null", entity.EvaluationModeTrialRun, 1, true, nil, false, false, 100, 2},
		{"trial_absent", entity.EvaluationModeTrialRun, 1, false, nil, false, true, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			in := selectionInput(tc.mode)
			in.Experiment.TrialRunItemCount = tc.count
			in.HasExplicitItemIDs = tc.explicit
			in.ItemIDs = tc.ids
			items := []*entity.EvaluationSetItem{{ItemID: 7, ItemVersionID: gptr.Of(int64(70))}, {ItemID: 8, ItemVersionID: gptr.Of(int64(80))}}
			if tc.batch {
				svc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
					require.Equal(t, int64(8), p.SpaceID)
					require.Equal(t, int64(10), p.EvaluationSetID)
					require.Equal(t, int64(20), *p.VersionID)
					require.Equal(t, []int64{7, 8}, p.ItemIDs)
					return items, nil
				})
			} else {
				svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
					require.Equal(t, int64(8), p.SpaceID)
					require.Equal(t, int64(10), p.EvaluationSetID)
					require.Equal(t, int64(20), *p.VersionID)
					require.Equal(t, tc.size, *p.PageSize)
					require.Nil(t, p.PageToken)
					if tc.ordered {
						require.Len(t, p.OrderBys, 1)
						require.Equal(t, "item_id", *p.OrderBys[0].Field)
						require.False(t, *p.OrderBys[0].IsAsc)
					} else {
						require.Empty(t, p.OrderBys)
					}
					return items[:tc.want], gptr.Of(int64(2)), nil, nil, nil
				})
			}
			var selector hook.PlanSelector = NewHookPlanSelector(svc, nil, nil, nil)
			before, _ := json.Marshal(in)
			page, err := selector.SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.True(t, page.Done)
			require.NotEmpty(t, page.NextCursor)
			require.Len(t, page.Items, tc.want)
			require.Equal(t, entity.HookPlanItem{SourceSpaceID: 8, EvalSetID: 10, EvalSetVersionID: 20, ItemID: 7, ItemVersionID: 70}, page.Items[0])
			after, _ := json.Marshal(in)
			require.Equal(t, string(before), string(after))
		})
	}
}

func TestHookSelectionMultiSetFilteredEmptyAndSpaces(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	in := selectionInput(entity.EvaluationModeSubmit)
	in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	values := make([]string, 1001)
	for i := range values {
		values[i] = strconv.Itoa(1000 + i)
	}
	in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{
		{EvalSetID: 10, EvalSetVersionID: 10, SourceSpaceID: 8, ItemFilter: &entity.ExptItemFilter{FilterFields: []*entity.ExptItemFilterField{{FieldName: "item_id", FieldType: "long", QueryType: "in", Values: values}}}},
		{EvalSetID: 11, EvalSetVersionID: 21, SourceSpaceID: 0},
	}}
	calls := 0
	svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
		calls++
		require.Equal(t, int32(100), *p.PageSize)
		require.Nil(t, p.Filter)
		switch calls {
		case 1:
			require.Equal(t, int64(8), p.SpaceID)
			require.Nil(t, p.VersionID)
			require.Nil(t, p.PageToken)
			return []*entity.EvaluationSetItem{{ItemID: 1}}, gptr.Of(int64(2)), nil, gptr.Of("next"), nil
		case 2:
			require.Equal(t, "next", *p.PageToken)
			return []*entity.EvaluationSetItem{{ItemID: 1000, ItemVersionID: gptr.Of(int64(77))}}, gptr.Of(int64(2)), nil, nil, nil
		default:
			require.Equal(t, int64(3), p.SpaceID)
			require.Equal(t, int64(11), p.EvaluationSetID)
			require.Equal(t, int64(21), *p.VersionID)
			return []*entity.EvaluationSetItem{{ItemID: 2001}}, gptr.Of(int64(1)), nil, nil, nil
		}
	}).Times(3)
	selector := NewHookPlanSelector(svc, nil, nil, nil)
	first, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, first.Items)
	require.False(t, first.Done)
	require.Equal(t, 1, calls)
	in.Cursor = first.NextCursor
	second, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, ItemID: 1000, ItemVersionID: 77}}, second.Items)
	require.False(t, second.Done)
	in.Cursor = second.NextCursor
	third, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, third.Done)
	require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 3, EvalSetID: 11, EvalSetVersionID: 21, ItemID: 2001}}, third.Items)
}

func TestHookSelectionRetryItemsUsesStoredVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	results := repomocks.NewMockIExptItemResultRepo(ctrl)
	in := selectionInput(entity.EvaluationModeRetryItems)
	in.ItemIDs = []int64{7}
	svc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 7, ItemVersionID: gptr.Of(int64(999))}}, nil)
	results.EXPECT().MGetItemResults(gomock.Any(), int64(1), []int64{7}, int64(3)).Return([]*entity.ExptItemResult{{ItemID: 7, ItemVersionID: 70}}, nil)
	page, err := NewHookPlanSelector(svc, nil, nil, results).SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, page.Done)
	require.Equal(t, int64(70), page.Items[0].ItemVersionID)
}

func TestHookSelectionAppendHasNoReads(t *testing.T) {
	in := selectionInput(entity.EvaluationModeAppend)
	in.Experiment.EvalSet = nil
	page, err := NewHookPlanSelector(nil, nil, nil, nil).SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, page.Done)
	require.Empty(t, page.Items)
}

func TestHookSelectionCursorBindingAndProgress(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	in := selectionInput(entity.EvaluationModeSubmit)
	svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 7}}, gptr.Of(int64(10)), nil, gptr.Of("again"), nil).Times(2)
	selector := NewHookPlanSelector(svc, nil, nil, nil)
	page, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.False(t, page.Done)
	in.Cursor = page.NextCursor
	for _, mutate := range []func(*entity.HookSelectionInput){
		func(v *entity.HookSelectionInput) { v.Key.RunID++ },
		func(v *entity.HookSelectionInput) { v.Mode = entity.EvaluationModeRetryAll },
		func(v *entity.HookSelectionInput) { v.ItemIDs = []int64{99} },
		func(v *entity.HookSelectionInput) { v.HasExplicitItemIDs = true },
		func(v *entity.HookSelectionInput) { v.Cursor = "{}" },
		func(v *entity.HookSelectionInput) { v.Cursor += "{}" },
	} {
		changed := in
		mutate(&changed)
		_, err = selector.SelectPage(context.Background(), changed)
		require.Error(t, err)
	}
	in.Experiment.EvalSetSpaceID = 9
	_, err = selector.SelectPage(context.Background(), in)
	require.Error(t, err)
	in.Experiment.EvalSetSpaceID = 8
	_, err = selector.SelectPage(context.Background(), in)
	require.Error(t, err, "repeated source token must not loop")
}

func TestHookSelectionExplicitIDsPageBySourceOffset(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	in := selectionInput(entity.EvaluationModeTrialRun)
	in.Experiment.TrialRunItemCount = 1
	in.HasExplicitItemIDs = true
	for i := int64(1); i <= 101; i++ {
		in.ItemIDs = append(in.ItemIDs, i)
	}
	call := 0
	svc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
		call++
		if call == 1 {
			require.Len(t, p.ItemIDs, 100)
			require.Equal(t, int64(100), p.ItemIDs[99])
			p.ItemIDs[0] = 999
			return nil, nil
		}
		require.Equal(t, []int64{101}, p.ItemIDs)
		return []*entity.EvaluationSetItem{{ItemID: 101}}, nil
	}).Times(2)
	selector := NewHookPlanSelector(svc, nil, nil, nil)
	page, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.False(t, page.Done)
	require.Equal(t, int64(1), in.ItemIDs[0])
	in.Cursor = page.NextCursor
	page, err = selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, page.Done)
	require.Equal(t, int64(101), page.Items[0].ItemID)
}

func TestHookSelectionMultiRetryUsesRefsWithoutMutatingThem(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(strconv.Itoa(int(mode)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			refs := repomocks.NewMockIExptItemRefRepo(ctrl)
			in := selectionInput(mode)
			in.ItemIDs = []int64{7, 8}
			in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
			in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 10, SourceSpaceID: 8}, {EvalSetID: 11, EvalSetVersionID: 21}}}
			stored := []*entity.ExptItemRef{{ID: 101, SpaceID: 3, ExptID: 1, ItemID: 7, ItemVersionID: 70, EvalSetID: 10}, {ID: 102, SpaceID: 3, ExptID: 1, ItemID: 8, ItemVersionID: 80, EvalSetID: 11, EvalSetVersionID: 21}}
			if mode == entity.EvaluationModeRetryAll {
				refs.EXPECT().ListByExptID(gomock.Any(), int64(3), int64(1), int64(0), int64(100)).Return(stored, int64(0), nil)
			} else {
				refs.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(3), int64(1), []int64{7, 8}).Return(stored, nil)
			}
			svc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
				require.Empty(t, p.ItemIDs)
				require.Len(t, p.ItemVersionQueries, 1)
				q := p.ItemVersionQueries[0]
				if p.EvaluationSetID == 10 {
					require.Equal(t, int64(8), p.SpaceID)
					require.Nil(t, p.VersionID)
					require.Equal(t, int64(70), *q.ItemVersionID)
				} else {
					require.Equal(t, int64(11), p.EvaluationSetID)
					require.Equal(t, int64(3), p.SpaceID)
					require.Equal(t, int64(21), *p.VersionID)
					require.Equal(t, int64(80), *q.ItemVersionID)
				}
				return []*entity.EvaluationSetItem{{ItemID: q.ItemID, ItemVersionID: gptr.Of(*q.ItemVersionID)}}, nil
			}).Times(2)
			page, err := NewHookPlanSelector(svc, refs, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.True(t, page.Done)
			require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, ItemID: 7, ItemVersionID: 70}, {SourceSpaceID: 3, EvalSetID: 11, EvalSetVersionID: 21, ItemID: 8, ItemVersionID: 80}}, page.Items)
			require.Zero(t, stored[0].EvalSetSourceSpaceID)
			require.Zero(t, stored[1].EvalSetSourceSpaceID)
		})
	}
}

func TestHookSelectionFailRetryDedupsOnlyWithinTurnPage(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	status := []int32{int32(entity.TurnRunState_Terminal), int32(entity.TurnRunState_Queueing), int32(entity.TurnRunState_Fail), int32(entity.TurnRunState_Processing)}
	rows := make([]*entity.ExptTurnResult, 50)
	for i := range rows {
		rows[i] = &entity.ExptTurnResult{ID: int64(i + 1), ItemID: 7, ItemVersionID: 70}
	}
	turns.EXPECT().ScanTurnResults(gomock.Any(), int64(1), status, int64(0), int64(50), int64(3)).Return(rows, int64(50), nil)
	turns.EXPECT().ScanTurnResults(gomock.Any(), int64(1), status, int64(50), int64(50), int64(3)).Return([]*entity.ExptTurnResult{{ID: 51, ItemID: 7, ItemVersionID: 70}, {ID: 52, ItemID: 8, ItemVersionID: 80}}, int64(52), nil)
	turns.EXPECT().ScanTurnResults(gomock.Any(), int64(1), status, int64(52), int64(50), int64(3)).Return(nil, int64(0), nil)
	in := selectionInput(entity.EvaluationModeFailRetry)
	selector := NewHookPlanSelector(nil, nil, turns, nil)
	page, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.False(t, page.Done)
	require.Len(t, page.Items, 1)
	require.Equal(t, int64(70), page.Items[0].ItemVersionID)
	in.Cursor = page.NextCursor
	page, err = selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.False(t, page.Done)
	require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, EvalSetVersionID: 20, ItemID: 7, ItemVersionID: 70}, {SourceSpaceID: 8, EvalSetID: 10, EvalSetVersionID: 20, ItemID: 8, ItemVersionID: 80}}, page.Items)
	in.Cursor = page.NextCursor
	page, err = selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, page.Done)
}

func TestHookSelectionMultiRetryDraftUsesActualItemVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	in := selectionInput(entity.EvaluationModeRetryItems)
	in.ItemIDs = []int64{7}
	in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	refs.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(3), int64(1), []int64{7}).Return([]*entity.ExptItemRef{{SpaceID: 3, ExptID: 1, ItemID: 7, EvalSetID: 10}}, nil)
	svc.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 7, ItemVersionID: gptr.Of(int64(71))}}, nil)
	page, err := NewHookPlanSelector(svc, refs, nil, nil).SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, int64(71), page.Items[0].ItemVersionID)
}

func TestHookSelectionRejectsMalformedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*entity.HookSelectionInput)
	}{
		{"wrong_workspace", func(v *entity.HookSelectionInput) { v.Key.WorkspaceID = 9 }},
		{"missing_experiment", func(v *entity.HookSelectionInput) { v.Experiment = nil }},
		{"unknown_mode", func(v *entity.HookSelectionInput) { v.Mode = 99 }},
		{"negative_id", func(v *entity.HookSelectionInput) { v.ItemIDs = []int64{-1} }},
		{"invalid_json", func(v *entity.HookSelectionInput) { v.Cursor = "{" }},
		{"null_cursor", func(v *entity.HookSelectionInput) { v.Cursor = "null" }},
		{"missing_version", func(v *entity.HookSelectionInput) { v.Experiment.EvalSet.EvaluationSetVersion = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := selectionInput(entity.EvaluationModeSubmit)
			tc.mutate(&in)
			_, err := NewHookPlanSelector(nil, nil, nil, nil).SelectPage(context.Background(), in)
			require.Error(t, err)
		})
	}
}

func TestHookSelectionSourceErrorsAreNotEmptySuccess(t *testing.T) {
	for _, name := range []string{"oversized", "nil_item", "source_error"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			items := []*entity.EvaluationSetItem{nil}
			var sourceErr error
			if name == "oversized" {
				items = make([]*entity.EvaluationSetItem, 101)
				for i := range items {
					items[i] = &entity.EvaluationSetItem{ItemID: int64(i + 1)}
				}
			}
			if name == "source_error" {
				sourceErr = context.Canceled
			}
			svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, nil, nil, nil, sourceErr)
			_, err := NewHookPlanSelector(svc, nil, nil, nil).SelectPage(context.Background(), selectionInput(entity.EvaluationModeSubmit))
			require.Error(t, err)
		})
	}
}

func TestHookSelectionRefMismatchCannotSelectAnotherItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	in := selectionInput(entity.EvaluationModeRetryItems)
	in.ItemIDs = []int64{7}
	in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	refs.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(3), int64(1), []int64{7}).Return([]*entity.ExptItemRef{{SpaceID: 3, ExptID: 1, EvalSetID: 10, ItemID: 8}}, nil)
	_, err := NewHookPlanSelector(nil, refs, nil, nil).SelectPage(context.Background(), in)
	require.Error(t, err)
}

func TestHookSelectionTrialFollowsCursorUntilRequestedCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	in := selectionInput(entity.EvaluationModeTrialRun)
	in.Experiment.TrialRunItemCount = 250
	call := 0
	svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
		call++
		require.Equal(t, int32(100), *p.PageSize)
		items := make([]*entity.EvaluationSetItem, 100)
		for i := range items {
			items[i] = &entity.EvaluationSetItem{ItemID: int64((call-1)*100 + i + 1)}
		}
		var total *int64
		if call == 1 {
			total = gptr.Of(int64(100))
		}
		return items, total, nil, gptr.Of(strconv.Itoa(call)), nil
	}).Times(3)
	selector := NewHookPlanSelector(svc, nil, nil, nil)
	for callIndex, wantCount := range []int{100, 100, 50} {
		page, err := selector.SelectPage(context.Background(), in)
		require.NoError(t, err)
		require.Len(t, page.Items, wantCount)
		require.Equal(t, callIndex == 2, page.Done)
		in.Cursor = page.NextCursor
	}
}

func TestHookSelectionFailRetryPreservesSortedItemsAndSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	in := selectionInput(entity.EvaluationModeFailRetry)
	in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
	in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, SourceSpaceID: 8}, {EvalSetID: 11}}}
	turns.EXPECT().ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(3)).Return([]*entity.ExptTurnResult{{ID: 1, ItemID: 8, ItemVersionID: 80}, {ID: 2, ItemID: 7, ItemVersionID: 70}}, int64(2), nil)
	refs.EXPECT().MGetByExptIDAndItemIDs(gomock.Any(), int64(3), int64(1), []int64{7, 8}).Return([]*entity.ExptItemRef{{SpaceID: 3, ExptID: 1, EvalSetID: 11, EvalSetVersionID: 21, ItemID: 8}, {SpaceID: 3, ExptID: 1, EvalSetID: 10, ItemID: 7}}, nil)
	page, err := NewHookPlanSelector(nil, refs, turns, nil).SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, ItemID: 7, ItemVersionID: 70}, {SourceSpaceID: 3, EvalSetID: 11, EvalSetVersionID: 21, ItemID: 8, ItemVersionID: 80}}, page.Items)
}

func TestHookSelectionTerminalCursorDoesNotReadSourceAgain(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 7}}, gptr.Of(int64(1)), nil, nil, nil).Times(1)
	in := selectionInput(entity.EvaluationModeSubmit)
	selector := NewHookPlanSelector(svc, nil, nil, nil)
	page, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, page.Done)
	require.NotEmpty(t, page.NextCursor, "persist the terminal boundary before FinishPlan")
	in.Cursor = page.NextCursor
	resumed, err := selector.SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.True(t, resumed.Done)
	require.Empty(t, resumed.Items)
	require.Equal(t, page.NextCursor, resumed.NextCursor)
}

func TestHookSelectionSingleDraftAliases(t *testing.T) {
	for _, version := range []int64{0, 10} {
		t.Run(strconv.FormatInt(version, 10), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			in := selectionInput(entity.EvaluationModeSubmit)
			in.Experiment.EvalSet.EvaluationSetVersion.ID = version
			svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
				require.Nil(t, p.VersionID)
				require.Equal(t, int64(8), p.SpaceID)
				return []*entity.EvaluationSetItem{{ItemID: 7}}, gptr.Of(int64(1)), nil, nil, nil
			})
			page, err := NewHookPlanSelector(svc, nil, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.Equal(t, []entity.HookPlanItem{{SourceSpaceID: 8, EvalSetID: 10, ItemID: 7}}, page.Items)
		})
	}
}

func TestHookSelectionTrialMultiSetFallbackRules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		count    int64
		explicit bool
		wantSet  int64
		ordered  bool
	}{
		{"non_positive", 0, false, 11, false}, {"explicit_empty", 1, true, 11, false}, {"absent_ids", 1, false, 10, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			in := selectionInput(entity.EvaluationModeTrialRun)
			in.Experiment.TrialRunItemCount = tc.count
			in.HasExplicitItemIDs = tc.explicit
			in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
			in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 11, EvalSetVersionID: 21}}}
			svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
				require.Equal(t, tc.wantSet, p.EvaluationSetID)
				require.Equal(t, tc.ordered, len(p.OrderBys) > 0)
				return []*entity.EvaluationSetItem{{ItemID: 7}}, gptr.Of(int64(1)), nil, nil, nil
			})
			page, err := NewHookPlanSelector(svc, nil, nil, nil).SelectPage(context.Background(), in)
			require.NoError(t, err)
			require.Equal(t, tc.wantSet, page.Items[0].EvalSetID)
		})
	}
}

func TestHookSelectionRejectsCyclicSourceAndOversizedCursor(t *testing.T) {
	for _, name := range []string{"cycle", "oversized"} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			call := 0
			times := 8
			if name == "oversized" {
				times = 1
			}
			svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
				call++
				tokens := []string{"a", "b"}
				token := tokens[(call-1)%len(tokens)]
				if name == "oversized" {
					token = strings.Repeat("x", 65536)
				}
				return []*entity.EvaluationSetItem{{ItemID: int64(call)}}, gptr.Of(int64(10)), nil, gptr.Of(token), nil
			}).MaxTimes(times)
			in := selectionInput(entity.EvaluationModeSubmit)
			selector := NewHookPlanSelector(svc, nil, nil, nil)
			for i := 1; i <= times; i++ {
				page, err := selector.SelectPage(context.Background(), in)
				require.False(t, page.Done)
				if err != nil {
					return
				}
				in.Cursor = page.NextCursor
			}
			t.Fatal("cyclic source or oversized cursor must fail, never finish successfully")
		})
	}
}

func TestHookSelectionRejectsBadKeysetPage(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeFailRetry} {
		t.Run(strconv.Itoa(int(mode)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			refs := repomocks.NewMockIExptItemRefRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			in := selectionInput(mode)
			if mode == entity.EvaluationModeRetryAll {
				in.Experiment.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				refs.EXPECT().ListByExptID(gomock.Any(), int64(3), int64(1), int64(0), int64(100)).Return(nil, int64(7), nil)
			} else {
				turns.EXPECT().ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(3)).Return([]*entity.ExptTurnResult{{ID: 7, ItemID: 7}}, int64(0), nil)
			}
			_, err := NewHookPlanSelector(nil, refs, turns, nil).SelectPage(context.Background(), in)
			require.Error(t, err)
		})
	}
}

func TestHookSelectionCursorContainsNoBusinessConfiguration(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	in := selectionInput(entity.EvaluationModeSubmit)
	in.Experiment.Name = "private-experiment-name"
	in.Experiment.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, Ext: map[string]string{"secret": "private-credential"}}}}
	svc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 7}}, gptr.Of(int64(3)), nil, gptr.Of("next"), nil)
	page, err := NewHookPlanSelector(svc, nil, nil, nil).SelectPage(context.Background(), in)
	require.NoError(t, err)
	require.NotContains(t, page.NextCursor, "private-")
	require.NotContains(t, page.NextCursor, "secret")
	require.NotContains(t, page.NextCursor, "Ext")
}
