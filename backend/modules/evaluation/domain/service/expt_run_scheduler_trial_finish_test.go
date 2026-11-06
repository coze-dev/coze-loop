// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Exercise Trial -> embedded Submit.ExptEnd -> the real manager while the
// scheduler owns the non-reentrant tick lock. A pending finisher is deliberate:
// the test must reach the finish port, rather than fail on a second acquisition.
func TestOfflineTrialHeldTickReachesFinisher(t *testing.T) {
	ctrl := gomock.NewController(t)
	event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeTrialRun}
	expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing, TrialRunItemCount: 2}
	port := &finishRepoProbe{result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishPending}}
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).AnyTimes()
	locker := lockmocks.NewMockILocker(ctrl)
	held, acquisitions, reacquisitions := false, 0, 0
	locker.EXPECT().LockBackoffWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
		require.False(t, held)
		held = true
		acquisitions++
		return true, ctx, func() {}, nil
	})
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
		require.True(t, held)
		reacquisitions++
		return false, ctx, func() {}, nil
	}).AnyTimes()
	locker.EXPECT().Unlock("expt_run_exec_lock:1:2").DoAndReturn(func(string) (bool, error) { held = false; return true, nil })
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).Return(nil, int64(0), nil)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	calibration := &finishTickCalibration{ready: true}
	manager := &ExptMangerImpl{runFinishRepo: port, exptRepo: experiments, manualRetryLocker: func() lock.ILocker { return locker }}
	trial := &ExptTrialRunExec{ExptSubmitExec: &ExptSubmitExec{manager: manager, exptItemResultRepo: items, idem: idem, resultSvc: calibration}}
	scheduler := &ExptSchedulerImpl{Manager: manager, Mutex: locker}
	err := scheduler.HandleEventLock(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		require.True(t, held)
		next, err := trial.ExptEnd(ctx, event, expt, 0, 0)
		require.False(t, next)
		return err
	})(context.Background(), event)
	t.Logf("Trial held tick: acquisitions=%d reacquisitions=%d finisher_calls=%d err=%v", acquisitions, reacquisitions, port.calls, err)
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, 1, port.calls, "Trial must enter the real manager's finisher under its held tick")
	require.Equal(t, 1, calibration.calls)
	require.Equal(t, 1, acquisitions)
	require.Zero(t, reacquisitions, "Trial must not reacquire its non-reentrant tick lock")
	require.False(t, held)
}

func TestOfflineTrialFinishPendingRearmsTick(t *testing.T) {
	f := newFinishTickFixture(t, entity.EvaluationModeTrialRun, entity.ExptDispatchModeLegacy)
	f.result.ready = true
	var followup *entity.ExptScheduleEvent
	f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, next *entity.ExptScheduleEvent, _ *time.Duration) error {
		followup = next
		return nil
	})
	require.NoError(t, f.scheduler.schedule(context.Background(), f.event))
	require.Equal(t, 1, f.manager.calls)
	require.NotNil(t, followup)
	f.manager.disposition = entity.ExptRunFinishFinished
	trial := f.mode.ExptSchedulerMode.(*ExptTrialRunExec)
	trial.idem.(*idemmocks.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	require.NoError(t, f.scheduler.schedule(context.Background(), followup))
	require.Equal(t, 2, f.manager.calls, "the pending Trial's follow-up tick must reach completion")
}

// This is the follow-up event after SQL has committed Success but cleanup
// failed. A second cleanup failure must remain retryable; the next replay
// must finish cleanup rather than ACK merely because the Trial run is terminal.
func TestOfflineTrialTerminalTickReplaysCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	f := newFinishTickFixture(t, entity.EvaluationModeTrialRun, entity.ExptDispatchModeLegacy)
	f.result.ready = true
	f.manager.err = errors.New("quota cleanup unavailable")
	f.manager.disposition = entity.ExptRunFinishFinished
	var followup *entity.ExptScheduleEvent
	f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, next *entity.ExptScheduleEvent, _ *time.Duration) error {
		followup = next
		return nil
	})
	require.NoError(t, f.scheduler.schedule(context.Background(), f.event), "cleanup failure must publish a follow-up Trial tick")
	require.NotNil(t, followup)
	require.Equal(t, 1, f.manager.calls)
	f.manager.disposition = entity.ExptRunFinishAlreadyFinished
	manager := &trialTerminalManager{naturalFinishProbe: f.manager}
	f.scheduler.Manager = manager
	config := componentmocks.NewMockIConfiger(ctrl)
	f.scheduler.Configer = config // terminal replay must not read the zombie deadline
	endpoint := f.scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("terminal Trial tick reentered execution")
		return nil
	})
	err := endpoint(context.Background(), followup)
	require.True(t, errno.NeedMQRetry(err), "failed Trial cleanup must stay visible to the broker")
	require.Equal(t, 2, f.manager.calls)
	f.manager.err = nil
	require.NoError(t, endpoint(context.Background(), followup))
	require.Equal(t, 3, f.manager.calls, "follow-up Trial tick must finish outstanding cleanup")
}

type trialTerminalManager struct{ *naturalFinishProbe }

func (*trialTerminalManager) GetRunLog(context.Context, int64, int64, int64, *entity.Session) (*entity.ExptRunLog, error) {
	return &entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil
}

// The real DAO sees only the two canonical Trial rows, while the bound eval
// set contains 100 items. SQLmock controls SQL I/O; status/count computation,
// the transaction, Trial's inherited endpoint and the manager remain real.
func TestOfflineTrialSubsetFinishesWithCanonicalCounts(t *testing.T) {
	for _, outcome := range []struct {
		name       string
		itemStatus entity.ItemRunState
		turnStatus entity.TurnRunState
		status     entity.ExptStatus
		fail, pass int64
	}{
		{"success", entity.ItemRunState_Success, entity.TurnRunState_Success, entity.ExptStatus_Success, 0, 2},
		{"failed", entity.ItemRunState_Fail, entity.TurnRunState_Fail, entity.ExptStatus_Failed, 1, 1},
	} {
		t.Run(outcome.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			sqlDB, sql, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			provider, err := db.NewDB(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sql.ExpectBegin()
			sql.ExpectQuery("SELECT .* FROM `experiment`").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status", "expt_type", "trial_run_item_count", "eval_set_id"}).AddRow(1, 3, 2, entity.ExptStatus_Processing, entity.ExptType_Offline, 2, 100))
			sql.ExpectQuery("SELECT .* FROM `expt_run_log`").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status", "mode"}).AddRow(2, 3, 1, 2, entity.ExptStatus_Processing, entity.EvaluationModeTrialRun))
			sql.ExpectQuery("SELECT .* FROM `expt_item_result`").WithArgs(int64(3), int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "expt_run_id", "item_version_id", "status"}).AddRow(101, 10, 2, 7, entity.ItemRunState_Success).AddRow(102, 11, 2, 7, outcome.itemStatus))
			sql.ExpectQuery("SELECT .* FROM `expt_item_result_run_log`").WithArgs(int64(3), int64(1), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "item_version_id", "status", "result_state"}).AddRow(201, 10, 7, entity.ItemRunState_Success, entity.ExptItemResultStateResulted).AddRow(202, 11, 7, outcome.itemStatus, entity.ExptItemResultStateResulted))
			sql.ExpectQuery("SELECT .* FROM `expt_turn_result`").WithArgs(int64(3), int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "turn_id", "status"}).AddRow(301, 10, 0, entity.TurnRunState_Success).AddRow(302, 11, 0, outcome.turnStatus))
			sql.ExpectQuery("SELECT .* FROM `expt_stats`").WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "pending_cnt"}).AddRow(401, 3, 1, 2))
			sql.ExpectExec("UPDATE `expt_run_log` SET").WithArgs(outcome.fail, int64(0), int64(0), int64(outcome.status), outcome.pass, int64(0), int64(3), int64(1), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			sql.ExpectExec("UPDATE `experiment` SET").WithArgs(sqlmock.AnyArg(), int64(outcome.status), int64(3), int64(1), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			sql.ExpectExec("UPDATE `expt_stats` SET").WithArgs(outcome.fail, int64(0), int64(0), outcome.pass, int64(0), int64(3), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			sql.ExpectCommit()
			event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeTrialRun, ExecEvalSetItemIDs: []int64{10, 11}}
			expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Processing, TrialRunItemCount: 2, EvalSet: &entity.EvaluationSet{ID: 100, ItemCount: 100}}
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).AnyTimes()
			quota := repomocks.NewMockQuotaRepo(ctrl)
			quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).Return(nil)
			idem := idemmocks.NewMockIdempotentService(ctrl)
			idem.EXPECT().Exist(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string) (bool, error) { return key == "finish-cleanup:1:2", nil }).AnyTimes()
			idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).Return(nil, int64(0), nil)
			factory, _ := finishRealLockerFactory(t)
			port := &trialSubsetFinishPort{IExptRunFinishRepo: exptmysql.NewExptRunFinishDAO(provider), current: expt}
			manager := &ExptMangerImpl{runFinishRepo: port, exptRepo: experiments, quotaRepo: quota, manualRetryLocker: factory, idem: idem}
			calibration := &finishTickCalibration{ready: true}
			trial := &ExptTrialRunExec{ExptSubmitExec: &ExptSubmitExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: calibration}}
			scheduler := &ExptSchedulerImpl{Manager: manager, Mutex: factory()}
			require.NoError(t, scheduler.HandleEventLock(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				next, err := trial.ExptEnd(ctx, event, expt, 0, 0)
				require.False(t, next, "the selected subset must finish without the other 98 eval-set items")
				return err
			})(context.Background(), event))
			require.NoError(t, sql.ExpectationsWereMet())
			require.Equal(t, 1, calibration.calls)
			require.Equal(t, outcome.status, expt.Status)
			require.Equal(t, entity.EvaluationModeTrialRun, trial.Mode())
			t.Logf("REAL_TRIAL_ENDPOINT_SQLMOCK_FINISH eval_set_total=100 canonical_items=2 success=%d failed=%d status=%d", outcome.pass, outcome.fail, outcome.status)
		})
	}
}

// Keep the repository double's read-after-commit projection in step with the
// real DAO result, without simulating the DAO's status/count calculation.
type trialSubsetFinishPort struct {
	repo.IExptRunFinishRepo
	current *entity.Experiment
}

func (p *trialSubsetFinishPort) FinishCurrentRun(ctx context.Context, request *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	result, err := p.IExptRunFinishRepo.FinishCurrentRun(ctx, request)
	if err == nil && result != nil && result.Experiment != nil {
		p.current.Status = result.Experiment.Status
		p.current.EndAt = result.Experiment.EndAt
	}
	return result, err
}
