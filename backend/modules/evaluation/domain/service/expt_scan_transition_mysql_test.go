// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestScanTransitionMySQLCompleteThenArchive(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[failed], func(t *testing.T) {
			ctx := context.Background()
			dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
			if dsn == "" {
				t.Skip("EXPT_RETRY_MYSQL_DSN missing; real MySQL not executed")
			}
			provider, err := db.NewDB(mysqldriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			conn := provider.NewSession(ctx, db.WithMaster())
			var identity struct{ Version, Schema string }
			require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
			require.Equal(t, "expt_running_retry_test", identity.Schema)
			t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)
			base := int64(870000000000000000) + time.Now().UnixNano()%1000000000000
			space, expt, run, item, turn := base+1, base+2, base+3, int64(11), int64(21)
			t.Cleanup(func() {
				require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", space, expt).Error)
				for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_stats", "expt_run_log"} {
					require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", space, expt).Error)
				}
				sqlDB, err := conn.DB()
				require.NoError(t, err)
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, conn.Exec("INSERT INTO experiment(id,space_id,latest_run_id,status,expt_type) VALUES(?,?,?,?,?)", expt, space, run, entity.ExptStatus_Processing, entity.ExptType_Offline).Error)
			require.NoError(t, conn.Create(&model.ExptRunLog{ID: run, SpaceID: space, ExptID: expt, ExptRunID: run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
			require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 4, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 31, Status: int32(entity.ItemRunState_Processing)}).Error)
			require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 5, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 31, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault)), RetryTimes: 3}).Error)
			require.NoError(t, conn.Create(&model.ExptStats{ID: base + 6, SpaceID: space, ExptID: expt, ProcessingCnt: 1}).Error)
			require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 7, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 31, TurnID: turn, Status: int32(entity.TurnRunState_Processing), WeightedScore: gptr.Of(0.0)}).Error)
			status, turnStatus := entity.ItemRunState_Success, entity.TurnRunState_Success
			var evalErr error
			if failed {
				status = entity.ItemRunState_Fail
				turnStatus = entity.TurnRunState_Fail
				evalErr = errors.New("evaluator final failure")
			}
			// Invocation stages already persisted this turn; the real completion and archival methods follow.
			require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 8, SpaceID: space, ExptID: expt, ExptRunID: run, ItemID: item, ItemVersionID: 31, TurnID: turn, Status: int32(turnStatus)}).Error)
			itemRepo := experimentrepo.NewExptItemResultRepo(experimentmysql.NewExptItemResultDAO(provider))
			turnRepo := experimentrepo.NewExptTurnResultRepo(nil, experimentmysql.NewExptTurnResultDAO(provider), experimentmysql.NewExptTurnEvaluatorResultRefDAO(provider))
			statsRepo := experimentrepo.NewExptStatsRepo(experimentmysql.NewExptStatsDAO(provider))
			ctrl := gomock.NewController(t)
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), space).Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 2}}).AnyTimes()
			if failed {
				config.EXPECT().GetErrRetryConf(gomock.Any(), space, evalErr).Return(&entity.RetryConf{RetryTimes: 3}).Times(2)
			}
			event := &entity.ExptItemEvalEvent{SpaceID: space, ExptID: expt, ExptRunID: run, EvalSetItemID: item, ExptRunMode: entity.EvaluationModeSubmit, RetryTimes: 3, MaxRetryTimes: 3}
			exptDO := &entity.Experiment{ID: expt, SpaceID: space, EvalConf: &entity.EvaluationConfiguration{}, EvalSet: &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 41}}}
			executor := &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo, Configer: config}
			require.NoError(t, executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event, Expt: exptDO}, evalErr))
			canonical, err := itemRepo.BatchGet(ctx, space, expt, []int64{item})
			require.NoError(t, err)
			require.Equal(t, status, canonical[0].Status)
			beforeArchiveStats, err := statsRepo.Get(ctx, expt, space)
			require.NoError(t, err)
			require.Zero(t, beforeArchiveStats.ProcessingItemCnt)
			if failed {
				require.Equal(t, int32(1), beforeArchiveStats.FailItemCnt)
			} else {
				require.Equal(t, int32(1), beforeArchiveStats.SuccessItemCnt)
			}
			log, err := itemRepo.GetItemRunLog(ctx, expt, run, item, space)
			require.NoError(t, err)
			require.Equal(t, int32(status), log.Status)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), log.ResultState)
			scan := &exptBaseExec{exptItemResultRepo: itemRepo, configer: config}
			scheduleEvent := &entity.ExptScheduleEvent{SpaceID: space, ExptID: expt, ExptRunID: run, ExptRunMode: entity.EvaluationModeSubmit}
			candidates, occupied, complete, err := scan.ScanEvalItems(ctx, scheduleEvent, exptDO)
			require.NoError(t, err)
			require.Empty(t, candidates)
			require.Empty(t, occupied)
			require.Len(t, complete, 1)
			require.Equal(t, status, complete[0].State)
			source, err := itemRepo.GetCurrentRequest(ctx, space, expt, run, item)
			require.NoError(t, err)
			require.False(t, source.Matches(run, ""), "completed log cannot invoke again")
			consumer := &ExptItemEventEvalServiceImpl{exptItemResultRepo: itemRepo}
			invoked := false
			require.NoError(t, consumer.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error { invoked = true; return nil })(ctx, event))
			require.False(t, invoked, "completed log blocks PreEval and invocation before canonical archival")

			archive := &ExptResultServiceImpl{ExptItemResultRepo: itemRepo, ExptTurnResultRepo: turnRepo, ExptStatsRepo: statsRepo, resultPublishRepo: experimentrepo.NewExptResultPublishRepo(provider), resultPublishLocker: lateWriteLocker(t, ctrl)}
			_, err = archive.RecordItemRunLogs(ctx, expt, run, item, space, exptDO)
			require.NoError(t, err)
			canonical, err = itemRepo.BatchGet(ctx, space, expt, []int64{item})
			require.NoError(t, err)
			require.Equal(t, status, canonical[0].Status)
			log, err = itemRepo.GetItemRunLog(ctx, expt, run, item, space)
			require.NoError(t, err)
			require.Equal(t, int32(entity.ExptItemResultStateResulted), log.ResultState)
			stats, err := statsRepo.Get(ctx, expt, space)
			require.NoError(t, err)
			require.Equal(t, *beforeArchiveStats, *stats)
			require.Zero(t, stats.ProcessingItemCnt)
			if failed {
				require.Equal(t, int32(1), stats.FailItemCnt)
			} else {
				require.Equal(t, int32(1), stats.SuccessItemCnt)
			}
			before := *stats
			candidates, occupied, complete, err = scan.ScanEvalItems(ctx, scheduleEvent, exptDO)
			require.NoError(t, err)
			require.Empty(t, candidates)
			require.Empty(t, occupied)
			require.Empty(t, complete)
			_, err = archive.RecordItemRunLogs(ctx, expt, run, item, space, exptDO)
			require.NoError(t, err)
			stats, err = statsRepo.Get(ctx, expt, space)
			require.NoError(t, err)
			require.Equal(t, before, *stats)
			turns, err := itemRepo.GetItemTurnResults(ctx, space, expt, item)
			require.NoError(t, err)
			require.Nil(t, turns[0].WeightedScore, "no score record writes NULL instead of retaining old0")
			readFailure := errors.New("injected primary query unavailable")
			callbackName := "scan_transition_read_failure"
			require.NoError(t, conn.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) { _ = tx.AddError(readFailure) }))
			manager := svcmocks.NewMockIExptManager(ctrl)
			scheduler := &ExptSchedulerImpl{Manager: manager}
			brokerErr := scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				_, _, _, err := scan.ScanEvalItems(ctx, event, exptDO)
				return err
			})(ctx, scheduleEvent)
			require.NoError(t, conn.Callback().Query().Remove(callbackName))
			require.True(t, errno.NeedMQRetry(brokerErr))
			require.ErrorIs(t, brokerErr, readFailure)
			require.Zero(t, scheduleEvent.InfraErrorRetryTimes)
			stats, err = statsRepo.Get(ctx, expt, space)
			require.NoError(t, err)
			require.Equal(t, before, *stats)
			t.Log("REAL_MYSQL_SCAN_READ_ERROR broker retry retained cause, no finish/state/stats effects")
			t.Logf("REAL_MYSQL_COMPLETE_SCAN_ARCHIVE status=%d canonical Processing -> logged scan -> Resulted; stats once, score0 preserved", status)
		})
	}
}

func TestScanTransitionStorageErrorBrokerBoundary(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	manager := svcmocks.NewMockIExptManager(ctrl)
	want := errors.New("primary source read failed")
	items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return(nil, want)
	scan := &exptBaseExec{exptItemResultRepo: items}
	scheduler := &ExptSchedulerImpl{Manager: manager}
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit, InfraErrorRetryTimes: 7}
	err := scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		_, _, _, err := scan.ScanEvalItems(ctx, event, nil)
		require.True(t, errno.NeedMQRetry(err), "storage observation failure must use broker retry")
		require.ErrorIs(t, err, want)
		return err
	})(context.Background(), event)
	require.True(t, errno.NeedMQRetry(err))
	require.ErrorIs(t, err, want)
	require.Equal(t, 7, event.InfraErrorRetryTimes)
}

func TestScanTransitionBusinessErrorKeepsFinishPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := svcmocks.NewMockIExptManager(ctrl)
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeUnknown}
	want := errors.New("unknown expt mode: 0")
	manager.EXPECT().CompleteRun(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	manager.EXPECT().CompleteExpt(gomock.Any(), int64(1), gomock.Any(), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	scheduler := &ExptSchedulerImpl{Manager: manager}
	require.NoError(t, scheduler.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return want })(context.Background(), event))
	require.Zero(t, event.InfraErrorRetryTimes)
}

func TestScanTransitionNilStorageObservationRetries(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return([]*entity.ExptItemCurrentRequest{nil}, nil)
	scan := &exptBaseExec{exptItemResultRepo: items}
	_, _, _, err := scan.ScanEvalItems(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, nil)
	require.True(t, errno.NeedMQRetry(err))
}
