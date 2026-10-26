// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// Only startup and scan inputs are controlled; ExptEnd, calibration and NextTick stay real.
type finishTickMode struct {
	entity.ExptSchedulerMode
	startErr, scheduleErr, scanErr, endErr error
	beforeEnd                              func()
}

func (m *finishTickMode) ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return m.startErr
}

func (m *finishTickMode) ScheduleStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error {
	return m.scheduleErr
}

func (m *finishTickMode) ScanEvalItems(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) ([]*entity.ExptEvalItem, []*entity.ExptEvalItem, []*entity.ExptEvalItem, error) {
	return nil, nil, nil, m.scanErr
}

func (m *finishTickMode) ExptEnd(ctx context.Context, event *entity.ExptScheduleEvent, expt *entity.Experiment, toSubmit, incomplete int) (bool, error) {
	if m.beforeEnd != nil {
		m.beforeEnd()
	}
	if m.endErr != nil {
		return false, m.endErr
	}
	return m.ExptSchedulerMode.ExptEnd(ctx, event, expt, toSubmit, incomplete)
}

type finishTickCalibration struct {
	ExptResultService
	ready bool
	err   error
	calls int
}

func (r *finishTickCalibration) ReconcileManualRetryFilterBeforeFinish(context.Context, int64, int64, int64) (bool, error) {
	r.calls++
	return r.ready, r.err
}

type finishTickFixture struct {
	scheduler *ExptSchedulerImpl
	mode      *finishTickMode
	manager   *naturalFinishProbe
	result    *finishTickCalibration
	publisher *eventmocks.MockExptEventPublisher
	event     *entity.ExptScheduleEvent
	expt      *entity.Experiment
}

func newFinishTickFixture(t *testing.T, runMode entity.ExptRunMode, dispatch string) *finishTickFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	event := &entity.ExptScheduleEvent{
		SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptType: entity.ExptType_Offline, ExptRunMode: runMode,
		CreatedAt: time.Now().Unix(), Ext: map[string]string{"__item_ids": "[10,11]", "retry_yield_enabled": "false", "other": "keep"},
		Session: &entity.Session{UserID: "operator", AppID: 7}, ItemRetryTimes: 4, InfraErrorRetryTimes: 6, ExecEvalSetItemIDs: []int64{10, 11},
	}
	expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, ExptDispatchMode: dispatch}
	oldManager := svcmocks.NewMockIExptManager(ctrl)
	oldManager.EXPECT().GetDetail(gomock.Any(), int64(1), int64(3), event.Session).Return(expt, nil).AnyTimes()
	oldManager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), event.Session).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
	manager := &naturalFinishProbe{IExptManager: oldManager, disposition: entity.ExptRunFinishPending}
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).Return(nil, int64(0), nil).AnyTimes()
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetConsumerConf(gomock.Any()).Return(&entity.ExptConsumerConf{}).AnyTimes()
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{DaemonIntervalSecond: 17}).AnyTimes()
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	result := &finishTickCalibration{}
	var realMode entity.ExptSchedulerMode
	switch runMode {
	case entity.EvaluationModeSubmit:
		realMode = &ExptSubmitExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result, publisher: publisher}
	case entity.EvaluationModeFailRetry:
		realMode = &ExptFailRetryExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result, publisher: publisher}
	case entity.EvaluationModeRetryAll:
		realMode = &ExptRetryAllExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result, publisher: publisher}
	case entity.EvaluationModeRetryItems:
		runLogs := repomocks.NewMockIExptRunLogRepo(ctrl)
		runLogs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{}, nil).AnyTimes()
		oldManager.EXPECT().LockCompletingRun(gomock.Any(), int64(1), int64(2), int64(3), event.Session).Return(nil).AnyTimes()
		oldManager.EXPECT().UnlockCompletingRun(gomock.Any(), int64(1), int64(2), int64(3), event.Session).Return(nil).AnyTimes()
		realMode = &ExptRetryItemsExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, exptRunLogRepo: runLogs, resultSvc: result, publisher: publisher}
	}
	mode := &finishTickMode{ExptSchedulerMode: realMode}
	factory := svcmocks.NewMockSchedulerModeFactory(ctrl)
	factory.EXPECT().NewSchedulerMode(gomock.Any()).Return(mode, nil).AnyTimes()
	scheduler := &ExptSchedulerImpl{Manager: manager, Configer: config, Publisher: publisher, ResultSvc: result, schedulerModeFactory: factory}
	return &finishTickFixture{scheduler: scheduler, mode: mode, manager: manager, result: result, publisher: publisher, event: event, expt: expt}
}

// Removing finish-error rearming strands the normal cadence even though calibration blocks completion.
func TestFinishCalibrationTickLiveness(t *testing.T) {
	for _, dispatch := range []string{entity.ExptDispatchModeLegacy, entity.ExptDispatchModeEnforce} {
		for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
			t.Run(fmt.Sprintf("%s/mode%d", dispatch, mode), func(t *testing.T) {
				f := newFinishTickFixture(t, mode, dispatch)
				for _, state := range []string{"calibration_pending", "calibration_error", "finish_pending"} {
					t.Run(state, func(t *testing.T) {
						f.result.ready = state == "finish_pending"
						f.result.err = nil
						if state == "calibration_error" {
							f.result.err = errors.New("projection unavailable")
						}
						before := *f.event
						var published *entity.ExptScheduleEvent
						f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, next *entity.ExptScheduleEvent, delay *time.Duration) error {
							require.NoError(t, ctx.Err())
							require.NotSame(t, f.event, next, "broker event must retain its original retry counters")
							require.Equal(t, before, *next)
							require.Equal(t, 17*time.Second, *delay)
							published = next
							return nil
						})
						err := f.scheduler.HandleEventErr(f.scheduler.schedule)(context.Background(), f.event)
						require.NoError(t, err, "successful next-tick publication should ACK current event")
						require.NotNil(t, published)
						require.Equal(t, before, *f.event)
						require.Equal(t, entity.ExptStatus_Processing, f.expt.Status)
						if state != "finish_pending" {
							require.Zero(t, f.manager.calls, "calibration must block the finish transaction")
						}
					})
				}
				require.Equal(t, 3, f.result.calls)
				require.Equal(t, 1, f.manager.calls, "only calibrated pending may reach finish composition")
			})
		}
	}
}

func TestFinishCalibrationTickBrokerFallback(t *testing.T) {
	for _, fault := range []string{"publish_failure", "cancel_before_end", "cancel_during_publish", "nil_publisher", "fatal_end"} {
		t.Run(fault, func(t *testing.T) {
			f := newFinishTickFixture(t, entity.EvaluationModeSubmit, entity.ExptDispatchModeEnforce)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			sentinel := errors.New("publisher unavailable")
			switch fault {
			case "publish_failure", "cancel_during_publish":
				f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptScheduleEvent, *time.Duration) error {
					if fault == "cancel_during_publish" {
						cancel()
						return nil
					}
					return sentinel
				})
			case "cancel_before_end":
				f.mode.beforeEnd = cancel
			case "nil_publisher":
				f.scheduler.Publisher = nil
			case "fatal_end":
				f.mode.endErr = sentinel
			}
			before := *f.event
			err := f.scheduler.schedule(ctx, f.event)
			require.Error(t, err)
			if fault == "fatal_end" {
				require.Same(t, sentinel, err)
			} else {
				require.True(t, errno.NeedMQRetry(err), "fallback must stay visible to broker")
			}
			if fault == "publish_failure" {
				require.ErrorIs(t, err, sentinel)
			}
			if fault == "cancel_before_end" || fault == "cancel_during_publish" {
				require.ErrorIs(t, err, context.Canceled)
			}
			require.Equal(t, before, *f.event)
			require.Zero(t, f.manager.calls)
		})
	}
}

func TestFinishCalibrationTickExcludedModesAndOldManager(t *testing.T) {
	for _, scope := range []string{"online", "append", "trial", "old_manager", "noncurrent"} {
		t.Run(scope, func(t *testing.T) {
			f := newFinishTickFixture(t, entity.EvaluationModeSubmit, entity.ExptDispatchModeLegacy)
			retry := errno.NewMQRetryErr("typed end retry")
			f.mode.endErr = retry
			switch scope {
			case "online":
				f.event.ExptType = entity.ExptType_Online
				f.expt.ExptType = entity.ExptType_Online
			case "append":
				f.event.ExptRunMode = entity.EvaluationModeAppend
			case "trial":
				f.event.ExptRunMode = entity.EvaluationModeTrialRun
			case "old_manager":
				f.scheduler.Manager = f.manager.IExptManager
			case "noncurrent":
				f.expt.LatestRunID = 99
			}
			err := f.scheduler.HandleEventErr(f.scheduler.schedule)(context.Background(), f.event)
			require.Same(t, retry, err)
			require.Zero(t, f.manager.calls)
		})
	}
}

func TestFinishCalibrationTickPriorErrorsKeepBrokerRetry(t *testing.T) {
	for _, stage := range []string{"start", "schedule_start", "scan", "lock_busy"} {
		t.Run(stage, func(t *testing.T) {
			f := newFinishTickFixture(t, entity.EvaluationModeSubmit, entity.ExptDispatchModeLegacy)
			retry := errno.NewMQRetryErr("typed earlier retry")
			endpoint := f.scheduler.schedule
			switch stage {
			case "start":
				f.mode.startErr = retry
			case "schedule_start":
				f.mode.scheduleErr = retry
			case "scan":
				f.mode.scanErr = retry
			case "lock_busy":
				locker := lockmocks.NewMockILocker(gomock.NewController(t))
				locker.EXPECT().LockBackoffWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, retry)
				f.scheduler.Mutex = locker
				endpoint = f.scheduler.HandleEventLock(endpoint)
			}
			err := f.scheduler.HandleEventErr(endpoint)(context.Background(), f.event)
			require.Same(t, retry, err)
			require.Zero(t, f.result.calls)
			require.Zero(t, f.manager.calls)
		})
	}
}

// A broker duplicate must still enter the existing run mutex before any calibration or tick publication.
func TestFinishCalibrationTickDuplicateUsesRunLock(t *testing.T) {
	f := newFinishTickFixture(t, entity.EvaluationModeRetryItems, entity.ExptDispatchModeEnforce)
	locker := lockmocks.NewMockILocker(gomock.NewController(t))
	held := false
	locker.EXPECT().LockBackoffWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
		require.False(t, held)
		held = true
		return true, ctx, func() {}, nil
	}).Times(2)
	locker.EXPECT().Unlock("expt_run_exec_lock:1:2").DoAndReturn(func(string) (bool, error) { require.True(t, held); held = false; return true, nil }).Times(2)
	f.scheduler.Mutex = locker
	f.publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptScheduleEvent, *time.Duration) error {
		require.True(t, held)
		return nil
	}).Times(2)
	endpoint := f.scheduler.HandleEventErr(f.scheduler.HandleEventCheck(f.scheduler.HandleEventLock(f.scheduler.schedule)))
	for i := 0; i < 2; i++ {
		require.NoError(t, endpoint(context.Background(), f.event))
		require.False(t, held)
	}
	require.Equal(t, 2, f.result.calls)
	require.Zero(t, f.manager.calls)
}

var _ lock.ILocker = (*lockmocks.MockILocker)(nil)
