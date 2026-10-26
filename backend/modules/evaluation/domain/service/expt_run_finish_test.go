// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/redis/go-redis/v9"

	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type naturalFinishProbe struct {
	IExptManager
	calls       int
	disposition entity.ExptRunFinishDisposition
	err         error
}

func (p *naturalFinishProbe) FinishCurrentRun(ctx context.Context, event *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	return p.FinishCurrentRunWithHeldTickLock(ctx, event)
}

func (p *naturalFinishProbe) FinishCurrentRunWithHeldTickLock(context.Context, *entity.ExptScheduleEvent) (*entity.ExptRunFinishResult, error) {
	p.calls++
	return &entity.ExptRunFinishResult{Disposition: p.disposition}, p.err
}

// The real common mode endpoint must use the composite transaction after
// calibration, including the database pending disposition.
func TestRunFinishCommonEndpointUsesComposition(t *testing.T) {
	for _, disposition := range []entity.ExptRunFinishDisposition{entity.ExptRunFinishPending, entity.ExptRunFinishFinished, entity.ExptRunFinishAlreadyFinished, entity.ExptRunFinishNonCurrent} {
		t.Run(string(rune('0'+disposition)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := &naturalFinishProbe{IExptManager: svcmocks.NewMockIExptManager(ctrl), disposition: disposition}
			idem := idemmocks.NewMockIdempotentService(ctrl)
			idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
			idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{}).AnyTimes()
			base := &exptBaseExec{Manager: manager, idem: idem, configer: config, resultSvc: &finishCalibrationTestResult{ready: true}}
			err := base.exptEnd(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit}, &entity.Experiment{ExptType: entity.ExptType_Offline})
			if disposition == entity.ExptRunFinishPending {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, 1, manager.calls)
		})
	}
}

func TestRunFinishCommonCalibrationBlocksComposition(t *testing.T) {
	for _, fault := range []error{nil, errors.New("calibration unavailable")} {
		ctrl := gomock.NewController(t)
		manager := &naturalFinishProbe{IExptManager: svcmocks.NewMockIExptManager(ctrl)}
		idem := idemmocks.NewMockIdempotentService(ctrl)
		idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
		base := &exptBaseExec{Manager: manager, idem: idem, resultSvc: &finishCalibrationTestResult{err: fault}}
		err := base.exptEnd(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, &entity.Experiment{ExptType: entity.ExptType_Offline})
		require.True(t, errno.NeedMQRetry(err))
		require.Zero(t, manager.calls)
	}
}

func TestRunFinishRescanIncludesLogged(t *testing.T) {
	ctrl := gomock.NewController(t)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64, *entity.ExptItemRunLogFilter, int64, int64, int64) ([]*entity.ExptItemResultRunLog, int64, error) {
		return nil, 0, nil
	})
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).DoAndReturn(func(_ context.Context, _ int64, _ int64, filter *entity.ExptItemRunLogFilter, _ int64, _ int64, _ int64) ([]*entity.ExptItemResultRunLog, int64, error) {
		require.NotNil(t, filter.ResultState)
		require.Equal(t, entity.ExptItemResultStateLogged, *filter.ResultState)
		return []*entity.ExptItemResultRunLog{{ItemID: 11}}, 1, nil
	})
	pending, err := (&exptBaseExec{exptItemResultRepo: &finishCurrentItemRepo{IExptItemResultRepo: items}}).hasPendingAfterRescan(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3})
	require.NoError(t, err)
	require.True(t, pending)
}

type finishRepoProbe struct {
	repo.IExptRunFinishRepo
	calls  int
	result *entity.ExptRunFinishResult
	err    error
}

func (p *finishRepoProbe) FinishCurrentRun(context.Context, *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	p.calls++
	return p.result, p.err
}

func TestRunFinishManagerCommitFailureAndPendingHaveZeroCleanup(t *testing.T) {
	for _, fault := range []error{errors.New("commit failed"), nil} {
		ctrl := gomock.NewController(t)
		port := &finishRepoProbe{err: fault, result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishPending}}
		manager := &ExptMangerImpl{runFinishRepo: port, mutex: lockmocks.NewMockILocker(ctrl)}
		result, err := manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
		if fault != nil {
			require.Error(t, err)
			require.Nil(t, result)
		} else {
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishPending, result.Disposition)
		}
		require.Equal(t, 1, port.calls)
	}
}

func TestRunFinishManagerCurrentRepositoryFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := &ExptMangerImpl{itemResultRepo: &finishCurrentItemRepo{IExptItemResultRepo: repomocks.NewMockIExptItemResultRepo(ctrl)}, mutex: lockmocks.NewMockILocker(ctrl)}
	_, err := manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.Error(t, err)
}

type finishCurrentItemRepo struct {
	repo.IExptItemResultRepo
	repo.IExptCurrentResultRepo
}

func TestRunFinishManagerCleanupFailureReplaysAfterCommit(t *testing.T) {
	ctrl := gomock.NewController(t)
	port := &finishRepoProbe{result: &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishFinished, Experiment: &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}}}
	locker := lockmocks.NewMockILocker(ctrl)
	var locks atomic.Int32
	locker.EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(false, nil).Times(2)
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_mutex_lock:1", gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
		if locks.Add(1) == 1 {
			return false, ctx, func() {}, errors.New("Redis unavailable")
		}
		return true, ctx, func() {}, nil
	}).Times(2)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99, Status: entity.ExptStatus_Processing}, nil)
	manager := &ExptMangerImpl{runFinishRepo: port, mutex: locker, exptRepo: expts, manualRetryLocker: func() lock.ILocker { return locker }}
	_, err := manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, 1, port.calls)
	port.result.Disposition = entity.ExptRunFinishAlreadyFinished
	_, err = manager.FinishCurrentRunWithHeldTickLock(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.NoError(t, err)
	require.Equal(t, 2, port.calls)
}

func TestRunFinishFinishedSchedulerEventReplaysCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	base := svcmocks.NewMockIExptManager(ctrl)
	base.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil)
	manager := &naturalFinishProbe{IExptManager: base, disposition: entity.ExptRunFinishAlreadyFinished}
	scheduler := &ExptSchedulerImpl{Manager: manager}
	err := scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("finished event entered execution")
		return nil
	})(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit})
	require.NoError(t, err)
	require.Equal(t, 1, manager.calls, "ACK swallowed failed postcommit cleanup retry")
}

func TestRunFinishLogRunBindsLifecycleOwnerToRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2", gomock.Any(), time.Second).Return(true, "2", nil)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	runs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(int64(3), int64(entity.EvaluationModeSubmit))
	manager := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: locker, configer: config, runLogRepo: runs, exptRepo: expts, mtr: metric}
	require.NoError(t, manager.LogRun(context.Background(), 1, 2, entity.EvaluationModeSubmit, 3, nil, &entity.Session{UserID: "test"}))
}

func finishRealLockerFactory(t *testing.T) (ManualRetryLockerFactory, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	return func() lock.ILocker { return filterRealTestLocker(client) }, server
}

func TestRunFinishCleanupDoesNotDeleteR2LifecycleOwner(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	key := "expt_run_mutex_lock:1"
	require.NoError(t, server.Set(key, "99"))
	manager := &ExptMangerImpl{manualRetryLocker: factory}
	err := manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, &entity.Experiment{ID: 1})
	require.True(t, errno.NeedMQRetry(err))
	owner, err := server.Get(key)
	require.NoError(t, err)
	require.Equal(t, "99", owner)
	// A pre-upgrade random owner cannot be safely migrated or force deleted.
	require.NoError(t, server.Set(key, "legacy-random-holder"))
	err = manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, &entity.Experiment{ID: 1})
	require.Error(t, err)
	owner, _ = server.Get(key)
	require.Equal(t, "legacy-random-holder", owner)
}

// Exercise the real common ExptEnd under the scheduler mutex, with an actual
// SQL finisher and a postcommit failure. Retry cannot rewrite counts/endAt.
func TestRunFinishActualModesCommitThenCleanupRetry(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			ctrl := gomock.NewController(t)
			require.NoError(t, f.conn.Model(&mysqlmodel.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Processing), "expt_type": int32(entity.ExptType_Offline)}).Error)
			require.NoError(t, f.conn.Model(&mysqlmodel.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Processing)).Error)
			require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Success)).Error)
			require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": 1}).Error)
			require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
			factory, server := finishRealLockerFactory(t)
			require.NoError(t, server.Set(fmt.Sprintf("expt_run_mutex_lock:%d", f.expt), fmt.Sprint(f.run)))
			experiments := repomocks.NewMockIExperimentRepo(ctrl)
			experiments.EXPECT().GetByID(gomock.Any(), f.expt, f.space).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.Experiment, error) {
				var row mysqlmodel.Experiment
				err := f.conn.WithContext(ctx).Where("space_id=? AND id=?", f.space, f.expt).Take(&row).Error
				return &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: row.LatestRunID, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus(row.Status), EndAt: row.EndAt}, err
			}).AnyTimes()
			quota := repomocks.NewMockQuotaRepo(ctrl)
			var releases atomic.Int32
			quota.EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, mutate func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
				var row mysqlmodel.Experiment
				require.NoError(t, f.conn.WithContext(ctx).Where("space_id=? AND id=?", f.space, f.expt).Take(&row).Error)
				require.True(t, entity.IsExptFinished(entity.ExptStatus(row.Status)), "cleanup before finish commit")
				// Cleanup's lifecycle key must reject a concurrent R2 creation.
				locked, _, err := factory().BackoffLockWithValue(ctx, fmt.Sprintf("expt_run_mutex_lock:%d", f.expt), fmt.Sprint(f.run+1), time.Minute, time.Millisecond)
				require.NoError(t, err)
				require.False(t, locked)
				if releases.Add(1) == 1 {
					return errors.New("quota cleanup fault")
				}
				_, _, err = mutate(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{f.expt: 1}})
				return err
			}).Times(2)
			result := &finishCalibrationTestResult{ExptResultService: svcmocks.NewMockExptResultService(ctrl), ready: true}
			result.ExptResultService.(*svcmocks.MockExptResultService).EXPECT().GetIncompleteTurns(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil, nil)
			publisher := eventmocks.NewMockExptEventPublisher(ctrl)
			publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			manager := &ExptMangerImpl{runFinishRepo: experimentrepo.NewExptRunFinishRepo(f.provider), manualRetryLocker: factory, exptRepo: experiments, quotaRepo: quota, exptResultService: result, publisher: publisher}
			idem := idemmocks.NewMockIdempotentService(ctrl)
			idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
			idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.idem = idem
			config := componentmocks.NewMockIConfiger(ctrl)
			config.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).Return(&entity.ExptExecConf{}).AnyTimes()
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptRunMode: mode, ExptType: entity.ExptType_Offline}
			items := f.itemRepo()
			expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, ExptType: entity.ExptType_Offline}
			var end func(context.Context, *entity.ExptScheduleEvent, *entity.Experiment, int, int) (bool, error)
			switch mode {
			case entity.EvaluationModeSubmit:
				end = (&ExptSubmitExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result}).ExptEnd
			case entity.EvaluationModeFailRetry:
				end = (&ExptFailRetryExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result}).ExptEnd
			case entity.EvaluationModeRetryAll:
				end = (&ExptRetryAllExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result}).ExptEnd
			case entity.EvaluationModeRetryItems:
				runs := repomocks.NewMockIExptRunLogRepo(ctrl)
				runs.EXPECT().Get(gomock.Any(), f.expt, f.run).Return(&entity.ExptRunLog{}, nil).AnyTimes()
				manager.runLogRepo = runs
				manager.mutex = factory()
				end = (&ExptRetryItemsExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, exptRunLogRepo: runs, resultSvc: result}).ExptEnd
			}
			scheduler := &ExptSchedulerImpl{Manager: manager, Mutex: factory()}
			err := scheduler.HandleEventLock(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				_, err := end(ctx, event, expt, 0, 0)
				return err
			})(context.Background(), event)
			require.True(t, errno.NeedMQRetry(err))
			var first mysqlmodel.Experiment
			require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&first).Error)
			require.NotNil(t, first.EndAt)
			before := lateWriteSnapshot(f)
			retryResult, retryErr := manager.FinishCurrentRun(context.Background(), event)
			require.NoError(t, retryErr)
			require.Equal(t, entity.ExptRunFinishAlreadyFinished, retryResult.Disposition)
			require.Equal(t, before, lateWriteSnapshot(f))
			require.Equal(t, int32(2), releases.Load())
		})
	}
}

func lateWriteSnapshot(f *lateWriteFixture) map[string]any {
	result := map[string]any{}
	for _, table := range []string{"experiment", "expt_run_log", "expt_stats"} {
		var rows []map[string]any
		require.NoError(f.t, f.conn.Table(table).Where("space_id=?", f.space).Find(&rows).Error)
		result[table] = rows
	}
	return result
}

func TestRunFinishDirectAcquiresTickAndLeaseLossStopsSQL(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	port := &finishRepoProbe{}
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
		child, cancel := context.WithCancel(ctx)
		cancel()
		return true, child, cancel, nil
	})
	manager := &ExptMangerImpl{runFinishRepo: port, manualRetryLocker: func() lock.ILocker { return locker }}
	_, err := manager.FinishCurrentRun(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
	require.Zero(t, port.calls)
}

func TestRunFinishHeldHelperCancelledLeaseStopsSQL(t *testing.T) {
	port := &finishRepoProbe{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager := &ExptMangerImpl{runFinishRepo: port}
	_, err := manager.FinishCurrentRunWithHeldTickLock(ctx, &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2})
	require.True(t, errno.NeedMQRetry(err))
	require.Zero(t, port.calls)
}

func TestRunFinishCleanupLostLeaseStopsQuotaAndNotifications(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	locker.EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(true, nil)
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_mutex_lock:1", 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (bool, context.Context, func(), error) {
		child, cancel := context.WithCancel(ctx)
		cancel()
		return true, child, cancel, nil
	})
	manager := &ExptMangerImpl{manualRetryLocker: func() lock.ILocker { return locker }, quotaRepo: repomocks.NewMockQuotaRepo(ctrl), publisher: eventmocks.NewMockExptEventPublisher(ctrl), exptRepo: repomocks.NewMockIExperimentRepo(ctrl)}
	require.Error(t, manager.cleanupNaturallyFinishedRun(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}, &entity.Experiment{ID: 1}))
}

// Numeric ownership on Submit must not broaden legacy live RetryItems join.
func TestRunFinishNumericSubmitOwnerDoesNotAllowRetryItemsJoin(t *testing.T) {
	ctrl := gomock.NewController(t)
	locker := lockmocks.NewMockILocker(ctrl)
	config := componentmocks.NewMockIConfiger(ctrl)
	ids := idgenmocks.NewMockIIDGenerator(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
	ids.EXPECT().GenID(gomock.Any()).Return(int64(99), nil)
	locker.EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "99", gomock.Any(), time.Second).Return(false, "2", nil)
	locker.EXPECT().Exists(gomock.Any(), "expt_completing_mutex_lock:1:2").Return(false, nil).AnyTimes()
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
	manager := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: locker, configer: config, idgenerator: ids, runLogRepo: runs}
	_, _, err := manager.LogRetryItemsRun(context.Background(), 1, entity.EvaluationModeRetryItems, 3, []int64{11}, &entity.Session{UserID: "test"})
	require.Error(t, err)
}
