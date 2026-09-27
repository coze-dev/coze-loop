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

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestLogRunPersistenceFailureReleasesOwnedLock(t *testing.T) {
	for _, tc := range []struct {
		name, failure         string
		retryItems, committed bool
	}{
		{"create_run_log", "persist", false, false},
		{"create_run_log_response_lost", "persist", false, true},
		{"update_latest_run", "latest", false, false},
		{"update_latest_run_response_lost", "latest", false, true},
		{"save_retry_items_run", "persist", true, false},
		{"save_lost_response_new_run", "persist", true, true},
		{"update_retry_items_latest_run", "latest", true, false},
		{"update_retry_items_latest_run_response_lost", "latest", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := newTestExptManager(ctrl)
			ctx := context.Background()
			const exptID, runID, nextRunID, spaceID = int64(1), int64(2), int64(3), int64(4)
			const oldRunID = int64(5)
			latestRunID := oldRunID
			session := &entity.Session{UserID: "u"}
			writeErr := errors.New("persistence unavailable")
			locked, attempts := false, 0
			var saved *entity.ExptRunLog
			persist := func(_ context.Context, run *entity.ExptRunLog) error {
				if attempts == 1 && tc.failure == "persist" {
					if tc.committed {
						saved = run
					}
					return writeErr
				}
				saved = run
				return nil
			}
			acquire := func() bool {
				if locked {
					return false
				}
				attempts++
				locked = true
				return true
			}
			if tc.retryItems {
				mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(ctx).
					DoAndReturn(func(context.Context) (int64, error) { return runID + int64(attempts), nil }).Times(2)
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().BackoffLockWithValue(ctx, mgr.makeExptMutexLockKey(exptID), gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(context.Context, string, string, time.Duration, time.Duration) (bool, string, error) {
						return acquire(), "2", nil
					}).Times(2)
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Save(ctx, gomock.Any()).DoAndReturn(persist).Times(2)
				mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, int64(entity.EvaluationModeRetryItems))
			} else {
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().LockBackoff(ctx, mgr.makeExptMutexLockKey(exptID), gomock.Any(), gomock.Any()).
					DoAndReturn(func(context.Context, string, time.Duration, time.Duration) (bool, error) { return acquire(), nil }).Times(2)
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Create(ctx, gomock.Any()).DoAndReturn(persist).Times(2)
				mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, int64(entity.EvaluationModeFailRetry)).Times(2)
			}
			updates := 1
			if tc.failure == "latest" {
				updates++
			}
			mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(ctx, gomock.Any()).
				DoAndReturn(func(_ context.Context, expt *entity.Experiment) error {
					if attempts == 1 {
						if tc.committed {
							latestRunID = expt.LatestRunID
						}
						return writeErr
					}
					latestRunID = expt.LatestRunID
					return nil
				}).Times(updates)
			mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), exptID, runID, gomock.Any()).
				DoAndReturn(func(cleanupCtx context.Context, _, _ int64, fields map[string]any) error {
					require.NoError(t, cleanupCtx.Err())
					require.Equal(t, map[string]any{"status": int64(entity.ExptStatus_Failed), "status_message": []byte(writeErr.Error())}, fields)
					if saved == nil {
						return nil
					}
					saved.Status = fields["status"].(int64)
					saved.StatusMessage = fields["status_message"].([]byte)
					return nil
				})
			mgr.mutex.(*lockMocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), mgr.makeExptMutexLockKey(exptID)).
				DoAndReturn(func(context.Context, string) (bool, error) { locked = false; return true, nil })
			if tc.retryItems {
				gotRunID, _, err := mgr.LogRetryItemsRun(ctx, exptID, entity.EvaluationModeRetryItems, spaceID, []int64{4}, session)
				require.ErrorIs(t, err, writeErr)
				require.Zero(t, gotRunID, "cleanup must retain the owned run ID even though the failed API returns zero")
			} else {
				require.ErrorIs(t, mgr.LogRun(ctx, exptID, runID, entity.EvaluationModeFailRetry, spaceID, nil, session), writeErr)
			}
			require.False(t, locked)
			wantLatest := oldRunID
			if tc.failure == "latest" && tc.committed {
				wantLatest = runID
			}
			require.Equal(t, wantLatest, latestRunID, "cleanup must not overwrite the persisted experiment pointer")
			if saved != nil {
				require.Equal(t, int64(entity.ExptStatus_Failed), saved.Status)
				require.Equal(t, writeErr.Error(), string(saved.StatusMessage))
			}
			if tc.retryItems {
				gotRunID, retried, err := mgr.LogRetryItemsRun(ctx, exptID, entity.EvaluationModeRetryItems, spaceID, []int64{4}, session)
				require.NoError(t, err)
				require.False(t, retried)
				require.Equal(t, nextRunID, gotRunID)
			} else {
				require.NoError(t, mgr.LogRun(ctx, exptID, nextRunID, entity.EvaluationModeFailRetry, spaceID, nil, session))
			}
			require.True(t, locked)
			require.Equal(t, nextRunID, latestRunID)
		})
	}
}

func TestLogRetryItemsPersistenceFailurePreservesExistingRun(t *testing.T) {
	for _, tc := range []struct {
		name      string
		committed bool
	}{
		{"save_failure_existing_run", false},
		{"save_lost_response_existing_run", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := newTestExptManager(ctrl)
			ctx := context.Background()
			const exptID, existingRunID, proposedRunID, spaceID = int64(1), int64(2), int64(3), int64(4)
			mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(ctx).Return(proposedRunID, nil)
			mgr.mutex.(*lockMocks.MockILocker).EXPECT().BackoffLockWithValue(ctx, mgr.makeExptMutexLockKey(exptID), "3", gomock.Any(), gomock.Any()).Return(false, "2", nil)
			mgr.mutex.(*lockMocks.MockILocker).EXPECT().Exists(ctx, gomock.Any()).Return(false, nil)
			stored := &entity.ExptRunLog{ExptID: exptID, ExptRunID: existingRunID, Status: int64(entity.ExptStatus_Processing)}
			loaded := *stored
			mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Get(ctx, exptID, existingRunID).Return(&loaded, nil)
			writeErr := errors.New("append failed")
			mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Save(ctx, &loaded).
				DoAndReturn(func(_ context.Context, run *entity.ExptRunLog) error {
					if tc.committed {
						*stored = *run
					}
					return writeErr
				})
			runID, retried, err := mgr.LogRetryItemsRun(ctx, exptID, entity.EvaluationModeRetryItems, spaceID, []int64{5}, &entity.Session{UserID: "u"})
			require.ErrorIs(t, err, writeErr)
			require.Zero(t, runID)
			require.False(t, retried)
			require.Equal(t, int64(entity.ExptStatus_Processing), stored.Status)
			if tc.committed {
				require.Len(t, stored.ItemIds, 1)
				require.Equal(t, []int64{5}, stored.ItemIds[0].ItemIDs)
			} else {
				require.Empty(t, stored.ItemIds)
			}
		})
	}
}

func TestLogRunLockErrorDoesNotCleanup(t *testing.T) {
	for _, retryItems := range []bool{false, true} {
		t.Run(map[bool]string{false: "LogRun", true: "LogRetryItemsRun"}[retryItems], func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mgr := newTestExptManager(ctrl)
			ctx := context.Background()
			lockErr := errors.New("lock outcome unknown")
			if retryItems {
				mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(ctx).Return(int64(2), nil)
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().BackoffLockWithValue(ctx, gomock.Any(), "2", gomock.Any(), gomock.Any()).Return(true, "", lockErr)
				_, _, err := mgr.LogRetryItemsRun(ctx, 1, entity.EvaluationModeRetryItems, 3, nil, &entity.Session{UserID: "u"})
				require.ErrorIs(t, err, lockErr)
			} else {
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().LockBackoff(ctx, gomock.Any(), gomock.Any(), gomock.Any()).Return(true, lockErr)
				require.ErrorIs(t, mgr.LogRun(ctx, 1, 2, entity.EvaluationModeFailRetry, 3, nil, &entity.Session{UserID: "u"}), lockErr)
			}
		})
	}
}

func TestCleanupUnscheduledRunRejectsInvalidRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mgr := newTestExptManager(ctrl)
	for _, runID := range []int64{0, -1} {
		mgr.cleanupUnscheduledRun(context.Background(), 1, runID, errors.New("run failed"))
	}
}

func TestLogRunLatestFailureDoesNotRollbackSupersededQuota(t *testing.T) {
	for _, retryItems := range []bool{false, true} {
		for _, committed := range []bool{false, true} {
			t.Run(fmt.Sprintf("retry_items_%v_committed_%v", retryItems, committed), func(t *testing.T) {
				ctrl := gomock.NewController(t)
				mgr := newTestExptManager(ctrl)
				ctx := context.Background()
				const exptID, oldRunID, newRunID, spaceID = int64(1), int64(2), int64(3), int64(4)
				guard := &fakeGuard{}
				mgr.centralGuard = guard
				expt := &entity.Experiment{
					ID: exptID, SpaceID: spaceID, LatestRunID: oldRunID,
					SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce,
				}
				if retryItems {
					mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(ctx).Return(newRunID, nil)
					mgr.mutex.(*lockMocks.MockILocker).EXPECT().BackoffLockWithValue(ctx, gomock.Any(), "3", gomock.Any(), gomock.Any()).Return(true, "", nil)
					mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Save(ctx, gomock.Any()).Return(nil)
				} else {
					mgr.mutex.(*lockMocks.MockILocker).EXPECT().LockBackoff(ctx, gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
					mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Create(ctx, gomock.Any()).Return(nil)
					mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, int64(entity.EvaluationModeFailRetry))
				}
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(expt, nil)
				mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
					ScanItemRunLogs(ctx, exptID, oldRunID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).
					Return([]*entity.ExptItemResultRunLog{{ItemID: 11}}, int64(1), nil)
				writeErr := errors.New("latest run write failed")
				mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(ctx, gomock.Any()).
					DoAndReturn(func(_ context.Context, update *entity.Experiment) error {
						// Releasing the old quota precedes this write; these operations are not a transaction.
						require.Len(t, guard.releases(), 1)
						if committed {
							expt.LatestRunID = update.LatestRunID
						}
						return writeErr
					})
				mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), exptID, newRunID, gomock.Any()).Return(nil)
				mgr.mutex.(*lockMocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), mgr.makeExptMutexLockKey(exptID)).Return(true, nil)
				if retryItems {
					_, _, err := mgr.LogRetryItemsRun(ctx, exptID, entity.EvaluationModeRetryItems, spaceID, nil, &entity.Session{UserID: "u"})
					require.ErrorIs(t, err, writeErr)
				} else {
					require.ErrorIs(t, mgr.LogRun(ctx, exptID, newRunID, entity.EvaluationModeFailRetry, spaceID, nil, &entity.Session{UserID: "u"}), writeErr)
				}
				require.Len(t, guard.releases(), 1)
				require.Equal(t, oldRunID, guard.releases()[0].RunID)
				wantLatest := oldRunID
				if committed {
					wantLatest = newRunID
				}
				require.Equal(t, wantLatest, expt.LatestRunID)
			})
		}
	}
}
