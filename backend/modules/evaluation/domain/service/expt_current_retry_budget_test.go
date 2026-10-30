// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func retryBudgetHandler(t *testing.T) (*ExptItemEventEvalServiceImpl, *repomocks.MockIExptCurrentResultRepo, *repomocks.MockIExperimentRepo, *entity.ExptCurrentItemRunSource, *entity.ExptItemEvalEvent, *bool) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mutex := lockmocks.NewMockILocker(ctrl)
	held := false
	mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:7:10", 5*time.Second, 5*time.Minute).
		DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
			held = true
			return true, ctx, func() {}, nil
		})
	mutex.EXPECT().Unlock("expt_item_eval_run_lock:7:10").DoAndReturn(func(string) (bool, error) {
		require.True(t, held)
		held = false
		return true, nil
	})
	port := repomocks.NewMockIExptCurrentResultRepo(ctrl)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	svc := &ExptItemEventEvalServiceImpl{
		itemLocker: func() lock.ILocker { return mutex }, experimentRepo: expts,
		configer: confmocks.NewMockIConfiger(ctrl), publisher: eventmocks.NewMockExptEventPublisher(ctrl),
	}
	source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{
		SpaceID: 9, ExptID: 7, RunID: 8, ItemID: 10, ItemVersionID: 11,
		RequestID: "current-request", Mode: entity.EvaluationModeFailRetry, Status: entity.ItemRunState_Processing,
	}}
	event := &entity.ExptItemEvalEvent{SpaceID: 9, ExptID: 7, ExptRunID: 8, EvalSetItemID: 10, RequestID: source.Current.RequestID, ExptRunMode: source.Current.Mode}
	return svc, port, expts, source, event, &held
}

func TestHandleCurrentEventErrFrozenRetryBudget(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		frozen                         *entity.EvaluationConfiguration
		space, eventLimit, db, message int
		want                           int32
		retry                          bool
	}{
		{name: "explicit_zero", frozen: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(0)}, space: 3, want: 0},
		{name: "remaining_db_max", frozen: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}, space: 3, db: 1, want: 2, retry: true},
		{name: "remaining_event_max", frozen: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}, space: 3, message: 1, want: 2, retry: true},
		{name: "exhausted", frozen: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}, space: 3, db: 3, message: 1, want: 3},
		{name: "frozen_overrides_positive_event", frozen: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3)}, eventLimit: 1, db: 1, want: 2, retry: true},
		{name: "absent_evalconf_event_fallback", eventLimit: 1, space: 3, db: 1, want: 1},
		{name: "absent_evalconf_space_fallback", space: 3, db: 1, want: 2, retry: true},
		{name: "absent_pointer_event_fallback", frozen: &entity.EvaluationConfiguration{}, eventLimit: 3, db: 1, want: 2, retry: true},
		{name: "absent_pointer_space_fallback", frozen: &entity.EvaluationConfiguration{}, space: 3, db: 1, want: 2, retry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, port, expts, source, event, held := retryBudgetHandler(t)
			source.Current.RetryTimes = int32(tc.db)
			event.RetryTimes, event.MaxRetryTimes = tc.message, tc.eventLimit
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(source, nil)
			reads := 0
			expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
				require.True(t, *held, "frozen budget read must remain inside the item lock")
				reads++
				return &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, EvalConf: tc.frozen}, nil
			}).AnyTimes()
			svc.configer.(*confmocks.MockIConfiger).EXPECT().GetErrRetryConf(gomock.Any(), int64(9), gomock.Any()).Return(&entity.RetryConf{RetryTimes: tc.space})
			var written *entity.ExptCurrentItemRunWrite
			port.EXPECT().CompleteCurrentItemRun(gomock.Any(), source, gomock.Any()).DoAndReturn(func(_ context.Context, _ *entity.ExptCurrentItemRunSource, w *entity.ExptCurrentItemRunWrite) (bool, error) {
				require.True(t, *held)
				written = w
				return true, nil
			})
			var published *entity.ExptItemEvalEvent
			svc.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), nil).
				DoAndReturn(func(_ context.Context, e *entity.ExptItemEvalEvent, _ *time.Duration, _ func(*entity.ExptItemEvalEvent)) error {
					published = e
					return nil
				}).AnyTimes()
			err := svc.handleCurrentEventErr(context.Background(), event, errors.New("execution failed"), port, &currentItemExecution{Source: source})
			require.NoError(t, err)
			require.NotNil(t, written)
			require.Equal(t, tc.retry, written.RetryOnly, "explicit zero must terminate rather than schedule an automatic retry")
			require.Equal(t, tc.want, written.RetryTimes)
			require.Equal(t, entity.ItemRunState_Fail, written.Status)
			require.Equal(t, !tc.retry, written.FailPendingTurns)
			require.False(t, written.Yield)
			require.Equal(t, 1, reads)
			require.Equal(t, tc.message, event.RetryTimes, "input event must remain unchanged")
			require.False(t, *held)
			if tc.retry {
				require.NotNil(t, published)
				require.Equal(t, int(tc.want), published.RetryTimes)
				require.Equal(t, event.RequestID, published.RequestID)
			} else {
				require.Nil(t, published)
			}
		})
	}
}

func TestHandleCurrentEventErrFrozenRetryReadFailure(t *testing.T) {
	svc, port, expts, source, event, held := retryBudgetHandler(t)
	port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(source, nil)
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
		require.True(t, *held)
		return nil, errors.New("frozen experiment read failed")
	})
	err := svc.handleCurrentEventErr(context.Background(), event, errors.New("execution failed"), port, &currentItemExecution{Source: source})
	require.ErrorContains(t, err, "frozen experiment read failed")
	require.True(t, errno.NeedMQRetry(err))
	require.False(t, *held)
	require.Zero(t, event.RetryTimes)
	require.Zero(t, source.Current.RetryTimes)
	// Unconfigured mock methods reject any result/source completion or publication.
}

func TestHandleCurrentEventErrStaleBeforeFrozenBudgetRead(t *testing.T) {
	for _, stale := range []string{"request", "run", "missing_source"} {
		t.Run(stale, func(t *testing.T) {
			svc, port, _, source, event, held := retryBudgetHandler(t)
			current := *source
			observed := &current
			switch stale {
			case "request":
				current.Current.RequestID = "replacement-request"
			case "run":
				current.Current.RunID = 18
			case "missing_source":
				observed = nil
			}
			port.EXPECT().GetCurrentItemRunSource(gomock.Any(), int64(9), int64(7), int64(8), int64(10)).Return(observed, nil)
			require.NoError(t, svc.handleCurrentEventErr(context.Background(), event, errors.New("late failure"), port, &currentItemExecution{Source: source}))
			require.False(t, *held)
			require.Zero(t, event.RetryTimes)
		})
	}
}
