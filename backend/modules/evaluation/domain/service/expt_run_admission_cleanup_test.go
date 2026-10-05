// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	lwtMocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestRunAdmissionCleanup(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mode        entity.ExptRunMode
		publishFail bool
	}{
		{"submit", entity.EvaluationModeSubmit, false},
		{"retry_all", entity.EvaluationModeRetryAll, false},
		{"retry_failure", entity.EvaluationModeFailRetry, false},
		{"retry_items", entity.EvaluationModeRetryItems, false},
		{"publish_failure_preserves_run", entity.EvaluationModeFailRetry, true},
		{"publish_failure_preserves_retry_items_run", entity.EvaluationModeRetryItems, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := newTestExptManager(ctrl)
			ctx := context.Background()
			session := &entity.Session{UserID: "u"}
			const exptID, runID, spaceID = int64(1), int64(2), int64(3)
			limit, logCalls := 2, 2
			publishErr := errors.New("publish timed out")
			if tc.publishFail {
				limit, logCalls = 3, 1
				mgr.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().PublishExptScheduleEvent(ctx, gomock.Any(), gomock.Any()).Return(publishErr)
			}
			configer := componentMocks.NewMockIConfiger(ctrl)
			configer.EXPECT().GetExptExecConf(ctx, spaceID).Return(&entity.ExptExecConf{
				SpaceExptConcurLimit: limit, ZombieIntervalSecond: 72 * 60 * 60,
			}).AnyTimes()
			configer.EXPECT().GetRetryYieldEnabled(ctx, spaceID).Return(false).AnyTimes()
			mgr.configer = configer
			quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{10: time.Now().Unix(), 11: time.Now().Unix()}}
			mgr.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().CreateOrUpdate(ctx, spaceID, gomock.Any(), session).
				DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
					_, changed, err := update(quota)
					require.Equal(t, tc.publishFail, changed)
					return err
				})
			mgr.lwt.(*lwtMocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(ctx, gomock.Any(), exptID).Return(false).AnyTimes()
			mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, []int64{exptID}, spaceID).
				Return([]*entity.Experiment{{ID: exptID, SpaceID: spaceID, Status: entity.ExptStatus_Failed}}, nil).AnyTimes()
			mgr.evaluationSetService.(*svcMocks.MockIEvaluationSetService).EXPECT().
				GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Nil()).Return(&entity.EvaluationSet{}, nil).AnyTimes()
			mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
				MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
				BatchGetExptAggrResultByExperimentIDs(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

			var runLog *entity.ExptRunLog
			locked := false
			lockKey := mgr.makeExptMutexLockKey(exptID)
			mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, int64(tc.mode)).Times(logCalls)
			var latestRunID int64
			mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(ctx, gomock.Any()).
				DoAndReturn(func(_ context.Context, expt *entity.Experiment) error { latestRunID = expt.LatestRunID; return nil }).Times(logCalls)
			if tc.mode == entity.EvaluationModeRetryItems {
				mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(ctx).Return(runID, nil).Times(logCalls)
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().BackoffLockWithValue(ctx, lockKey, strconv.FormatInt(runID, 10), gomock.Any(), gomock.Any()).
					DoAndReturn(func(context.Context, string, string, time.Duration, time.Duration) (bool, string, error) {
						if locked {
							return false, strconv.FormatInt(runID, 10), nil
						}
						locked = true
						return true, "", nil
					}).Times(logCalls)
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Save(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, got *entity.ExptRunLog) error { runLog = got; return nil }).Times(logCalls)
			} else {
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().LockBackoff(ctx, lockKey, gomock.Any(), gomock.Any()).
					DoAndReturn(func(context.Context, string, time.Duration, time.Duration) (bool, error) {
						if locked {
							return false, nil
						}
						locked = true
						return true, nil
					}).Times(logCalls)
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Create(ctx, gomock.Any()).
					DoAndReturn(func(_ context.Context, got *entity.ExptRunLog) error { runLog = got; return nil }).Times(logCalls)
			}
			if !tc.publishFail {
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), exptID, runID, gomock.Any()).
					DoAndReturn(func(_ context.Context, _, _ int64, fields map[string]any) error {
						runLog.Status = fields["status"].(int64)
						runLog.StatusMessage = fields["status_message"].([]byte)
						return nil
					})
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), lockKey).
					DoAndReturn(func(context.Context, string) (bool, error) {
						require.Equal(t, int64(entity.ExptStatus_Failed), runLog.Status)
						locked = false
						return true, nil
					})
			}

			var err error
			if tc.mode == entity.EvaluationModeRetryItems {
				_, retried, logErr := mgr.LogRetryItemsRun(ctx, exptID, tc.mode, spaceID, []int64{4}, session)
				require.NoError(t, logErr)
				require.False(t, retried)
				err = mgr.RetryItems(ctx, exptID, runID, spaceID, 0, []int64{4}, session, nil)
			} else {
				require.NoError(t, mgr.LogRun(ctx, exptID, runID, tc.mode, spaceID, nil, session))
				err = mgr.Run(ctx, exptID, runID, spaceID, 0, session, tc.mode, nil)
			}
			if tc.publishFail {
				require.ErrorIs(t, err, publishErr)
				require.True(t, locked)
				require.Equal(t, int64(entity.ExptStatus_Pending), runLog.Status)
				return
			}
			status, ok := errorx.FromStatusError(err)
			require.True(t, ok)
			require.Equal(t, int32(errno.ExperimentRunningCountLimitCode), status.Code())
			require.Len(t, quota.ExptID2RunTime, 2)
			require.NotContains(t, quota.ExptID2RunTime, exptID)
			require.False(t, locked)
			require.Equal(t, int64(entity.ExptStatus_Failed), runLog.Status)
			require.Contains(t, string(runLog.StatusMessage), "max limit: 2")
			require.Equal(t, runID, latestRunID, "admission cleanup must leave the latest run pointing at the failed attempt")
			if tc.mode == entity.EvaluationModeRetryItems {
				_, retried, logErr := mgr.LogRetryItemsRun(ctx, exptID, tc.mode, spaceID, []int64{4}, session)
				require.NoError(t, logErr)
				require.False(t, retried)
			} else {
				require.NoError(t, mgr.LogRun(ctx, exptID, runID+1, tc.mode, spaceID, nil, session))
			}
			require.True(t, locked)
		})
	}
}

func TestRunPreparationReadFailure(t *testing.T) {
	for _, tc := range []struct {
		name                                              string
		updateFails, updateTimesOut, updateCommittedError bool
		lockPresent, released                             bool
		unlockErr                                         error
	}{
		{name: "success", lockPresent: true, released: true},
		{name: "update_failure_still_unlocks", updateFails: true, lockPresent: true, released: true},
		{name: "update_committed_then_error_still_unlocks", updateCommittedError: true, lockPresent: true, released: true},
		{name: "update_timeout_leaves_full_unlock_budget", updateTimesOut: true, lockPresent: true, released: true},
		{name: "lock_already_absent"},
		{name: "unlock_error_without_delete_preserves_original", lockPresent: true, unlockErr: errors.New("redis unavailable")},
		{name: "update_and_unlock_fail_preserves_original", updateFails: true, lockPresent: true, unlockErr: errors.New("redis unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := newTestExptManager(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			const exptID, runID, spaceID = int64(1), int64(2), int64(3)
			mgr.lwt.(*lwtMocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(ctx, gomock.Any(), exptID).Return(false)
			mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, []int64{exptID}, spaceID).
				DoAndReturn(func(context.Context, []int64, int64) ([]*entity.Experiment, error) {
					cancel()
					return nil, context.Canceled
				})
			runLog := &entity.ExptRunLog{ExptID: exptID, ExptRunID: runID, Status: int64(entity.ExptStatus_Pending)}
			locked := tc.lockPresent
			var updateCtx context.Context
			mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), exptID, runID, gomock.Any()).
				DoAndReturn(func(cleanupCtx context.Context, _, _ int64, fields map[string]any) error {
					updateCtx = cleanupCtx
					require.NoError(t, cleanupCtx.Err())
					deadline, ok := cleanupCtx.Deadline()
					require.True(t, ok)
					require.InDelta(t, 5, time.Until(deadline).Seconds(), 1)
					if tc.updateTimesOut {
						<-cleanupCtx.Done()
						return cleanupCtx.Err()
					}
					if tc.updateFails {
						return errors.New("run log unavailable")
					}
					runLog.Status = fields["status"].(int64)
					runLog.StatusMessage = fields["status_message"].([]byte)
					if tc.updateCommittedError {
						return errors.New("run log response lost")
					}
					return nil
				})
			mgr.mutex.(*lockMocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), mgr.makeExptMutexLockKey(exptID)).
				DoAndReturn(func(unlockCtx context.Context, _ string) (bool, error) {
					require.NotSame(t, updateCtx, unlockCtx)
					require.NoError(t, unlockCtx.Err())
					deadline, ok := unlockCtx.Deadline()
					require.True(t, ok)
					require.InDelta(t, 2, time.Until(deadline).Seconds(), 0.5)
					if tc.unlockErr == nil {
						locked = false
					}
					return tc.released, tc.unlockErr
				})
			err := mgr.Run(ctx, exptID, runID, spaceID, 0, &entity.Session{UserID: "u"}, entity.EvaluationModeFailRetry, nil)
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, tc.lockPresent && tc.unlockErr != nil, locked)
			if !tc.updateFails && !tc.updateTimesOut {
				require.Equal(t, int64(entity.ExptStatus_Failed), runLog.Status)
				require.Equal(t, context.Canceled.Error(), string(runLog.StatusMessage))
			}
		})
	}
}
