// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	mysqlmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestLifecycleProtocolLogRunUsesInjectedEvaluation(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun, entity.EvaluationModeRetryAll, entity.EvaluationModeFailRetry} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			ctrl := gomock.NewController(t)
			cfg := componentmocks.NewMockIConfiger(ctrl)
			cfg.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 3600})
			runs := repomocks.NewMockIExptRunLogRepo(ctrl)
			runs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			expts.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil).AnyTimes()
			metric := metricsmocks.NewMockExptMetric(ctrl)
			metric.EXPECT().EmitExptExecRun(int64(3), int64(mode))
			mgr := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: factory().WithHolder("evaluation"), configer: cfg, exptRepo: expts, runLogRepo: runs, mtr: metric}
			require.NoError(t, mgr.LogRun(context.Background(), 1, 2, mode, 3, nil, &entity.Session{}))
			holder, err := server.Get("expt_run_mutex_lock:1")
			require.NoError(t, err)
			require.Equal(t, "evaluation", holder, "legacy RetryItems readers must not mistake an ordinary run for a numeric retry run")
		})
	}
}

func TestLifecycleProtocolEvaluationInitializationFailureReleases(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	ctrl := gomock.NewController(t)
	cfg := componentmocks.NewMockIConfiger(ctrl)
	cfg.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 3600}).Times(2)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	failure := errors.New("run Create rejected")
	runs.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptRunLog) error {
		holder, err := server.Get("expt_run_mutex_lock:1")
		require.NoError(t, err)
		require.Equal(t, "evaluation", holder)
		return failure
	})
	runs.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil).AnyTimes()
	expts.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(int64(3), int64(entity.EvaluationModeSubmit)).Times(2)
	mgr := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: factory().WithHolder("evaluation"), manualRetryLocker: factory, configer: cfg, exptRepo: expts, runLogRepo: runs, mtr: metric}
	require.ErrorIs(t, mgr.LogRun(context.Background(), 1, 2, entity.EvaluationModeSubmit, 3, nil, &entity.Session{}), failure)
	require.False(t, server.Exists("expt_run_mutex_lock:1"))
	require.NoError(t, mgr.LogRun(context.Background(), 1, 4, entity.EvaluationModeSubmit, 3, nil, &entity.Session{}))
}

func TestLifecycleProtocolInitialPairPrecedesSharedEffects(t *testing.T) {
	for _, holder := range []string{"evaluation", "2"} {
		for _, endpoint := range []string{"natural", "termination", "cancelled"} {
			t.Run(holder+"/"+endpoint, func(t *testing.T) {
				factory, server := finishRealLockerFactory(t)
				key := "expt_run_mutex_lock:1"
				require.NoError(t, server.Set(key, holder))
				server.SetTTL(key, time.Hour)
				ctrl := gomock.NewController(t)
				current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}
				expts := repomocks.NewMockIExperimentRepo(ctrl)
				expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil).AnyTimes()
				quota := repomocks.NewMockQuotaRepo(ctrl)
				quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
					require.NoError(t, ctx.Err())
					require.False(t, server.Exists(key), "initial admission release precedes post-commit shared effects")
					return nil
				})
				mgr := &ExptMangerImpl{manualRetryLocker: factory, mutex: factory().WithHolder("evaluation"), exptRepo: expts, quotaRepo: quota, idem: &cleanupTestIdem{}}
				event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, Session: &entity.Session{}}
				if holder == "2" {
					event.ExptRunMode = entity.EvaluationModeRetryItems
				}
				require.NoError(t, mgr.releaseCompleteAdmissionForMode(context.Background(), 1, 2, event.ExptRunMode))
				ctx := context.Background()
				var err error
				if endpoint == "natural" {
					err = completeNaturalFixture(mgr, ctx, event, current)
				} else {
					if endpoint == "cancelled" {
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						cancel()
					}
					err = completeTerminationFixture(mgr, ctx, event, current, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})
				}
				if endpoint == "cancelled" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				require.False(t, server.Exists(key), "finished current run must release its lifecycle lock")
			})
		}
	}
}

func TestLifecycleProtocolCompletedReplayDoesNotTouchReplacementAdmission(t *testing.T) {
	for _, newerSQL := range []bool{false, true} {
		for _, endpoint := range []string{"natural", "termination", "cancelled"} {
			t.Run(fmt.Sprintf("sql_%v/%s", newerSQL, endpoint), func(t *testing.T) {
				factory, server := finishRealLockerFactory(t)
				require.NoError(t, server.Set("expt_run_mutex_lock:1", "evaluation"))
				ctrl := gomock.NewController(t)
				current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}
				frozen := *current
				if newerSQL {
					current.LatestRunID = 99
					current.Status = entity.ExptStatus_Processing
				}
				expts := repomocks.NewMockIExperimentRepo(ctrl)
				expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil).AnyTimes()
				mgr := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: expts, quotaRepo: repomocks.NewMockQuotaRepo(ctrl), idem: &cleanupTestIdem{seen: true}}
				if newerSQL {
					port := repomocks.NewMockIExptRunFinishRepo(ctrl)
					request := &entity.ExptRunFinishRequest{SpaceID: 3, ExptID: 1, RunID: 2}
					if endpoint == "cancelled" {
						port.EXPECT().FinishCurrentRun(gomock.Any(), request).DoAndReturn(func(ctx context.Context, _ *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
							if err := ctx.Err(); err != nil {
								require.ErrorIs(t, err, context.Canceled)
								return nil, err
							}
							return &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishNonCurrent}, nil
						}).Times(2)
					} else {
						port.EXPECT().FinishCurrentRun(gomock.Any(), request).Return(&entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishNonCurrent}, nil).Times(1)
					}
					mgr.runFinishRepo = port
				}
				event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
				ctx := context.Background()
				if endpoint == "cancelled" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				var err error
				if endpoint == "natural" {
					err = completeNaturalFixture(mgr, context.Background(), event, &frozen)
				} else {
					err = completeTerminationFixture(mgr, ctx, event, &frozen, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})
				}
				if newerSQL && endpoint == "cancelled" {
					require.ErrorIs(t, err, context.Canceled)
					require.True(t, errno.NeedMQRetry(err))
				} else {
					require.NoError(t, err)
				}
				actual, err := server.Get("expt_run_mutex_lock:1")
				require.NoError(t, err)
				require.Equal(t, "evaluation", actual, "completed replay must exit before deleting the next admission or releasing shared quota")
				if newerSQL && endpoint == "cancelled" {
					require.NoError(t, completeTerminationFixture(mgr, context.Background(), event, &frozen, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
					actual, err = server.Get("expt_run_mutex_lock:1")
					require.NoError(t, err)
					require.Equal(t, "evaluation", actual, "healthy R1 replay must preserve the R2 admission and quota")
				}
			})
		}
	}
}

func TestLifecycleProtocolRetryItemsNumericJoinPreserved(t *testing.T) {
	factory, server := finishRealLockerFactory(t)
	require.NoError(t, server.Set("expt_run_mutex_lock:1", "2"))
	ctrl := gomock.NewController(t)
	cfg := componentmocks.NewMockIConfiger(ctrl)
	cfg.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 3600})
	ids := idgenmocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenID(gomock.Any()).Return(int64(4), nil)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{SpaceID: 3, ExptID: 1, ExptRunID: 2, Mode: int32(entity.EvaluationModeRetryItems), Status: int64(entity.ExptStatus_Processing)}, nil)
	runs.EXPECT().AppendRunItemIDs(gomock.Any(), int64(1), int64(2), int64(3), []int64{11}).Return(nil)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3}, nil).AnyTimes()
	mgr := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, mutex: factory().WithHolder("evaluation"), configer: cfg, runLogRepo: runs, idgenerator: ids, exptRepo: expts}
	run, joined, err := mgr.LogRetryItemsRun(context.Background(), 1, entity.EvaluationModeRetryItems, 3, []int64{11}, &entity.Session{})
	require.NoError(t, err)
	require.True(t, joined)
	require.EqualValues(t, 2, run)
	actual, err := server.Get("expt_run_mutex_lock:1")
	require.NoError(t, err)
	require.Equal(t, "2", actual)
}

// The real SQL composite finisher must preserve its atomic counts/endAt and
// current-run condition through all public completion entry points.
func TestLifecycleProtocolSQLCompletionEndpoints(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems} {
		for _, endpoint := range []string{"natural", "complete_run", "complete_expt"} {
			t.Run(fmt.Sprintf("mode_%v/%s", mode, endpoint), func(t *testing.T) {
				f := newLateWriteFixture(t, "rq1")
				require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResult{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Success)).Error)
				require.NoError(t, f.conn.Model(&mysqlmodel.ExptItemResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
				require.NoError(t, f.conn.Model(&mysqlmodel.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Success)).Error)
				require.NoError(t, f.conn.Model(&mysqlmodel.ExptRunLog{}).Where("space_id=?", f.space).Update("mode", int32(mode)).Error)
				factory, server := finishRealLockerFactory(t)
				key := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
				holder := "evaluation"
				if mode == entity.EvaluationModeRetryItems {
					holder = fmt.Sprint(f.run)
				}
				require.NoError(t, server.Set(key, holder))
				server.SetTTL(key, time.Hour)
				ctrl := gomock.NewController(t)
				quota := repomocks.NewMockQuotaRepo(ctrl)
				quota.EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
					var row mysqlmodel.Experiment
					require.NoError(t, f.conn.WithContext(ctx).Where("space_id=?", f.space).Take(&row).Error)
					require.Equal(t, int32(entity.ExptStatus_Success), row.Status, "SQL transaction must finish before shared quota is released")
					require.False(t, server.Exists(key), "initial paired release precedes post-commit side effects")
					return nil
				})
				resultSvc := svcmocks.NewMockExptResultService(ctrl)
				resultSvc.EXPECT().GetIncompleteTurns(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil, nil).AnyTimes()
				mgr := &ExptMangerImpl{runFinishRepo: experimentrepo.NewExptRunFinishRepo(f.provider), runLogRepo: experimentrepo.NewExptRunLogRepo(experimentmysql.NewExptRunLogDAO(f.provider)), manualRetryLocker: factory, mutex: factory().WithHolder("evaluation"), exptRepo: f.experimentRepo(), quotaRepo: quota, idem: &cleanupTestIdem{}, exptResultService: &finishCalibrationTestResult{ExptResultService: resultSvc, ready: true}}
				event := &entity.ExptScheduleEvent{ExptID: f.expt, ExptRunID: f.run, SpaceID: f.space, Session: &entity.Session{}}
				complete := func() error {
					switch endpoint {
					case "complete_run":
						return mgr.CompleteRun(context.Background(), f.expt, f.run, f.space, event.Session)
					case "complete_expt":
						return mgr.CompleteExpt(context.Background(), f.expt, &f.run, f.space, event.Session)
					default:
						_, err := mgr.FinishCurrentRun(context.Background(), event)
						return err
					}
				}
				// No incomplete turns remain; the calibration fixture also supplies
				// the cleanup reader without a mock that could hide missing SQL work.
				require.NoError(t, complete())
				require.False(t, server.Exists(key))
				before := lateWriteSnapshot(f)
				require.NoError(t, complete(), "duplicate completion must not recalculate settled counts or release quota twice")
				require.Equal(t, before, lateWriteSnapshot(f))
				require.NoError(t, f.conn.Model(&mysqlmodel.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"latest_run_id": f.run + 1, "status": int32(entity.ExptStatus_Processing)}).Error)
				require.NoError(t, server.Set(key, "evaluation"))
				require.NoError(t, complete(), "old terminal message must exit after SQL confirms a replacement")
				actual, err := server.Get(key)
				require.NoError(t, err)
				require.Equal(t, "evaluation", actual)
			})
		}
	}
}

func TestLifecycleProtocolFailedPreparationReleaseContracts(t *testing.T) {
	for _, tc := range []struct {
		name, holder        string
		mode                entity.ExptRunMode
		latest              int64
		cancelled, released bool
	}{
		{"ordinary_evaluation", "evaluation", entity.EvaluationModeSubmit, 2, false, true},
		{"retry_items_numeric", "2", entity.EvaluationModeRetryItems, 2, false, true},
		{"other_numeric_owner", "99", entity.EvaluationModeSubmit, 2, false, false},
		{"confirmed_replacement", "evaluation", entity.EvaluationModeSubmit, 99, false, false},
		{"cancelled_preparation", "evaluation", entity.EvaluationModeSubmit, 2, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			factory, server := finishRealLockerFactory(t)
			key := "expt_run_mutex_lock:1"
			require.NoError(t, server.Set(key, tc.holder))
			ctrl := gomock.NewController(t)
			current := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: tc.latest, Status: entity.ExptStatus_Failed}
			expts := repomocks.NewMockIExperimentRepo(ctrl)
			expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(current, nil)
			runs := repomocks.NewMockIExptRunLogRepo(ctrl)
			logged := &entity.ExptRunLog{ExptID: 1, ExptRunID: 2, SpaceID: 3, Mode: int32(tc.mode), Status: int64(entity.ExptStatus_Pending)}
			if tc.latest == 2 {
				runs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(logged, nil).AnyTimes()
				runs.EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).DoAndReturn(func(ctx context.Context, _, _, _ int64, fields map[string]any) error {
					require.NoError(t, ctx.Err(), "bounded setup cleanup must survive request cancellation")
					logged.Status = fields["status"].(int64)
					return nil
				})
			}
			mgr := &ExptMangerImpl{runFinishRepo: &finishRepoProbe{}, manualRetryLocker: factory, mutex: factory().WithHolder("evaluation"), exptRepo: expts, runLogRepo: runs}
			ctx := context.Background()
			if tc.cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			require.NoError(t, mgr.cleanupCurrentUnscheduledRun(ctx, 1, 2, 3, errors.New("preparation failed")))
			require.Equal(t, tc.released, !server.Exists(key))
			require.Equal(t, tc.latest, current.LatestRunID)
			require.Equal(t, entity.ExptStatus_Failed, current.Status)
			if tc.latest == 2 {
				require.Equal(t, int64(entity.ExptStatus_Failed), logged.Status)
			} else {
				require.Equal(t, int64(entity.ExptStatus_Pending), logged.Status)
			}
		})
	}
}

func TestLifecycleProtocolLateCleanupSkipsConfirmedNumericReplacement(t *testing.T) {
	for _, holder := range []string{"99", "evaluation"} {
		for _, endpoint := range []string{"natural", "termination", "cancelled"} {
			t.Run(holder+"/"+endpoint, func(t *testing.T) {
				factory, server := finishRealLockerFactory(t)
				key := "expt_run_mutex_lock:1"
				require.NoError(t, server.Set(key, holder))
				ctrl := gomock.NewController(t)
				expts := repomocks.NewMockIExperimentRepo(ctrl)
				expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 99, Status: entity.ExptStatus_Processing}, nil).AnyTimes()
				mgr := &ExptMangerImpl{manualRetryLocker: factory, exptRepo: expts, quotaRepo: repomocks.NewMockQuotaRepo(ctrl), idem: &cleanupTestIdem{}}
				port := repomocks.NewMockIExptRunFinishRepo(ctrl)
				request := &entity.ExptRunFinishRequest{SpaceID: 3, ExptID: 1, RunID: 2}
				if endpoint == "cancelled" {
					port.EXPECT().FinishCurrentRun(gomock.Any(), request).DoAndReturn(func(ctx context.Context, _ *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
						if err := ctx.Err(); err != nil {
							require.ErrorIs(t, err, context.Canceled)
							return nil, err
						}
						return &entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishNonCurrent}, nil
					}).Times(2)
				} else {
					port.EXPECT().FinishCurrentRun(gomock.Any(), request).Return(&entity.ExptRunFinishResult{Disposition: entity.ExptRunFinishNonCurrent}, nil).Times(1)
				}
				mgr.runFinishRepo = port
				frozen := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Success, EndAt: gptr.Of(time.Now())}
				event := &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3}
				ctx := context.Background()
				var err error
				if endpoint == "natural" {
					err = completeNaturalFixture(mgr, ctx, event, frozen)
				} else {
					if endpoint == "cancelled" {
						var cancel context.CancelFunc
						ctx, cancel = context.WithCancel(ctx)
						cancel()
					}
					err = completeTerminationFixture(mgr, ctx, event, frozen, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true})
				}
				if endpoint == "cancelled" {
					require.ErrorIs(t, err, context.Canceled)
					require.True(t, errno.NeedMQRetry(err))
				} else {
					require.NoError(t, err, "confirmed replacement is an idempotent exit")
				}
				actual, err := server.Get(key)
				require.NoError(t, err)
				require.Equal(t, holder, actual)
				if endpoint == "cancelled" {
					require.NoError(t, completeTerminationFixture(mgr, context.Background(), event, frozen, nil, &entity.CompleteExptOption{NoCompleteItemTurn: true}))
					actual, err = server.Get(key)
					require.NoError(t, err)
					require.Equal(t, holder, actual, "healthy R1 replay must preserve the R2 holder and quota")
				}
			})
		}
	}
}
