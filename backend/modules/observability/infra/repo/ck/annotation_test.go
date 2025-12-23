// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package ck

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	repodao "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
)

func newInsertAnnotationDao(t *testing.T, failUntil int) (*AnnotationCkDaoImpl, func(), *int) {
	t.Helper()
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)

	db, err := gorm.Open(clickhouse.New(clickhouse.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	count := 0
	_ = db.Callback().Create().Replace("gorm:create", func(tx *gorm.DB) {
		count++
		if count <= failUntil {
			tx.Error = errors.New("insert error")
			return
		}
	})

	provider := &mockCkProvider{db: db.Session(&gorm.Session{DryRun: true})}
	return &AnnotationCkDaoImpl{db: provider}, func() {
		_ = sqlDB.Close()
	}, &count
}

func newAnnotationDao(t *testing.T) (*AnnotationCkDaoImpl, sqlmock.Sqlmock, *gorm.DB, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	db, err := gorm.Open(clickhouse.New(clickhouse.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)

	provider := &mockCkProvider{db: db}
	return &AnnotationCkDaoImpl{db: provider}, mock, db, func() {
		_ = sqlDB.Close()
	}
}

func TestAnnotationCkDaoImpl_Insert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil params", func(t *testing.T) {
		dao := &AnnotationCkDaoImpl{}
		err := dao.Insert(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("empty annotations", func(t *testing.T) {
		dao := &AnnotationCkDaoImpl{}
		err := dao.Insert(ctx, &repodao.InsertAnnotationParam{})
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		dao, cleanup, calls := newInsertAnnotationDao(t, 0)
		defer cleanup()
		annotation := &repodao.Annotation{ID: "anno-1"}

		err := dao.Insert(ctx, &repodao.InsertAnnotationParam{
			Table:       "observability_annotations",
			Annotations: []*repodao.Annotation{annotation},
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, *calls)
	})

	t.Run("retry failed", func(t *testing.T) {
		dao, cleanup, calls := newInsertAnnotationDao(t, 3)
		defer cleanup()
		annotation := &repodao.Annotation{ID: "anno-2"}

		err := dao.Insert(ctx, &repodao.InsertAnnotationParam{
			Table:       "observability_annotations",
			Annotations: []*repodao.Annotation{annotation},
		})
		assert.Error(t, err)
		assert.Equal(t, 3, *calls)
	})
}

func TestAnnotationCkDaoImpl_Get(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("invalid params", func(t *testing.T) {
		dao := &AnnotationCkDaoImpl{}
		_, err := dao.Get(ctx, &repodao.GetAnnotationParam{})
		assert.Error(t, err)
	})

	t.Run("build sql error", func(t *testing.T) {
		dao, _, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		_, err := dao.Get(ctx, &repodao.GetAnnotationParam{
			ID:        "anno-1",
			StartTime: 1,
			EndTime:   2,
		})
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		dao, mock, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		rows := sqlmock.NewRows([]string{"id", "span_id"}).AddRow("anno-1", "span-1")
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		anno, err := dao.Get(ctx, &repodao.GetAnnotationParam{
			ID:        "anno-1",
			Tables:    []string{"observability_annotations"},
			StartTime: 1,
			EndTime:   2,
		})
		assert.NoError(t, err)
		assert.NotNil(t, anno)
		assert.Equal(t, "anno-1", anno.ID)
		assert.Equal(t, "span-1", anno.SpanID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("multiple results returns first", func(t *testing.T) {
		dao, mock, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		rows := sqlmock.NewRows([]string{"id", "span_id"}).
			AddRow("anno-1", "span-1").
			AddRow("anno-2", "span-2")
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		anno, err := dao.Get(ctx, &repodao.GetAnnotationParam{
			ID:        "anno-1",
			Tables:    []string{"observability_annotations"},
			StartTime: 1,
			EndTime:   2,
		})
		assert.NoError(t, err)
		assert.Equal(t, "anno-1", anno.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		dao, mock, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("db error"))

		_, err := dao.Get(ctx, &repodao.GetAnnotationParam{
			ID:        "anno-1",
			Tables:    []string{"observability_annotations"},
			StartTime: 1,
			EndTime:   2,
		})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestAnnotationCkDaoImpl_List(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("nil params", func(t *testing.T) {
		dao := &AnnotationCkDaoImpl{}
		annos, err := dao.List(ctx, nil)
		assert.NoError(t, err)
		assert.Nil(t, annos)
	})

	t.Run("empty span ids", func(t *testing.T) {
		dao := &AnnotationCkDaoImpl{}
		annos, err := dao.List(ctx, &repodao.ListAnnotationsParam{})
		assert.NoError(t, err)
		assert.Nil(t, annos)
	})

	t.Run("success", func(t *testing.T) {
		dao, mock, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		rows := sqlmock.NewRows([]string{"id", "span_id"}).AddRow("anno-1", "span-1")
		mock.ExpectQuery("SELECT").WillReturnRows(rows)

		annos, err := dao.List(ctx, &repodao.ListAnnotationsParam{
			Tables:    []string{"observability_annotations"},
			SpanIDs:   []string{"span-1"},
			StartTime: 1,
			EndTime:   2,
			Limit:     10,
		})
		assert.NoError(t, err)
		require.Len(t, annos, 1)
		assert.Equal(t, "span-1", annos[0].SpanID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error", func(t *testing.T) {
		dao, mock, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		mock.ExpectQuery("SELECT").WillReturnError(fmt.Errorf("db error"))

		_, err := dao.List(ctx, &repodao.ListAnnotationsParam{
			Tables:    []string{"observability_annotations"},
			SpanIDs:   []string{"span-1"},
			StartTime: 1,
			EndTime:   2,
			Limit:     10,
		})
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("build sql error", func(t *testing.T) {
		dao, _, _, cleanup := newAnnotationDao(t)
		defer cleanup()

		_, err := dao.List(ctx, &repodao.ListAnnotationsParam{
			SpanIDs:   []string{"span-1"},
			StartTime: 1,
			EndTime:   2,
		})
		assert.Error(t, err)
	})
}

func TestAnnotationCkDaoImpl_buildSingleSql(t *testing.T) {
	t.Parallel()

	dao, _, db, cleanup := newAnnotationDao(t)
	defer cleanup()

	ctx := context.Background()
	baseSession := dao.db.NewSession(ctx)
	require.NotNil(t, baseSession)

	testCases := []struct {
		name   string
		param  *annoSqlParam
		assert func(t *testing.T, sql string)
	}{
		{
			name: "with id filter",
			param: &annoSqlParam{
				Tables:    []string{"observability_annotations"},
				ID:        "anno-1",
				StartTime: 1,
				EndTime:   2,
				Limit:     10,
			},
			assert: func(t *testing.T, sql string) {
				assert.Contains(t, sql, "FROM `observability_annotations`")
				assert.Contains(t, sql, "id = 'anno-1'")
				assert.Contains(t, sql, "ORDER BY `created_at`")
				assert.Contains(t, sql, "LIMIT 10")
			},
		},
		{
			name: "with span ids and desc",
			param: &annoSqlParam{
				Tables:          []string{"observability_annotations"},
				SpanIDs:         []string{"span-1"},
				StartTime:       10,
				EndTime:         20,
				Limit:           5,
				DescByUpdatedAt: true,
			},
			assert: func(t *testing.T, sql string) {
				assert.Contains(t, sql, "span_id IN ('span-1')")
				assert.Contains(t, sql, "ORDER BY `updated_at` DESC")
				assert.Contains(t, sql, "LIMIT 5")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			session := baseSession.Session(&gorm.Session{DryRun: true})
			query, err := dao.buildSingleSql(ctx, session, tc.param.Tables[0], tc.param)
			require.NoError(t, err)
			sql := query.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return tx.Find([]*model.ObservabilityAnnotation{})
			})
			tc.assert(t, sql)
		})
	}

	_ = db // silence unused (db kept for completeness)
}

func TestAnnotationCkDaoImpl_buildSql(t *testing.T) {
	t.Parallel()

	dao, _, _, cleanup := newAnnotationDao(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("no tables", func(t *testing.T) {
		_, err := dao.buildSql(ctx, &annoSqlParam{})
		assert.Error(t, err)
	})

	t.Run("single table", func(t *testing.T) {
		result, err := dao.buildSql(ctx, &annoSqlParam{
			Tables:    []string{"observability_annotations"},
			StartTime: 1,
			EndTime:   2,
			Limit:     3,
		})
		assert.NoError(t, err)
		sql := result.Statement.SQL.String()
		assert.Contains(t, sql, "FROM `observability_annotations`")
		assert.Contains(t, sql, "LIMIT 3")
		assert.Contains(t, sql, "SETTINGS final = 1")
	})

	t.Run("multiple tables", func(t *testing.T) {
		result, err := dao.buildSql(ctx, &annoSqlParam{
			Tables:          []string{"observability_annotations", "observability_annotations_v2"},
			StartTime:       1,
			EndTime:         2,
			Limit:           5,
			DescByUpdatedAt: true,
		})
		assert.NoError(t, err)
		sql := result.Statement.SQL.String()
		assert.Contains(t, sql, "UNION ALL")
		assert.Contains(t, sql, "ORDER BY updated_at DESC")
		assert.Contains(t, sql, "LIMIT 5")
		assert.Contains(t, sql, "SETTINGS final = 1")
	})
}
