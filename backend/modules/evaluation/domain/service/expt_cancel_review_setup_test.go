// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

// Keep the SQL repositories and lifecycle locker real; only detail dependencies
// and quota are faulted. LogRun creates Pending and establishes its run owner.
func newCancelSetupFixture(t *testing.T) (*lateWriteFixture, *ExptMangerImpl, *gomock.Controller, func() bool) {
	t.Helper()
	f := newLateWriteFixture(t, "")
	for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_run_log"} {
		require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
	}
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Pending), "latest_run_id": 0}).Error)
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	factory, server := finishRealLockerFactory(t)
	m.mutex, m.manualRetryLocker = factory(), factory
	m.runFinishRepo = exptrepo.NewExptRunFinishRepo(f.provider)
	m.runLogRepo = exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider))
	m.exptRepo = exptrepo.NewExptRepo(exptmysql.NewExptDAO(f.provider), exptmysql.NewExptEvaluatorRefDAO(f.provider), m.idgenerator)
	m.itemResultRepo = f.itemRepo()
	m.turnResultRepo = f.turnRepo()
	m.lwt.(*mocks.MockILatestWriteTracker).EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(false).AnyTimes()
	m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecRun(f.space, int64(entity.EvaluationModeSubmit)).AnyTimes()
	return f, m, ctrl, func() bool { return server.Exists(m.makeExptMutexLockKey(f.expt)) }
}

func TestCancelSetupRealLogRunDetailAndQuotaFailure(t *testing.T) {
	for _, failure := range []string{"GetDetail", "AllowExptRun"} {
		t.Run(failure, func(t *testing.T) {
			f, m, _, ownsLock := newCancelSetupFixture(t)
			cause := errors.New("setup dependency unavailable")
			sets := m.evaluationSetService.(*svcmocks.MockIEvaluationSetService)
			if failure == "GetDetail" {
				sets.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(0), gomock.Any(), gomock.Any()).Return(nil, cause)
			} else {
				sets.EXPECT().GetEvaluationSet(gomock.Any(), gomock.Any(), int64(0), gomock.Any(), gomock.Any()).Return(&entity.EvaluationSet{}, nil)
				m.exptResultService.(*svcmocks.MockExptResultService).EXPECT().MGetStats(gomock.Any(), []int64{f.expt}, f.space, gomock.Any()).Return(nil, nil)
				m.exptAggrResultService.(*svcmocks.MockExptAggrResultService).EXPECT().BatchGetExptAggrResultByExperimentIDs(gomock.Any(), f.space, []int64{f.expt}).Return(nil, nil)
				m.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).Return(cause)
			}
			session := &entity.Session{UserID: "setup"}
			require.NoError(t, m.LogRun(context.Background(), f.expt, f.run, entity.EvaluationModeSubmit, f.space, nil, session))
			require.True(t, ownsLock())
			run, err := m.runLogRepo.Get(context.Background(), f.expt, f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
			err = m.Run(context.Background(), f.expt, f.run, f.space, 0, session, entity.EvaluationModeSubmit, nil)
			require.ErrorIs(t, err, cause, "the caller keeps the original setup failure")
			run, err = m.runLogRepo.Get(context.Background(), f.expt, f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Failed), run.Status)
			require.Equal(t, cause.Error(), string(run.StatusMessage))
			require.False(t, ownsLock(), "a failed first setup must release its 36h owner")
			expt, err := m.exptRepo.GetByID(context.Background(), f.expt, f.space)
			require.NoError(t, err)
			require.Equal(t, entity.ExptStatus_Pending, expt.Status, "setup abort leaves the experiment ready for its first execution")
			require.Nil(t, expt.EndAt)
			require.NoError(t, m.LogRun(context.Background(), f.expt, f.run+100, entity.EvaluationModeSubmit, f.space, nil, session))
			require.True(t, ownsLock())
			t.Log("REAL_LOGRUN_RUN_SETUP_ABORT_RETRY executed")
		})
	}
}

// These failures must never turn a read error into stale ACK or leave the
// original lifecycle value trapped. SQL is still bounded by the caller context.
func TestCancelSetupCleanupFaultsReleaseOriginalOwner(t *testing.T) {
	for _, fault := range []string{"read_error", "write_error", "parent_cancel"} {
		t.Run(fault, func(t *testing.T) {
			f, m, _, ownsLock := newCancelSetupFixture(t)
			require.NoError(t, m.LogRun(context.Background(), f.expt, f.run, entity.EvaluationModeSubmit, f.space, nil, &entity.Session{UserID: "setup"}))
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			switch fault {
			case "parent_cancel":
				cancel()
			case "read_error":
				require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("setup_read_fault", func(tx *gorm.DB) { _ = tx.AddError(errors.New("read unavailable")) }))
			case "write_error":
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("setup_write_fault", func(tx *gorm.DB) { _ = tx.AddError(errors.New("write unavailable")) }))
			}
			m.cleanupUnscheduledRun(ctx, f.expt, f.run, f.space, errors.New("setup failure"))
			require.False(t, ownsLock(), fmt.Sprintf("%s must release only the captured value", fault))
		})
	}
}

func TestCancelSetupOldRunCannotChangeReplacement(t *testing.T) {
	f, m, _, ownsLock := newCancelSetupFixture(t)
	session := &entity.Session{UserID: "setup"}
	require.NoError(t, m.LogRun(context.Background(), f.expt, f.run, entity.EvaluationModeSubmit, f.space, nil, session))
	released, err := m.mutex.UnlockWithValue(context.Background(), m.makeExptMutexLockKey(f.expt), fmt.Sprint(f.run))
	require.NoError(t, err)
	require.True(t, released)
	require.NoError(t, m.LogRun(context.Background(), f.expt, f.run+100, entity.EvaluationModeSubmit, f.space, nil, session))
	before := f.snapshot()
	m.cleanupUnscheduledRun(context.Background(), f.expt, f.run, f.space, errors.New("late R1 setup"))
	require.True(t, ownsLock())
	require.Equal(t, before, f.snapshot(), "late R1 cleanup cannot write either run")
	// Check the real lifecycle value by trying an R1 compare-and-delete.
	released, err = m.mutex.UnlockWithValue(context.Background(), m.makeExptMutexLockKey(f.expt), fmt.Sprint(f.run))
	require.NoError(t, err)
	require.False(t, released)
	require.True(t, ownsLock())
}

func TestCancelSetupRepositoryRechecksLatestAndPending(t *testing.T) {
	for _, kind := range []string{"normal_cancel", "replacement", "write_rollback"} {
		t.Run(kind, func(t *testing.T) {
			f, m, _, _ := newCancelSetupFixture(t)
			require.NoError(t, m.LogRun(context.Background(), f.expt, f.run, entity.EvaluationModeSubmit, f.space, nil, &entity.Session{UserID: "setup"}))
			request := &entity.ExptRunTerminationRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, SetupAbort: kind != "normal_cancel", Finalize: kind == "normal_cancel", Status: entity.ExptStatus_Terminated}
			if kind == "replacement" {
				require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: f.run + 100, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run + 100, Status: gptr.Of(int64(entity.ExptStatus_Pending))}).Error)
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+100).Error)
			}
			if kind == "write_rollback" {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("setup_abort_rollback", func(tx *gorm.DB) { _ = tx.AddError(errors.New("setup abort SQL failure")) }))
			}
			before := f.snapshot()
			port, err := m.terminationRepo()
			require.NoError(t, err)
			result, err := port.TerminateCurrentRun(context.Background(), request)
			if kind == "replacement" {
				require.NoError(t, err)
				require.True(t, result.NonCurrent)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestNewRunSupersededSourceFixtureScale(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	f := p.f
	const count = 32
	for i := 1; i < count; i++ {
		item := f.item + int64(i)
		require.NoError(t, f.conn.Create(&model.ExptItemResult{ID: f.base + 20000 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Terminal)}).Error)
		require.NoError(t, f.conn.Create(&model.ExptItemResultRunLog{ID: f.base + 21000 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Terminal), ResultState: gptr.Of(int32(entity.ExptItemResultStateLogged))}).Error)
	}
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"scheduler_mode": entity.ExptDispatchModeEnforce, "scheduler_scope": testScope}).Error)
	guard := &fakeGuard{}
	p.manager.centralGuard = guard
	var sourceReads atomic.Int32
	require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("superseded_source_scale", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_turn_evaluator_result_ref" {
			sourceReads.Add(1)
		}
	}))
	start := time.Now()
	require.NoError(t, p.manager.releaseCurrentSupersededRunQuota(context.Background(), f.expt, f.space, f.run+100, entity.EvaluationModeRetryAll))
	releases := guard.releases()
	require.Len(t, releases, count, "terminal residual debt remains included at fixture scale")
	seen := map[int64]bool{}
	for _, release := range releases {
		require.Equal(t, f.run, release.RunID)
		require.Empty(t, release.RequestID)
		seen[release.ItemID] = true
	}
	require.Len(t, seen, count)
	t.Logf("SUPERSEDED_SOURCE_FIXTURE items=%d full_primary_sources=%d elapsed=%s; candidate measurement only, no production performance bound claimed", count, sourceReads.Load(), time.Since(start))
}
