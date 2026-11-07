// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// 自然完成（exptEnd -> CompleteExpt，没有显式 Status）时，未跑完 item 的账本释放
// 过去是 void：Release 失败只打 Warn，随后 terminateIncompleteItemRunLogs 把 run log
// 改成 Terminal、实验落终态。此后：
//   - 实验终态 => 调度拍不再扫这个 run；
//   - run log Terminal + 投影 none => ReconcileTerminalResourceDebt 的 reserved 判据不成立；
//   - Completion 已通知 => 重投递也不会再走 release。
//
// 三处兜底都不接，reservation 永久留在账本里。本用例要求完成侧保持可重放：
// 释放失败必须转 MQRetry，且不得写终态、不得把 run log 改 Terminal。
func TestCompleteExpt_QuotaReleaseFailureKeepsRunRetryable(t *testing.T) {
	for _, tc := range []struct {
		name       string
		scanErr    error
		releaseErr error
	}{
		{name: "release_failure", releaseErr: assert.AnError},
		{name: "observation_failure", scanErr: assert.AnError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := newTestExptManager(ctrl)
			manager.runFinishRepo = &originalCompletionRunFinishRepo{}
			guard := &fakeGuard{releaseErr: tc.releaseErr}
			manager.centralGuard = guard
			items := repoMocks.NewMockIExptItemResultRepo(ctrl)
			manager.itemResultRepo = items

			ctx := context.Background()
			session := &entity.Session{UserID: "operator"}
			const exptID, spaceID, runID = int64(1), int64(3), int64(2)
			expt := &entity.Experiment{
				ID: exptID, SpaceID: spaceID, LatestRunID: runID, Status: entity.ExptStatus_Processing,
				ExptType: entity.ExptType_Offline, StartAt: gptr.Of(time.Now()),
				SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce,
			}

			manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
			manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(expt, nil).AnyTimes()
			finalized := false
			manager.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, _ *entity.Experiment) error { finalized = true; return nil }).AnyTimes()
			manager.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), exptID, runID).
				Return(&entity.ExptRunLog{ExptID: exptID, ExptRunID: runID, Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
			manager.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), exptID, runID, spaceID, gomock.Any()).Return(nil).AnyTimes()
			manager.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
				ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any(), false).Return(nil, int64(0), nil).AnyTimes()
			manager.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
				CalculateStats(gomock.Any(), exptID, spaceID, session).Return(&entity.ExptCalculateStats{ProcessingItemCnt: 2}, nil)
			manager.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
				GetIncompleteTurns(gomock.Any(), exptID, spaceID, session).Return(nil, nil).AnyTimes()
			manager.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
				UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
				UpdateByExptID(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil)
			manager.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
				Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{ProcessingItemCnt: 2}, nil)
			manager.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().
				CreateOrUpdate(gomock.Any(), spaceID, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
				PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
				PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().
				EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			manager.mutex.(*lockMocks.MockILocker).EXPECT().
				UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(true, nil).AnyTimes()

			items.EXPECT().ScanIncompleteDispatchObservations(gomock.Any(), spaceID, exptID, runID).DoAndReturn(
				func(context.Context, int64, int64, int64) ([]*repo.ExptDispatchObservation, error) {
					if tc.scanErr != nil {
						return nil, tc.scanErr
					}
					return []*repo.ExptDispatchObservation{
						{ItemID: 11, Status: int32(entity.ItemRunState_Processing), RequestID: "r11"},
						{ItemID: 12, Status: int32(entity.ItemRunState_Processing), RequestID: "r12"},
					}, nil
				})
			items.EXPECT().ScanItemRunLogs(gomock.Any(), exptID, runID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).
				Return([]*entity.ExptItemResultRunLog{{ItemID: 11}, {ItemID: 12}}, int64(2), nil).AnyTimes()
			items.EXPECT().UpdateItemRunLog(gomock.Any(), exptID, runID, gomock.Any(), gomock.Any(), spaceID).Return(nil).AnyTimes()

			err := manager.CompleteExpt(ctx, exptID, gptr.Of(runID), spaceID, session)

			require.Error(t, err, "释放失败的完成不能被当作成功 ACK")
			require.True(t, errno.NeedMQRetry(err), "完成必须保持可重放，用原 RequestID 再释放一次")
			require.False(t, finalized, "reservation 还没归还时不得把实验写成终态")

			releases := guard.releases()
			if tc.scanErr != nil {
				require.Empty(t, releases, "扫描失败时不得构造释放请求")
			} else {
				require.Len(t, releases, 2)
				assert.Equal(t, []int64{11, 12}, []int64{releases[0].ItemID, releases[1].ItemID})
				assert.Equal(t, []string{"r11", "r12"}, []string{releases[0].RequestID, releases[1].RequestID})
				assert.Equal(t, testScope, releases[0].Scope)
				assert.Equal(t, runID, releases[0].RunID)
			}
		})
	}
}

// 正常完成（没有未完成 item）必须保持原语义：不产生任何账本调用，也不因释放逻辑失败。
func TestCompleteExpt_NoIncompleteItemsKeepsNormalSuccess(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	manager.runFinishRepo = &originalCompletionRunFinishRepo{}
	guard := &fakeGuard{releaseErr: assert.AnError}
	manager.centralGuard = guard
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	manager.itemResultRepo = items

	ctx := context.Background()
	session := &entity.Session{UserID: "operator"}
	const exptID, spaceID, runID = int64(1), int64(3), int64(2)
	expt := &entity.Experiment{
		ID: exptID, SpaceID: spaceID, LatestRunID: runID, Status: entity.ExptStatus_Processing,
		ExptType: entity.ExptType_Offline, StartAt: gptr.Of(time.Now()),
		SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce,
	}

	manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), exptID, spaceID).Return(expt, nil).AnyTimes()
	manager.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	manager.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), exptID, runID).
		Return(&entity.ExptRunLog{ExptID: exptID, ExptRunID: runID, Status: int64(entity.ExptStatus_Processing)}, nil).AnyTimes()
	manager.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), exptID, runID, spaceID, gomock.Any()).Return(nil).AnyTimes()
	manager.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any(), false).
		Return([]*entity.ExptTurnResult{{Status: int32(entity.TurnRunState_Success)}}, int64(1), nil).AnyTimes()
	manager.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		CalculateStats(gomock.Any(), exptID, spaceID, session).Return(&entity.ExptCalculateStats{SuccessItemCnt: 1}, nil)
	manager.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		GetIncompleteTurns(gomock.Any(), exptID, spaceID, session).Return(nil, nil).AnyTimes()
	manager.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		UpdateByExptID(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil)
	manager.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{SuccessItemCnt: 1}, nil)
	manager.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().
		CreateOrUpdate(gomock.Any(), spaceID, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
		PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
		PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().
		EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	manager.mutex.(*lockMocks.MockILocker).EXPECT().
		UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(true, nil).AnyTimes()

	items.EXPECT().ScanIncompleteDispatchObservations(gomock.Any(), spaceID, exptID, runID).Return(nil, nil)
	items.EXPECT().ScanItemRunLogs(gomock.Any(), exptID, runID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).
		Return(nil, int64(0), nil).AnyTimes()

	require.NoError(t, manager.CompleteExpt(ctx, exptID, gptr.Of(runID), spaceID, session))
	assert.Empty(t, guard.releases(), "全部 item 已完成时不该产生账本调用")
}
