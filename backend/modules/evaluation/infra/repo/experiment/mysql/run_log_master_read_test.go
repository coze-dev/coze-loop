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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func newRunLogResolverTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	sourceSQL, sourceMock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sourceSQL.Close() })
	replicaSQL, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
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
	return gormDB, sourceMock
}

func TestExptItemResultDAO_MGetItemRunLog_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &exptItemResultDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_item_result_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.MGetItemRunLog(contexts.WithCtxWriteDB(context.Background()), 1, 2, []int64{3}, 4)
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestExptTurnResultDAO_MGetItemTurnRunLogs_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
	gormDB, sourceMock := newRunLogResolverTestDB(t)
	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB)
	dao := &ExptTurnResultDAOImpl{provider: provider}

	sourceMock.ExpectQuery("SELECT .* FROM `expt_turn_result_run_log`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	_, err := dao.MGetItemTurnRunLogs(contexts.WithCtxWriteDB(context.Background()), 1, 2, []int64{3}, 4)
	require.NoError(t, err)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}
