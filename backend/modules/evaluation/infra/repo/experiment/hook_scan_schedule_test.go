// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
	"time"
)

func TestHookScheduleScanRecoversReadyPendingAndAdvancesRawCursor(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	columns := []string{"space_id", "expt_id", "expt_run_id", "version"}
	readyColumns := []string{"space_id", "expt_id", "expt_run_id", "version", "plan_state", "gate", "execution_started", "finalize_state", "run_status", "run_mode", "marker"}
	mock.ExpectQuery("SELECT .*expt_lifecycle_run").WithArgs("scope", 0, 3).WillReturnRows(sqlmock.NewRows(columns))
	mock.ExpectQuery("SELECT .*expt_lifecycle_run.*LEFT JOIN expt_run_log").WithArgs("scope", 1, 3).WillReturnRows(sqlmock.NewRows(readyColumns).
		AddRow(1, 2, 3, 4, 1, 1, true, 0, int(entity.ExptStatus_Pending), 1, 1).
		AddRow(1, 2, 4, 4, 1, 1, false, 0, int(entity.ExptStatus_Pending), 1, 1).
		AddRow(1, 2, 5, 4, 1, 1, false, 0, int(entity.ExptStatus_Pending), 1, 1))
	in := entity.HookScanInput{ExecutionScope: "scope", Limit: 2, Now: time.Unix(1700000000, 0)}
	page, err := NewHookScheduleScanRepo(p).ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, int64(4), page.NextCursor.RunID, "raw row identity must survive ORM decoding")
	require.Len(t, page.Candidates, 1)
	require.Equal(t, int64(4), page.Candidates[0].Key.RunID)
	require.True(t, page.HasMore)
	require.Equal(t, int64(4), page.NextCursor.RunID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHookScheduleScanPureMergesBothRangesWithoutSkipping(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	old := []string{"space_id", "expt_id", "expt_run_id", "version"}
	ready := []string{"space_id", "expt_id", "expt_run_id", "version", "plan_state", "gate", "execution_started", "finalize_state", "run_status", "run_mode", "marker"}
	mock.ExpectQuery("SELECT .*expt_lifecycle_run").WithArgs("scope", 0, 3).WillReturnRows(sqlmock.NewRows(old).AddRow(1, 2, 3, 1).AddRow(1, 2, 8, 1))
	mock.ExpectQuery("SELECT .*LEFT JOIN expt_run_log").WithArgs("scope", 1, 3).WillReturnRows(sqlmock.NewRows(ready).AddRow(1, 2, 2, 1, 1, 1, false, 0, 2, 1, 1).AddRow(1, 2, 4, 1, 1, 1, false, 0, 2, 1, 1).AddRow(1, 2, 9, 1, 1, 1, false, 0, 2, 1, 1))
	in := entity.HookScanInput{ExecutionScope: "scope", Limit: 2, Now: time.Unix(1700000000, 0)}
	r := NewHookScheduleScanRepo(p)
	first, err := r.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, first.Candidates, 2)
	require.Equal(t, int64(2), first.Candidates[0].Key.RunID)
	require.Equal(t, int64(3), first.NextCursor.RunID)
	in.Cursor = first.NextCursor
	mock.ExpectQuery("SELECT .*expt_lifecycle_run").WithArgs("scope", 0, int64(1), int64(1), int64(3), 3).WillReturnRows(sqlmock.NewRows(old).AddRow(1, 2, 8, 1))
	mock.ExpectQuery("SELECT .*LEFT JOIN expt_run_log").WithArgs("scope", 1, int64(1), int64(1), int64(3), 3).WillReturnRows(sqlmock.NewRows(ready).AddRow(1, 2, 4, 1, 1, 1, false, 0, 2, 6, 1).AddRow(1, 2, 9, 1, 1, 1, false, 0, 2, 1, 1))
	second, err := r.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, second.Candidates, 2)
	require.Equal(t, int64(4), second.Candidates[0].Key.RunID)
	require.Equal(t, int64(8), second.NextCursor.RunID)
	require.True(t, second.HasMore)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHookScheduleScanPureFilteredPageStillAdvances(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	mock.ExpectQuery("SELECT .*expt_lifecycle_run").WithArgs("scope", 0, 3).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version"}))
	mock.ExpectQuery("SELECT .*LEFT JOIN expt_run_log").WithArgs("scope", 1, 3).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version", "plan_state", "gate", "execution_started", "finalize_state", "run_status", "run_mode", "marker"}).
		AddRow(1, 2, 3, 1, 1, 2, false, 0, 2, 1, 1).AddRow(1, 2, 4, 1, 1, 1, false, 0, 3, 1, 1).AddRow(1, 2, 5, 1, 1, 1, false, 0, 2, 1, 1))
	page, err := NewHookScheduleScanRepo(p).ScanPreparingPlans(context.Background(), entity.HookScanInput{ExecutionScope: "scope", Limit: 2, Now: time.Unix(1700000000, 0)})
	require.NoError(t, err)
	require.Empty(t, page.Candidates)
	require.True(t, page.HasMore)
	require.Equal(t, int64(4), page.NextCursor.RunID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHookScheduleRetryScanIncludesModesTwoAndFour(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	mock.ExpectQuery("SELECT .*expt_lifecycle_run").WithArgs("scope", 0, 3).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version"}))
	mock.ExpectQuery("SELECT .*LEFT JOIN expt_run_log").WithArgs("scope", 1, 3).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version", "plan_state", "gate", "execution_started", "finalize_state", "run_status", "run_mode", "marker"}).AddRow(1, 2, 3, 1, 1, 1, false, 0, 2, 2, 1).AddRow(1, 2, 4, 1, 1, 1, false, 0, 2, 4, 1))
	page, err := NewHookScheduleScanRepo(p).ScanPreparingPlans(context.Background(), entity.HookScanInput{ExecutionScope: "scope", Limit: 2, Now: time.Unix(1700000000, 0)})
	require.NoError(t, err)
	require.Len(t, page.Candidates, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestHookScheduleRetryItemsActiveTailRecovery(t *testing.T) {
	conn, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer conn.Close()
	p, err := db.NewDB(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	mock.ExpectQuery("SELECT .*expt_lifecycle_run").WithArgs("scope", 0, 3).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version"}))
	mock.ExpectQuery("SELECT .*LEFT JOIN expt_run_log").WithArgs("scope", 1, 3).WillReturnRows(sqlmock.NewRows([]string{"space_id", "expt_id", "expt_run_id", "version", "plan_state", "gate", "execution_started", "finalize_state", "run_status", "run_mode", "marker"}).
		AddRow(1, 2, 3, 1, 1, 1, true, 0, int(entity.ExptStatus_Processing), 5, 1).
		AddRow(1, 2, 4, 1, 1, 1, true, 0, int(entity.ExptStatus_Processing), 4, 1))
	page, err := NewHookScheduleScanRepo(p).ScanPreparingPlans(context.Background(), entity.HookScanInput{ExecutionScope: "scope", Limit: 2, Now: time.Unix(1700000000, 0)})
	require.NoError(t, err)
	require.Len(t, page.Candidates, 1)
	require.Equal(t, int64(3), page.Candidates[0].Key.RunID)
	require.Equal(t, int64(4), page.NextCursor.RunID)
	require.NoError(t, mock.ExpectationsWereMet())
}
