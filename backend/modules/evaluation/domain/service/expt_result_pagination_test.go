// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestListTurnResultItemPagination(t *testing.T) {
	for _, turnsPerItem := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d_turns_per_item", turnsPerItem), func(t *testing.T) {
			seen := make(map[int64]bool)
			for _, pageNumber := range []int{1, 2, 3, 4, 5, 6, 1000} {
				t.Run(fmt.Sprintf("page_%d", pageNumber), func(t *testing.T) {
					ctrl := gomock.NewController(t)
					items := repomocks.NewMockIExptItemResultRepo(ctrl)
					turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
					svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns}
					page := entity.NewPage(pageNumber, 20)
					var pageItems []*entity.ExptItemResult
					var ids []int64
					for i := page.Offset(); i < min(page.Offset()+page.Limit(), 89); i++ {
						id := int64(i + 1)
						ids = append(ids, id)
						pageItems = append(pageItems, &entity.ExptItemResult{ItemID: id, ItemIdx: int32(i)})
					}
					items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), page, false).
						Return(pageItems, int64(89), nil)
					if len(ids) > 0 {
						var pageTurns []*entity.ExptTurnResult
						// Return reversed items to exercise the existing item ordering too.
						for i := len(ids) - 1; i >= 0; i-- {
							for turn := 1; turn <= turnsPerItem; turn++ {
								pageTurns = append(pageTurns, &entity.ExptTurnResult{ItemID: ids[i], TurnID: int64(turn)})
							}
						}
						turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), ids, entity.Page{}, false).
							Return(pageTurns, int64(len(pageTurns)), nil)
						items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), ids).Return(pageItems, nil)
					}
					// Empty pages intentionally have no turn-query or BatchGet expectation.
					got, _, total, err := svc.ListTurnResult(context.Background(), &entity.MGetExperimentResultParam{
						SpaceID: 3, BaseExptID: gptr.Of(int64(1)), Page: page, UseAccelerator: true,
					}, &entity.Experiment{ExptType: entity.ExptType_Offline})
					require.NoError(t, err)
					require.Equal(t, int64(89), total)
					require.Len(t, got, len(ids)*turnsPerItem)
					for i, id := range ids {
						require.False(t, seen[id], "item must not cross page boundaries")
						seen[id] = true
						turnIDs := make(map[int64]bool)
						for j := 0; j < turnsPerItem; j++ {
							row := got[i*turnsPerItem+j]
							require.Equal(t, id, row.ItemID)
							require.False(t, turnIDs[row.TurnID])
							turnIDs[row.TurnID] = true
						}
					}
				})
			}
			require.Len(t, seen, 89)
		})
	}
}

func TestListTurnResultItemPageReadError(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	readErr := errors.New("item page unavailable")
	items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), gomock.Any(), false).
		Return(nil, int64(0), readErr)
	svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns}
	got, _, _, err := svc.ListTurnResult(context.Background(), &entity.MGetExperimentResultParam{
		SpaceID: 3, BaseExptID: gptr.Of(int64(1)), Page: entity.NewPage(6, 20), UseAccelerator: true,
	}, &entity.Experiment{ExptType: entity.ExptType_Offline})
	require.ErrorIs(t, err, readErr)
	require.Empty(t, got)
}

func TestMGetExperimentResultEmptyPageTotal(t *testing.T) {
	for _, tc := range []struct {
		name        string
		accelerator bool
		cursor      bool
		hasItem     bool
	}{
		{name: "item_page", accelerator: true},
		{name: "item_page_without_turns", accelerator: true, hasItem: true},
		{name: "turn_page"},
		{name: "turn_cursor", cursor: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
			metric := metricsmocks.NewMockExptMetric(ctrl)
			lwt := lwtmocks.NewMockILatestWriteTracker(ctrl)
			expt := &entity.Experiment{ID: 1, SpaceID: 3, ExptType: entity.ExptType_Offline,
				EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig}
			expts.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(3)).Return([]*entity.Experiment{expt}, nil)
			expts.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(3)).Return(nil, nil)
			annotations.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), []int64{1}, int64(3)).Return(nil, nil)
			metric.EXPECT().EmitGetExptResult(int64(3), false)
			lwt.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(false)
			if tc.accelerator {
				var pageItems []*entity.ExptItemResult
				if tc.hasItem {
					pageItems = []*entity.ExptItemResult{{ItemID: 1}}
					turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{1}, entity.Page{}, false).
						Return(nil, int64(0), nil)
				}
				items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), gomock.Any(), false).
					Return(pageItems, int64(89), nil)
			} else if tc.cursor {
				turns.EXPECT().ListTurnResultWithCursor(gomock.Any(), int64(3), int64(1), nil, nil, 20, false).
					Return(nil, int64(89), nil, nil)
			} else {
				turns.EXPECT().ListTurnResult(gomock.Any(), int64(3), int64(1), nil, gomock.Any(), false).
					Return(nil, int64(89), nil)
			}
			svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns,
				ExperimentRepo: expts, ExptAnnotateRepo: annotations, Metric: metric, lwt: lwt}
			got, err := svc.MGetExperimentResult(context.Background(), &entity.MGetExperimentResultParam{
				SpaceID: 3, ExptIDs: []int64{1}, BaseExptID: gptr.Of(int64(1)),
				Page: entity.NewPage(6, 20), UseAccelerator: tc.accelerator, UseTurnListCursor: tc.cursor,
			})
			require.NoError(t, err)
			require.Empty(t, got.ItemResults)
			require.Equal(t, int64(89), got.Total)
		})
	}
}

func TestMGetExperimentResultEmptyPageSharedColumns(t *testing.T) {
	for _, source := range []entity.ExptEvalSetSourceType{entity.ExptEvalSetSourceType_SingleSet, entity.ExptEvalSetSourceType_MultiSetConfig} {
		for _, level := range []string{entity.SharedAccessLevelExecute, entity.SharedAccessLevelReadable} {
			for _, pageNumber := range []int{6, 1000} {
				t.Run(fmt.Sprintf("%v/%s/page_%d", source, level, pageNumber), func(t *testing.T) {
					ctrl := gomock.NewController(t)
					items := repomocks.NewMockIExptItemResultRepo(ctrl)
					turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
					expts := repomocks.NewMockIExperimentRepo(ctrl)
					annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
					metric := metricsmocks.NewMockExptMetric(ctrl)
					lwt := lwtmocks.NewMockILatestWriteTracker(ctrl)
					versions := servicemocks.NewMockEvaluationSetVersionService(ctrl)
					expt := &entity.Experiment{
						ID: 1, SpaceID: 3, ExptType: entity.ExptType_Offline, EvalSetSourceType: source,
						EvalSetID: 100, EvalSetVersionID: 101, EvalSetSpaceID: 7, EvalSetAccessLevel: level,
					}
					if source == entity.ExptEvalSetSourceType_MultiSetConfig {
						expt.EvalSetAccessLevel = entity.SharedAccessLevelReadable
						expt.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{
							{EvalSetID: 100, EvalSetVersionID: 101, SourceSpaceID: 7, AccessLevel: level},
						}}
					}
					expts.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(3)).Return([]*entity.Experiment{expt}, nil)
					expts.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(3)).Return(nil, nil)
					annotations.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), []int64{1}, int64(3)).Return(nil, nil)
					metric.EXPECT().EmitGetExptResult(int64(3), false)
					lwt.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(false)
					versions.EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(7), int64(101), gptr.Of(true), nil).
						Return(&entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{{Key: "input", Name: "private field", Description: "private description", TextSchema: "{}"}},
						}}, nil, nil)
					page := entity.NewPage(pageNumber, 20)
					items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(3), page, false).
						Return(nil, int64(89), nil)
					svc := ExptResultServiceImpl{ExptItemResultRepo: items, ExptTurnResultRepo: turns,
						ExperimentRepo: expts, ExptAnnotateRepo: annotations, Metric: metric, lwt: lwt,
						evaluationSetVersionService: versions}
					got, err := svc.MGetExperimentResult(context.Background(), &entity.MGetExperimentResultParam{
						SpaceID: 3, ExptIDs: []int64{1}, BaseExptID: gptr.Of(int64(1)), Page: page, UseAccelerator: true,
					})
					require.NoError(t, err)
					require.Empty(t, got.ItemResults)
					require.Equal(t, int64(89), got.Total)
					if level == entity.SharedAccessLevelExecute {
						require.Empty(t, got.ColumnEvalSetFields)
					} else {
						require.Len(t, got.ColumnEvalSetFields, 1)
						require.Equal(t, "private field", gptr.Indirect(got.ColumnEvalSetFields[0].Name))
					}
				})
			}
		}
	}
}
