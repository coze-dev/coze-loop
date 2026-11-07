// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// latestRunRunLogRepo is an ownership-aware run log repository. UpdateIfLatestRun
// mirrors the production CAS; Update records what the legacy unconditional write
// would have persisted.
type latestRunRunLogRepo struct {
	repo.IExptRunLogRepo
	latestRunID int64
	updates     []map[string]any
	legacyCalls int
	casCalls    int
}

func (r *latestRunRunLogRepo) Update(_ context.Context, _, _, _ int64, ufields map[string]any) error {
	r.legacyCalls++
	r.updates = append(r.updates, ufields)
	return nil
}

func (r *latestRunRunLogRepo) UpdateIfLatestRun(_ context.Context, _, exptRunID, _ int64, ufields map[string]any) (bool, error) {
	r.casCalls++
	if r.latestRunID != exptRunID {
		return false, nil
	}
	r.updates = append(r.updates, ufields)
	return true, nil
}

// TestCompleteRunSkipsSupersededRunLogWrite covers the check-to-write window in
// CompleteRun: R2 takes over after the point ownership read but before the
// run-log write, so the canonical scan already reflects R2's rows.
func TestCompleteRunSkipsSupersededRunLogWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	m.runFinishRepo = &originalCompletionRunFinishRepo{}
	var exptID, r1, r2, spaceID int64 = 1, 2, 9, 3

	fake := &latestRunRunLogRepo{latestRunID: r1}
	mock := repomocks.NewMockIExptRunLogRepo(ctrl)
	fake.IExptRunLogRepo = mock
	m.runLogRepo = fake

	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), exptID, spaceID).DoAndReturn(
		func(context.Context, int64, int64) (*entity.Experiment, error) {
			return &entity.Experiment{
				ID: exptID, SpaceID: spaceID, LatestRunID: fake.latestRunID,
				Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline,
			}, nil
		}).AnyTimes()

	mock.EXPECT().Get(gomock.Any(), exptID, r1).DoAndReturn(
		func(context.Context, int64, int64) (*entity.ExptRunLog, error) {
			// R2 takes over after CompleteRun's ownership check, before the scan.
			fake.latestRunID = r2
			return &entity.ExptRunLog{ExptID: exptID, ExptRunID: r1, Status: int64(entity.ExptStatus_Processing)}, nil
		}).AnyTimes()

	m.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any(), false).
		Return([]*entity.ExptTurnResult{{ExptRunID: r2, Status: int32(entity.TurnRunState_Queueing)}}, int64(1), nil).AnyTimes()

	require.NoError(t, m.CompleteRun(context.Background(), exptID, r1, spaceID, &entity.Session{}))
	require.Empty(t, fake.updates, "superseded R1 must not persist R2-derived run counts")
}

// TestCompleteExptTailSkipsSupersededRunLogWrite covers the unguarded tail after
// the last ownership read: R2 takes over during the lifecycle publish, then the
// tail scan sees R2's canonical reset and would otherwise write it into R1.
func TestCompleteExptTailSkipsSupersededRunLogWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	m.runFinishRepo = &originalCompletionRunFinishRepo{}
	var exptID, r1, r2, spaceID int64 = 1, 2, 9, 3

	fake := &latestRunRunLogRepo{latestRunID: r1}
	mock := repomocks.NewMockIExptRunLogRepo(ctrl)
	fake.IExptRunLogRepo = mock
	m.runLogRepo = fake

	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), exptID, spaceID).DoAndReturn(
		func(context.Context, int64, int64) (*entity.Experiment, error) {
			return &entity.Experiment{
				ID: exptID, SpaceID: spaceID, LatestRunID: fake.latestRunID,
				Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline,
			}, nil
		}).AnyTimes()
	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	m.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().
		UpdateByExptID(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil).AnyTimes()
	m.statsRepo.(*repomocks.MockIExptStatsRepo).EXPECT().
		Get(gomock.Any(), exptID, spaceID).Return(&entity.ExptStats{ExptID: exptID, SuccessItemCnt: 1}, nil).AnyTimes()
	m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().
		CalculateStats(gomock.Any(), exptID, spaceID, gomock.Any()).
		Return(&entity.ExptCalculateStats{SuccessItemCnt: 1}, nil).AnyTimes()
	m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().
		GetIncompleteTurns(gomock.Any(), exptID, spaceID, gomock.Any()).Return(nil, nil).AnyTimes()
	m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().
		CreateOrUpdate(gomock.Any(), spaceID, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().
		PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	m.mtr.(*metricmocks.MockExptMetric).EXPECT().
		EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	m.idem.(*idemmocks.MockIdempotentService).EXPECT().
		Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	m.idem.(*idemmocks.MockIdempotentService).EXPECT().
		Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// No lock call is expected: losing ownership must not release R2's holder.
	mock.EXPECT().Get(gomock.Any(), exptID, r1).Return(
		&entity.ExptRunLog{ExptID: exptID, ExptRunID: r1, Status: int64(entity.ExptStatus_Processing)}, nil,
	).AnyTimes()

	// R2 takes over during the lifecycle publish, after the last ownership read.
	m.publisher.(*eventmocks.MockExptEventPublisher).EXPECT().
		PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
			fake.latestRunID = r2
			return nil
		}).AnyTimes()

	m.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any(), false).
		Return([]*entity.ExptTurnResult{{ExptRunID: r2, Status: int32(entity.TurnRunState_Queueing)}}, int64(1), nil).AnyTimes()

	require.NoError(t, m.CompleteExpt(context.Background(), exptID, &r1, spaceID, &entity.Session{}))
	require.Empty(t, fake.updates, "superseded R1 must not persist R2-derived run stats in the completion tail")
}

func TestCompleteRunKeepsOnlineWriteOutsideLatestRunCAS(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	m.runFinishRepo = &originalCompletionRunFinishRepo{}
	var exptID, runID, otherRunID, spaceID int64 = 1, 2, 9, 3

	fake := &latestRunRunLogRepo{latestRunID: otherRunID}
	mock := repomocks.NewMockIExptRunLogRepo(ctrl)
	fake.IExptRunLogRepo = mock
	m.runLogRepo = fake
	m.exptRepo.(*repomocks.MockIExperimentRepo).EXPECT().GetByID(gomock.Any(), exptID, spaceID).
		Return(&entity.Experiment{ID: exptID, SpaceID: spaceID, LatestRunID: otherRunID, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Online}, nil)
	mock.EXPECT().Get(gomock.Any(), exptID, runID).
		Return(&entity.ExptRunLog{ExptID: exptID, ExptRunID: runID, Status: int64(entity.ExptStatus_Processing)}, nil)
	m.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), spaceID, exptID, gomock.Any(), gomock.Any(), false).
		Return(nil, int64(0), nil)
	m.mutex.(*lockmocks.MockILocker).EXPECT().UnlockForce(gomock.Any(), "expt_run_mutex_lock:1").Return(true, nil)

	require.NoError(t, m.CompleteRun(context.Background(), exptID, runID, spaceID, &entity.Session{}))
	require.Equal(t, 1, fake.legacyCalls, "online completion must keep its legacy write and release behavior")
	require.Zero(t, fake.casCalls, "online completion must not depend on offline latest_run ownership")
}
