// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPendingCancelFencesDelayedStartInsideRunLock(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminating, entity.ExptStatus_Terminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager, locker := svcmocks.NewMockIExptManager(ctrl), lockmocks.NewMockILocker(ctrl)
			ctx := context.Background()
			event := &entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3, ExptType: entity.ExptType_Offline}
			gomock.InOrder(
				locker.EXPECT().LockBackoffWithRenew(ctx, "expt_run_exec_lock:2:3", gomock.Any(), gomock.Any()).Return(true, ctx, func() {}, nil),
				manager.EXPECT().GetRunLog(gomock.Any(), int64(2), int64(3), int64(1), gomock.Any()).
					Return(&entity.ExptRunLog{Status: int64(status)}, nil),
				locker.EXPECT().Unlock("expt_run_exec_lock:2:3").Return(true, nil),
			)
			scheduler := &ExptSchedulerImpl{Manager: manager, Mutex: locker}
			started := false
			err := scheduler.HandleEventLock(func(context.Context, *entity.ExptScheduleEvent) error {
				started = true
				return nil
			})(ctx, event)
			require.NoError(t, err)
			require.False(t, started)
		})
	}
}

type pendingTerminationRepo struct {
	repo.IExptRunFinishRepo
	*repomocks.MockIExptRunTerminationRepo
}

func TestPendingCancelZeroRunSharesAdmissionLock(t *testing.T) {
	for _, acquired := range []bool{true, false} {
		t.Run(fmt.Sprint(acquired), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := newTestExptManager(ctrl)
			locker := lockmocks.NewMockILocker(ctrl)
			manager.manualRetryLocker = func() lock.ILocker { return locker }
			port := repomocks.NewMockIExptRunTerminationRepo(ctrl)
			manager.runFinishRepo = &pendingTerminationRepo{MockIExptRunTerminationRepo: port}
			ctx := context.Background()
			locker.EXPECT().LockWithRenew(ctx, manager.makeExptMutexLockKey(2), gomock.Any(), gomock.Any()).Return(acquired, ctx, func() {}, nil)
			if acquired {
				port.EXPECT().TerminateCurrentRun(ctx, gomock.Any()).DoAndReturn(
					func(_ context.Context, r *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
						require.True(t, r.Begin)
						require.False(t, r.SetupAbort)
						require.Zero(t, r.RunID)
						return &entity.ExptRunTerminationResult{Applied: true}, nil
					})
				locker.EXPECT().Unlock(manager.makeExptMutexLockKey(2)).Return(true, nil)
			}
			err := manager.beginCurrentRunTermination(ctx, 2, 0, 1)
			if acquired {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestPendingCancelUsesOriginalQuotaCleanup(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	manager.mtr, manager.publisher = nil, nil
	locker := lockmocks.NewMockILocker(ctrl)
	manager.manualRetryLocker = func() lock.ILocker { return locker }
	port := repomocks.NewMockIExptRunTerminationRepo(ctrl)
	manager.runFinishRepo = &pendingTerminationRepo{MockIExptRunTerminationRepo: port}
	ctx := context.Background()
	finished := &entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Terminated}
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:2:3", gomock.Any(), gomock.Any()).Return(true, ctx, func() {}, nil)
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).
		Return(&entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, Status: entity.ExptStatus_Pending}, nil)
	manager.itemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().ScanItemRunLogs(gomock.Any(), int64(2), int64(3), gomock.Any(), int64(0), int64(500), int64(1)).
		Return(nil, int64(0), nil)
	port.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
			require.True(t, r.Finalize)
			require.False(t, r.SetupAbort)
			return &entity.ExptRunTerminationResult{Applied: true, Experiment: finished}, nil
		})
	locker.EXPECT().UnlockWithValue(gomock.Any(), manager.makeExptMutexLockKey(2), "3").Return(true, nil)
	locker.EXPECT().LockWithRenew(gomock.Any(), manager.makeExptMutexLockKey(2), gomock.Any(), gomock.Any()).Return(true, ctx, func() {}, nil)
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(finished, nil)
	manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			quota, changed, err := update(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{2: 10, 4: 20}})
			require.NoError(t, err)
			require.True(t, changed)
			require.Equal(t, map[int64]int64{4: 20}, quota.ExptID2RunTime)
			return nil
		})
	// Skip notifications already completed by another cancellation; resource release still runs.
	manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), "finish-cleanup:2:3").Return(true, nil)
	applied, err := manager.TerminateCurrentRun(ctx, 2, 3, 1, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.True(t, applied)
}

func TestPendingCancelFencesLateInitialLogRun(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminating, entity.ExptStatus_Terminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := newTestExptManager(ctrl)
			manager.runFinishRepo = &struct{ repo.IExptRunFinishRepo }{}
			ctx := context.Background()
			locker := manager.mutex.(*lockmocks.MockILocker)
			gomock.InOrder(
				locker.EXPECT().BackoffLockWithValue(ctx, manager.makeExptMutexLockKey(2), "3", gomock.Any(), gomock.Any()).Return(true, "", nil),
				manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).
					Return(&entity.Experiment{ID: 2, SpaceID: 1, Status: status}, nil),
				locker.EXPECT().UnlockWithValue(gomock.Any(), manager.makeExptMutexLockKey(2), "3").Return(true, nil),
			)
			err := manager.LogRun(ctx, 2, 3, entity.EvaluationModeSubmit, 1, nil, &entity.Session{})
			require.Error(t, err)
		})
	}
}
