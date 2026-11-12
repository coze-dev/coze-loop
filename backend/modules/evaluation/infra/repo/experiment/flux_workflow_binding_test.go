package experiment

import (
	"context"
	"database/sql/driver"
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

type keepsFluxExperimentField struct{}

type keepsFluxSetupLauncher struct{}

func (keepsFluxSetupLauncher) Match(value driver.Value) bool {
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
		Binding entity.FluxWorkflowBinding `json:"flux_workflow_binding"`
	}
	return json.Unmarshal(raw, &conf) == nil &&
		conf.Binding.SetupLauncherURL == "https://tos.example/bridge.sh" &&
		conf.Binding.SetupLauncherSHA256 == "abc123"
}

func (keepsFluxExperimentField) Match(value driver.Value) bool {
	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return false
	}
	var conf map[string]json.RawMessage
	return json.Unmarshal(raw, &conf) == nil && string(conf["existing"]) == `"keep"`
}

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
	mock.ExpectExec("UPDATE `experiment` SET `eval_conf`=.*").WithArgs(keepsFluxExperimentField{}, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
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

func TestReadyFluxWorkflowBindingRejectsChangedAssetMap(t *testing.T) {
	repo := NewFluxWorkflowBindingRepo(nil)
	err := repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", AssetMapJSON: json.RawMessage(`{"skill":"version"}`),
		AssetMapSHA256: "stale",
	})
	require.ErrorContains(t, err, "asset map digest differs")
}

func TestReadyFluxWorkflowBindingRejectsChangedInlineSource(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxWorkflowBindingRepo(provider)
	stored := &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", SourceSpaceID: "source-space",
		ExecutionSpaceID: "source-space", InlineYAMLRef: "frozen/a.yaml", InlineYAMLSHA256: "sha-a",
	}
	conf, err := json.Marshal(map[string]any{"flux_workflow_binding": stored})
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").
		WithArgs(int64(9), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "eval_conf"}).AddRow(9, conf))
	mock.ExpectRollback()
	changed := *stored
	changed.InlineYAMLSHA256 = "sha-b"
	err = repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &changed)
	require.ErrorContains(t, err, "another source")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadyFluxWorkflowBindingRejectsChangedResolvedModel(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxWorkflowBindingRepo(provider)
	stored := &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", SourceSpaceID: "source-space",
		ExecutionSpaceID: "source-space", InlineYAMLRef: "frozen/a.yaml", InlineYAMLSHA256: "sha-a",
		ResolvedModelName: "gpt-6-sol", ResolvedAgentType: "codex", ModelSource: "workflow_entry_agent",
	}
	conf, err := json.Marshal(map[string]any{"flux_workflow_binding": stored})
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").
		WithArgs(int64(9), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "eval_conf"}).AddRow(9, conf))
	mock.ExpectRollback()
	changed := *stored
	changed.ResolvedModelName = "gpt-5.6"
	err = repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &changed)
	require.ErrorContains(t, err, "another source")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadyFluxWorkflowBindingRejectsChangedSetupLauncher(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxWorkflowBindingRepo(provider)
	stored := &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", SourceSpaceID: "source-space",
		ExecutionSpaceID: "source-space", InlineYAMLRef: "frozen/a.yaml", InlineYAMLSHA256: "sha-a",
		SetupLauncherURL: "https://tos.example/bridge-v1.sh", SetupLauncherSHA256: "sha-v1",
	}
	conf, err := json.Marshal(map[string]any{"flux_workflow_binding": stored})
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").
		WithArgs(int64(9), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "eval_conf"}).AddRow(9, conf))
	mock.ExpectRollback()
	changed := *stored
	changed.SetupLauncherURL = "https://tos.example/bridge-v2.sh"
	err = repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &changed)
	require.ErrorContains(t, err, "another source")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadyFluxWorkflowBindingPersistsSetupLauncher(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	database, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmock.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any()).Return(database)
	repo := NewFluxWorkflowBindingRepo(provider)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `experiment` WHERE .* FOR UPDATE").
		WithArgs(int64(9), int64(8), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "eval_conf"}).AddRow(9, []byte(`{"flux_workflow_binding":{"binding_id":"binding","state":"preparing"}}`)))
	mock.ExpectExec("UPDATE `experiment` SET `eval_conf`=.*").WithArgs(keepsFluxSetupLauncher{}, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	err = repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", SetupLauncherURL: "https://tos.example/bridge.sh", SetupLauncherSHA256: "abc123",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReadyFluxWorkflowBindingRejectsCrossSpaceInlineExecution(t *testing.T) {
	repo := NewFluxWorkflowBindingRepo(nil)
	err := repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", SourceSpaceID: "source-space",
		ExecutionSpaceID: "other-space", InlineYAMLRef: "frozen/a.yaml", InlineYAMLSHA256: "sha-a",
	})
	require.ErrorContains(t, err, "run in its source space")
}

func TestReadyFluxWorkflowBindingRejectsIncompleteInlineSource(t *testing.T) {
	repo := NewFluxWorkflowBindingRepo(nil)
	err := repo.PutReadyFluxWorkflowBinding(context.Background(), 8, 9, &entity.FluxWorkflowBinding{
		BindingID: "binding", State: "ready", SourceSpaceID: "source-space",
		ExecutionSpaceID: "source-space", InlineYAMLSHA256: "sha-a",
	})
	require.ErrorContains(t, err, "inline source must have")
}
