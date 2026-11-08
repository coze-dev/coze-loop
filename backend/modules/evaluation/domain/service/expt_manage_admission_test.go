// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestExptAdmissionJoinRejectsFirstCreationRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(101), int64(104)).Return(&entity.Experiment{ID: 101, SpaceID: 104, ExptType: entity.ExptType_Offline}, nil)
	manager.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(103), nil)
	manager.mutex.(*lockmocks.MockILocker).EXPECT().BackoffLockWithValue(gomock.Any(), "expt_run_mutex_lock:101", "103", gomock.Any(), gomock.Any()).Return(false, "102", nil)
	id, joined, err := manager.LogRetryItemsRun(context.Background(), 101, entity.EvaluationModeRetryItems, 104, []int64{42}, &entity.Session{UserID: "test"})
	require.True(t, errno.NeedMQRetry(err))
	require.Zero(t, id)
	require.False(t, joined)
}

func TestAdmissionCleanupPreservesCallerCancellationContract(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(fmt.Sprint(owned), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := newTestExptManager(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if owned {
				ctx = context.WithValue(ctx, admissionTickOwnerKey{}, admissionTickOwner{ctx: context.Background(), stop: func() { t.Error("live admission owner must not be replaced") }})
			}
			manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(101), int64(103), int64(104), gomock.Any()).DoAndReturn(func(state context.Context, _, _, _ int64, fields map[string]any) error {
				require.NoError(t, state.Err())
				require.Equal(t, int64(entity.ExptStatus_Failed), fields["status"])
				return nil
			})
			manager.mutex.(*lockmocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), "expt_run_mutex_lock:101").DoAndReturn(func(release context.Context, _ string) (bool, error) {
				require.NoError(t, release.Err())
				return true, nil
			})
			manager.cleanupUnscheduledRun(ctx, 101, 103, 104, context.Canceled)
		})
	}
}

func TestAdmissionCleanupLostOwnerDoesNotReleaseChangedCurrent(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	owner, cancel := context.WithCancel(context.Background())
	cancel()
	ctx := context.WithValue(context.Background(), admissionTickKey{}, admissionTickIdentity{101, 102})
	ctx = context.WithValue(ctx, admissionTickOwnerKey{}, admissionTickOwner{ctx: owner, stop: func() {}})
	manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(101), int64(103), int64(104), gomock.Any()).DoAndReturn(func(state context.Context, _, _, _ int64, fields map[string]any) error {
		require.NoError(t, state.Err())
		require.Equal(t, int64(entity.ExptStatus_Failed), fields["status"])
		return nil
	})
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(101), int64(104)).Return(&entity.Experiment{ID: 101, SpaceID: 104, ExptType: entity.ExptType_Offline, LatestRunID: 105}, nil).AnyTimes()
	manager.runTickLocker = func() lock.ILocker {
		locker := lockmocks.NewMockILocker(ctrl)
		locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), 5*time.Second, 5*time.Minute).DoAndReturn(func(parent context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
			child, stop := context.WithCancel(parent)
			return true, child, stop, nil
		})
		locker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
		return locker
	}
	manager.cleanupUnscheduledRun(ctx, 101, 103, 104, context.Canceled)
}

func TestAdmissionCleanupLostOwnerCanRecoverOwnCommittedRun(t *testing.T) {
	for _, held := range []bool{true, false} {
		t.Run(fmt.Sprint(held), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := newTestExptManager(ctrl)
			owner, cancel := context.WithCancel(context.Background())
			cancel()
			stopped := false
			ctx := context.WithValue(context.Background(), admissionTickKey{}, admissionTickIdentity{101, 102})
			ctx = context.WithValue(ctx, admissionTickOwnerKey{}, admissionTickOwner{ctx: owner, stop: func() { stopped = true }})
			manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(101), int64(103), int64(104), gomock.Any()).DoAndReturn(func(state context.Context, _, _, _ int64, fields map[string]any) error {
				require.NoError(t, state.Err())
				require.Equal(t, int64(entity.ExptStatus_Failed), fields["status"])
				return nil
			})
			manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(101), int64(104)).Return(&entity.Experiment{ID: 101, SpaceID: 104, ExptType: entity.ExptType_Offline, LatestRunID: 103}, nil).AnyTimes()
			manager.runTickLocker = func() lock.ILocker {
				locker := lockmocks.NewMockILocker(ctrl)
				locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:101:103", 5*time.Second, 5*time.Minute).DoAndReturn(func(parent context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
					child, stop := context.WithCancel(parent)
					return held, child, stop, nil
				})
				if held {
					locker.EXPECT().Unlock("expt_run_exec_lock:101:103").Return(true, nil)
				}
				return locker
			}
			if held {
				manager.mutex.(*lockmocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), "expt_run_mutex_lock:101").DoAndReturn(func(release context.Context, _ string) (bool, error) {
					require.NoError(t, release.Err())
					return true, nil
				})
			}
			manager.cleanupUnscheduledRun(ctx, 101, 103, 104, context.Canceled)
			require.True(t, stopped)
		})
	}
}
