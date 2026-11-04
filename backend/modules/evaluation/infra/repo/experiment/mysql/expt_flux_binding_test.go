package mysql

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type matchesLatestBinding struct{}

func (matchesLatestBinding) Match(value driver.Value) bool {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return false
	}
	var conf struct {
		Other   string `json:"other"`
		Binding struct {
			ID string `json:"binding_id"`
		} `json:"flux_workflow_binding"`
	}
	return json.Unmarshal(raw, &conf) == nil && conf.Other == "new" && conf.Binding.ID == "latest"
}

func TestExperimentUpdatesKeepLatestFluxBinding(t *testing.T) {
	for _, fieldsOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "Update", true: "UpdateFields"}[fieldsOnly], func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
			require.NoError(t, err)
			provider := dbmock.NewMockProvider(gomock.NewController(t))
			provider.EXPECT().NewSession(gomock.Any()).Return(database).Times(2)
			dao := NewExptDAO(provider)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").WithArgs(int64(9), 1).
				WillReturnRows(sqlmock.NewRows([]string{"eval_conf"}).AddRow([]byte(`{"other":"old","flux_workflow_binding":{"binding_id":"latest","state":"ready"}}`)))
			if fieldsOnly {
				mock.ExpectExec("UPDATE `experiment` SET .*").WithArgs(matchesLatestBinding{}, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec("UPDATE `experiment` SET .*").WithArgs(int64(9), matchesLatestBinding{}, sqlmock.AnyArg(), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectCommit()
			incoming := []byte(`{"other":"new","flux_workflow_binding":{"binding_id":"stale","state":"preparing"}}`)
			if fieldsOnly {
				err = dao.UpdateFields(context.Background(), 9, map[string]any{"eval_conf": &incoming})
			} else {
				err = dao.Update(context.Background(), &model.Experiment{ID: 9, EvalConf: &incoming})
			}
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
