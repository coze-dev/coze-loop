// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	eventsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket/producer"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	confmocks "github.com/coze-dev/coze-loop/backend/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestDispatchCancellationCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	loader := confmocks.NewMockIConfigLoader(ctrl)
	cfgFactory := confmocks.NewMockIConfigLoaderFactory(ctrl)
	cfgFactory.EXPECT().NewConfigLoader(gomock.Any()).Return(loader, nil).AnyTimes()
	loader.EXPECT().UnmarshalKey(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, value any, _ ...conf.DecodeOptionFn) error {
			disabled := true
			value.(*rocket.RMQConf).DisableProduce = &disabled
			return nil
		}).AnyTimes()
	// No MQ producer is constructed when production is disabled.
	missingProducer, err := producer.NewExptEventPublisher(context.Background(), cfgFactory, mqmocks.NewMockIFactory(ctrl))
	require.NoError(t, err)
	cause := errors.New("compile failed before delegate")
	marked := fmt.Errorf("%w: %w", events.ErrPublishNotSent, cause)
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		for _, tc := range []struct {
			name          string
			replaced      bool
			sendError     error
			realProducer  bool
			cancelOnError bool
			liveContext   bool
		}{
			{name: "not_invoked_cleanup_then_retry"},
			{name: "not_invoked_superseded_run", replaced: true},
			{name: "unknown_send_error", sendError: errors.New("send timed out")},
			{name: "unknown_send_canceled", sendError: context.Canceled},
			{name: "unknown_send_deadline", sendError: context.DeadlineExceeded},
			{name: "unknown_send_live_context", sendError: errors.New("publication result unknown"), liveContext: true},
			{name: "marked_cleanup_then_retry", sendError: marked},
			{name: "wrapped_marked_cleanup_then_retry", sendError: errorx.Wrapf(fmt.Errorf("schedule: %w", marked), "wrapper"), cancelOnError: true},
			{name: "marked_superseded_run", sendError: marked, replaced: true},
			{name: "real_producer_missing_cleanup_then_retry", sendError: events.ErrPublishNotSent, realProducer: true},
			{name: "lookalike_unknown_error", sendError: errors.New(events.ErrPublishNotSent.Error())},
		} {
			t.Run(fmt.Sprintf("%d/%s", mode, tc.name), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				m := newTestExptManager(ctrl)
				factory, redis := finishRealLockerFactory(t)
				m.manualRetryLocker, m.mutex = factory, factory()
				m.runFinishRepo = &pendingTerminationRepo{MockIExptRunTerminationRepo: repomocks.NewMockIExptRunTerminationRepo(ctrl)}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				session := &entity.Session{UserID: "u"}
				const exptID, spaceID = int64(2), int64(1)
				current := &entity.Experiment{ID: exptID, SpaceID: spaceID, LatestRunID: 7,
					ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Failed,
					StatusMessage: "original evaluation failure"}
				runs := make(map[int64]*entity.ExptRunLog)
				expts := m.exptRepo.(*repomocks.MockIExperimentRepo)
				runRepo := m.runLogRepo.(*repomocks.MockIExptRunLogRepo)
				expts.EXPECT().GetByID(gomock.Any(), exptID, spaceID).
					DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.Experiment, error) {
						require.True(t, contexts.CtxWriteDB(ctx))
						require.NoError(t, ctx.Err(), "cleanup must outlive request cancellation")
						return current, nil
					}).AnyTimes()
				runRepo.EXPECT().Get(gomock.Any(), exptID, gomock.Any()).
					DoAndReturn(func(_ context.Context, _, runID int64) (*entity.ExptRunLog, error) {
						return runs[runID], nil
					}).AnyTimes()
				cleanup := (tc.sendError == nil || errors.Is(tc.sendError, events.ErrPublishNotSent)) && !tc.replaced
				admissions := 1
				if cleanup {
					admissions = 2
				}
				runRepo.EXPECT().Create(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, run *entity.ExptRunLog) error {
						runs[run.ExptRunID] = run
						return nil
					}).Times(admissions)
				expts.EXPECT().Update(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, expt *entity.Experiment) error {
						require.Equal(t, exptID, expt.ID)
						current.LatestRunID = expt.LatestRunID
						return nil
					}).Times(admissions)
				m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, int64(mode)).Times(admissions)
				if mode == entity.EvaluationModeRetryItems {
					next := int64(7)
					m.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).
						DoAndReturn(func(context.Context) (int64, error) { next++; return next, nil }).Times(admissions)
				}
				m.lwt.(*lwtmocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), exptID).Return(false).Times(admissions)
				expts.EXPECT().MGetByID(gomock.Any(), []int64{exptID}, spaceID).Return([]*entity.Experiment{current}, nil).Times(admissions)
				m.evaluationSetService.(*svcmocks.MockIEvaluationSetService).EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil).Times(admissions)
				m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), []int64{exptID}, spaceID, session).Return(nil, nil).Times(admissions)
				m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), spaceID, []int64{exptID}).Return(nil, nil).Times(admissions)

				quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{20: time.Now().Unix()}}
				quotaCalls := admissions
				if admissions == 2 {
					quotaCalls++ // Release after the first, definitely unsent attempt.
				}
				m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), spaceID, gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						require.NoError(t, ctx.Err())
						var err error
						quota, _, err = update(quota)
						return err
					}).Times(quotaCalls)
				config := componentmocks.NewMockIConfiger(ctrl)
				m.configer = config
				config.EXPECT().GetExptExecConf(gomock.Any(), spaceID).Return(&entity.ExptExecConf{
					SpaceExptConcurLimit: 3, ZombieIntervalSecond: 86400,
				}).AnyTimes()
				prepared := false
				config.EXPECT().GetRetryYieldEnabled(gomock.Any(), spaceID).
					DoAndReturn(func(context.Context, int64) bool {
						if !prepared {
							prepared = true
							require.Contains(t, quota.ExptID2RunTime, exptID)
							if tc.sendError == nil {
								if tc.replaced {
									current.LatestRunID = 99
									require.NoError(t, redis.Set(m.makeExptMutexLockKey(exptID), "99"))
								}
								cancel() // Preparation succeeded, but publication has not been invoked.
							}
						}
						return false
					}).Times(admissions)
				publisher := m.publisher.(*eventsmocks.MockExptEventPublisher)
				var publicationErr error
				if tc.sendError != nil {
					publisher.EXPECT().PublishExptScheduleEvent(ctx, gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, event *entity.ExptScheduleEvent, delay *time.Duration) error {
							require.Equal(t, int64(8), event.ExptRunID)
							publicationErr = tc.sendError
							if tc.realProducer {
								publicationErr = missingProducer.PublishExptScheduleEvent(ctx, event, delay)
								require.ErrorIs(t, publicationErr, events.ErrPublishNotSent)
							}
							if tc.replaced {
								current.LatestRunID = 99
								require.NoError(t, redis.Set(m.makeExptMutexLockKey(exptID), "99"))
							}
							if !errors.Is(tc.sendError, events.ErrPublishNotSent) && !tc.liveContext || tc.cancelOnError {
								cancel() // A lost acknowledgement cannot prove that delivery failed.
							}
							return publicationErr
						})
				}
				if cleanup {
					runRepo.EXPECT().Update(gomock.Any(), exptID, int64(8), spaceID, gomock.Any()).
						DoAndReturn(func(ctx context.Context, _, _, _ int64, fields map[string]any) error {
							require.NoError(t, ctx.Err())
							runs[8].Status = fields["status"].(int64)
							runs[8].StatusMessage = fields["status_message"].([]byte)
							return nil
						})
					publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).
						DoAndReturn(func(ctx context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
							require.NoError(t, ctx.Err())
							require.Equal(t, int64(9), event.ExptRunID, "only the second attempt may be dispatched")
							require.Equal(t, mode, event.ExptRunMode)
							return nil
						})
				}
				admitAndRun := func(ctx context.Context, id int64) error {
					if mode == entity.EvaluationModeRetryItems {
						runID, joined, err := m.LogRetryItemsRun(ctx, exptID, mode, spaceID, []int64{4}, session)
						require.NoError(t, err)
						require.False(t, joined)
						require.Equal(t, id, runID)
						return m.RetryItems(ctx, exptID, id, spaceID, 0, []int64{4}, session, nil)
					}
					require.NoError(t, m.LogRun(ctx, exptID, id, mode, spaceID, nil, session))
					return m.Run(ctx, exptID, id, spaceID, 0, session, mode, nil)
				}
				err := admitAndRun(ctx, 8)
				require.True(t, prepared)
				require.Equal(t, entity.ExptStatus_Failed, current.Status)
				require.Equal(t, "original evaluation failure", current.StatusMessage)
				require.Contains(t, quota.ExptID2RunTime, int64(20))
				if !cleanup {
					if tc.sendError != nil {
						require.ErrorIs(t, err, tc.sendError)
					} else {
						require.ErrorIs(t, err, context.Canceled)
					}
					require.Equal(t, int64(entity.ExptStatus_Pending), runs[8].Status)
					require.Contains(t, quota.ExptID2RunTime, exptID)
					holder, readErr := redis.Get(m.makeExptMutexLockKey(exptID))
					require.NoError(t, readErr)
					require.Equal(t, strconv.FormatInt(current.LatestRunID, 10), holder)
					return
				}
				if tc.sendError != nil {
					require.ErrorIs(t, err, events.ErrPublishNotSent)
					require.ErrorIs(t, err, publicationErr)
					if !tc.realProducer {
						require.ErrorIs(t, err, cause)
					}
				} else {
					require.ErrorIs(t, err, context.Canceled)
				}
				require.Equal(t, int64(entity.ExptStatus_Failed), runs[8].Status)
				require.Equal(t, err.Error(), string(runs[8].StatusMessage))
				require.NotContains(t, quota.ExptID2RunTime, exptID)
				require.False(t, redis.Exists(m.makeExptMutexLockKey(exptID)))
				require.NoError(t, admitAndRun(context.Background(), 9))
				require.Equal(t, int64(9), current.LatestRunID)
				require.Equal(t, int64(entity.ExptStatus_Pending), runs[9].Status)
				require.Contains(t, quota.ExptID2RunTime, exptID)
			})
		}
	}
}

func TestMarkedDispatchPreservesOnlineAndLegacy(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		for _, online := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/online_%t", mode, online), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				m := newTestExptManager(ctrl)
				ctx := context.Background()
				const exptID, runID, spaceID = int64(2), int64(8), int64(1)
				expt := &entity.Experiment{ID: exptID, SpaceID: spaceID, ExptType: entity.ExptType_Offline, LatestRunID: runID}
				if online {
					expt.ExptType = entity.ExptType_Online
					m.runFinishRepo = &pendingTerminationRepo{MockIExptRunTerminationRepo: repomocks.NewMockIExptRunTerminationRepo(ctrl)}
					if mode != entity.EvaluationModeRetryItems {
						m.mutex.(*lockmocks.MockILocker).EXPECT().LockWithRenew(ctx, m.makeOnlineExptDaemonLockKey(exptID, runID), gomock.Any(), gomock.Any()).
							Return(true, ctx, func() {}, nil)
					}
				}
				m.lwt.(*lwtmocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(ctx, gomock.Any(), exptID).Return(false)
				m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().MGetByID(ctx, []int64{exptID}, spaceID).Return([]*entity.Experiment{expt}, nil)
				m.evaluationSetService.(*svcmocks.MockIEvaluationSetService).EXPECT().
					GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil)
				m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), spaceID, []int64{exptID}).Return(nil, nil)
				cfg := componentmocks.NewMockIConfiger(ctrl)
				m.configer = cfg
				cfg.EXPECT().GetExptExecConf(ctx, spaceID).Return(&entity.ExptExecConf{SpaceExptConcurLimit: 2}).AnyTimes()
				cfg.EXPECT().GetRetryYieldEnabled(ctx, spaceID).Return(false)
				m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(ctx, spaceID, gomock.Any(), gomock.Any()).Return(nil)
				err := fmt.Errorf("%w: producer unavailable", events.ErrPublishNotSent)
				m.publisher.(*eventsmocks.MockExptEventPublisher).EXPECT().PublishExptScheduleEvent(ctx, gomock.Any(), gomock.Any()).Return(err)
				if mode == entity.EvaluationModeRetryItems {
					require.ErrorIs(t, m.RetryItems(ctx, exptID, runID, spaceID, 0, []int64{4}, &entity.Session{}, nil), err)
				} else {
					require.ErrorIs(t, m.Run(ctx, exptID, runID, spaceID, 0, &entity.Session{}, mode, nil), err)
				}
				// Strict mocks reject run updates, quota release, and either unlock path.
			})
		}
	}
}
