// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func newRunLogResolverTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, sqlmock.Sqlmock) {
	t.Helper()

	sourceSQL, sourceMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sourceSQL.Close() })
	replicaSQL, replicaMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = replicaSQL.Close() })

	sourceDialector := mysql.New(mysql.Config{Conn: sourceSQL, SkipInitializeWithVersion: true})
	replicaDialector := mysql.New(mysql.Config{Conn: replicaSQL, SkipInitializeWithVersion: true})
	gormDB, err := gorm.Open(sourceDialector, &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gormDB.Use(dbresolver.Register(dbresolver.Config{
		Sources:  []gorm.Dialector{sourceDialector},
		Replicas: []gorm.Dialector{replicaDialector},
	})))
	return gormDB, sourceMock, replicaMock
}

func TestExptTurnResultDAO_MGetItemTurnRunLogs_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock, _ := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_turn_result_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.MGetItemTurnRunLogs(contexts.WithCtxWriteDB(context.Background()), 1, 2, []int64{3}, 4)
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestExptTurnResultDAO_GetItemTurnRunLogs_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock, _ := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_turn_result_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.GetItemTurnRunLogs(contexts.WithCtxWriteDB(context.Background()), 1, 2, 3, 4)
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestExptTurnResultDAO_ScanTurnResults_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock, _ := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_turn_result`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, _, err := dao.ScanTurnResults(contexts.WithCtxWriteDB(context.Background()), 1, nil, 0, 50, 2)
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestExptTurnResultDAO_GetItemTurnResults_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock, _ := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_turn_result`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.GetItemTurnResults(contexts.WithCtxWriteDB(context.Background()), 1, 3, 4)
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestExptTurnEvaluatorResultRefDAO_BatchGet_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock, _ := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnEvaluatorResultRefDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_turn_evaluator_result_ref`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.BatchGet(contexts.WithCtxWriteDB(context.Background()), 1, []int64{2})
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestExptTurnResultDAO_GetItemTurnRunLogs_ReadContextReadsReplica(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, _, replicaMock := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}

	replicaMock.ExpectQuery("SELECT .* FROM `expt_turn_result_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.GetItemTurnRunLogs(context.Background(), 1, 2, 3, 4)
	require.NoError(t, err)
	require.NoError(t, replicaMock.ExpectationsWereMet())
}

// Projection candidates and per-item snapshot refs must not use lagging replicas.
func TestFilterProjection_WriteContextReadsPrimary(t *testing.T) {
	for _, kind := range []string{"item_candidates", "item_ref"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			conn, master, _ := newRunLogResolverTestDB(t)
			provider := dbmock.NewMockProvider(ctrl)
			provider.EXPECT().NewSession(gomock.Any()).Return(conn).AnyTimes()
			ctx := contexts.WithCtxWriteDB(context.Background())
			if kind == "item_candidates" {
				master.ExpectQuery("SELECT count\\(\\*\\) FROM `expt_item_result`").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				master.ExpectQuery("SELECT .* FROM `expt_item_result`").WillReturnRows(sqlmock.NewRows([]string{"id", "item_id"}).AddRow(1, 7384603622000000011))
				rows, _, err := (&exptItemResultDAOImpl{provider: provider}).ListItemResultsByExptID(ctx, 1, 3, entity.Page{}, false)
				require.NoError(t, err)
				require.Len(t, rows, 1)
			} else {
				master.ExpectQuery("SELECT .* FROM `expt_item_ref`").WillReturnRows(sqlmock.NewRows([]string{"id", "item_id", "eval_set_version_id"}).AddRow(1, 7384603622000000011, 7384603622000000099))
				rows, err := (&exptItemRefDAOImpl{provider: provider}).MGetByExptIDAndItemIDs(ctx, 3, 1, []int64{7384603622000000011})
				require.NoError(t, err)
				require.Len(t, rows, 1)
			}
			require.NoError(t, master.ExpectationsWereMet())
		})
	}
}
