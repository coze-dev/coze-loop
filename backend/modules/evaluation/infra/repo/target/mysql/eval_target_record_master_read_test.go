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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func TestEvalTargetRecordDAO_GetByIDAndSpaceID_WriteContextReadsPrimary(t *testing.T) {
	ctrl := gomock.NewController(t)
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

	provider := dbmock.NewMockProvider(ctrl)
	dao := &EvalTargetRecordDAOImpl{db: provider, query: query.Use(gormDB)}
	sourceMock.ExpectQuery("SELECT .* FROM `eval_target_record`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id"}).AddRow(int64(1), int64(2)))

	record, err := dao.GetByIDAndSpaceID(contexts.WithCtxWriteDB(context.Background()), 1, 2)
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(1), record.ID)
	require.NoError(t, sourceMock.ExpectationsWereMet())
}

func TestEvalTargetRecordDAO_GetByIDAndSpaceID_ReadContextReadsReplica(t *testing.T) {
	ctrl := gomock.NewController(t)
	sourceSQL, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
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

	provider := dbmock.NewMockProvider(ctrl)
	dao := &EvalTargetRecordDAOImpl{db: provider, query: query.Use(gormDB)}
	replicaMock.ExpectQuery("SELECT .* FROM `eval_target_record`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id"}).AddRow(int64(1), int64(2)))

	record, err := dao.GetByIDAndSpaceID(context.Background(), 1, 2)
	require.NoError(t, err)
	require.NotNil(t, record)
	require.Equal(t, int64(1), record.ID)
	require.NoError(t, replicaMock.ExpectationsWereMet())
}
