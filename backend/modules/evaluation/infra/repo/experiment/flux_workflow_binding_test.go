package experiment

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReserveFluxWorkflowBindingOnlyFirstCallerOwnsCopy(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database).Times(2)
	repo := NewFluxWorkflowBindingRepo(provider)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").
		WithArgs(int64(9), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "eval_conf"}).AddRow(9, []byte(`{"existing":"keep"}`)))
	mock.ExpectExec("UPDATE `experiment` SET `eval_conf`=.*").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	first, err := repo.ReserveFluxWorkflowBinding(context.Background(), 8, 9, &entity.FluxWorkflowBinding{BindingID: "owner-1", State: "preparing"})
	require.NoError(t, err)
	require.Equal(t, "owner-1", first.BindingID)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").
		WithArgs(int64(9), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "eval_conf"}).AddRow(9, []byte(`{"existing":"keep","flux_workflow_binding":{"binding_id":"owner-1","state":"preparing"}}`)))
	mock.ExpectCommit()
	second, err := repo.ReserveFluxWorkflowBinding(context.Background(), 8, 9, &entity.FluxWorkflowBinding{BindingID: "owner-2", State: "preparing"})
	require.NoError(t, err)
	require.Equal(t, "owner-1", second.BindingID)
	require.NoError(t, mock.ExpectationsWereMet())
}
