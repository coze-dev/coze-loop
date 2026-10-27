// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
)

func TestListEvaluatorDescriptionSearch(t *testing.T) {
	for _, tc := range []struct {
		name, description, searchName string
		creatorIDs                    []int64
		types                         []int32
		pageNum, pageSize             int32
		wantIDs                       []int64
		wantTotal                     int64
	}{
		{name: "mixed case substring", description: "cEsHi", wantIDs: []int64{1, 2}, wantTotal: 2},
		{name: "no match", description: "missing", wantIDs: []int64{}, wantTotal: 0},
		{name: "empty restores list", wantIDs: []int64{1, 2, 3, 4, 7, 8}, wantTotal: 6},
		{name: "combined filters", description: "CESHI", searchName: "alpha", creatorIDs: []int64{7}, types: []int32{1}, wantIDs: []int64{1}, wantTotal: 1},
		{name: "count before pagination", description: "ceshi", pageNum: 2, pageSize: 1, wantIDs: []int64{2}, wantTotal: 2},
		{name: "unicode substring", description: "\u63cf\u8ff0", wantIDs: []int64{8}, wantTotal: 1},
		{name: "SQL remains parameterized", description: "' OR 1=1 --", wantIDs: []int64{}, wantTotal: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := database.DB()
			require.NoError(t, err)
			t.Cleanup(func() { _ = sqlDB.Close() })
			require.NoError(t, database.Exec("PRAGMA case_sensitive_like = ON").Error)
			require.NoError(t, database.Exec(`CREATE TABLE evaluator (
                id INTEGER PRIMARY KEY, space_id INTEGER, name TEXT, description TEXT,
                created_by INTEGER, evaluator_type INTEGER, deleted_at DATETIME, created_at DATETIME
            )`).Error)
			require.NoError(t, database.Exec(`INSERT INTO evaluator VALUES
                (1, 10, 'alpha', 'Prefix CeShI suffix', 7, 1, NULL, '2026-01-01 00:00:01'),
                (2, 10, 'beta', 'ceshi', 8, 2, NULL, '2026-01-01 00:00:02'),
                (3, 10, 'empty', '', 7, 1, NULL, '2026-01-01 00:00:03'),
                (4, 10, 'null', NULL, 7, 1, NULL, '2026-01-01 00:00:04'),
                (5, 11, 'other space', 'ceshi', 7, 1, NULL, '2026-01-01 00:00:05'),
                (6, 10, 'deleted', 'ceshi', 7, 1, '2026-01-01 00:00:06', '2026-01-01 00:00:06'),
                (7, 10, 'CeShi', 'unrelated', 7, 1, NULL, '2026-01-01 00:00:07'),
                (8, 10, 'unicode', ?, 7, 1, NULL, '2026-01-01 00:00:08')`, "\u4e2d\u6587\u63cf\u8ff0").Error)
			provider := dbmock.NewMockProvider(gomock.NewController(t))
			provider.EXPECT().NewSession(gomock.Any()).Return(database)
			dao := &EvaluatorDAOImpl{provider: provider}
			req := &ListEvaluatorRequest{
				SpaceID: 10, SearchDescription: tc.description, SearchName: tc.searchName, CreatorIDs: tc.creatorIDs,
				EvaluatorType: tc.types, PageNum: tc.pageNum, PageSize: tc.pageSize,
				OrderBy: []*OrderBy{{Field: "created_at"}},
			}
			result, err := dao.ListEvaluator(context.Background(), req)
			require.NoError(t, err)
			ids := make([]int64, 0, len(result.Evaluators))
			for _, row := range result.Evaluators {
				ids = append(ids, row.ID)
			}
			require.Equal(t, tc.wantTotal, result.TotalCount)
			require.Equal(t, tc.wantIDs, ids)
		})
	}
}
