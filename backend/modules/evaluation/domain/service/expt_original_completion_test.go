// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
)

type originalCompletionRunFinishRepo struct {
	repo.IExptRunFinishRepo
}

func (*originalCompletionRunFinishRepo) FinishCurrentRun(context.Context, *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	return nil, nil
}

func TestExptEndUsesOriginalCompletionWithCurrentRepository(t *testing.T) {
	for _, tc := range []struct {
		name         string
		mode         entity.ExptRunMode
		exptStatus   entity.ExptStatus
		completeSeen bool
		runReads     int
		turnStatus   entity.TurnRunState
		runStatus    entity.ExptStatus
	}{
		{name: "submit", mode: entity.EvaluationModeSubmit, exptStatus: entity.ExptStatus_Processing, runReads: 2, turnStatus: entity.TurnRunState_Success, runStatus: entity.ExptStatus_Success},
		{name: "trial_run", mode: entity.EvaluationModeTrialRun, exptStatus: entity.ExptStatus_Processing, runReads: 2, turnStatus: entity.TurnRunState_Success, runStatus: entity.ExptStatus_Success},
		{name: "resume_before_terminal", mode: entity.EvaluationModeSubmit, exptStatus: entity.ExptStatus_Success, completeSeen: true, runReads: 3, turnStatus: entity.TurnRunState_Success, runStatus: entity.ExptStatus_Success},
		{name: "run_failure_preserved", mode: entity.EvaluationModeSubmit, exptStatus: entity.ExptStatus_Processing, runReads: 2, turnStatus: entity.TurnRunState_Fail, runStatus: entity.ExptStatus_Failed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := newTestExptManager(ctrl)
			manager.runFinishRepo = &originalCompletionRunFinishRepo{}

			tickLocker := lockmocks.NewMockILocker(ctrl)
			tickLockCalls := 0
			tickLocker.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:1:2", 5*time.Second, 5*time.Minute).DoAndReturn(func(ctx context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
				tickLockCalls++
				return false, ctx, func() {}, nil
			}).AnyTimes()
			manager.manualRetryLocker = func() lock.ILocker { return tickLocker }

			ctx := context.Background()
			session := &entity.Session{UserID: "operator"}
			expt := &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: tc.exptStatus, ExptType: entity.ExptType_Offline, StartAt: gptr.Of(time.Now())}

			manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string) (bool, error) {
				return tc.completeSeen && key == "CompleteExpt:exptexec:onend:2", nil
			}).AnyTimes()
			manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(expt, nil).AnyTimes()
			manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
			var persistedFields map[string]any
			manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(1), int64(2)).DoAndReturn(func(context.Context, int64, int64) (*entity.ExptRunLog, error) {
				run := &entity.ExptRunLog{ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)}
				if persistedFields != nil {
					run.FailCnt = persistedFields["fail_cnt"].(int32)
					run.PendingCnt = persistedFields["pending_cnt"].(int32)
				}
				return run, nil
			}).Times(tc.runReads)
			runStatsUpdated := manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _ int64, fields map[string]any) error {
				require.Equal(t, int64(entity.ExptStatus_Processing), fields["status"], "run must stay retryable until all completion side effects finish")
				persistedFields = fields
				return nil
			})
			runTerminalUpdated := manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _ int64, fields map[string]any) error {
				require.Equal(t, int64(tc.runStatus), fields["status"])
				persistedFields = fields
				return nil
			})
			manager.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().ListTurnResult(gomock.Any(), int64(3), int64(1), gomock.Any(), gomock.Any(), false).Return([]*entity.ExptTurnResult{{Status: int32(tc.turnStatus)}}, int64(1), nil)
			unlock := manager.mutex.(*lockmocks.MockILocker).EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").Return(true, nil)
			manager.exptResultService.(*svcmocks.MockExptResultService).EXPECT().CalculateStats(gomock.Any(), int64(1), int64(3), session).Return(&entity.ExptCalculateStats{SuccessItemCnt: 1}, nil)
			manager.exptResultService.(*svcmocks.MockExptResultService).EXPECT().GetIncompleteTurns(gomock.Any(), int64(1), int64(3), session).Return(nil, nil)
			manager.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(nil)
			manager.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().Get(gomock.Any(), int64(1), int64(3)).Return(&entity.ExptStats{SuccessItemCnt: 1}, nil)
			quotaReleased := manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), session).Return(nil)
			aggregatePublished := manager.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			lifecyclePublished := manager.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptLifecycleEvent, _ *time.Duration, _ string) error {
				require.Equal(t, entity.ExptStatus_Processing, event.FromStatus, "completion retries must keep a stable lifecycle idempotency key")
				return nil
			})
			gomock.InOrder(runStatsUpdated, quotaReleased, aggregatePublished, lifecyclePublished, runTerminalUpdated, unlock)
			manager.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecResult(int64(3), int64(entity.ExptType_Offline), int64(entity.ExptStatus_Success), gomock.Any())

			result := &finishCalibrationTestResult{ExptResultService: manager.exptResultService, ready: true}
			manager.exptResultService = result
			base := &exptBaseExec{Manager: manager, idem: manager.idem, configer: manager.configer, exptItemResultRepo: manager.itemResultRepo, resultSvc: result}
			err := base.exptEnd(ctx, &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptType: entity.ExptType_Offline, ExptRunMode: tc.mode, Session: session}, expt)
			require.NoError(t, err)
			require.Zero(t, tickLockCalls, "completion must reuse the scheduler's existing tick lease")
		})
	}
}

func TestCompleteExptDoesNotOverwriteTerminatedExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	manager.runFinishRepo = &originalCompletionRunFinishRepo{}
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Terminated, ExptType: entity.ExptType_Offline}, nil)
	runID := int64(2)
	require.NoError(t, manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
}

func TestFinishedRunAndExperimentACKWithoutTickLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := svcmocks.NewMockIExptManager(ctrl)
	manager.EXPECT().GetRunLog(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil)
	scheduler := &ExptSchedulerImpl{Manager: manager}
	err := scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("finished run reentered execution")
		return nil
	})(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit})
	require.NoError(t, err)
}
