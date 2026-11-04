package mysql

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/datatypes"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSaveEvalTargetRecordPreservesConcurrentFluxRun(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database).Times(2)
	dao := NewEvalTargetRecordDAO(provider)
	record := &model.TargetRecord{ID: 5, SpaceID: 4, Ext: datatypes.JSON(`{"other":"callback"}`)}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .* FOR UPDATE").
		WithArgs(int64(5), int64(4), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ext"}).AddRow(5, `{"flux_workflow_run":"{\"version\":2}","other":"before"}`))
	mock.ExpectExec("UPDATE `eval_target_record` SET .*").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, dao.Save(context.Background(), record))
	var ext map[string]string
	require.NoError(t, json.Unmarshal(record.Ext, &ext))
	require.Equal(t, `{"version":2}`, ext["flux_workflow_run"])
	require.Equal(t, "callback", ext["other"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateEvalTargetRecordPreservesConcurrentFluxRun(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database).Times(2)
	dao := NewEvalTargetRecordDAO(provider)
	record := &model.TargetRecord{ID: 5, SpaceID: 4, Ext: datatypes.JSON(`{"other":"callback"}`)}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .* FOR UPDATE").
		WithArgs(int64(5), int64(4), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ext"}).AddRow(5, `{"flux_workflow_run":"{\"version\":2}","other":"before"}`))
	mock.ExpectExec("UPDATE `eval_target_record` SET .*").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, dao.Update(context.Background(), record))
	var ext map[string]string
	require.NoError(t, json.Unmarshal(record.Ext, &ext))
	require.Equal(t, `{"version":2}`, ext["flux_workflow_run"])
	require.Equal(t, "callback", ext["other"])
	require.NoError(t, mock.ExpectationsWereMet())
}
