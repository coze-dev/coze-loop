package target

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestScanPendingFluxCleanupUsesBoundedPrimaryKeyPage(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxItemRunRepo(provider)
	since := time.Unix(1_767_000_000, 0)
	before := since.Add(time.Hour)
	lower := since.Unix() << 32
	upper := before.Unix() << 32
	ext := func(run string) string {
		encoded, err := json.Marshal(map[string]string{"flux_workflow_run": run})
		require.NoError(t, err)
		return string(encoded)
	}
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .*JSON_CONTAINS_PATH.* ORDER BY id DESC LIMIT \\?").
		WithArgs(upper, lower, 4).
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "status", "deleted_at", "ext"}).
			AddRow(lower+4, 11, 2, nil, ext(`{"experiment_id":22,"space_id":11,"invoke_id":`+jsonInt(lower+4)+`,"binding_id":"b","delivery_id":"d-4","cleanup_state":"pending","report_completed":true,"state":"finalized"}`)).
			AddRow(lower+3, 11, 2, nil, ext(`{"experiment_id":22,"space_id":11,"invoke_id":`+jsonInt(lower+3)+`,"binding_id":"b","delivery_id":"d-3","cleanup_state":"archived","report_completed":true,"state":"finalized"}`)).
			AddRow(lower+2, 11, 1, nil, ext(`{"experiment_id":22,"space_id":11,"invoke_id":`+jsonInt(lower+2)+`,"binding_id":"b","delivery_id":"d-2","cleanup_state":"pending","report_completed":true,"state":"finalized"}`)).
			AddRow(lower+1, 11, 3, nil, ext(`{"experiment_id":22,"space_id":11,"invoke_id":`+jsonInt(lower+1)+`,"binding_id":"b","delivery_id":"d-1","cleanup_state":"pending","report_completed":false,"state":"delivery_created"}`)))
	runs, next, err := repo.ScanPendingFluxCleanup(context.Background(), 0, 4, since, before)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.Equal(t, []string{"d-4", "d-2"}, []string{runs[0].DeliveryID, runs[1].DeliveryID})
	require.Equal(t, int32(2), runs[0].RecordStatus)
	require.Equal(t, lower+1, next)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScanPendingFluxCleanupFindsFailedItemWithoutRuntimeReport(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database).Times(3)
	repo := NewFluxItemRunRepo(provider)
	since := time.Unix(1_767_000_000, 0)
	before := since.Add(time.Hour)
	lower := since.Unix() << 32
	upper := before.Unix() << 32
	ext := func(id int64) string {
		encoded, err := json.Marshal(map[string]string{"flux_workflow_run": `{"experiment_id":22,"space_id":11,"invoke_id":` + jsonInt(id) + `,"binding_id":"b","delivery_id":"d","cleanup_state":"pending","report_completed":false,"state":"delivery_created"}`})
		require.NoError(t, err)
		return string(encoded)
	}
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .*JSON_CONTAINS_PATH.* ORDER BY id DESC LIMIT \\?").
		WithArgs(upper, lower, 4).
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "experiment_run_id", "item_id", "status", "deleted_at", "ext"}).
			AddRow(lower+3, 11, 33, 46, 1, nil, ext(lower+3)).
			AddRow(lower+2, 11, 33, 45, 2, nil, ext(lower+2)).
			AddRow(lower+1, 11, 33, 44, 2, nil, ext(lower+1)))
	mock.ExpectQuery("SELECT .* FROM `expt_item_result_run_log` WHERE .*space_id = \\?.*expt_id = \\?.*expt_run_id = \\?.*item_id = \\?.*deleted_at IS NULL").
		WithArgs(int64(11), int64(22), int64(33), int64(45), 1).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(int32(entity.ItemRunState_Processing)))
	mock.ExpectQuery("SELECT .* FROM `expt_item_result_run_log` WHERE .*space_id = \\?.*expt_id = \\?.*expt_run_id = \\?.*item_id = \\?.*deleted_at IS NULL").
		WithArgs(int64(11), int64(22), int64(33), int64(44), 1).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(int32(entity.ItemRunState_Fail)))
	runs, next, err := repo.ScanPendingFluxCleanup(context.Background(), 0, 4, since, before)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, lower+1, runs[0].InvokeID)
	require.True(t, runs[0].ItemRunTerminal)
	require.Zero(t, next)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScanPendingFluxCleanupAdvancesPastEmptyMatchPage(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxItemRunRepo(provider)
	since := time.Unix(1_767_000_000, 0)
	before := since.Add(time.Hour)
	lower := since.Unix() << 32
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .*JSON_CONTAINS_PATH.* ORDER BY id DESC LIMIT \\?").
		WithArgs(lower+10, lower, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "status", "deleted_at", "ext"}).
			AddRow(lower+9, 11, 1, nil, `{"other":"record"}`).
			AddRow(lower+8, 11, 1, nil, `{"other":"record"}`))
	runs, next, err := repo.ScanPendingFluxCleanup(context.Background(), lower+10, 2, since, before)
	require.NoError(t, err)
	require.Empty(t, runs)
	require.Equal(t, lower+8, next)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScanPendingFluxCleanupFinalPartialPageResetsCursor(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxItemRunRepo(provider)
	since := time.Unix(1_767_000_000, 0)
	before := since.Add(time.Hour)
	lower := since.Unix() << 32
	mock.ExpectQuery("SELECT .* FROM `eval_target_record` WHERE .*JSON_CONTAINS_PATH.* ORDER BY id DESC LIMIT \\?").
		WithArgs(lower+11, lower, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "status", "deleted_at", "ext"}).
			AddRow(lower+10, 11, 1, nil, `{"other":"record"}`))
	runs, next, err := repo.ScanPendingFluxCleanup(context.Background(), lower+11, 2, since, before)
	require.NoError(t, err)
	require.Empty(t, runs)
	require.Zero(t, next)
	require.NoError(t, mock.ExpectationsWereMet())
}

func jsonInt(v int64) string {
	encoded, _ := json.Marshal(v)
	return string(encoded)
}

func TestFluxRecordIDFloorMatchesGeneratorTimeBits(t *testing.T) {
	const seconds int64 = 1_767_000_000
	id, err := fluxRecordIDFloor(time.Unix(seconds, 123_000_000))
	require.NoError(t, err)
	require.Equal(t, seconds<<32|123<<22, id)
}

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
