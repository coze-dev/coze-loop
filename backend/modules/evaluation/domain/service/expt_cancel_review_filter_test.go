// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptck "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/ck"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newCancelFilterFixture(t *testing.T) (*filterSyncFixture, *ExptMangerImpl) {
	t.Helper()
	f := newFilterSyncFixture(t)
	// Keep a single real canonical item with a pending run and original request.
	for _, table := range []string{"expt_item_result_run_log", "expt_item_result"} {
		require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=? AND item_id<>?", f.space, f.expt, f.item).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Update("status", entity.ItemRunState_Processing).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Updates(map[string]any{"status": entity.ItemRunState_Processing, "result_state": entity.ExptItemResultStateDefault}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", entity.TurnRunState_Processing).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=?", f.space).Update("status", entity.TurnRunState_Processing).Error)
	ctrl := gomock.NewController(t)
	m := newTestExptManager(ctrl)
	m.manualRetryLocker = func() lock.ILocker { return filterRealTestLocker(f.client) }
	m.mutex = m.manualRetryLocker()
	m.runLogRepo = exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider))
	m.exptRepo = exptrepo.NewExptRepo(exptmysql.NewExptDAO(f.provider), exptmysql.NewExptEvaluatorRefDAO(f.provider), m.idgenerator)
	m.runFinishRepo = exptrepo.NewExptRunFinishRepo(f.provider)
	m.itemResultRepo, m.turnResultRepo = f.itemRepo(), f.turnRepo()
	m.exptResultService = f.svc
	f.svc.ExperimentRepo = m.exptRepo
	m.quotaRepo, m.publisher, m.exptAggrResultService = nil, nil, nil
	m.evalTargetService = nil
	m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	m.idem = idemrepo.NewIdempotentService(idemredis.NewIdemDAO(f.client))
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("start_at", now).Error)
	return f, m
}

func TestFilterRefreshTerminationRealPrimaryCKSQL(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	ctrl := gomock.NewController(t)
	sqlDB, sink, err := sqlmock.New(sqlmock.ValueConverterOption(filterSQLConverter{}))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	conn, err := gorm.Open(clickhouse.New(clickhouse.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	mapping := mysqlmocks.NewMockIExptTurnResultFilterKeyMappingDAO(ctrl)
	mapping.EXPECT().GetByExptID(gomock.Any(), f.space, f.expt).Return(nil, nil).AnyTimes()
	f.svc.exptTurnResultFilterRepo = exptrepo.NewExptTurnResultFilterRepo(exptck.NewExptTurnResultFilterDAO(&filterCKProvider{conn: conn}, nil), mapping)
	// The final insert must contain the primary canonical Terminal payload under
	// the item lock; cached Processing turns must never reach the CK sink.
	sink.ExpectBegin()
	sink.ExpectPrepare("INSERT INTO `expt_turn_result_filter`").ExpectExec().WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), filterLockedSQLArg{f.server, f.key(), fmt.Sprint(f.item)}, fmt.Sprint(f.version), int64(0), "0", int64(entity.ItemRunState_Terminal), fmt.Sprintf("{\"actual_output\":\"%d\"}", f.base+41), "{}", "{}", "{}", "{}", "{}", int64(0), "0", float64(0), "0", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	sink.ExpectCommit()
	applied, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, sink.ExpectationsWereMet())
	source, err := m.terminationRepo()
	require.NoError(t, err)
	current, err := source.GetTerminationItemSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Equal(t, entity.ItemRunState_Terminal, current.Current.Status)
	require.Equal(t, entity.ExptItemResultStateLogged, current.Current.ResultState)
	rows, _, err := f.turnRepo().ListTurnResultByItemIDs(context.Background(), f.space, f.expt, []int64{f.item}, entity.Page{}, false)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int32(entity.TurnRunState_Terminal), rows[0].Status)
	// Read through the production filtered query and then primary turn rows.
	sink.ExpectQuery(`SELECT COUNT\(DISTINCT item_id\).*etrf.status = \?`).WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), int64(entity.ItemRunState_Terminal), "2026-10-20").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	sink.ExpectQuery(`SELECT  etrf.item_id, etrf.status.*etrf.status = \?`).WithArgs(fmt.Sprint(f.space), fmt.Sprint(f.expt), int64(entity.ItemRunState_Terminal), "2026-10-20", 20, 0).WillReturnRows(sqlmock.NewRows([]string{"item_id", "status"}).AddRow(fmt.Sprint(f.item), int32(entity.ItemRunState_Terminal)))
	expt, err := m.exptRepo.GetByID(context.Background(), f.expt, f.space)
	require.NoError(t, err)
	f.svc.Metric = m.mtr
	m.mtr.(*metricsmocks.MockExptMetric).EXPECT().EmitExptTurnResultFilterQueryLatency(f.space, gomock.Any(), false).AnyTimes()
	turns, states, count, err := f.svc.ListTurnResult(context.Background(), &entity.MGetExperimentResultParam{SpaceID: f.space, BaseExptID: &f.expt, UseAccelerator: true, Page: entity.NewPage(1, 20), FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{f.expt: {ItemRunStatus: []*entity.FieldFilter{{Op: "=", Values: []any{int64(entity.ItemRunState_Terminal)}}}}}}, expt)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.Len(t, turns, 1)
	require.Equal(t, entity.ItemRunState_Terminal, states[f.item])
	require.NoError(t, sink.ExpectationsWereMet())
	t.Log("REAL_PRIMARY_TERMINAL_LOGGED_SOURCE_AND_CANONICAL_READ_CK_SQL_ITEM_LOCK executed")
}

func TestFilterRefreshFinishedReplayBeforeNotificationMarker(t *testing.T) {
	for _, fault := range []string{"filter_write", "lifecycle_mq"} {
		t.Run(fault, func(t *testing.T) {
			f, m := newCancelFilterFixture(t)
			store := &filterStore{fail: fault == "filter_write"}
			f.svc.exptTurnResultFilterRepo = store
			attempts := 0
			store.beforeSave = func(rows []*entity.ExptTurnResultFilterEntity) {
				attempts++
				require.True(t, f.server.Exists(f.key()))
				require.Equal(t, entity.ItemRunState_Terminal, rows[0].Status)
			}
			if fault == "lifecycle_mq" {
				pub := eventmocks.NewMockExptEventPublisher(gomock.NewController(t))
				pub.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("broker unavailable"))
				pub.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.publisher = pub
			}
			_, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
			require.Error(t, err, "a failed filter or notification must remain retryable after commit")
			require.Positive(t, attempts)
			before := f.snapshot()
			store.fail = false
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeSubmit, Session: &entity.Session{}}
			scheduler := &ExptSchedulerImpl{Manager: m}
			require.NoError(t, scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
				t.Fatal("finished retry entered execution")
				return nil
			})(context.Background(), event))
			require.Equal(t, before, f.snapshot(), "replay cannot rewrite stats, end time or logs")
			previous := attempts
			require.NoError(t, scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error { return nil })(context.Background(), event))
			require.Greater(t, attempts, previous, "notification marker must not skip filter calibration")
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestFilterRefreshMissingPayloadCannotMarkNotificationComplete(t *testing.T) {
	f, m := newCancelFilterFixture(t)
	store := &filterStore{}
	f.svc.exptTurnResultFilterRepo = store
	turns := f.svc.ExptTurnResultRepo
	f.svc.ExptTurnResultRepo = &cancelMissingFilterTurns{IExptTurnResultRepo: turns}
	_, err := m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.Error(t, err)
	require.True(t, errno.NeedMQRetry(err))
	marked, err := m.idem.Exist(context.Background(), fmt.Sprintf("finish-cleanup:%d:%d", f.expt, f.run))
	require.NoError(t, err)
	require.False(t, marked)
	require.Empty(t, store.rows)
	before := f.snapshot()
	f.svc.ExptTurnResultRepo = turns
	_, err = m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
	require.NoError(t, err)
	require.NotEmpty(t, store.rows)
	require.Equal(t, entity.ItemRunState_Terminal, store.rows[0].Status)
	require.Equal(t, before, f.snapshot())
}

type cancelMissingFilterTurns struct{ repo.IExptTurnResultRepo }

func (*cancelMissingFilterTurns) ListTurnResultByItemIDs(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
	return nil, 0, nil
}

func TestFilterRefreshParentCancelOnlyResourcesAndNoCompleteTurns(t *testing.T) {
	for _, noComplete := range []bool{false, true} {
		t.Run(fmt.Sprint(noComplete), func(t *testing.T) {
			f, m := newCancelFilterFixture(t)
			store := &filterStore{}
			f.svc.exptTurnResultFilterRepo = store
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			store.beforeSave = func([]*entity.ExptTurnResultFilterEntity) { calls++ }
			if !noComplete {
				m.runFinishRepo = &cancelFilterCommitHook{IExptRunFinishRepo: m.runFinishRepo, port: m.runFinishRepo.(repo.IExptRunTerminationRepo), after: cancel}
			}
			opts := []entity.CompleteExptOptionFn{entity.NoAggrCalculate()}
			if noComplete {
				opts = append(opts, entity.NoCompleteItemTurn())
			}
			_, err := m.TerminateCurrentRun(ctx, f.expt, f.run, f.space, &entity.Session{}, nil, opts...)
			if noComplete {
				require.NoError(t, err)
				require.Zero(t, calls)
				var turn model.ExptTurnResult
				require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&turn).Error)
				require.Equal(t, int32(entity.TurnRunState_Processing), turn.Status)
			} else {
				require.Error(t, err)
				require.Zero(t, calls, "parent cancellation must prevent CK writes after commit")
				var expt model.Experiment
				require.NoError(t, f.conn.Where("space_id=?", f.space).Take(&expt).Error)
				require.Equal(t, int32(entity.ExptStatus_Terminated), expt.Status)
				m.runFinishRepo = m.runFinishRepo.(*cancelFilterCommitHook).IExptRunFinishRepo
				_, err = m.TerminateCurrentRun(context.Background(), f.expt, f.run, f.space, &entity.Session{}, nil, entity.NoAggrCalculate())
				require.NoError(t, err)
				require.Positive(t, calls)
			}
		})
	}
}

type cancelFilterCommitHook struct {
	repo.IExptRunFinishRepo
	port  repo.IExptRunTerminationRepo
	after func()
}

func (p *cancelFilterCommitHook) GetTerminationItemSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptCurrentItemRunSource, error) {
	return p.port.GetTerminationItemSource(ctx, space, expt, run, item)
}

func (p *cancelFilterCommitHook) TerminateCurrentRun(ctx context.Context, r *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
	result, err := p.port.TerminateCurrentRun(ctx, r)
	if err == nil && r.Finalize {
		p.after()
	}
	return result, err
}
