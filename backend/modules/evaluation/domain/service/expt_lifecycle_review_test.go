package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reviewFailingPreparer struct {
	events.ExptEventPublisher
	prepared bool
}

func (p *reviewFailingPreparer) BeforeExptSchedule(context.Context, *entity.Experiment, *entity.Session) error {
	p.prepared = true
	return errors.New("injected workflow preparation failure")
}

func TestReviewCancelledPrepareCannotReadmitQuota(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	factory, server := finishRealLockerFactory(t)
	m.manualRetryLocker, m.mutex = factory, factory()
	require.NoError(t, server.Set(m.makeExptMutexLockKey(2), "3"))
	m.mtr = nil
	preparer := &reviewFailingPreparer{}
	m.publisher = preparer
	current := &entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, ExptType: entity.ExptType_Offline, Status: entity.ExptStatus_Pending}
	run := &entity.ExptRunLog{SpaceID: 1, ExptID: 2, ExptRunID: 3, Status: int64(entity.ExptStatus_Pending)}
	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(current, nil).AnyTimes()
	m.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(2), int64(3)).Return(run, nil).AnyTimes()
	m.itemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().ScanItemRunLogs(gomock.Any(), int64(2), int64(3), gomock.Any(), gomock.Any(), gomock.Any(), int64(1)).Return(nil, int64(0), nil).AnyTimes()
	port := repomocks.NewMockIExptRunTerminationRepo(ctrl)
	m.runFinishRepo = &pendingTerminationRepo{MockIExptRunTerminationRepo: port}
	port.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
			require.True(t, r.Finalize)
			current.Status, run.Status = entity.ExptStatus_Terminated, int64(entity.ExptStatus_Terminated)
			return &entity.ExptRunTerminationResult{Applied: true, Experiment: current}, nil
		})
	m.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	var quota *entity.QuotaSpaceExpt
	m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			var err error
			quota, _, err = update(quota)
			return err
		}).AnyTimes()
	m.lwt.(*lwtmocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false)
	beforeCancel := *current
	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().MGetByID(gomock.Any(), []int64{2}, int64(1)).Return([]*entity.Experiment{&beforeCancel}, nil)
	m.evaluationSetService.(*svcmocks.MockIEvaluationSetService).EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil)
	m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), int64(1), []int64{2}).Return(nil, nil)
	m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), []int64{2}, int64(1), gomock.Any()).DoAndReturn(func(ctx context.Context, _ []int64, _ int64, session *entity.Session) ([]*entity.ExptStats, error) {
		_, err := m.TerminateCurrentRun(ctx, 2, 3, 1, session, nil, entity.NoAggrCalculate())
		require.NoError(t, err)
		return nil, nil
	})
	require.Error(t, m.Run(context.Background(), 2, 3, 1, 0, &entity.Session{}, entity.EvaluationModeSubmit, nil))
	require.False(t, preparer.prepared)
	if quota != nil {
		require.Empty(t, quota.ExptID2RunTime)
	}
}

func TestReviewTerminalSetupFailureReleasesOnlyCurrentRunQuota(t *testing.T) {
	for _, tc := range []struct{ replaced, cancelled bool }{{false, false}, {false, true}, {true, false}, {true, true}} {
		t.Run(fmt.Sprintf("replaced_%t_cancelled_%t", tc.replaced, tc.cancelled), func(t *testing.T) {
			replaced := tc.replaced
			ctrl := gomock.NewController(t)
			m := newTestExptManager(ctrl)
			factory, server := finishRealLockerFactory(t)
			m.manualRetryLocker, m.mutex = factory, factory()
			require.NoError(t, server.Set(m.makeExptMutexLockKey(2), "3"))
			port := repomocks.NewMockIExptRunTerminationRepo(ctrl)
			m.runFinishRepo = &pendingTerminationRepo{MockIExptRunTerminationRepo: port}
			current := &entity.Experiment{ID: 2, SpaceID: 1, LatestRunID: 3, Status: entity.ExptStatus_Terminated, ExptType: entity.ExptType_Offline}
			if replaced {
				current.LatestRunID = 9
				require.NoError(t, server.Set(m.makeExptMutexLockKey(2), "9"))
			}
			m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(2), int64(1)).Return(current, nil).AnyTimes()
			m.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(2), int64(3)).
				Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Terminated)}, nil).AnyTimes()
			quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{2: time.Now().Unix()}}
			if !replaced {
				m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(1), gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
						var err error
						quota, _, err = update(quota)
						return err
					})
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			require.NoError(t, m.cleanupCurrentUnscheduledRun(ctx, 2, 3, 1, errors.New("setup failed")))
			require.Equal(t, replaced, len(quota.ExptID2RunTime) > 0)
			if replaced {
				holder, err := server.Get(m.makeExptMutexLockKey(2))
				require.NoError(t, err)
				require.Equal(t, "9", holder)
			}
		})
	}
}
