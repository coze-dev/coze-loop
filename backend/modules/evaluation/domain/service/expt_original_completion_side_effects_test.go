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
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
)

type originalCompletionSideEffects struct {
	manager           *ExptMangerImpl
	expt              *entity.Experiment
	run               *entity.ExptRunLog
	stats             *entity.ExptCalculateStats
	turnStatus        entity.TurnRunState
	idemExists        bool
	notificationSeen  bool
	notificationMarks int
	notificationTTL   time.Duration
	aggregateErr      error
	lifecycleErr      error
	templateErr       error
	calculateErr      error
	incompleteErr     error
	quotaErr          error
	statsUpdateErr    error
	statsGetOverride  *entity.ExptStats
	notificationErr   error
	idemSetErr        error
	runGetErr         error
	runGetErrAt       int
	runGetNilAt       int
	runUpdateErr      error
	runGets           int
	onCalculateStats  func()
	onExptUpdate      func()
	onLifecycle       func()
	onAggregate       func()
	onQuota           func()
	getByIDCalls      int
	getByIDErrAt      int
	getByIDErr        error
	exptUpdates       int
	statsUpdates      int
	quotaReleases     int
	aggregateEvents   int
	lifecycleEvents   int
	runUpdates        []map[string]any
	unlocks           int
	updatedExpt       *entity.Experiment
}

type originalCompletionRunLogRepo struct {
	*repomocks.MockIExptRunLogRepo
}

func (r *originalCompletionRunLogRepo) UpdateStatusIfCurrent(ctx context.Context, exptID, exptRunID, spaceID, _, status int64) (bool, error) {
	return true, r.Update(ctx, exptID, exptRunID, spaceID, map[string]any{"status": status})
}

func newOriginalCompletionSideEffects(t *testing.T) *originalCompletionSideEffects {
	t.Helper()
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	manager.runLogRepo = &originalCompletionRunLogRepo{MockIExptRunLogRepo: manager.runLogRepo.(*repomocks.MockIExptRunLogRepo)}
	manager.runFinishRepo = &originalCompletionRunFinishRepo{}
	f := &originalCompletionSideEffects{
		manager:    manager,
		expt:       &entity.Experiment{ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline},
		run:        &entity.ExptRunLog{ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)},
		stats:      &entity.ExptCalculateStats{SuccessItemCnt: 1},
		turnStatus: entity.TurnRunState_Success,
	}
	manager.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64) (*entity.Experiment, error) {
		f.getByIDCalls++
		if f.getByIDErrAt > 0 && f.getByIDCalls == f.getByIDErrAt {
			return nil, f.getByIDErr
		}
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
	manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string) (bool, error) {
		if key == "finish-cleanup:1:2" {
			return f.notificationSeen, f.notificationErr
		}
		return f.idemExists, nil
	}).AnyTimes()
	manager.idem.(*idemmocks.MockIdempotentService).EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, key string, ttl time.Duration) error {
		if f.idemSetErr != nil {
			return f.idemSetErr
		}
		if key == "finish-cleanup:1:2" {
			f.notificationMarks++
			f.notificationTTL = ttl
			f.notificationSeen = true
		}
		return nil
	}).AnyTimes()
	manager.exptResultService.(*svcmocks.MockExptResultService).EXPECT().CalculateStats(gomock.Any(), int64(1), int64(3), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.Session) (*entity.ExptCalculateStats, error) {
		if f.onCalculateStats != nil {
			f.onCalculateStats()
		}
		if f.calculateErr != nil {
			return nil, f.calculateErr
		}
		return f.stats, nil
	}).AnyTimes()
	manager.exptResultService.(*svcmocks.MockExptResultService).EXPECT().GetIncompleteTurns(gomock.Any(), int64(1), int64(3), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.Session) ([]*entity.ItemTurnID, error) {
		return nil, f.incompleteErr
	}).AnyTimes()
	manager.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().ListTurnResult(gomock.Any(), int64(3), int64(1), gomock.Any(), gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, *entity.ExptTurnResultFilter, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
		return []*entity.ExptTurnResult{{Status: int32(f.turnStatus)}}, 1, nil
	}).AnyTimes()
	manager.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(gomock.Any(), int64(1), int64(3), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, *entity.ExptStats) error {
		f.statsUpdates++
		return f.statsUpdateErr
	}).AnyTimes()
	manager.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().Get(gomock.Any(), int64(1), int64(3)).DoAndReturn(func(context.Context, int64, int64) (*entity.ExptStats, error) {
		if f.statsGetOverride != nil {
			return f.statsGetOverride, nil
		}
		return &entity.ExptStats{SuccessItemCnt: int32(f.stats.SuccessItemCnt), PendingItemCnt: int32(f.stats.PendingItemCnt), FailItemCnt: int32(f.stats.FailItemCnt), ProcessingItemCnt: int32(f.stats.ProcessingItemCnt), TerminatedItemCnt: int32(f.stats.TerminatedItemCnt)}, nil
	}).AnyTimes()
	manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		if f.onQuota != nil {
			f.onQuota()
		}
		if f.quotaErr != nil {
			return f.quotaErr
		}
		next, changed, err := update(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{1: 1}})
		if err != nil {
			return err
		}
		if changed {
			if _, stillPresent := next.ExptID2RunTime[1]; !stillPresent {
				f.quotaReleases++
			}
		}
		return nil
	}).AnyTimes()
	manager.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.AggrCalculateEvent, *time.Duration) error {
		f.aggregateEvents++
		if f.onAggregate != nil {
			f.onAggregate()
		}
		return f.aggregateErr
	}).AnyTimes()
	manager.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
		f.lifecycleEvents++
		if f.onLifecycle != nil {
			f.onLifecycle()
		}
		return f.lifecycleErr
	}).AnyTimes()
	manager.templateManager.(*svcmocks.MockIExptTemplateManager).EXPECT().UpdateExptInfo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, entity.ExptStatus, int64, *int64) error {
		return f.templateErr
	}).AnyTimes()
	manager.runLogRepo.(*originalCompletionRunLogRepo).MockIExptRunLogRepo.EXPECT().Get(gomock.Any(), int64(1), int64(2)).DoAndReturn(func(context.Context, int64, int64) (*entity.ExptRunLog, error) {
		f.runGets++
		if f.runGetErrAt > 0 && f.runGets == f.runGetErrAt {
			return nil, f.runGetErr
		}
		if f.runGetNilAt > 0 && f.runGets == f.runGetNilAt {
			return nil, nil
		}
		current := *f.run
		return &current, nil
	}).AnyTimes()
	manager.runLogRepo.(*originalCompletionRunLogRepo).MockIExptRunLogRepo.EXPECT().Update(gomock.Any(), int64(1), int64(2), int64(3), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _ int64, fields map[string]any) error {
		if f.runUpdateErr != nil {
			return f.runUpdateErr
		}
		f.runUpdates = append(f.runUpdates, fields)
		f.run.Status = fields["status"].(int64)
		for field, target := range map[string]*int32{"pending_cnt": &f.run.PendingCnt, "success_cnt": &f.run.SuccessCnt, "fail_cnt": &f.run.FailCnt, "processing_cnt": &f.run.ProcessingCnt, "terminated_cnt": &f.run.TerminatedCnt} {
			if value, ok := fields[field]; ok {
				*target = value.(int32)
			}
		}
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
			require.Len(t, f.runUpdates, 1)
			require.Equal(t, map[string]any{"status": int64(entity.ExptStatus_Success)}, f.runUpdates[0], "R1 may only write its own terminal status")
			require.Zero(t, f.unlocks)
		})
	}
}

func TestCompleteExptRetriesSecondExperimentReadFailure(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.getByIDErrAt = 2
	f.getByIDErr = errors.New("temporary mysql failure")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err), "a transient second read failure must not fail the experiment, got %v", err)
	require.Zero(t, f.exptUpdates)
	require.Zero(t, f.runUpdates)
}

func TestCompleteExptSkipsQuotaWhenReplacementAppearsInsideRelease(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.onQuota = func() { f.expt.LatestRunID = 3 }
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Zero(t, f.quotaReleases, "the quota update itself must reject a stale run")
}

func TestCompleteExptRecalculatesRunCountsBeforeTerminal(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.run.PendingCnt = 1
	f.turnStatus = entity.TurnRunState_Success
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Equal(t, int64(entity.ExptStatus_Success), f.run.Status)
	require.Zero(t, f.run.PendingCnt)
	require.Equal(t, int32(1), f.run.SuccessCnt)
}

func TestCompleteExptRepairsStatsDrift(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Equal(t, 1, f.statsUpdates)
	require.Equal(t, int64(entity.ExptStatus_Success), f.run.Status)
}

func TestCompleteExptProcessingRunUsesNewEndAt(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	previousRunEnd := time.Now().Add(-time.Hour)
	f.expt.EndAt = &previousRunEnd
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.NotNil(t, f.updatedExpt.EndAt)
	require.NotEqual(t, previousRunEnd, *f.updatedExpt.EndAt)
}

func TestCompleteExptResumeSkipsAlreadyPublishedNotifications(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.idemExists = true
	f.notificationSeen = true
	f.expt.Status = entity.ExptStatus_Failed
	f.stats = &entity.ExptCalculateStats{FailItemCnt: 1}
	f.turnStatus = entity.TurnRunState_Fail
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Equal(t, 1, f.quotaReleases, "resource cleanup remains replayable")
	require.Zero(t, f.aggregateEvents)
	require.Zero(t, f.lifecycleEvents)
	require.Zero(t, f.notificationMarks)
	require.Equal(t, int64(entity.ExptStatus_Failed), f.run.Status)
	require.Equal(t, 1, f.unlocks)
}

func TestCompleteExptStopsLifecycleWhenReplacementAppearsAfterAggregate(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.onAggregate = func() { f.expt.LatestRunID = 3 }
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Zero(t, f.lifecycleEvents)
	require.Len(t, f.runUpdates, 1)
	require.Equal(t, map[string]any{"status": int64(entity.ExptStatus_Success)}, f.runUpdates[0], "R1 may only write its own terminal status")
	require.Zero(t, f.unlocks)
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

func TestCompleteExptRetriesTemplateUpdateFailure(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.expt.ExptTemplateMeta = &entity.ExptTemplateMeta{ID: 4}
	f.templateErr = errors.New("template unavailable")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err), "template update failure must remain retryable, got %v", err)
	require.Zero(t, f.runUpdates)
	require.Zero(t, f.unlocks)
}

func TestCompleteExptDuplicateReleasesOnlyCurrentRunHolder(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy bool
		fresh  bool
	}{
		{name: "numeric"},
		{name: "legacy_xid", legacy: true},
		{name: "fresh_legacy_xid", legacy: true, fresh: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOriginalCompletionSideEffects(t)
			f.idemExists = true
			f.notificationSeen = true
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
				if tc.fresh {
					holder = legacyHolderAt(t, time.Now())
				}
			}
			require.NoError(t, server.Set(key, holder))
			if tc.legacy {
				server.SetTTL(key, time.Hour)
			}
			runID := int64(2)
			err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}, entity.WithCID("exptexec:onend:2"))
			if tc.fresh {
				require.True(t, errno.NeedMQRetry(err), "a fresh legacy holder must retain a retry until migration is safe")
				owner, getErr := server.Get(key)
				require.NoError(t, getErr)
				require.Equal(t, holder, owner)
				return
			}
			require.NoError(t, err)
			_, err = server.Get(key)
			require.Error(t, err, "duplicate terminal completion must release its own lifecycle holder")
			require.Zero(t, f.statsUpdates)
			require.Zero(t, f.exptUpdates)
			require.Zero(t, f.quotaReleases)
			require.Zero(t, f.aggregateEvents)
			require.Zero(t, f.lifecycleEvents)
		})
	}
}

func TestCompleteExptRunTerminalBeforeExperimentRemainsRetryable(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.run.Status = int64(entity.ExptStatus_Success)
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err), "a run committed before its experiment must not be acknowledged, got %v", err)
	require.Zero(t, f.unlocks)
	require.Zero(t, f.quotaReleases)
	require.Zero(t, f.aggregateEvents)
	require.Zero(t, f.lifecycleEvents)
}

func TestCompleteExptDuplicateWithoutCleanupMarkerDoesNotRelease(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.expt.Status = entity.ExptStatus_Success
	f.run.Status = int64(entity.ExptStatus_Success)
	factory, server := finishRealLockerFactory(t)
	f.manager.mutex = factory()
	key := "expt_run_mutex_lock:1"
	require.NoError(t, server.Set(key, "2"))
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err), "unproven cleanup must not be acknowledged, got %v", err)
	owner, getErr := server.Get(key)
	require.NoError(t, getErr)
	require.Equal(t, "2", owner)
	require.Zero(t, f.quotaReleases)
	require.Zero(t, f.aggregateEvents)
	require.Zero(t, f.lifecycleEvents)
}

func TestCompleteExptFailedOutcomeReleasesResources(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	config := confmocks.NewMockIConfiger(gomock.NewController(t))
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 60}).AnyTimes()
	f.manager.configer = config
	f.stats = &entity.ExptCalculateStats{FailItemCnt: 1}
	f.turnStatus = entity.TurnRunState_Fail
	f.run.FailCnt = 1
	runID := int64(2)
	require.NoError(t, f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}))
	require.Equal(t, entity.ExptStatus_Failed, f.expt.Status)
	require.Equal(t, int64(entity.ExptStatus_Failed), f.run.Status)
	require.Equal(t, 1, f.quotaReleases)
	require.Equal(t, 1, f.aggregateEvents)
	require.Equal(t, 1, f.lifecycleEvents)
	require.Equal(t, 1, f.notificationMarks)
	require.Equal(t, 2*time.Minute, f.notificationTTL)
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

func TestCompleteExptOfflineRequiresRunIdentity(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	err := f.manager.CompleteExpt(context.Background(), 1, nil, 3, &entity.Session{}, entity.WithStatus(entity.ExptStatus_Failed))
	require.True(t, errno.NeedMQRetry(err), "offline terminal completion without a run id must retry, got %v", err)
}

func TestCompletionIdemDurationFallsBackWithoutConfig(t *testing.T) {
	m := &ExptMangerImpl{}
	require.Equal(t, 3*time.Minute, m.completionIdemDuration(context.Background(), 1))
}

func TestReleaseCompletedRunLifecycleHolderRequiresLocker(t *testing.T) {
	m := &ExptMangerImpl{}
	err := m.releaseCompletedRunLifecycleHolder(context.Background(), 1, 2, 3, nil)
	require.True(t, errno.NeedMQRetry(err))
}

func TestReleaseCurrentRunQuotaSkipsRunWithoutEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
	quota := repomocks.NewMockQuotaRepo(ctrl)
	m := &ExptMangerImpl{exptRepo: exptRepo, quotaRepo: quota}

	exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, LatestRunID: 2}, nil)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			_, changed, err := update(&entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{99: 1}})
			require.NoError(t, err)
			require.False(t, changed, "a run without a quota entry must not change the ledger")
			return nil
		})
	require.NoError(t, m.releaseCurrentRunQuota(context.Background(), 1, 2, 3, nil))
}

func TestReleaseCurrentRunQuotaPropagatesReadFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
	quota := repomocks.NewMockQuotaRepo(ctrl)
	m := &ExptMangerImpl{exptRepo: exptRepo, quotaRepo: quota}

	exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("primary read failed"))
	quota.EXPECT().CreateOrUpdate(gomock.Any(), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			_, _, err := update(&entity.QuotaSpaceExpt{})
			return err
		})
	require.Error(t, m.releaseCurrentRunQuota(context.Background(), 1, 2, 3, nil))
}

func TestCompleteExptDuplicateCIDSkipsWhenNoRunLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
	idm := idemmocks.NewMockIdempotentService(ctrl)
	m := &ExptMangerImpl{runFinishRepo: &originalCompletionRunFinishRepo{}, exptRepo: exptRepo, idem: idm}

	exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, ExptType: entity.ExptType_Online}, nil)
	idm.EXPECT().Exist(gomock.Any(), "CompleteExpt:dup").Return(true, nil)
	runID := int64(2)
	require.NoError(t, m.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}, entity.WithCID("dup")))
}

func TestCompleteExptRunLockRequiresIdem(t *testing.T) {
	ctrl := gomock.NewController(t)
	exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
	runLogRepo := repomocks.NewMockIExptRunLogRepo(ctrl)
	m := &ExptMangerImpl{runFinishRepo: &originalCompletionRunFinishRepo{}, exptRepo: exptRepo, runLogRepo: runLogRepo}

	exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{
		ID: 1, SpaceID: 3, ExptType: entity.ExptType_Offline, LatestRunID: 2, Status: entity.ExptStatus_Processing,
	}, nil)
	runLogRepo.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil)
	runID := int64(2)
	err := m.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptNotificationLookupFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.notificationErr = errors.New("redis down")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptIdemSetFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.idemSetErr = errors.New("redis down")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{}, entity.WithCID("c1"))
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptCalculateStatsFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.calculateErr = errors.New("turn scan failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptLegacyStatsWriteFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.statsUpdateErr = errors.New("stats write failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptStatsVerificationFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.statsGetOverride = &entity.ExptStats{SuccessItemCnt: 999}
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptIncompleteTurnsFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.incompleteErr = errors.New("turn scan failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

// Every ownership re-read guards a committed side effect. A transient primary
// read failure must stop the completion and let the broker retry it.
func TestCompleteExptOwnershipReadFailureKeepsRetry(t *testing.T) {
	for _, at := range []int{3, 4, 5, 6, 7, 8, 9} {
		f := newOriginalCompletionSideEffects(t)
		f.getByIDErrAt = at
		f.getByIDErr = errors.New("temporary mysql failure")
		runID := int64(2)
		err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
		require.True(t, errno.NeedMQRetry(err), "ownership read %d must stay retryable, got %v", at, err)
	}
}

func TestCompleteExptAggregatePublishFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.aggregateErr = errors.New("mq unavailable")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptQuotaReleaseFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.quotaErr = errors.New("quota write failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptEarlyRunLogReadFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.runGetErrAt = 1
	f.runGetErr = errors.New("run log read failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptEarlyMissingRunLogKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.runGetNilAt = 1
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptFinalRunLogReadFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.runGetErrAt = 2
	f.runGetErr = errors.New("run log read failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptFinalMissingRunLogKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.runGetNilAt = 2
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptFinalRunLogUpdateFailureKeepsRetry(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.runUpdateErr = errors.New("run log write failed")
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteExptWaitsForCleanupAfterRunFinished(t *testing.T) {
	f := newOriginalCompletionSideEffects(t)
	f.run.Status = int64(entity.ExptStatus_Success)
	f.expt.Status = entity.ExptStatus_Success
	runID := int64(2)
	err := f.manager.CompleteExpt(context.Background(), 1, &runID, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err), "run finished before the cleanup marker must retry, got %v", err)
}

type supersededRunLogBase struct {
	repo.IExptRunLogRepo
	run *entity.ExptRunLog
	err error
}

func (r *supersededRunLogBase) Get(context.Context, int64, int64) (*entity.ExptRunLog, error) {
	return r.run, r.err
}

type supersededRunLogTerminal struct {
	supersededRunLogBase
	applied   bool
	casErr    error
	casCalled bool
}

func (r *supersededRunLogTerminal) UpdateStatusIfCurrent(context.Context, int64, int64, int64, int64, int64) (bool, error) {
	r.casCalled = true
	return r.applied, r.casErr
}

func TestFinishSupersededRunLogBranches(t *testing.T) {
	processing := &entity.ExptRunLog{ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Processing)}
	finished := &entity.ExptRunLog{ExptID: 1, ExptRunID: 2, Status: int64(entity.ExptStatus_Success)}

	t.Run("read_error", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogBase{err: errors.New("db down")}}
		require.Error(t, m.finishSupersededRunLog(context.Background(), 1, 2, 3))
	})
	t.Run("missing_run_log", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogBase{}}
		require.True(t, errno.NeedMQRetry(m.finishSupersededRunLog(context.Background(), 1, 2, 3)))
	})
	t.Run("already_finished", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogBase{run: finished}}
		require.NoError(t, m.finishSupersededRunLog(context.Background(), 1, 2, 3))
	})
	t.Run("non_terminal_status", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogBase{run: processing}}
		require.NoError(t, m.finishSupersededRunLog(context.Background(), 1, 2, 3, entity.WithStatus(entity.ExptStatus_Processing)))
	})
	t.Run("missing_terminal_cas", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogBase{run: processing}}
		require.True(t, errno.NeedMQRetry(m.finishSupersededRunLog(context.Background(), 1, 2, 3)))
	})
	t.Run("cas_error", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogTerminal{supersededRunLogBase: supersededRunLogBase{run: processing}, casErr: errors.New("db down")}}
		require.Error(t, m.finishSupersededRunLog(context.Background(), 1, 2, 3))
	})
	t.Run("cas_lost_race", func(t *testing.T) {
		m := &ExptMangerImpl{runLogRepo: &supersededRunLogTerminal{supersededRunLogBase: supersededRunLogBase{run: processing}}}
		require.True(t, errno.NeedMQRetry(m.finishSupersededRunLog(context.Background(), 1, 2, 3)))
	})
	t.Run("cas_applied", func(t *testing.T) {
		repo := &supersededRunLogTerminal{supersededRunLogBase: supersededRunLogBase{run: processing}, applied: true}
		m := &ExptMangerImpl{runLogRepo: repo}
		require.NoError(t, m.finishSupersededRunLog(context.Background(), 1, 2, 3))
		require.True(t, repo.casCalled)
	})
}

func TestCurrentRunCompletionOwnedBranches(t *testing.T) {
	t.Run("read_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
		exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, errors.New("db down"))
		m := &ExptMangerImpl{exptRepo: exptRepo}
		owned, err := m.currentRunCompletionOwned(context.Background(), 1, 2, 3)
		require.False(t, owned)
		require.True(t, errno.NeedMQRetry(err))
	})
	t.Run("missing_experiment", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
		exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
		m := &ExptMangerImpl{exptRepo: exptRepo}
		owned, err := m.currentRunCompletionOwned(context.Background(), 1, 2, 3)
		require.False(t, owned)
		require.True(t, errno.NeedMQRetry(err))
	})
}

func TestCompleteRunRunLogReadFailureStaysRetryable(t *testing.T) {
	ctrl := gomock.NewController(t)
	exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
	runLogRepo := repomocks.NewMockIExptRunLogRepo(ctrl)
	m := &ExptMangerImpl{runFinishRepo: &originalCompletionRunFinishRepo{}, exptRepo: exptRepo, runLogRepo: runLogRepo}

	exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{
		ID: 1, SpaceID: 3, ExptType: entity.ExptType_Offline, LatestRunID: 2, Status: entity.ExptStatus_Processing,
	}, nil)
	runLogRepo.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(nil, errors.New("db down"))
	err := m.CompleteRun(context.Background(), 1, 2, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}

func TestCompleteRunMissingRunLogKeepsRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
	runLogRepo := repomocks.NewMockIExptRunLogRepo(ctrl)
	m := &ExptMangerImpl{runFinishRepo: &originalCompletionRunFinishRepo{}, exptRepo: exptRepo, runLogRepo: runLogRepo}

	exptRepo.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{
		ID: 1, SpaceID: 3, ExptType: entity.ExptType_Offline, LatestRunID: 2, Status: entity.ExptStatus_Processing,
	}, nil)
	runLogRepo.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(nil, nil)
	err := m.CompleteRun(context.Background(), 1, 2, 3, &entity.Session{})
	require.True(t, errno.NeedMQRetry(err))
}
