// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOrdinaryRetrySchedulerWithPersistedGate(t *testing.T) {
	for _, old := range []entity.ExptStatus{entity.ExptStatus_Success, entity.ExptStatus_Failed, entity.ExptStatus_Terminated} {
		t.Run(fmt.Sprint(old), func(t *testing.T) {
			conn, sql, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, sql.ExpectationsWereMet()); _ = conn.Close() })
			provider, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
			require.NoError(t, err)
			gate := exptrepo.NewHookGateRepo(provider, nil)
			f := newSchedulerHookFixture(t, nil)
			f.manager.run.Status = int64(entity.ExptStatus_Pending)
			f.event.ExptRunMode = entity.EvaluationModeRetryAll
			// Both middleware admission checks must reach the existing ExptStart path.
			for i := 0; i < 2; i++ {
				sql.ExpectQuery("SELECT .* FROM expt_run_log AS l").WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "status", "experiment_id", "experiment_space_id", "latest_run_id", "experiment_status"}).AddRow(3, 1, 2, 3, int64(entity.ExptStatus_Pending), 2, 1, 3, int64(old)))
			}
			scheduler := f.scheduler.(*ExptSchedulerImpl)
			scheduler.hookGate = gate
			scheduler.hookScheduler = exptrepo.NewHookFinalizationRepo(provider).(repo.IHookSchedulerRepo)
			scheduler.hookRuns = exptrepo.NewHookRunRepo(provider)
			scheduler.hookInitialization = scheduler.hookRuns.(repo.IHookExecutionInitializationRepo)
			// Startup, sandbox sweep, and zombie handling each classify the persisted Run.
			for i := 0; i < 3; i++ {
				sql.ExpectQuery("SELECT .* FROM experiment AS e").WithArgs(int64(3), int64(2), int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "latest_run_id", "status", "run_id", "run_space_id", "run_expt_id", "run_expt_run_id", "run_status", "run_mode"}).AddRow(2, 1, 3, int64(old), 3, 1, 2, 3, int64(entity.ExptStatus_Pending), int32(entity.EvaluationModeRetryAll)))
			}
			require.NoError(t, scheduler.Schedule(context.Background(), f.event))
			require.Equal(t, []string{"ExptStart", "ScheduleStart", "ScanEvalItems", "ExptEnd"}, f.mode.calls)
			require.Empty(t, f.publisher.published)
		})
	}
}
