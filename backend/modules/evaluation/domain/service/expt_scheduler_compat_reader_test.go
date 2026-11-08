// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func compatReaderEvent(mode entity.ExptRunMode, typ entity.ExptType) *entity.ExptScheduleEvent {
	return &entity.ExptScheduleEvent{
		SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: mode, ExptType: typ,
		CreatedAt: 1730000123, Ext: map[string]string{"opaque": "keep"}, Session: &entity.Session{UserID: "u", AppID: 17},
		ItemRetryTimes: 3, ExecEvalSetItemIDs: []int64{41, 42}, InfraErrorRetryTimes: 10,
	}
}

func compatReaderScheduler(ctrl *gomock.Controller) (*ExptSchedulerImpl, *ExptMangerImpl, *eventmocks.MockExptEventPublisher) {
	mgr := newTestExptManager(ctrl)
	config := configmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{DaemonIntervalSecond: 7}).AnyTimes()
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	s := &ExptSchedulerImpl{
		Manager: mgr, Configer: config, Publisher: publisher,
		schedulerModeFactory: &DefaultSchedulerModeFactory{manager: mgr, configer: config, publisher: publisher, exptRunLogRepo: mgr.runLogRepo},
	}
	return s, mgr, publisher
}

func TestCompatReaderTerminalEvent(t *testing.T) {
	modes := []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems, entity.EvaluationModeAppend}
	statuses := []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated, entity.ExptStatus_Terminating, entity.ExptStatus_Draining}
	for _, mode := range modes {
		for _, status := range statuses {
			t.Run(fmt.Sprintf("mode_%d_status_%d", mode, status), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				s, mgr, publisher := compatReaderScheduler(ctrl)
				event := compatReaderEvent(mode, entity.ExptType_Offline)
				mgr.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), event.ExptID, event.ExptRunID).Return(&entity.ExptRunLog{Status: int64(status)}, nil)
				before, err := json.Marshal(event)
				require.NoError(t, err)
				if mode != entity.EvaluationModeAppend {
					publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), event, gomock.Any()).DoAndReturn(func(_ context.Context, got *entity.ExptScheduleEvent, delay *time.Duration) error {
						after, err := json.Marshal(got)
						require.NoError(t, err)
						require.JSONEq(t, string(before), string(after))
						require.Equal(t, time.Second*7, *delay)
						return nil
					})
				}
				err = s.HandleEventErr(s.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
					t.Fatal("terminal event reached downstream")
					return nil
				}))(context.Background(), event)
				require.NoError(t, err)
			})
		}
	}
	t.Run("online", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		s, mgr, _ := compatReaderScheduler(ctrl)
		event := compatReaderEvent(entity.EvaluationModeSubmit, entity.ExptType_Online)
		mgr.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil)
		require.NoError(t, s.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
			t.Fatal("terminal online event reached downstream")
			return nil
		})(context.Background(), event))
	})
	t.Run("send_failure", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		s, mgr, publisher := compatReaderScheduler(ctrl)
		event := compatReaderEvent(entity.EvaluationModeTrialRun, entity.ExptType_Offline)
		mgr.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Terminated)}, nil)
		publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), event, gomock.Any()).Return(errors.New("opaque publish failure"))
		err := s.HandleEventErr(s.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
			t.Fatal("terminal event reached downstream")
			return nil
		}))(context.Background(), event)
		require.True(t, errno.NeedMQRetry(err))
		require.Equal(t, 10, event.InfraErrorRetryTimes)
	})
}

func TestCompatReaderBusyTick(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems, entity.EvaluationModeAppend} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode_%d_fail_%v", mode, fail), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				s, _, publisher := compatReaderScheduler(ctrl)
				mutex := lockmocks.NewMockILocker(ctrl)
				s.Mutex = mutex
				event := compatReaderEvent(mode, entity.ExptType_Offline)
				mutex.EXPECT().LockBackoffWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", time.Second*5, time.Minute*5).Return(false, context.Background(), func() {}, nil)
				before, err := json.Marshal(event)
				require.NoError(t, err)
				if mode != entity.EvaluationModeAppend {
					publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), event, gomock.Any()).DoAndReturn(func(_ context.Context, got *entity.ExptScheduleEvent, delay *time.Duration) error {
						after, err := json.Marshal(got)
						require.NoError(t, err)
						require.JSONEq(t, string(before), string(after))
						require.Equal(t, time.Second*7, *delay)
						if fail {
							return errors.New("opaque publish failure")
						}
						return nil
					})
				}
				err = s.HandleEventErr(s.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
					t.Fatal("busy event reached downstream")
					return nil
				}))(context.Background(), event)
				if fail && mode != entity.EvaluationModeAppend {
					require.True(t, errno.NeedMQRetry(err))
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, 10, event.InfraErrorRetryTimes)
			})
		}
	}
	t.Run("online", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		s, _, _ := compatReaderScheduler(ctrl)
		mutex := lockmocks.NewMockILocker(ctrl)
		s.Mutex = mutex
		mutex.EXPECT().LockBackoffWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(false, context.Background(), func() {}, nil)
		require.NoError(t, s.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
			t.Fatal("busy online event reached downstream")
			return nil
		})(context.Background(), compatReaderEvent(entity.EvaluationModeSubmit, entity.ExptType_Online)))
	})
}

func TestCompatReaderMQRetryPriority(t *testing.T) {
	for _, message := range []string{"opaque failure", "send batch message fail"} {
		t.Run(message, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			s, _, _ := compatReaderScheduler(ctrl)
			event := compatReaderEvent(entity.EvaluationModeSubmit, entity.ExptType_Offline)
			cause := errno.WrapMQRetryErr(errors.New(message))
			err := s.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return cause })(context.Background(), event)
			require.Same(t, cause, err)
			require.Equal(t, 10, event.InfraErrorRetryTimes)
		})
	}
}

func TestCompatReaderCompletingLockError(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lockErr  error
		toSubmit int
	}{{"busy", nil, 0}, {"redis_error", errors.New("opaque Redis failure"), 0}, {"pending_items", nil, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			s, mgr, _ := compatReaderScheduler(ctrl)
			event := compatReaderEvent(entity.EvaluationModeRetryItems, entity.ExptType_Offline)
			if tc.toSubmit == 0 {
				mgr.mutex.(*lockmocks.MockILocker).EXPECT().Lock(gomock.Any(), "expt_completing_mutex_lock:1:2", time.Minute*3).Return(false, tc.lockErr)
			}
			mode, err := s.schedulerModeFactory.NewSchedulerMode(event.ExptRunMode)
			require.NoError(t, err)
			nextTick := false
			err = s.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				var err error
				nextTick, err = mode.ExptEnd(ctx, event, &entity.Experiment{}, tc.toSubmit, 0)
				return err
			})(context.Background(), event)
			if tc.toSubmit > 0 {
				require.NoError(t, err)
				require.True(t, nextTick)
			} else {
				require.True(t, errno.NeedMQRetry(err))
				require.False(t, nextTick)
			}
		})
	}
}
