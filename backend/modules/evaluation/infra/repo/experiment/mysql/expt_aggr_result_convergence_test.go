// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func TestAggrScorePersistence_InsertAndVersionFence(t *testing.T) {
	ctx := context.Background()
	provider, err := db.NewDB(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	require.NoError(t, err)
	conn := provider.NewSession(ctx)
	sqlDB, err := conn.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, conn.Exec(`CREATE TABLE expt_aggr_result (
        id INTEGER PRIMARY KEY, space_id INTEGER NOT NULL, experiment_id INTEGER NOT NULL,
        field_type INTEGER, field_key TEXT NOT NULL, score REAL, aggr_result BLOB,
        version INTEGER NOT NULL, status INTEGER NOT NULL,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, deleted_at DATETIME,
        UNIQUE(experiment_id, field_type, field_key)
    )`).Error)
	dao := NewExptAggrResultDAO(provider)
	empty := []byte(`{"AggregatorResults":[]}`)
	latest := []byte(`{"AggregatorResults":[{"AggregatorType":1,"Data":{"DataType":0,"Value":0.9}}]}`)
	old := []byte(`{"AggregatorResults":[{"AggregatorType":1,"Data":{"DataType":0,"Value":0.25}}]}`)
	row := &model.ExptAggrResult{ID: 1, SpaceID: 100, ExperimentID: 1, FieldType: gptr.Of(int32(1)), FieldKey: "100:judge", Score: gptr.Of(0.0), AggrResult: &empty, Status: 1}
	require.NoError(t, dao.CreateExptAggrResult(ctx, row))
	oldVersion, err := dao.UpdateAndGetLatestVersion(ctx, 1, 1, "100:judge")
	require.NoError(t, err)
	latestVersion, err := dao.UpdateAndGetLatestVersion(ctx, 1, 1, "100:judge")
	require.NoError(t, err)
	require.Greater(t, latestVersion, oldVersion)
	row.Score, row.AggrResult = gptr.Of(0.9), &latest
	require.NoError(t, dao.UpdateExptAggrResultByVersion(ctx, row, latestVersion))
	stale := *row
	stale.ID, stale.Score, stale.AggrResult = 2, gptr.Of(0.25), &old
	assert.Error(t, dao.CreateExptAggrResult(ctx, &stale))
	require.NoError(t, dao.UpdateExptAggrResultByVersion(ctx, &stale, oldVersion))
	got, err := dao.GetExptAggrResult(ctx, 1, 1, "100:judge")
	require.NoError(t, err)
	assert.Equal(t, 0.9, *got.Score)
	assert.Equal(t, latest, *got.AggrResult)
	assert.Equal(t, latestVersion, got.Version)
	emptyVersion, err := dao.UpdateAndGetLatestVersion(ctx, 1, 1, "100:judge")
	require.NoError(t, err)
	row.Score, row.AggrResult = gptr.Of(0.0), &empty
	require.NoError(t, dao.UpdateExptAggrResultByVersion(ctx, row, emptyVersion))
	require.NoError(t, dao.UpdateExptAggrResultByVersion(ctx, &stale, latestVersion))
	got, err = dao.GetExptAggrResult(ctx, 1, 1, "100:judge")
	require.NoError(t, err)
	assert.Equal(t, empty, *got.AggrResult)
	assert.Equal(t, emptyVersion, got.Version)
	assert.Equal(t, int32(1), got.Status)
}

func TestAggrScoreRefs_PrimaryAndReplicaRouting(t *testing.T) {
	for _, query := range []string{"experiment", "evaluator"} {
		t.Run(query, func(t *testing.T) {
			source, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			replica, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			sourceSQL, err := source.DB()
			require.NoError(t, err)
			replicaSQL, err := replica.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sourceSQL.Close(); _ = replicaSQL.Close() })
			for _, conn := range []*gorm.DB{source, replica} {
				require.NoError(t, conn.Exec(`CREATE TABLE expt_turn_evaluator_result_ref (id INTEGER PRIMARY KEY, space_id INTEGER, expt_id INTEGER, evaluator_version_id INTEGER, evaluator_result_id INTEGER, deleted_at DATETIME)`).Error)
			}
			require.NoError(t, replica.Exec(`INSERT INTO expt_turn_evaluator_result_ref (id, space_id, expt_id, evaluator_version_id, evaluator_result_id) VALUES (11, 100, 1, 200, 300)`).Error)
			require.NoError(t, source.Use(dbresolver.Register(dbresolver.Config{Sources: []gorm.Dialector{sqlite.Dialector{Conn: sourceSQL}}, Replicas: []gorm.Dialector{sqlite.Dialector{Conn: replicaSQL}}})))
			ctrl := gomock.NewController(t)
			provider := dbmock.NewMockProvider(ctrl)
			provider.EXPECT().NewSession(gomock.Any()).Return(source).Times(3)
			dao := NewExptTurnEvaluatorResultRefDAO(provider)
			read := func(ctx context.Context) ([]*model.ExptTurnEvaluatorResultRef, error) {
				if query == "experiment" {
					return dao.GetByExptID(ctx, 100, 1)
				}
				return dao.GetByExptEvaluatorVersionID(ctx, 100, 1, 200)
			}
			stale, err := read(context.Background())
			require.NoError(t, err)
			require.Len(t, stale, 1)
			assert.Equal(t, int64(300), stale[0].EvaluatorResultID)
			current, err := read(contexts.WithCtxWriteDB(context.Background()))
			require.NoError(t, err)
			assert.Empty(t, current, "scoring reads must see the primary's cleared refs")
			ordinary, err := read(context.Background())
			require.NoError(t, err)
			assert.Len(t, ordinary, 1, "ordinary reads must retain replica routing")
		})
	}
}
