// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type filterSourceQueryCounter struct {
	logger.Interface
	queries int
}

func (c *filterSourceQueryCounter) Trace(_ context.Context, _ time.Time, sql func() (string, int64), _ error) {
	query, _ := sql()
	if strings.HasPrefix(strings.TrimSpace(query), "SELECT ") {
		c.queries++
	}
}

func newFilterSourceScopeFixture(t *testing.T, rows []map[string]any) (repo.IExptFilterSourceScanner, *gorm.DB, *filterSourceQueryCounter) {
	t.Helper()
	counter := &filterSourceQueryCounter{Interface: logger.Discard}
	conn, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	// Exercise the production query dialect against a local SQLite engine.
	orm, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{Logger: counter})
	require.NoError(t, err)
	require.NoError(t, orm.Exec("CREATE TABLE experiment (id INTEGER PRIMARY KEY, space_id INTEGER NOT NULL, deleted_at DATETIME)").Error)
	require.NoError(t, orm.Table("experiment").CreateInBatches(rows, 100).Error)
	provider := dbmocks.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ ...db.Option) *gorm.DB {
		return orm.WithContext(ctx)
	}).AnyTimes()
	counter.queries = 0
	return NewExptFilterSourceScanner(provider), orm, counter
}

func scanFilterSourceScopes(t *testing.T, scanner repo.IExptFilterSourceScanner, bounds entity.ExptFilterScanBounds, cursor int64, spaceIDs, exptIDs []int64) []entity.ExptFilterScope {
	t.Helper()
	var scopes []entity.ExptFilterScope
	for {
		page, err := scanner.ScanExperiments(context.Background(), bounds, cursor, 1, spaceIDs, exptIDs)
		require.NoError(t, err)
		if len(page.Scopes) == 0 {
			return scopes
		}
		scopes = append(scopes, page.Scopes...)
		cursor = page.NextExptID
	}
}

func TestFilterSourceSelectedExperimentAvoidsUnrelatedPages(t *testing.T) {
	rows := make([]map[string]any, 0, 10001)
	for id := int64(1); id < 10000; id++ {
		rows = append(rows, map[string]any{"id": id, "space_id": int64(2)})
	}
	rows = append(rows, map[string]any{"id": int64(30000), "space_id": int64(2)})
	rows = append(rows, map[string]any{"id": int64(20001), "space_id": int64(1)})
	scanner, _, counter := newFilterSourceScopeFixture(t, rows)
	bounds, err := scanner.BeginScan(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(30000), bounds.MaxExptID, "the global watermark remains unchanged")
	scopes := scanFilterSourceScopes(t, scanner, bounds, 0, []int64{1}, []int64{20001})
	require.Equal(t, 3, counter.queries, "one global MAX, one selected page and one terminal page")
	require.Equal(t, []entity.ExptFilterScope{{SpaceID: 1, ExptID: 20001}}, scopes)
}

func TestFilterSourceScopeIntersectionAndSnapshot(t *testing.T) {
	rows := []map[string]any{
		{"id": int64(10), "space_id": int64(1), "deleted_at": nil},
		{"id": int64(20), "space_id": int64(2), "deleted_at": nil},
		{"id": int64(30), "space_id": int64(1), "deleted_at": nil},
		{"id": int64(40), "space_id": int64(1), "deleted_at": "2026-01-01 00:00:00"},
	}
	scanner, orm, counter := newFilterSourceScopeFixture(t, rows)
	bounds, err := scanner.BeginScan(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(30), bounds.MaxExptID)
	require.NoError(t, orm.Exec("INSERT INTO experiment(id,space_id) VALUES (50,1)").Error)
	for _, tc := range []struct {
		name              string
		spaceIDs, exptIDs []int64
		cursor            int64
		want              []entity.ExptFilterScope
	}{
		{name: "intersection", spaceIDs: []int64{1}, exptIDs: []int64{20, 30}, want: []entity.ExptFilterScope{{SpaceID: 1, ExptID: 30}}},
		{name: "unordered_duplicates", spaceIDs: []int64{2, 1, 1}, exptIDs: []int64{30, 10, 10}, want: []entity.ExptFilterScope{{SpaceID: 1, ExptID: 10}, {SpaceID: 1, ExptID: 30}}},
		{name: "space_only", spaceIDs: []int64{1}, want: []entity.ExptFilterScope{{SpaceID: 1, ExptID: 10}, {SpaceID: 1, ExptID: 30}}},
		{name: "experiment_only", exptIDs: []int64{20}, want: []entity.ExptFilterScope{{SpaceID: 2, ExptID: 20}}},
		{name: "no_match", spaceIDs: []int64{9}, exptIDs: []int64{10}},
		{name: "nil_all", want: []entity.ExptFilterScope{{SpaceID: 1, ExptID: 10}, {SpaceID: 2, ExptID: 20}, {SpaceID: 1, ExptID: 30}}},
		{name: "empty_all", spaceIDs: []int64{}, exptIDs: []int64{}, want: []entity.ExptFilterScope{{SpaceID: 1, ExptID: 10}, {SpaceID: 2, ExptID: 20}, {SpaceID: 1, ExptID: 30}}},
		{name: "resume_after_cursor", spaceIDs: []int64{1}, cursor: 10, want: []entity.ExptFilterScope{{SpaceID: 1, ExptID: 30}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			counter.queries = 0
			got := scanFilterSourceScopes(t, scanner, bounds, tc.cursor, tc.spaceIDs, tc.exptIDs)
			require.Equal(t, len(tc.want)+1, counter.queries)
			require.Equal(t, tc.want, got)
		})
	}
	nextBounds, err := scanner.BeginScan(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(50), nextBounds.MaxExptID)
}

func TestFilterSourceScannerUsesBoundedCurrentRows(t *testing.T) {
	conn, sql, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	g, err := gorm.Open(mysql.New(mysql.Config{Conn: conn, SkipInitializeWithVersion: true}), &gorm.Config{})
	require.NoError(t, err)
	provider := dbmocks.NewMockProvider(gomock.NewController(t))
	provider.EXPECT().NewSession(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ ...db.Option) *gorm.DB { return g.WithContext(ctx) }).AnyTimes()
	scanner := NewExptFilterSourceScanner(provider)
	sql.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\) FROM `experiment` WHERE `experiment`.`deleted_at` IS NULL").WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(20))
	bounds, err := scanner.BeginScan(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(20), bounds.MaxExptID)
	sql.ExpectQuery("SELECT .* FROM `experiment` WHERE \\(id > \\? AND id <= \\?\\) AND `experiment`.`deleted_at` IS NULL ORDER BY id ASC LIMIT \\?").WithArgs(5, 20, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id"}).AddRow(8, 1).AddRow(15, 2))
	page, err := scanner.ScanExperiments(context.Background(), bounds, 5, 2, nil, nil)
	require.NoError(t, err)
	require.Equal(t, []entity.ExptFilterScope{{SpaceID: 1, ExptID: 8}, {SpaceID: 2, ExptID: 15}}, page.Scopes)
	require.Equal(t, int64(15), page.NextExptID)
	scope := entity.ExptFilterScope{SpaceID: 1, ExptID: 8}
	sql.ExpectQuery("SELECT COALESCE\\(MAX\\(id\\), 0\\) FROM `expt_turn_result` WHERE \\(space_id = \\? AND expt_id = \\?\\) AND `expt_turn_result`.`deleted_at` IS NULL").WithArgs(1, 8).WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(50))
	upper, err := scanner.TurnUpperBound(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, int64(50), upper)
	sql.ExpectQuery("SELECT \\* FROM `expt_turn_result` WHERE \\(space_id = \\? AND expt_id = \\? AND id > \\? AND id <= \\?\\) AND `expt_turn_result`.`deleted_at` IS NULL ORDER BY id ASC LIMIT \\?").WithArgs(1, 8, 10, 50, 2).WillReturnRows(sqlmock.NewRows([]string{"id", "space_id", "expt_id", "item_id", "turn_id", "status"}).AddRow(11, 1, 8, 4, 0, 0).AddRow(49, 1, 8, 5, 1, 2))
	turns, next, err := scanner.ScanTurns(context.Background(), scope, 10, upper, 2)
	require.NoError(t, err)
	require.Len(t, turns, 2)
	require.Equal(t, int64(49), next)
	_, _, err = scanner.ScanTurns(context.Background(), scope, 10, upper, 0)
	require.Error(t, err)
	require.NoError(t, sql.ExpectationsWereMet())
}
