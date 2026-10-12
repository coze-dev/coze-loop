// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idm "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	idemm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type ordinaryCompletionSource struct {
	repo.IHookFinalizationRepo
	source  *entity.HookFinalizationSource
	manager *ExptMangerImpl
}

func (s ordinaryCompletionSource) ReadFinalizationOwner(context.Context, string) (string, error) {
	return "", nil
}

func (s ordinaryCompletionSource) WithManualCancellationFence(ctx context.Context, key entity.HookRunKey, fn func(context.Context, repo.ManualCancellationRepositories) error) error {
	if s.source.Key != key || s.source.Experiment.LatestRunID != key.RunID || s.source.Managed {
		return entity.ErrHookStoreConflict
	}
	m := s.manager
	return fn(ctx, repo.ManualCancellationRepositories{Source: s, Experiments: m.exptRepo, Runs: m.runLogRepo, Items: m.itemResultRepo, Turns: m.turnResultRepo, Stats: m.statsRepo})
}

type ordinaryItemLocker struct {
	lock.ILocker
	busy   bool
	onLock func()
}

func (l ordinaryItemLocker) LockWithRenew(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
	if l.onLock != nil {
		l.onLock()
	}
	return !l.busy, ctx, func() {}, nil
}

func (ordinaryItemLocker) Unlock(string) (bool, error) { return true, nil }

type ordinaryResultService struct{ ExptResultService }

func (s ordinaryResultService) WithManualCancellationRepositories(r repo.ManualCancellationRepositories) ExptResultService {
	return ordinaryResultService{s.ExptResultService.(*ExptResultServiceImpl).WithManualCancellationRepositories(r)}
}

func (ordinaryResultService) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

func (s ordinaryCompletionSource) ReadFinalizationSource(context.Context, entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	return s.source, nil
}

func TestOrdinaryCancellationClosesBeforeRunStatistics(t *testing.T) {
	for _, completion := range []string{"manual", "fail-retry", "fail-retry-preeval"} {
		for _, state := range []entity.ItemRunState{entity.ItemRunState_Queueing, entity.ItemRunState_Processing} {
			if completion == "fail-retry-preeval" && state == entity.ItemRunState_Queueing {
				continue
			}
			t.Run(fmt.Sprintf("%s/%v", completion, state), func(t *testing.T) {
				failRetry := completion == "fail-retry" || completion == "fail-retry-preeval"
				ctrl := gomock.NewController(t)
				m := newTestExptManager(ctrl)
				ctx := context.Background()
				key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
				expt := &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: 30, Status: entity.ExptStatus_Terminating}
				run := &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, Status: int64(entity.ExptStatus_Terminating)}
				if failRetry {
					run.Mode = int32(entity.EvaluationModeFailRetry)
				}
				store := ordinaryCompletionSource{manager: m, source: &entity.HookFinalizationSource{Key: key, RunLog: run, Experiment: expt}}
				m.finalization = &ExptManagerFinalizationDependencies{Repository: store, Owners: store, NewItemLocker: func() lock.ILocker { return ordinaryItemLocker{} }}
				item := &entity.ExptItemResultRunLog{ID: 40, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, Status: int32(state)}
				canonical := &entity.ExptItemResult{ID: 41, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, Status: state}
				turn := &entity.ExptTurnResult{SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 60, Status: int32(entity.TurnRunState_Processing)}
				turnLog := &entity.ExptTurnResultRunLog{ID: 42, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 60, Status: entity.TurnRunState_Processing}
				successTurn := &entity.ExptTurnResult{ID: 43, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 61, Status: int32(entity.TurnRunState_Success)}
				successLog := &entity.ExptTurnResultRunLog{ID: 44, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 61, Status: entity.TurnRunState_Success}
				itemRepo := m.itemResultRepo.(*rm.MockIExptItemResultRepo)
				turnRepo := m.turnResultRepo.(*rm.MockIExptTurnResultRepo)
				m.exptResultService = ordinaryResultService{&ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo, ExptStatsRepo: m.statsRepo, idgen: m.idgenerator}}
				// Storage doubles retain state; statistics below read the post-cleanup result.
				itemRepo.EXPECT().ScanItemRunLogs(gomock.Any(), int64(20), int64(30), gomock.Any(), gomock.Any(), gomock.Any(), int64(10)).Return([]*entity.ExptItemResultRunLog{item}, int64(40), nil).AnyTimes()
				itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), int64(20), int64(30), []int64{50}, int64(10)).Return([]*entity.ExptItemResultRunLog{item}, nil).AnyTimes()
				turnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), int64(10), int64(20), int64(30), []int64{50}, entity.TurnRunState_Terminal).DoAndReturn(func(context.Context, int64, int64, int64, []int64, entity.TurnRunState) error {
					turnLog.Status = entity.TurnRunState_Terminal
					successLog.Status = entity.TurnRunState_Terminal
					return nil
				}).AnyTimes()
				itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), int64(20), int64(30), []int64{50}, gomock.Any(), int64(10)).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, fields map[string]any, _ int64) error {
					if status, ok := fields["status"].(int32); ok {
						item.Status = status
					}
					if state, ok := fields["result_state"].(int32); ok {
						item.ResultState = state
					}
					return nil
				}).AnyTimes()
				itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), int64(10), int64(20), []int64{50}, gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, fields map[string]any) error {
					if status, ok := fields["status"].(int32); ok {
						canonical.Status = entity.ItemRunState(status)
					}
					return nil
				}).AnyTimes()
				turnRepo.EXPECT().BatchGet(gomock.Any(), int64(10), int64(20), []int64{50}).Return([]*entity.ExptTurnResult{turn, successTurn}, nil).AnyTimes()
				turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), int64(20), gomock.Any(), int64(10), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, _ []*entity.ItemTurnID, _ int64, fields map[string]any) error {
					turn.Status = fields["status"].(int32)
					if runID, ok := fields["expt_run_id"].(int64); ok {
						turn.ExptRunID = runID
					}
					return nil
				}).AnyTimes()
				m.exptRepo.(*rm.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).Return(expt, nil).AnyTimes()
				m.evalTargetService = nil
				itemRepo.EXPECT().GetItemRunLog(gomock.Any(), int64(20), int64(30), int64(50), int64(10)).Return(item, nil).AnyTimes()
				hasTurnLog := state != entity.ItemRunState_Queueing
				hasSuccessLog := !failRetry
				if failRetry {
					hasTurnLog = false
					successTurn.ExptRunID = 29
					successTurn.TargetResultID, successTurn.LogID = 701, "retained-success"
					successTurn.EvaluatorResults = &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "check", RecordID: 801}}}
				}
				if !hasTurnLog || !hasSuccessLog {
					id := int64(41)
					m.idgenerator.(*idm.MockIIDGenerator).EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) { id++; return id, nil }).AnyTimes()
				}
				turnRepo.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(20), int64(30), int64(50), int64(10)).DoAndReturn(func(context.Context, int64, int64, int64, int64) ([]*entity.ExptTurnResultRunLog, error) {
					var result []*entity.ExptTurnResultRunLog
					if hasTurnLog {
						result = append(result, turnLog)
					}
					if hasSuccessLog {
						result = append(result, successLog)
					}
					return result, nil
				}).AnyTimes()
				itemRepo.EXPECT().GetItemTurnResults(gomock.Any(), int64(10), int64(20), int64(50)).Return([]*entity.ExptTurnResult{turn, successTurn}, nil).AnyTimes()
				itemRepo.EXPECT().BatchGet(gomock.Any(), int64(10), int64(20), []int64{50}).Return([]*entity.ExptItemResult{canonical}, nil).AnyTimes()
				turnRepo.EXPECT().SaveTurnResults(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				turnRepo.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, updates []*entity.ExptTurnResultRunLog) error {
					for _, updated := range updates {
						if updated.TurnID == 60 {
							*turnLog = *updated
							hasTurnLog = true
						} else {
							*successLog = *updated
							hasSuccessLog = true
						}
					}
					return nil
				}).AnyTimes()
				m.statsRepo.(*rm.MockIExptStatsRepo).EXPECT().ArithOperateCount(gomock.Any(), int64(20), int64(10), gomock.Any()).Return(nil).AnyTimes()
				itemRepo.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(20), int64(10), gomock.Any(), false).Return([]*entity.ExptItemResult{canonical}, int64(1), nil).AnyTimes()
				turnRepo.EXPECT().ListTurnResult(gomock.Any(), int64(10), int64(20), nil, gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, *entity.ExptTurnResultFilter, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
					return []*entity.ExptTurnResult{turn, successTurn}, 2, nil
				}).AnyTimes()
				m.runLogRepo.(*rm.MockIExptRunLogRepo).EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, saved *entity.ExptRunLog) error { *run = *saved; return nil })
				var stats *entity.ExptStats
				m.statsRepo.(*rm.MockIExptStatsRepo).EXPECT().UpdateByExptID(gomock.Any(), int64(20), int64(10), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, updated *entity.ExptStats) error { stats = updated; return nil })
				updates := 1
				if failRetry {
					updates++
				}
				m.exptRepo.(*rm.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, updated *entity.Experiment) error {
					expt.Status, expt.StatusMessage = updated.Status, updated.StatusMessage
					return nil
				}).Times(updates)
				m.quotaRepo.(*rm.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.mtr.(*metricsm.MockExptMetric).EXPECT().EmitExptExecResult(int64(10), gomock.Any(), gomock.Any(), gomock.Any())
				m.publisher.(*eventsm.MockExptEventPublisher).EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				if failRetry {
					// Run the actual retry initializer: only the failed turn moves to Run B.
					turn.ExptRunID, turn.Status = 29, int32(entity.TurnRunState_Fail)
					m.idem.(*idemm.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
					m.idem.(*idemm.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					turnRepo.EXPECT().ScanTurnResults(gomock.Any(), int64(20), gomock.Any(), int64(0), int64(50), int64(10)).Return([]*entity.ExptTurnResult{turn}, int64(60), nil)
					turnRepo.EXPECT().ScanTurnResults(gomock.Any(), int64(20), gomock.Any(), int64(60), int64(50), int64(10)).Return(nil, int64(60), nil)
					turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(20), int64(30), []int64{50}, int64(10)).Return(nil, nil)
					m.idgenerator.(*idm.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 1).Return([]int64{40}, nil).AnyTimes()
					itemRepo.EXPECT().BatchCreateNXRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptItemResultRunLog) error { *item = *rows[0]; return nil })
					m.statsRepo.(*rm.MockIExptStatsRepo).EXPECT().Get(gomock.Any(), int64(20), int64(10)).Return(&entity.ExptStats{FailItemCnt: 1}, nil)
					m.statsRepo.(*rm.MockIExptStatsRepo).EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
					turnRepo.EXPECT().CreateTurnEvaluatorRefs(gomock.Any(), gomock.Any()).Return(nil)
					mode := NewExptFailRetryMode(m, itemRepo, m.statsRepo, turnRepo, m.idgenerator, m.exptRepo, m.idem, m.configer, m.publisher, nil, nil)
					require.NoError(t, mode.ExptStart(ctx, &entity.ExptScheduleEvent{SpaceID: 10, ExptID: 20, ExptRunID: 30, ExptRunMode: entity.EvaluationModeFailRetry}, expt))
					require.Equal(t, int64(29), successTurn.ExptRunID)
					require.Equal(t, int64(30), turn.ExptRunID)
					require.False(t, hasSuccessLog)
					expt.Status = entity.ExptStatus_Terminating
				}
				if completion == "fail-retry-preeval" {
					item.Status, canonical.Status = int32(entity.ItemRunState_Processing), entity.ItemRunState_Processing
					target := sm.NewMockIEvalTargetService(ctrl)
					records := sm.NewMockEvaluatorRecordService(ctrl)
					expt.TargetVersionID = 700
					turnRepo.EXPECT().GetItemTurnResults(gomock.Any(), int64(20), int64(50), int64(10)).Return([]*entity.ExptTurnResult{turn, successTurn}, nil)
					turnRepo.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(10), []int64{0, 43}).Return([]*entity.ExptTurnEvaluatorResultRef{{ExptTurnResultID: 43, EvaluatorResultID: 801, SourceType: int32(entity.EvaluatorRecordSourceTypeInline), InlineKey: "check"}}, nil).Times(2)
					m.idgenerator.(*idm.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{42, 44}, nil)
					target.EXPECT().GetRecordByID(gomock.Any(), int64(10), int64(701)).Return(&entity.EvalTargetRecord{ID: 701, Status: gptr.Of(entity.EvalTargetRunStatusSuccess)}, nil)
					records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{801}, false, false).Return([]*entity.EvaluatorRecord{{ID: 801, ExperimentID: 20, ExperimentRunID: 29, ItemID: 50, TurnID: 61, Status: entity.EvaluatorRunStatusSuccess, SourceType: entity.EvaluatorRecordSourceTypeInline, InlineKey: "check"}}, nil)
					turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(20), int64(29), []int64{50}, int64(10)).Return([]*entity.ExptTurnResultRunLog{{SpaceID: 10, ExptID: 20, ExptRunID: 29, ItemID: 50, TurnID: 61, TargetResultID: 701, EvaluatorResultIds: &entity.EvaluatorResults{Inline: []*entity.InlineEvalResult{{InlineKey: "check", RecordID: 801}}}}}, nil)
					turnRepo.EXPECT().BatchCreateNXRunLog(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
						for _, row := range rows {
							if row.TurnID == 60 {
								*turnLog = *row
								hasTurnLog = true
							} else {
								*successLog = *row
								hasSuccessLog = true
							}
						}
						return nil
					})
					event := &entity.ExptItemEvalEvent{SpaceID: 10, ExptID: 20, ExptRunID: 30, EvalSetItemID: 50, ExptRunMode: entity.EvaluationModeFailRetry, MaxRetryTimes: 3, Session: &entity.Session{}}
					eiec := &entity.ExptItemEvalCtx{Event: event, Expt: expt, ExistItemEvalResult: &entity.ExptItemEvalResult{ItemResultRunLog: item}, EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{{ID: 61}, {ID: 60}}, BaseInfo: &entity.BaseInfo{CreatedAt: gptr.Of(int64(1))}}}
					pre := &ExptRecordEvalModeFailRetry{resultSvc: m.exptResultService, exptTurnResultRepo: turnRepo, idgen: m.idgenerator, evalTargetService: target, evaluatorRecordSvc: records}
					require.NoError(t, pre.PreEval(ctx, eiec))
					require.True(t, hasSuccessLog)
					require.Equal(t, entity.TurnRunState_Processing, successLog.Status)
					require.Equal(t, int64(29), successTurn.ExptRunID)
					require.Equal(t, int64(801), successLog.EvaluatorResultIds.Inline[0].RecordID)
					// The cancellation repository reads canonical rows without evaluator-ref joins.
					successTurn.EvaluatorResults = nil
					failure := errors.New("temporary target record read failure")
					target.EXPECT().GetRecordByID(gomock.Any(), int64(10), int64(701)).Return(nil, failure)
					cfg := m.configer.(*cm.MockIConfiger)
					cfg.EXPECT().BuildEvalExt(gomock.Any(), int64(10), gomock.Any()).Return(nil)
					cfg.EXPECT().GetErrRetryConf(gomock.Any(), int64(10), failure).Return(&entity.RetryConf{RetryTimes: 3}).Times(2)
					executor := &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo, Configer: cfg, evalTargetService: target}
					_, err := executor.buildExptTurnEvalCtx(ctx, eiec.EvalSetItem.Turns[0], eiec, nil)
					require.ErrorIs(t, err, failure)
					require.ErrorIs(t, executor.CompleteItemRun(ctx, eiec, err), failure)
					m.mtr.(*metricsm.MockExptMetric).EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
					m.publisher.(*eventsm.MockExptEventPublisher).EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					consumer := &ExptItemEventEvalServiceImpl{configer: cfg, metric: m.mtr, publisher: m.publisher}
					require.NoError(t, consumer.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return failure })(ctx, event))
					require.Equal(t, entity.TurnRunState_Processing, successLog.Status)
					require.NotEqual(t, int32(entity.ExptItemResultStateLogged), item.ResultState)
				}
				require.NoError(t, m.CompleteManualCancellation(ctx, key, &entity.Session{}))
				require.Equal(t, entity.ExptStatus_Terminated, expt.Status)
				require.Equal(t, int32(0), stats.PendingItemCnt)
				require.Equal(t, int32(0), stats.ProcessingItemCnt)
				require.Equal(t, int32(1), stats.TerminatedItemCnt)
				require.Equal(t, int64(entity.ExptStatus_Terminated), run.Status)
				require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
				require.Equal(t, int32(entity.ExptItemResultStateResulted), item.ResultState)
				require.Equal(t, int32(0), run.PendingCnt)
				require.Equal(t, int32(0), run.ProcessingCnt)
				require.Equal(t, int32(1), run.TerminatedCnt)
				require.Equal(t, int32(1), run.SuccessCnt)
				require.Equal(t, entity.TurnRunState_Terminal, turnLog.Status)
				require.Equal(t, entity.TurnRunState_Success, successLog.Status)
				require.Equal(t, entity.ItemRunState_Terminal, canonical.Status)
				if failRetry {
					require.Equal(t, int64(701), successLog.TargetResultID)
					require.Equal(t, int64(801), successLog.EvaluatorResultIds.Inline[0].RecordID)
					require.Equal(t, "retained-success", successTurn.LogID)
					require.Equal(t, int32(entity.TurnRunState_Success), successTurn.Status)
					require.Equal(t, int64(701), successTurn.TargetResultID)
					if completion == "fail-retry-preeval" {
						require.Equal(t, int64(44), successLog.ID)
						require.Equal(t, int64(30), successLog.ExptRunID)
						require.Empty(t, successLog.ErrMsg)
					}
				}
			})
		}
	}
}

func TestOrdinaryCancellationCannotUnlockOrPublishOnCleanupFailure(t *testing.T) {
	for _, scenario := range []string{"busy", "successor-after-lock", "turn-write-failure"} {
		t.Run(scenario, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := newTestExptManager(ctrl)
			key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
			source := &entity.HookFinalizationSource{Key: key, Experiment: &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: 30, Status: entity.ExptStatus_Terminating}, RunLog: &entity.ExptRunLog{ID: 30, SpaceID: 10, ExptID: 20, ExptRunID: 30, Status: int64(entity.ExptStatus_Terminating)}}
			locker := ordinaryItemLocker{busy: scenario == "busy"}
			if scenario == "successor-after-lock" {
				locker.onLock = func() { source.Experiment.LatestRunID = 31 }
			}
			store := ordinaryCompletionSource{manager: m, source: source}
			m.finalization = &ExptManagerFinalizationDependencies{Repository: store, Owners: store, NewItemLocker: func() lock.ILocker { return locker }}
			m.exptResultService = ordinaryResultService{&ExptResultServiceImpl{ExptItemResultRepo: m.itemResultRepo, ExptTurnResultRepo: m.turnResultRepo, ExptStatsRepo: m.statsRepo}}
			item := &entity.ExptItemResultRunLog{ID: 40, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, Status: int32(entity.ItemRunState_Processing)}
			m.itemResultRepo.(*rm.MockIExptItemResultRepo).EXPECT().ScanItemRunLogs(gomock.Any(), int64(20), int64(30), nil, int64(0), int64(100), int64(10)).Return([]*entity.ExptItemResultRunLog{item}, int64(40), nil)
			if scenario == "turn-write-failure" {
				m.itemResultRepo.(*rm.MockIExptItemResultRepo).EXPECT().GetItemRunLog(gomock.Any(), int64(20), int64(30), int64(50), int64(10)).Return(item, nil)
				m.itemResultRepo.(*rm.MockIExptItemResultRepo).EXPECT().GetItemTurnResults(gomock.Any(), int64(10), int64(20), int64(50)).Return([]*entity.ExptTurnResult{{SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 60}}, nil)
				m.turnResultRepo.(*rm.MockIExptTurnResultRepo).EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(20), int64(30), int64(50), int64(10)).Return([]*entity.ExptTurnResultRunLog{{ID: 42, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 60, Status: entity.TurnRunState_Processing}}, nil)
				m.turnResultRepo.(*rm.MockIExptTurnResultRepo).EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(errors.New("injected write failure"))
			}
			require.Error(t, m.CompleteManualCancellation(context.Background(), key, &entity.Session{}))
			require.Equal(t, int64(entity.ExptStatus_Terminating), source.RunLog.Status)
			require.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
		})
	}
}

func TestOrdinaryCancellationRejectsSupersededRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	m.finalization = &ExptManagerFinalizationDependencies{Repository: ordinaryCompletionSource{source: &entity.HookFinalizationSource{
		Key: key, Experiment: &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: 31},
		RunLog: &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, Status: int64(entity.ExptStatus_Terminating)},
	}}}
	// No legacy writes, statistics or force-unlock may reach the successor.
	require.ErrorIs(t, m.CompleteManualCancellation(context.Background(), key, &entity.Session{}), entity.ErrHookStoreConflict)
}

func TestOrdinaryCancellationDoesNotReclassifyCompletedRun(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed} {
		for _, exptStatus := range []entity.ExptStatus{entity.ExptStatus_Processing, status} {
			t.Run(fmt.Sprintf("run-%v/expt-%v", status, exptStatus), func(t *testing.T) {
				m := newTestExptManager(gomock.NewController(t))
				key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
				m.finalization = &ExptManagerFinalizationDependencies{Repository: ordinaryCompletionSource{source: &entity.HookFinalizationSource{
					Key: key, Experiment: &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: 30, Status: exptStatus},
					RunLog: &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, Status: int64(status)},
				}}}
				// The manual entry must not reclassify completed Runs.
				require.ErrorIs(t, m.CompleteManualCancellation(context.Background(), key, &entity.Session{}), entity.ErrHookStoreConflict)
			})
		}
	}
}
