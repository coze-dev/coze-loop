// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type sqlMockProvider struct {
	db *gorm.DB
}

func (p *sqlMockProvider) NewSession(context.Context, ...db.Option) *gorm.DB { return p.db }
func (p *sqlMockProvider) Transaction(ctx context.Context, fn func(tx *gorm.DB) error, _ ...db.Option) error {
	return fn(p.db.WithContext(ctx))
}

func newCentralRepoWithMock(t *testing.T) (*centralActiveRunRepo, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return NewCentralActiveRunRepo(&sqlMockProvider{db: gormDB}).(*centralActiveRunRepo), mock
}

func TestCentralActiveRunRepo_ListRecentRuns(t *testing.T) {
	repo, mock := newCentralRepoWithMock(t)
	now := time.Unix(100_000, 0)
	mock.ExpectQuery(`(?s)SELECT r\.expt_id, r\.space_id, r\.expt_run_id.*FROM expt_run_log r.*INNER JOIN experiment e.*r\.updated_at >= \?.*LIMIT \?`).
		WithArgs(now.Add(-time.Hour), 50).
		WillReturnRows(sqlmock.NewRows([]string{
			"expt_id", "space_id", "expt_run_id", "mode", "status", "created_by",
			"run_created_at", "expt_type", "source_type", "trigger_type", "expt_created_at",
		}).AddRow(100, 1, 1000, int32(entity.EvaluationModeSubmit), int64(entity.ExptStatus_Processing),
			"user-1", now, int64(entity.ExptType_Offline), int64(entity.SourceType_Evaluation), "manual", now))

	runs, err := repo.ListRecentRuns(context.Background(), now.Add(-time.Hour), 50)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, int64(100), runs[0].ExptID)
	assert.Equal(t, int64(1000), runs[0].RunID)
	assert.Equal(t, entity.EvaluationModeSubmit, runs[0].RunMode)
	assert.Equal(t, entity.ExptStatus_Processing, runs[0].RunStatus)
	assert.Equal(t, "user-1", runs[0].CreatedBy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCentralActiveRunRepo_LoadExperiment(t *testing.T) {
	repo, mock := newCentralRepoWithMock(t)
	evalConf := []byte(`{"ItemConcurNum":12,"Ext":{"central_scheduling_priority":"88"}}`)
	mock.ExpectQuery(`(?s)SELECT id, space_id, expt_type, source_type, trigger_type, target_id, target_type, eval_conf.*FROM experiment.*WHERE id = \? AND space_id = \?`).
		WithArgs(int64(100), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "space_id", "expt_type", "source_type", "trigger_type", "target_id", "target_type", "eval_conf",
		}).AddRow(100, 1, int64(entity.ExptType_Offline), int64(entity.SourceType_Evaluation), "manual", 7, int64(entity.EvalTargetTypeSandboxAgent), evalConf))
	mock.ExpectQuery(`(?s)SELECT evaluator_version_id.*FROM expt_evaluator_ref.*WHERE expt_id = \? AND space_id = \?`).
		WithArgs(int64(100), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"evaluator_version_id"}).AddRow(101).AddRow(102))

	expt, err := repo.LoadExperiment(context.Background(), 100, 1)
	require.NoError(t, err)
	assert.Equal(t, 12, expt.ItemConcurNum)
	assert.Equal(t, []int64{101, 102}, expt.EvaluatorVersionIDs)
	assert.Equal(t, int64(7), expt.TargetID)
	assert.Equal(t, entity.EvalTargetTypeSandboxAgent, expt.TargetType)
	assert.Equal(t, "88", expt.Ext["central_scheduling_priority"])
	claims := expt.ResourceDemand()
	assert.Equal(t, int64(2), claims[entity.CentralResourceEvaluator])
	assert.Equal(t, int64(1), claims[entity.CentralResourceSandbox])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCentralActiveRunRepo_ItemScans(t *testing.T) {
	repo, mock := newCentralRepoWithMock(t)
	run := &entity.CentralActiveRun{ExptID: 100, SpaceID: 1, RunID: 1000}
	mock.ExpectQuery(`(?s)SELECT item_id.*FROM expt_item_result_run_log.*status = \?.*ORDER BY item_id ASC.*LIMIT \?`).
		WithArgs(int64(100), int64(1000), int64(1), int32(entity.ItemRunState_Queueing), 3).
		WillReturnRows(sqlmock.NewRows([]string{"item_id"}).AddRow(11).AddRow(12))
	ids, err := repo.QueueingItemIDs(context.Background(), run, 3)
	require.NoError(t, err)
	assert.Equal(t, []int64{11, 12}, ids)

	mock.ExpectQuery(`(?s)SELECT item_id, status.*FROM expt_item_result_run_log.*item_id IN \(\?,\?\)`).
		WithArgs(int64(100), int64(1000), int64(1), int64(11), int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"item_id", "status"}).
			AddRow(11, int32(entity.ItemRunState_Processing)).
			AddRow(12, int32(entity.ItemRunState_Success)))
	states, err := repo.ItemStates(context.Background(), run, []int64{11, 12})
	require.NoError(t, err)
	assert.Equal(t, entity.ItemRunState_Processing, states[11])
	assert.Equal(t, entity.ItemRunState_Success, states[12])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCentralActiveRunRepo_Snapshots(t *testing.T) {
	repo, mock := newCentralRepoWithMock(t)
	run := &entity.CentralActiveRun{ExptID: 100, SpaceID: 1, RunID: 1000}
	oldest := time.Unix(90_000, 0)
	mock.ExpectQuery(`(?s)SELECT expt_id, expt_run_id.*FROM expt_item_result_run_log.*GROUP BY expt_id, expt_run_id`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"expt_id", "expt_run_id", "in_flight", "queueing", "terminal", "oldest_queued_at",
		}).AddRow(100, 1000, 1, 4, 2, oldest))

	snaps, err := repo.Snapshots(context.Background(), []*entity.CentralActiveRun{run})
	require.NoError(t, err)
	require.NotNil(t, snaps["100:1000"])
	assert.Equal(t, 1, snaps["100:1000"].InFlight)
	assert.Equal(t, 4, snaps["100:1000"].Queueing)
	assert.Equal(t, 2, snaps["100:1000"].Terminal)
	require.NoError(t, mock.ExpectationsWereMet())
}
