// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
	"testing"
)

func TestSandboxInvocationOwnerUsesRunKeyAndPrimary(t *testing.T) {
	source, expect, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = source.Close() }()
	replica, replicaExpect, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = replica.Close() }()
	primaryDial := mysql.New(mysql.Config{Conn: source, SkipInitializeWithVersion: true})
	replicaDial := mysql.New(mysql.Config{Conn: replica, SkipInitializeWithVersion: true})
	database, err := gorm.Open(primaryDial, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.Use(dbresolver.Register(dbresolver.Config{Sources: []gorm.Dialector{primaryDial}, Replicas: []gorm.Dialector{replicaDial}})))
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	svc := &EvalTargetRepoImpl{dbProvider: provider}
	expect.ExpectQuery("SELECT .* FROM `expt_run_log` WHERE \\(expt_run_id = \\? AND expt_id = \\? AND space_id = \\?\\)").WithArgs(int64(31), int64(30), int64(99), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "expt_run_id", "expt_id", "space_id", "status"}).AddRow(888, 31, 30, 99, int64(entity.ExptStatus_Processing)))
	expect.ExpectQuery("SELECT .* FROM `expt_item_result_run_log` WHERE \\(space_id = \\? AND expt_id = \\? AND expt_run_id = \\? AND item_id = \\?\\)").WithArgs(int64(99), int64(30), int64(31), int64(40), 1).WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(889, int32(entity.ItemRunState_Processing)))
	active, err := svc.SandboxInvocationOwnerActive(context.Background(), 99, 30, 31, 40)
	require.NoError(t, err)
	require.True(t, active)
	require.NoError(t, expect.ExpectationsWereMet())
	require.NoError(t, replicaExpect.ExpectationsWereMet())
}

func TestSandboxInvocationReadyCASCannotOverwriteReportedStatus(t *testing.T) {
	sqlDB, expect, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	svc := &EvalTargetRepoImpl{dbProvider: provider}
	status := entity.EvalTargetRunStatusAsyncInvoking
	record := &entity.EvalTargetRecord{ID: 50, SpaceID: 2, Status: &status, Ext: map[string]string{"sandbox_lifecycle_version": "1", "sandbox_lifecycle_phase": "ready"}}
	expect.ExpectBegin()
	expect.ExpectExec("UPDATE `eval_target_record` SET .* WHERE \\(id = \\? AND space_id = \\? AND status = \\? AND JSON_UNQUOTE\\(JSON_EXTRACT\\(ext, '\\$.sandbox_lifecycle_phase'\\)\\) = \\?\\)").WillReturnResult(sqlmock.NewResult(0, 0))
	expect.ExpectCommit()
	updated, err := svc.TransitionSandboxInvocation(context.Background(), record, "initializing")
	require.NoError(t, err)
	require.False(t, updated)
	require.NoError(t, expect.ExpectationsWereMet())
}
