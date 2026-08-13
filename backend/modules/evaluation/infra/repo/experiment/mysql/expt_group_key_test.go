// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
)

// newGroupKeyTestDAO 用 sqlmock(正则匹配器)起一个真实 GORM 连接。
// ExpectQuery 的正则同时充当「SQL 必须长这样」的断言 —— 匹配不上则查询报错、用例失败，
// 因此 LIMIT/OFFSET/ORDER BY 的形状被逐字钉死。
func newGroupKeyTestDAO(t *testing.T) (*exptDAOImpl, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}
	return &exptDAOImpl{query: query.Use(gormDB)}, mock, func() { _ = sqlDB.Close() }
}

func idRows(ids []int64) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id"})
	for _, id := range ids {
		rows.AddRow(id)
	}
	return rows
}

// makeIDs 造 n 个单调递增 id，模拟按主键升序的 DB 返回。
func makeIDs(n int) []int64 {
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, int64(1000+i))
	}
	return ids
}

// TestExptDAO_GetIDsByGroupKey_NoPagination 钉死 R1：不传分页 → 全量返回、total = 返回条数、
// **不发 count 查询**（ExpectationsWereMet 会因多出的期望或未消费的查询而失败），
// 且条数 >100 —— 若有人在 DAO 引入 defaultLimit 兜底（如 :59 的 defaultLimit=20），此用例必红。
func TestExptDAO_GetIDsByGroupKey_NoPagination(t *testing.T) {
	all := makeIDs(137)

	cases := []struct {
		name           string
		page, pageSize int32
	}{
		{"both zero", 0, 0},
		{"page only", 3, 0},
		{"page size only", 0, 10},
		{"negative page", -1, 10},
		{"negative page size", 1, -5},
		{"both negative", -2, -3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dao, mock, cleanup := newGroupKeyTestDAO(t)
			defer cleanup()

			// 只允许一条 SELECT id 查询：没有 COUNT，也没有 LIMIT/OFFSET（正则以 ORDER BY 收尾并锚定 $）。
			mock.ExpectQuery("SELECT `id` FROM `experiment` WHERE `experiment`.`space_id` = \\? AND `experiment`.`experiment_group_key` = \\? AND `experiment`.`deleted_at` IS NULL.* ORDER BY `experiment`.`id`$").
				WithArgs(int64(100), "g1").
				WillReturnRows(idRows(all))

			ids, total, err := dao.GetIDsByGroupKey(context.Background(), 100, "g1", tc.page, tc.pageSize)
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
			assert.Equal(t, all, ids)
			assert.Greater(t, len(ids), 100, "不传分页必须返回全量，不得被截断")
			assert.EqualValues(t, len(all), total, "非分页路径 total = 返回条数")
		})
	}
}

// TestExptDAO_GetIDsByGroupKey_Paginated 钉死分页路径：先 COUNT 得全量 total，
// 再带 ORDER BY id + LIMIT/OFFSET 取当页；total 是全量数而非当页条数。
// LIMIT/OFFSET 是占位符，故用 WithArgs 钉死实际下推的数值（offset 必须是 (page-1)*pageSize）。
func TestExptDAO_GetIDsByGroupKey_Paginated(t *testing.T) {
	dao, mock, cleanup := newGroupKeyTestDAO(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `experiment` WHERE `experiment`.`space_id` = \\? AND `experiment`.`experiment_group_key` = \\? AND `experiment`.`deleted_at` IS NULL").
		WithArgs(int64(100), "g1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(57)))
	mock.ExpectQuery("SELECT `id` FROM `experiment` WHERE .+ ORDER BY `experiment`.`id` LIMIT \\? OFFSET \\?").
		WithArgs(int64(100), "g1", 3, 6).
		WillReturnRows(idRows([]int64{1006, 1007, 1008}))

	ids, total, err := dao.GetIDsByGroupKey(context.Background(), 100, "g1", 3, 3)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, []int64{1006, 1007, 1008}, ids)
	assert.EqualValues(t, 57, total, "分页路径 total 必须是全量数，不是当页条数")
}

// TestExptDAO_GetIDsByGroupKey_SequentialPagesCoverAll 钉死 R8/翻页稳定性：
// 顺序翻完所有页 → 并集 == 全集、无重复。WithArgs 钉死每页下推的 limit/offset，
// 若 DAO 偏移算错（例如忘了 -1）则 args 匹配不上、用例失败。
func TestExptDAO_GetIDsByGroupKey_SequentialPagesCoverAll(t *testing.T) {
	all := makeIDs(11)
	const pageSize = int32(4)

	var union []int64
	seen := make(map[int64]struct{}, len(all))

	for page := int32(1); ; page++ {
		offset := int((page - 1) * pageSize)
		if offset >= len(all) {
			break
		}
		end := offset + int(pageSize)
		if end > len(all) {
			end = len(all)
		}
		want := all[offset:end]

		dao, mock, cleanup := newGroupKeyTestDAO(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `experiment`").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(len(all))))
		listQ := mock.ExpectQuery("SELECT `id` FROM `experiment` WHERE .+ ORDER BY `experiment`.`id` LIMIT \\?")
		if offset > 0 {
			// gorm 在 offset=0 时不会输出 OFFSET 子句，故仅在 >0 时才有第 4 个 arg。
			listQ = listQ.WithArgs(int64(100), "g1", int(pageSize), offset)
		} else {
			listQ = listQ.WithArgs(int64(100), "g1", int(pageSize))
		}
		listQ.WillReturnRows(idRows(want))

		ids, total, err := dao.GetIDsByGroupKey(context.Background(), 100, "g1", page, pageSize)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.EqualValues(t, len(all), total)
		for _, id := range ids {
			_, dup := seen[id]
			assert.False(t, dup, "翻页出现重复 id: %d", id)
			seen[id] = struct{}{}
		}
		union = append(union, ids...)
		cleanup()
	}

	assert.Equal(t, all, union, "顺序翻页并集必须等于全集且保持升序")
}

// TestExptDAO_GetIDsByGroupKey_SamePageStable 同一页重复请求 → 结果与顺序一致（ORDER BY id 保证）。
func TestExptDAO_GetIDsByGroupKey_SamePageStable(t *testing.T) {
	page := []int64{1004, 1005, 1006, 1007}

	var first []int64
	for i := 0; i < 3; i++ {
		dao, mock, cleanup := newGroupKeyTestDAO(t)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `experiment`").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(20)))
		mock.ExpectQuery("SELECT `id` FROM `experiment` WHERE .+ ORDER BY `experiment`.`id` LIMIT \\? OFFSET \\?").
			WithArgs(int64(100), "g1", 4, 4).
			WillReturnRows(idRows(page))

		ids, total, err := dao.GetIDsByGroupKey(context.Background(), 100, "g1", 2, 4)
		require.NoError(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		assert.EqualValues(t, 20, total)
		if i == 0 {
			first = ids
		} else {
			assert.Equal(t, first, ids, "同页重复请求结果与顺序必须一致")
		}
		cleanup()
	}
}

// TestExptDAO_GetIDsByGroupKey_CountError 分页路径 count 失败 → 直接返回错误，不再发列表查询。
func TestExptDAO_GetIDsByGroupKey_CountError(t *testing.T) {
	dao, mock, cleanup := newGroupKeyTestDAO(t)
	defer cleanup()

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `experiment`").WillReturnError(errors.New("count boom"))

	ids, total, err := dao.GetIDsByGroupKey(context.Background(), 100, "g1", 1, 10)
	assert.Error(t, err)
	assert.Nil(t, ids)
	assert.EqualValues(t, 0, total)
}

// TestExptDAO_GetIDsByGroupKey_PluckError 列表查询失败 → 返回错误。
func TestExptDAO_GetIDsByGroupKey_PluckError(t *testing.T) {
	dao, mock, cleanup := newGroupKeyTestDAO(t)
	defer cleanup()

	mock.ExpectQuery("SELECT `id` FROM `experiment`").WillReturnError(errors.New("pluck boom"))

	ids, total, err := dao.GetIDsByGroupKey(context.Background(), 100, "g1", 0, 0)
	assert.Error(t, err)
	assert.Nil(t, ids)
	assert.EqualValues(t, 0, total)
}
