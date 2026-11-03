// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestExptStartWithoutTurns(t *testing.T) {
	empty := &entity.EvaluationSetItem{ItemID: 1}
	valid := &entity.EvaluationSetItem{ItemID: 2, Turns: []*entity.Turn{{ID: 20}, {ID: 21}}}
	for _, mode := range []string{"submit", "trial", "selected_trial", "multiset", "retry_all", "retry_items"} {
		for _, tc := range []struct {
			name  string
			pages [][]*entity.EvaluationSetItem
			count int32
		}{
			{name: "all_empty", pages: [][]*entity.EvaluationSetItem{{empty}}},
			{name: "empty_page_then_valid", pages: [][]*entity.EvaluationSetItem{{empty}, {valid}}, count: 1},
			{name: "mixed", pages: [][]*entity.EvaluationSetItem{{empty, valid}}, count: 1},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				ctx := context.Background()
				event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "user"}, ExecEvalSetItemIDs: []int64{1, 2}}
				expt := &entity.Experiment{
					ID: 1, SpaceID: 3, TrialRunItemCount: 2,
					EvalSet:          &entity.EvaluationSet{ID: 10, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 11}},
					ExptTemplateMeta: &entity.ExptTemplateMeta{ID: 1},
				}
				itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
				itemRepo := repomocks.NewMockIExptItemResultRepo(ctrl)
				turnRepo := repomocks.NewMockIExptTurnResultRepo(ctrl)
				refRepo := repomocks.NewMockIExptItemRefRepo(ctrl)
				statsRepo := repomocks.NewMockIExptStatsRepo(ctrl)
				exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
				idgen := idgenmocks.NewMockIIDGenerator(ctrl)
				idem := idemmocks.NewMockIdempotentService(ctrl)
				config := configmocks.NewMockIConfiger(ctrl)
				resultSvc := svcmocks.NewMockExptResultService(ctrl)
				base := &ExptSubmitExec{evaluationSetItemService: itemService, exptItemResultRepo: itemRepo,
					exptTurnResultRepo: turnRepo, exptItemRefRepo: refRepo, exptStatsRepo: statsRepo,
					exptRepo: exptRepo, idgenerator: idgen, idem: idem, configer: config, resultSvc: resultSvc}

				var allItems []*entity.EvaluationSetItem
				for _, page := range tc.pages {
					allItems = append(allItems, page...)
				}
				if mode == "selected_trial" || mode == "retry_items" {
					itemService.EXPECT().BatchGetEvaluationSetItems(ctx, gomock.Any()).Return(allItems, nil)
				} else {
					for pageIdx, page := range tc.pages {
						var next *string
						if pageIdx < len(tc.pages)-1 {
							next = gptr.Of("next")
						}
						itemService.EXPECT().ListEvaluationSetItems(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, param *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
							if pageIdx == 0 {
								assert.Nil(t, param.PageToken)
							} else {
								assert.Equal(t, "next", gptr.Indirect(param.PageToken))
							}
							return page, gptr.Of(int64(len(allItems))), nil, next, nil
						})
					}
				}
				idem.EXPECT().Exist(ctx, gomock.Any()).Return(false, nil)
				idem.EXPECT().Set(ctx, gomock.Any(), gomock.Any()).Return(nil)
				config.EXPECT().GetExptExecConf(ctx, int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 1})
				exptRepo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, updated *entity.Experiment) error {
					assert.Equal(t, entity.ExptStatus_Processing, updated.Status)
					return nil
				})
				resultSvc.EXPECT().UpsertExptTurnResultFilter(ctx, int64(3), int64(1), gomock.Any()).Return(nil).AnyTimes()

				var stats *entity.ExptStats
				if mode == "retry_all" || mode == "retry_items" {
					statsRepo.EXPECT().Get(ctx, int64(1), int64(3)).Return(&entity.ExptStats{SuccessItemCnt: tc.count}, nil)
					statsRepo.EXPECT().Save(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, got *entity.ExptStats) error {
						stats = got
						return nil
					})
				} else {
					statsRepo.EXPECT().UpdateByExptID(ctx, int64(1), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, got *entity.ExptStats) error {
						stats = got
						return nil
					})
				}

				var itemResults []*entity.ExptItemResult
				var turnResults []*entity.ExptTurnResult
				var runLogs []*entity.ExptItemResultRunLog
				if tc.count > 0 {
					var nextID int64
					idgen.EXPECT().GenMultiIDs(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, count int) ([]int64, error) {
						require.Positive(t, count)
						ids := make([]int64, count)
						for i := range ids {
							nextID++
							ids[i] = nextID
						}
						return ids, nil
					}).AnyTimes()
					itemRepo.EXPECT().BatchCreateNXRunLogs(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, got []*entity.ExptItemResultRunLog) error {
						require.Len(t, got, 1)
						runLogs = append(runLogs, got...)
						return nil
					})
					if mode == "retry_all" || mode == "retry_items" {
						itemRepo.EXPECT().UpdateItemsResult(ctx, int64(3), int64(1), []int64{2}, gomock.Any()).Return(nil)
						turnRepo.EXPECT().UpdateTurnResults(ctx, int64(1), []*entity.ItemTurnID{{ItemID: 2, TurnID: 20}, {ItemID: 2, TurnID: 21}}, int64(3), gomock.Any()).Return(nil)
						turnRepo.EXPECT().UpdateTurnRunLogWithItemIDs(ctx, int64(3), int64(1), int64(2), []int64{2}, gomock.Any()).Return(nil)
						if mode == "retry_items" {
							itemRepo.EXPECT().MGetItemResults(ctx, int64(1), []int64{2}, int64(3)).Return([]*entity.ExptItemResult{{ItemID: 2, Status: entity.ItemRunState_Success}}, nil)
						}
					} else {
						itemRepo.EXPECT().BatchCreateNX(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, got []*entity.ExptItemResult) error {
							itemResults = append(itemResults, got...)
							return nil
						})
						turnRepo.EXPECT().BatchCreateNX(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, got []*entity.ExptTurnResult) error {
							turnResults = append(turnResults, got...)
							return nil
						})
						if mode == "multiset" {
							refRepo.EXPECT().BatchCreate(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, refs []*entity.ExptItemRef) error {
								require.Len(t, refs, 1)
								assert.Equal(t, int64(2), refs[0].ItemID)
								return nil
							})
						}
					}
				}

				var exec entity.ExptSchedulerMode = base
				switch mode {
				case "trial", "selected_trial":
					exec = &ExptTrialRunExec{ExptSubmitExec: base}
					if mode == "selected_trial" {
						event.Ext = map[string]string{"__item_ids": "[1,2]"}
					}
				case "multiset":
					expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
					expt.EvalConf = &entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 10, EvalSetVersionID: 11}}}
				case "retry_all":
					exec = &ExptRetryAllExec{evaluationSetItemService: itemService, exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo,
						exptStatsRepo: statsRepo, exptRepo: exptRepo, idgenerator: idgen, idem: idem, configer: config}
				case "retry_items":
					exec = &ExptRetryItemsExec{evaluationSetItemService: itemService, exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo,
						exptStatsRepo: statsRepo, exptRepo: exptRepo, idgenerator: idgen, idem: idem, configer: config}
				}
				require.NoError(t, exec.ExptStart(ctx, event, expt))
				require.NotNil(t, stats)
				assert.Equal(t, tc.count, stats.PendingItemCnt)
				assert.Zero(t, stats.SuccessItemCnt)
				assert.Zero(t, stats.FailItemCnt)
				assert.Len(t, runLogs, int(tc.count))
				if tc.count > 0 {
					assert.Equal(t, int64(2), runLogs[0].ItemID)
				}
				if mode != "retry_all" && mode != "retry_items" {
					assert.Len(t, itemResults, int(tc.count))
					assert.Len(t, turnResults, 2*int(tc.count))
					if tc.count > 0 {
						assert.Equal(t, int64(2), itemResults[0].ItemID)
						assert.Zero(t, itemResults[0].ItemIdx)
					}
				}
			})
		}
	}
}

func TestExptWithoutTurnsCompletesWithZeroStats(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	mgr := newTestExptManager(ctrl)
	itemRepo := mgr.itemResultRepo.(*repomocks.MockIExptItemResultRepo)
	turnRepo := mgr.turnResultRepo.(*repomocks.MockIExptTurnResultRepo)
	exptRepo := mgr.exptRepo.(*repomocks.MockIExperimentRepo)
	resultSvc := mgr.exptResultService.(*svcmocks.MockExptResultService)
	itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{UserID: "user"}}
	expt := &entity.Experiment{ID: 1, SpaceID: 3, ExptType: entity.ExptType_Offline,
		EvalSet: &entity.EvaluationSet{ID: 10, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 11}}}
	itemService.EXPECT().ListEvaluationSetItems(ctx, gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: 1}}, gptr.Of(int64(1)), nil, nil, nil)
	mgr.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(ctx, gomock.Any()).Return(false, nil).AnyTimes()
	mgr.idem.(*idemmocks.MockIdempotentService).EXPECT().Set(ctx, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	exptRepo.EXPECT().GetByID(ctx, int64(1), int64(3)).Return(expt, nil).AnyTimes()
	var statuses []entity.ExptStatus
	exptRepo.EXPECT().Update(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, got *entity.Experiment) error {
		statuses = append(statuses, got.Status)
		expt.Status = got.Status
		return nil
	}).Times(2)
	mgr.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(ctx, int64(1), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, stats *entity.ExptStats) error {
		assert.Zero(t, stats.PendingItemCnt)
		assert.Zero(t, stats.SuccessItemCnt)
		assert.Zero(t, stats.FailItemCnt)
		assert.Zero(t, stats.ProcessingItemCnt)
		assert.Zero(t, stats.TerminatedItemCnt)
		return nil
	}).Times(2)
	resultSvc.EXPECT().UpsertExptTurnResultFilter(ctx, int64(3), int64(1), nil).Return(nil)
	itemRepo.EXPECT().ScanItemRunLogs(ctx, int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).Return(nil, int64(0), nil)
	itemRepo.EXPECT().ListItemResultsByExptID(ctx, int64(1), int64(3), gomock.Any(), false).Return(nil, int64(0), nil)
	turnRepo.EXPECT().ListTurnResult(ctx, int64(3), int64(1), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), nil).AnyTimes()

	// Exercise the real final statistics and completion code, with no item or turn rows.
	calculator := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo}
	resultSvc.EXPECT().CalculateStats(ctx, int64(1), int64(3), event.Session).DoAndReturn(calculator.CalculateStats)
	resultSvc.EXPECT().GetIncompleteTurns(ctx, int64(1), int64(3), event.Session).DoAndReturn(calculator.GetIncompleteTurns)
	runLog := &entity.ExptRunLog{ExptID: 1, ExptRunID: 2}
	mgr.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(ctx, int64(1), int64(2)).Return(runLog, nil)
	mgr.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Save(ctx, runLog).Return(nil)
	mgr.mutex.(*lockmocks.MockILocker).EXPECT().UnlockForce(ctx, gomock.Any()).Return(true, nil)
	mgr.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(ctx, int64(3), gomock.Any(), event.Session).Return(nil)
	mgr.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().PublishExptAggrResultEvent(ctx, gomock.Any(), gomock.Any()).Return(nil)
	mgr.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptLifecycleEvent(ctx, gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mgr.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecResult(int64(3), int64(entity.ExptType_Offline), int64(entity.ExptStatus_Success), gomock.Any())

	exec := &ExptSubmitExec{manager: mgr, evaluationSetItemService: itemService, exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo,
		exptStatsRepo: mgr.statsRepo, exptRepo: exptRepo, idgenerator: mgr.idgenerator, idem: mgr.idem, configer: mgr.configer, resultSvc: resultSvc}
	require.NoError(t, exec.ExptStart(ctx, event, expt))
	nextTick, err := exec.ExptEnd(ctx, event, expt, 0, 0)
	require.NoError(t, err)
	assert.False(t, nextTick)
	assert.Equal(t, []entity.ExptStatus{entity.ExptStatus_Processing, entity.ExptStatus_Success}, statuses)
	assert.Equal(t, int64(entity.ExptStatus_Success), runLog.Status)
	assert.Zero(t, runLog.PendingCnt)
	assert.Zero(t, runLog.SuccessCnt)
	assert.Zero(t, runLog.FailCnt)
}

func TestInvokeWithoutTurnsIsNoop(t *testing.T) {
	ctrl := gomock.NewController(t)
	mgr := newTestExptManager(ctrl)
	mgr.itemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().GetItemIDListByExptID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
	require.NoError(t, mgr.Invoke(context.Background(), &entity.InvokeExptReq{
		ExptID: 1, RunID: 2, SpaceID: 3, Items: []*entity.EvaluationSetItem{{ItemID: 1}},
	}))
}

func TestResetRetryRunLogsWithoutTurns(t *testing.T) {
	ctrl := gomock.NewController(t)
	itemIDs, err := resetRetryRunLogsForItems(context.Background(), retryItemResetDeps{
		idgenerator: idgenmocks.NewMockIIDGenerator(ctrl), exptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl),
		exptTurnResultRepo: repomocks.NewMockIExptTurnResultRepo(ctrl),
	}, &entity.ExptScheduleEvent{ExptID: 1}, []*entity.EvaluationSetItem{{ItemID: 1}}, nil)
	require.NoError(t, err)
	assert.Empty(t, itemIDs)
}
