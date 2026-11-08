// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

type onlineTerminationQuota struct {
	repo.QuotaRepo
	releases int
}

func (q *onlineTerminationQuota) CreateOrUpdate(ctx context.Context, space int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), session *entity.Session) error {
	q.releases++
	return q.QuotaRepo.CreateOrUpdate(ctx, space, update, session)
}

// Keep SQL statistics and terminal writes real; the filter sink is external.
type onlineTerminationResult struct {
	ExptResultService
}

func (r *onlineTerminationResult) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

type onlineTerminationFixture struct {
	*independentAuditFixture
	quota     *onlineTerminationQuota
	service   *ExptItemEventEvalServiceImpl
	event     *entity.ExptItemEvalEvent
	lifecycle []*entity.ExptLifecycleEvent
	aggr      int
	evals     int
}

func newOnlineTerminationFixture(t *testing.T, kind entity.ExptType, mode entity.ExptRunMode) *onlineTerminationFixture {
	t.Helper()
	f := newIndependentAuditFixture(t)
	f.seedSources(1, false)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("expt_type", int32(kind)).Error)
	require.NoError(t, f.manager.runLogRepo.Update(f.ctx, f.expt, f.r1, f.space, map[string]any{"mode": int32(mode)}))
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Update("ext", []byte(`{}`)).Error)
	f.manager.statsRepo = exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.provider))
	f.manager.exptResultService = &onlineTerminationResult{ExptResultService: f.manager.exptResultService}
	q := &onlineTerminationQuota{QuotaRepo: f.manager.quotaRepo}
	f.manager.quotaRepo = q
	p := &onlineTerminationFixture{independentAuditFixture: f, quota: q}
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{IsInDebt: true}).AnyTimes()
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	publisher.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptLifecycleEvent, _ *time.Duration, _ string) error {
		p.lifecycle = append(p.lifecycle, event)
		return nil
	}).AnyTimes()
	f.manager.publisher = publisher
	aggr := svcmocks.NewMockExptAggrResultService(ctrl)
	aggr.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.AggrCalculateEvent, _ *time.Duration) error {
		require.Equal(t, f.expt, event.ExperimentID)
		require.Equal(t, f.space, event.SpaceID)
		p.aggr++
		return nil
	}).AnyTimes()
	f.manager.exptAggrResultService = aggr
	p.service = &ExptItemEventEvalServiceImpl{manager: f.manager, experimentRepo: f.manager.exptRepo, exptItemResultRepo: f.manager.itemResultRepo, configer: config, itemLocker: func() lock.ILocker { return lock.NewRedisLocker(f.client) }}
	p.event = &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.r1, EvalSetItemID: f.base + 100, ExptRunMode: mode, Session: &entity.Session{}}
	require.True(t, f.slot(), "fixture must own a real Redis experiment quota slot")
	require.NotEqual(t, "<absent>", f.admission())
	return p
}

func (p *onlineTerminationFixture) debt() error {
	return p.service.HandleEventErr(p.service.HandleCurrentSource(func(context.Context, *entity.ExptItemEvalEvent) error {
		p.evals++
		return errors.New("online credit exhausted")
	}))(p.ctx, p.event)
}

func (p *onlineTerminationFixture) assertComplete(t *testing.T) {
	t.Helper()
	expt, err := p.manager.exptRepo.GetByID(p.ctx, p.expt, p.space)
	require.NoError(t, err)
	run, err := p.manager.runLogRepo.Get(p.ctx, p.expt, p.r1)
	require.NoError(t, err)
	t.Logf("REAL_ONLINE_DEBT_STATE type=%d mode=%d run=%d experiment=%d quota_slot=%t", expt.ExptType, run.Mode, run.Status, expt.Status, p.slot())
	require.Equal(t, int64(entity.ExptStatus_Terminated), run.Status)
	require.Equal(t, entity.ExptStatus_Terminated, expt.Status, "run completion must also complete the online experiment")
	require.Equal(t, "online credit exhausted", expt.StatusMessage)
	require.NotNil(t, expt.EndAt)
	require.Equal(t, p.r1, expt.LatestRunID)
	require.False(t, p.slot(), "CompleteExpt must release the real experiment quota slot")
	require.Equal(t, "<absent>", p.admission())
	require.Equal(t, 1, p.quota.releases, "Offline must retain its existing single CompleteExpt cleanup")
	require.Len(t, p.lifecycle, 1)
	require.Equal(t, entity.ExptStatus_Terminated, p.lifecycle[0].ToStatus)
	require.Equal(t, p.r1, *p.lifecycle[0].ExptRunID)
	require.Equal(t, 1, p.aggr)
	var item model.ExptItemResult
	require.NoError(t, p.conn.Where("space_id=?", p.space).Take(&item).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
	var turn model.ExptTurnResult
	require.NoError(t, p.conn.Where("space_id=?", p.space).Take(&turn).Error)
	require.Equal(t, int32(entity.TurnRunState_Terminal), turn.Status)
	var stats model.ExptStats
	require.NoError(t, p.conn.Where("space_id=?", p.space).Take(&stats).Error)
	if expt.ExptType == entity.ExptType_Online {
		// Preserve the legacy pre-termination statistic calculation.
		require.Equal(t, int32(1), stats.ProcessingCnt)
	} else {
		require.Equal(t, int32(1), stats.TerminatedCnt)
	}
}

func TestOnlineTerminationDebtCompletesExperiment(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeAppend} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			p := newOnlineTerminationFixture(t, entity.ExptType_Online, mode)
			require.NoError(t, p.debt())
			p.assertComplete(t)
		})
	}
}

func TestOnlineTerminationOfflineCompletesOnce(t *testing.T) {
	p := newOnlineTerminationFixture(t, entity.ExptType_Offline, entity.EvaluationModeSubmit)
	require.NoError(t, p.debt())
	p.assertComplete(t)
}

func TestOnlineTerminationCompletionErrorsPreserveCauseAndState(t *testing.T) {
	for _, table := range []string{"expt_run_log", "expt_stats"} {
		t.Run(table, func(t *testing.T) {
			p := newOnlineTerminationFixture(t, entity.ExptType_Online, entity.EvaluationModeAppend)
			fault := errors.New("completion SQL unavailable")
			name := "online_termination_completion_fault"
			require.NoError(t, p.conn.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					_ = tx.AddError(fault)
				}
			}))
			err := p.debt()
			require.NoError(t, p.conn.Callback().Update().Remove(name))
			require.ErrorIs(t, err, fault)
			require.True(t, errno.NeedMQRetry(err), "the first completion error must retain its MQ retry marker")
			expt, err := p.manager.exptRepo.GetByID(p.ctx, p.expt, p.space)
			require.NoError(t, err)
			require.Equal(t, entity.ExptStatus_Processing, expt.Status)
			run, err := p.manager.runLogRepo.Get(p.ctx, p.expt, p.r1)
			require.NoError(t, err)
			wantRunStatus := entity.ExptStatus_Processing
			if table == "expt_stats" {
				wantRunStatus = entity.ExptStatus_Terminated
			}
			require.Equal(t, int64(wantRunStatus), run.Status)
			require.True(t, p.slot())
			require.Zero(t, p.quota.releases)
			require.Empty(t, p.lifecycle)
			require.Zero(t, p.aggr)
			t.Logf("FIRST_COMPLETION_ERROR table=%s cause_preserved=true mq_retry=true run=%d experiment=%d quota_slot=%t", table, run.Status, expt.Status, p.slot())
			if table == "expt_stats" {
				// Both e6 and F install Check before source/admission/execution.
				// A terminal message is ACKed; this old limitation is not repaired here.
				before := p.snapshotTables()
				p.service.endpoints = RecordEvalChain(
					p.service.HandleEventErr,
					p.service.HandleEventCheck,
					p.service.HandleCentralAdmission,
					p.service.HandleEventLock,
					p.service.HandleCurrentSource,
					p.service.HandleCentralReservation,
					p.service.HandleEventExec,
				)(func(context.Context, *entity.ExptItemEvalEvent) error {
					t.Fatal("terminal Eval must stop at Check")
					return nil
				})
				require.NoError(t, p.service.Eval(p.ctx, p.event))
				require.Equal(t, before, p.snapshotTables())
				require.True(t, p.slot())
				require.Zero(t, p.quota.releases)
				require.Empty(t, p.lifecycle)
				require.Equal(t, 1, p.evals)
				t.Log("OLD_BASELINE_LIMITATION terminal_Eval_ACK experiment=Processing quota_slot=true; no MQ recovery claimed")
			}
		})
	}
}

func TestOnlineTerminationOfflinePreservesOriginalSource(t *testing.T) {
	p := newOnlineTerminationFixture(t, entity.ExptType_Offline, entity.EvaluationModeSubmit)
	port := p.manager.itemResultRepo.(repo.IExptCurrentResultRepo)
	source, err := port.GetCurrentItemRunSource(p.ctx, p.space, p.expt, p.r1, p.event.EvalSetItemID)
	require.NoError(t, err)
	require.NoError(t, p.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", p.space).Update("target_result_id", p.base+999).Error)
	before := p.snapshotTables()
	applied, err := p.manager.TerminateCurrentRun(p.ctx, p.expt, p.r1, p.space, p.event.Session, source)
	require.NoError(t, err)
	require.False(t, applied)
	require.Equal(t, before, p.snapshotTables())
	require.True(t, p.slot())
	require.NotEqual(t, "<absent>", p.admission())
	require.Zero(t, p.quota.releases)
	require.Empty(t, p.lifecycle)
}

func (p *onlineTerminationFixture) snapshotTables() map[string][]map[string]any {
	p.t.Helper()
	out := map[string][]map[string]any{}
	for _, table := range []string{"experiment", "expt_run_log", "expt_stats", "expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log"} {
		var rows []map[string]any
		require.NoError(p.t, p.conn.Table(table).Where("space_id=?", p.space).Order("id").Find(&rows).Error)
		out[table] = rows
	}
	return out
}
