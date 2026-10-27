// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"

	"github.com/coze-dev/coze-loop/backend/infra/db"
)

func TestListEvaluatorDescription(t *testing.T) {
	ctx := context.Background()
	provider, err := db.NewDB(sqlite.Open(":memory:"))
	require.NoError(t, err)
	session := provider.NewSession(ctx)
	sqlDB, err := session.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	// Enforce case-sensitive LIKE so the query, not the database default, ignores case.
	require.NoError(t, session.Exec("PRAGMA case_sensitive_like = ON").Error)
	require.NoError(t, session.Exec(`CREATE TABLE evaluator (
        id INTEGER PRIMARY KEY, space_id INTEGER, name TEXT,
        description TEXT, deleted_at DATETIME
    )`).Error)
	require.NoError(t, session.Exec(`INSERT INTO evaluator VALUES
        (1, 1, 'first', 'prefix Alpha suffix', NULL),
        (2, 1, 'second', 'ALPHA', NULL),
        (3, 1, 'third', 'Beta', NULL),
        (4, 1, 'empty', NULL, NULL),
        (5, 1, 'deleted', 'alpha', '2026-01-01'),
        (6, 2, 'other-space', 'alpha', NULL)
    `).Error)
	dao := &EvaluatorDAOImpl{provider: provider}

	for _, tc := range []struct {
		name        string
		description string
		searchName  string
		pageSize    int32
		pageNum     int32
		wantTotal   int64
		wantIDs     []int64
	}{
		{name: "mixed case", description: "aLpHa", wantTotal: 2, wantIDs: []int64{1, 2}},
		{name: "substring", description: "LPH", wantTotal: 2, wantIDs: []int64{1, 2}},
		{name: "different description", description: "beta", wantTotal: 1, wantIDs: []int64{3}},
		{name: "no matches", description: "missing", wantIDs: []int64{}},
		{name: "clear description", wantTotal: 4, wantIDs: []int64{1, 2, 3, 4}},
		{name: "name and description", description: "alpha", searchName: "first", wantTotal: 1, wantIDs: []int64{1}},
		{name: "pagination preserves count", description: "alpha", pageSize: 1, pageNum: 2, wantTotal: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &ListEvaluatorRequest{SpaceID: 1, SearchName: tc.searchName, SearchDescription: tc.description, PageSize: tc.pageSize, PageNum: tc.pageNum}
			result, err := dao.ListEvaluator(ctx, req)
			require.NoError(t, err)
			require.Equal(t, tc.wantTotal, result.TotalCount)
			if tc.pageSize != 0 {
				require.Len(t, result.Evaluators, int(tc.pageSize))
				return
			}
			ids := make([]int64, 0, len(result.Evaluators))
			for _, item := range result.Evaluators {
				ids = append(ids, item.ID)
			}
			require.ElementsMatch(t, tc.wantIDs, ids)
		})
	}
}
