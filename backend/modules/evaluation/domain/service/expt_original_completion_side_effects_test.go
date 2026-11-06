// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
)

type originalCompletionSideEffects struct {
	manager          *ExptMangerImpl
	expt             *entity.Experiment
	run              *entity.ExptRunLog
	stats            *entity.ExptCalculateStats
	idemExists       bool
	aggregateErr     error
	lifecycleErr     error
	onCalculateStats func()
	onExptUpdate     func()
	onLifecycle      func()
	exptUpdates      int
	statsUpdates     int
	quotaReleases    int
	aggregateEvents  int
	lifecycleEvents  int
	runUpdates       int
	unlocks          int
	updatedExpt      *entity.Experiment
}

func newOriginalCompletionSideEffects(t *testing.T) *originalCompletionSideEffects {
	t.Helper()
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	manager.runFinishRepo = &originalCompletionRunFinishRepo{}
	f := &originalCompletionSideEffects{
		manager: manager,
		expt:    &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline},
		run:     &entity.ExptRunLog{ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)},
		stats:   &entity.ExptCalculateStats{SuccessItemCnt: 1},
	}
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
		current := *f.expt
		return &current, nil
	}).AnyTimes()
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, updated *entity.Experiment) error {
		f.exptUpdates++
		f.updatedExpt = updated
		f.expt.Status = updated.Status
		if updated.EndAt != nil {
			f.expt.EndAt = updated.EndAt
		}
		if f.onExptUpdate != nil {
			f.onExptUpdate()
		}
		return nil
	}).AnyTimes()
	manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, string) (bool, error) {
		return f.idemExists, nil
	}).AnyTimes()
	manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	manager.exptResultService.(*svcmocks.MockExptResultService).EXPECT().CalculateStats(gomock.Any(), int64(1), int64(3), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.Session) (*entity.ExptCalculateStats, error) {
		if f.onCalculateStats != nil {
			f.onCalculateStats()
		}
		return f.stats, nil
	}).AnyTimes()
	manager.exptResultService.(*svcmocks.MockExptResultService).EXPECT().GetIncompleteTurns(gomock.Any(), int64(1), int64(3), gomock.Any()).Return(nil, nil).AnyTimes()
	manager.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(gomock.Any(), int64(1), int64(3), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.ExptStats) error {
		f.statsUpdates++
		return nil
	}).AnyTimes()
	manager.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().Get(gomock.Any(), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64) (*entity.ExptStats, error) {
		return &entity.ExptStats{SuccessItemCnt: int32(f.stats.SuccessItemCnt), PendingItemCnt: int32(f.stats.PendingItemCnt), FailItemCnt: int32(f.stats.FailItemCnt), ProcessingItemCnt: int32(f.stats.ProcessingItemCnt), TerminatedItemCnt: int32(f.stats.TerminatedItemCnt)}, nil
	}).AnyTimes()
	manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), *entity.Session) error {
		f.quotaReleases++
		return nil
	}).AnyTimes()
	manager.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.AggrCalculateEvent, *time.Duration) error {
		f.aggregateEvents++
		return f.aggregateErr
	}).AnyTimes()
	manager.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
		f.lifecycleEvents++
		if f.onLifecycle != nil {
			f.onLifecycle()
		}
		return f.lifecycleErr
	}).AnyTimes()
	manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(1), int64(2)).DoAndReturn(func(context.Context, int64, int64) (*entity.ExptRunLog, error) {
		current := *f.run
		return &current, nil
	}).AnyTimes()
	manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _ int64, fields map[string]any) error {
		f.runUpdates++
		f.run.Status = fields["status"].(int64)
		return nil
	}).AnyTimes()
	manager.mutex.(*lockmocks.MockILocker).EXPECT().UnlockWithValue(gomock.Any(), "expt_run_mutex_lock:1", "2").DoAndReturn(func(context.Context, string, string) (bool, error) {
		f.unlocks++
		return true, nil
	}).AnyTimes()
	manager.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	return f
}

func TestCompleteExptStopsWhenReplacementRunAppears(t *testing.T) {
	for _, tc := range []struct {
		name             string
		changePoint      string
		wantStatsUpdates int
		wantExptUpdates  int
	}{
		{name: "calculate_stats", changePoint: "stats"},
		{name: "experiment_update", changePoint: "expt", wantStatsUpdates: 1, wantExptUpdates: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOriginalCompletionSideEffects(t)
			if tc.changePoint == "stats" {
				f.onCalculateStats = func() { f.expt.LatestRunID = 3 }
			} else {
				f.onExptUpdate = func() { f.expt.LatestRunID = 3 }
			}
			runID := int64(2)
			require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
			require.Equal(t, tc.wantStatsUpdates, f.statsUpdates)
			require.Equal(t, tc.wantExptUpdates, f.exptUpdates)
			require.Zero(t, f.quotaReleases, "stale completion must not release the replacement run quota")
			require.Zero(t, f.aggregateEvents)
			require.Zero(t, f.lifecycleEvents)
			require.Zero(t, f.runUpdates)
			require.Zero(t, f.unlocks)
		})
	}
}

func TestCompleteExptRetriesPublicationFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		aggregate bool
	}{
		{name: "aggregate", aggregate: true},
		{name: "lifecycle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOriginalCompletionSideEffects(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.aggregate {
				f.aggregateErr = errors.New("aggregate unavailable")
			} else {
				f.lifecycleErr = errors.New("lifecycle unavailable")
				f.onLifecycle = cancel
			}
			runID := int64(2)
			err := f.manager.CompleteExpt(ctx, 1, &runID, 3, &entity.Session{})
			require.True(t, errno.NeedMQRetry(err), "publication failure must remain retryable, got %v", err)
			require.Zero(t, f.runUpdates, "run must remain Processing until every completion event is published")
			require.Zero(t, f.unlocks)
		})
	}
}

func TestCompleteExptDuplicateReleasesOnlyCurrentRunHolder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy bool
	}{
		{name: "numeric"},
		{name: "legacy_xid", legacy: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOriginalCompletionSideEffects(t)
			f.idemExists = true
			now := time.Now()
			f.expt.Status = entity.ExptStatus_Success
			f.expt.EndAt = &now
			f.run.Status = int64(entity.ExptStatus_Success)
			factory, server := finishRealLockerFactory(t)
			f.manager.mutex = factory()
			key := "expt_run_mutex_lock:1"
			holder := "2"
			if tc.legacy {
				holder = legacyHolderAt(t, time.Now().Add(-10*time.Minute))
			}
			require.NoError(t, server.Set(key, holder))
			if tc.legacy {
				server.SetTTL(key, time.Hour)
			}
			runID := int64(2)
			require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}, entity.WithCID("exptexec:onend:2")))
			_, err := server.Get(key)
			require.Error(t, err, "duplicate terminal completion must release its own lifecycle holder")
			require.Zero(t, f.statsUpdates)
			require.Zero(t, f.exptUpdates)
			require.Zero(t, f.quotaReleases)
			require.Zero(t, f.aggregateEvents)
			require.Zero(t, f.lifecycleEvents)
		})
	}
}

func TestCompleteExptFailedOutcomeReleasesResources(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.stats = &entity.ExptCalculateStats{FailItemCnt: 1}
	f.run.FailCnt = 1
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Equal(t, entity.ExptStatus_Failed, f.expt.Status)
	require.Equal(t, int64(entity.ExptStatus_Failed), f.run.Status)
	require.Equal(t, 1, f.quotaReleases)
	require.Equal(t, 1, f.aggregateEvents)
	require.Equal(t, 1, f.lifecycleEvents)
	require.Equal(t, 1, f.unlocks)
}

func TestCompleteExptResumePreservesCommittedEndAt(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.idemExists = true
	committed := time.Now().Add(-time.Hour).Truncate(time.Second)
	f.expt.Status = entity.ExptStatus_Success
	f.expt.EndAt = &committed
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}, entity.WithCID("exptexec:onend:2")))
	require.NotNil(t, f.updatedExpt)
	require.NotNil(t, f.updatedExpt.EndAt)
	require.Equal(t, committed, *f.updatedExpt.EndAt)
}
