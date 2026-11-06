package target

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestFluxItemRunCASRejectsStaleVersionWithoutWriting(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxItemRunRepo(provider)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .* FOR UPDATE").
		WithArgs(int64(5), int64(4), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ext"}).AddRow(5, `{"flux_workflow_run":"{\"experiment_id\":3,\"space_id\":4,\"invoke_id\":5,\"state\":\"reserved\",\"version\":2}"}`))
	mock.ExpectCommit()
	updated, current, err := repo.CompareAndSwapFluxItemRun(context.Background(), 4, 5, 1, &entity.FluxItemRun{ExperimentID: 3, SpaceID: 4, InvokeID: 5, State: "delivery_created"})
	require.NoError(t, err)
	require.False(t, updated)
	require.Equal(t, int64(2), current.Version)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFluxItemRunCASStoresFullRuntimeState(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxItemRunRepo(provider)
	var next entity.FluxItemRun
	require.NoError(t, json.Unmarshal([]byte(`{"experiment_id":3,"space_id":4,"invoke_id":"5","binding_id":"b","state":"reserved","plan":{"targets":[{"node_key":"code"}]},"space_request":{"execution_policy":null}}`), &next))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .* FOR UPDATE").
		WithArgs(int64(5), int64(4), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "ext"}).AddRow(5, `{"other":"preserve"}`))
	mock.ExpectExec("UPDATE `eval_target_record` SET `ext`=.* WHERE .*").
		WithArgs(sqlmock.AnyArg(), int64(5), int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	updated, current, err := repo.CompareAndSwapFluxItemRun(context.Background(), 4, 5, 0, &next)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, int64(1), current.Version)
	output, err := json.Marshal(current)
	require.NoError(t, err)
	require.Contains(t, string(output), `"execution_policy":null`)
	require.NoError(t, mock.ExpectationsWereMet())
}
