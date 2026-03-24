// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestExptTemplateDAOImpl_filterNeedJoin(t *testing.T) {
	t.Parallel()

	dao := &exptTemplateDAOImpl{}

	tests := []struct {
		name   string
		filter *entity.ExptTemplateListFilter
		want   bool
	}{
		{
			name:   "nil filter",
			filter: nil,
			want:   false,
		},
		{
			name: "includes evaluator ids",
			filter: &entity.ExptTemplateListFilter{
				Includes: &entity.ExptTemplateFilterFields{EvaluatorIDs: []int64{1}},
			},
			want: true,
		},
		{
			name: "excludes evaluator ids",
			filter: &entity.ExptTemplateListFilter{
				Excludes: &entity.ExptTemplateFilterFields{EvaluatorIDs: []int64{2}},
			},
			want: true,
		},
		{
			name: "cron activate only",
			filter: &entity.ExptTemplateListFilter{
				Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{1}},
			},
			want: false,
		},
		{
			name: "empty evaluator ids does not join",
			filter: &entity.ExptTemplateListFilter{
				Includes: &entity.ExptTemplateFilterFields{EvaluatorIDs: []int64{}},
				Excludes: &entity.ExptTemplateFilterFields{EvaluatorIDs: []int64{}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, dao.filterNeedJoin(tt.filter))
		})
	}
}

func newDryRunSQL(t *testing.T, conds []func(tx *gorm.DB) *gorm.DB) string {
	t.Helper()

	sqlDB, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer func() { _ = sqlDB.Close() }()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	tx := gormDB.Model(&model.ExptTemplate{})
	for _, cond := range conds {
		tx = cond(tx)
	}
	tx = tx.Find(&[]model.ExptTemplate{})
	return tx.Statement.SQL.String()
}

func TestExptTemplateDAOImpl_toConditions_CronActivateAndValidation(t *testing.T) {
	t.Parallel()

	dao := &exptTemplateDAOImpl{}

	t.Run("invalid filter returns false", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{2}},
		}, nil)
		assert.False(t, ok)
		assert.Nil(t, conds)
	})

	t.Run("cron activate include and exclude without join", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			FuzzyName: "nightly",
			Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{1}},
			Excludes: &entity.ExptTemplateFilterFields{CronActivate: []int64{0}},
		}, nil)
		assert.True(t, ok)
		assert.NotEmpty(t, conds)
		assert.False(t, dao.filterNeedJoin(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{1}},
		}))
		assert.Len(t, conds, 4)
	})

	t.Run("cron activate with evaluator join and explicit order", func(t *testing.T) {
		field := "created_at"
		isAsc := true
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				CronActivate: []int64{1},
				EvaluatorIDs: []int64{1001},
			},
		}, []*entity.OrderBy{{Field: &field, IsAsc: &isAsc}})
		assert.True(t, ok)
		assert.Len(t, conds, 3)
	})

	t.Run("mixed invalid cron activate in includes returns false", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{0, 2}},
		}, nil)
		assert.False(t, ok)
		assert.Nil(t, conds)
	})

	t.Run("nil filter with explicit order keeps only order condition", func(t *testing.T) {
		field := "updated_at"
		conds, ok := dao.toConditions(nil, []*entity.OrderBy{{Field: &field}})
		assert.True(t, ok)
		assert.Len(t, conds, 1)
	})

	t.Run("fuzzy name only adds fuzzy and default order", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{FuzzyName: "nightly"}, nil)
		assert.True(t, ok)
		assert.Len(t, conds, 2)
	})

	t.Run("empty order field falls back to default order", func(t *testing.T) {
		empty := ""
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{0}},
		}, []*entity.OrderBy{{Field: &empty}})
		assert.True(t, ok)
		assert.Len(t, conds, 2)
	})

	t.Run("invalid excludes still returns conditions", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{1}},
			Excludes: &entity.ExptTemplateFilterFields{CronActivate: []int64{2}},
		}, nil)
		assert.True(t, ok)
		assert.Len(t, conds, 3)
	})

	t.Run("fuzzy name with evaluator join keeps joined conditions", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			FuzzyName: "nightly",
			Includes:  &entity.ExptTemplateFilterFields{EvaluatorIDs: []int64{1001}},
		}, nil)
		assert.True(t, ok)
		assert.Len(t, conds, 3)
	})

	t.Run("multiple explicit orders skip default order", func(t *testing.T) {
		createdAt := "created_at"
		updatedAt := "updated_at"
		isAsc := true
		conds, ok := dao.toConditions(nil, []*entity.OrderBy{{Field: &createdAt, IsAsc: &isAsc}, {Field: &updatedAt}})
		assert.True(t, ok)
		assert.Len(t, conds, 2)
	})

	t.Run("sql contains include and exclude field comparators", func(t *testing.T) {
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{
				CreatedBy:  []string{"u1"},
				UpdatedBy:  []string{"u2"},
				TargetIDs:  []int64{1},
				EvalSetIDs: []int64{2},
				TargetType: []int64{3},
				ExptType:   []int64{4},
			},
			Excludes: &entity.ExptTemplateFilterFields{
				CreatedBy:  []string{"x1"},
				UpdatedBy:  []string{"x2"},
				TargetIDs:  []int64{5},
				EvalSetIDs: []int64{6},
				TargetType: []int64{7},
				ExptType:   []int64{8},
			},
		}, nil)
		assert.True(t, ok)
		assert.Len(t, conds, 13)
		sql := newDryRunSQL(t, conds)
		assert.Contains(t, sql, "created_by IN")
		assert.Contains(t, sql, "updated_by IN")
		assert.Contains(t, sql, "target_id IN")
		assert.Contains(t, sql, "eval_set_id IN")
		assert.Contains(t, sql, "target_type IN")
		assert.Contains(t, sql, "expt_type IN")
		assert.Contains(t, sql, "created_by NOT IN")
		assert.Contains(t, sql, "updated_by NOT IN")
		assert.Contains(t, sql, "target_id NOT IN")
		assert.Contains(t, sql, "eval_set_id NOT IN")
		assert.Contains(t, sql, "target_type NOT IN")
		assert.Contains(t, sql, "expt_type NOT IN")
		assert.Contains(t, sql, "ORDER BY created_at desc")
	})

	t.Run("sql uses joined prefixes and asc order", func(t *testing.T) {
		field := "created_at"
		isAsc := true
		conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{
			FuzzyName: "nightly",
			Includes: &entity.ExptTemplateFilterFields{
				CreatedBy:    []string{"u1"},
				EvaluatorIDs: []int64{1001},
			},
		}, []*entity.OrderBy{{Field: &field, IsAsc: &isAsc}})
		assert.True(t, ok)
		sql := newDryRunSQL(t, conds)
		assert.Contains(t, sql, "expt_template.name like")
		assert.Contains(t, sql, "expt_template.created_by IN")
		assert.Contains(t, sql, "expt_template_evaluator_ref.evaluator_id IN")
		assert.Contains(t, sql, "ORDER BY expt_template.created_at asc")
	})

	t.Run("invalid include matrices return false", func(t *testing.T) {
		tests := []entity.ExptTemplateFilterFields{
			{CreatedBy: []string{""}},
			{TargetIDs: []int64{-1}},
			{EvalSetIDs: []int64{-1}},
			{EvaluatorIDs: []int64{-1}},
			{TargetType: []int64{-1}},
			{ExptType: []int64{-1}},
		}
		for _, includes := range tests {
			conds, ok := dao.toConditions(&entity.ExptTemplateListFilter{Includes: &includes}, nil)
			assert.False(t, ok)
			assert.Nil(t, conds)
		}
	})
}

func TestExptTemplateDAOImpl_List_InvalidFilterReturnsEmpty(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sqlDB, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)

	provider := dbmock.NewMockProvider(ctrl)
	provider.EXPECT().NewSession(gomock.Any()).Return(gormDB).Times(1)

	dao := &exptTemplateDAOImpl{db: provider}
	templates, count, err := dao.List(context.Background(), 1, 10, &entity.ExptTemplateListFilter{
		Includes: &entity.ExptTemplateFilterFields{CronActivate: []int64{2}},
	}, nil, 100)
	assert.NoError(t, err)
	assert.Empty(t, templates)
	assert.Zero(t, count)
}

func TestExptTemplateDAOImpl_List_SQLShapes(t *testing.T) {
	t.Parallel()

	t.Run("non join query uses default order and limit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sqlDB, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer func() { _ = sqlDB.Close() }()

		gormDB, err := gorm.Open(mysql.New(mysql.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true,
		}), &gorm.Config{})
		assert.NoError(t, err)

		provider := dbmock.NewMockProvider(ctrl)
		provider.EXPECT().NewSession(gomock.Any()).Return(gormDB).Times(1)

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		mock.ExpectQuery("SELECT count\\(\\*\\) FROM `expt_template` WHERE space_id = \\? AND deleted_at IS NULL AND `expt_template`\\.`deleted_at` IS NULL").
			WithArgs(100).
			WillReturnRows(countRows)
		mock.ExpectQuery("SELECT \\* FROM `expt_template` WHERE space_id = \\? AND deleted_at IS NULL AND `expt_template`\\.`deleted_at` IS NULL LIMIT \\?").
			WithArgs(100, 20).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		dao := &exptTemplateDAOImpl{db: provider}
		templates, count, err := dao.List(context.Background(), 0, 0, nil, nil, 100)
		assert.NoError(t, err)
		assert.Empty(t, templates)
		assert.Zero(t, count)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("join query uses group explicit order and paging", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		sqlDB, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer func() { _ = sqlDB.Close() }()

		gormDB, err := gorm.Open(mysql.New(mysql.Config{
			Conn:                      sqlDB,
			SkipInitializeWithVersion: true,
		}), &gorm.Config{})
		assert.NoError(t, err)

		provider := dbmock.NewMockProvider(ctrl)
		provider.EXPECT().NewSession(gomock.Any()).Return(gormDB).Times(1)

		countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
		countSQL := "SELECT count\\(\\*\\) FROM `expt_template` INNER JOIN expt_template_evaluator_ref ON expt_template.id = expt_template_evaluator_ref.expt_template_id WHERE expt_template.space_id = \\? AND expt_template.deleted_at IS NULL AND expt_template_evaluator_ref.evaluator_id IN \\(\\?\\) AND `expt_template`\\.`deleted_at` IS NULL GROUP BY `expt_template`\\.`id` ORDER BY expt_template.created_at asc"
		findSQL := fmt.Sprintf("SELECT `expt_template`\\.`id`,`expt_template`\\.`space_id`,`expt_template`\\.`name`,`expt_template`\\.`description`,`expt_template`\\.`eval_set_id`,`expt_template`\\.`eval_set_version_id`,`expt_template`\\.`target_id`,`expt_template`\\.`target_type`,`expt_template`\\.`target_version_id`,`expt_template`\\.`expt_type`,`expt_template`\\.`cron_activate`,`expt_template`\\.`template_conf`,`expt_template`\\.`expt_info`,`expt_template`\\.`created_by`,`expt_template`\\.`updated_by`,`expt_template`\\.`created_at`,`expt_template`\\.`updated_at`,`expt_template`\\.`deleted_at` FROM `expt_template` INNER JOIN expt_template_evaluator_ref ON expt_template.id = expt_template_evaluator_ref.expt_template_id WHERE expt_template.space_id = \\? AND expt_template.deleted_at IS NULL AND expt_template_evaluator_ref.evaluator_id IN \\(\\?\\) AND `expt_template`\\.`deleted_at` IS NULL GROUP BY `expt_template`\\.`id` ORDER BY expt_template.created_at asc LIMIT \\? OFFSET \\?")
		mock.ExpectQuery(countSQL).
			WithArgs(100, 1001).
			WillReturnRows(countRows)
		mock.ExpectQuery(findSQL).
			WithArgs(100, 1001, 5, 5).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))

		field := "created_at"
		isAsc := true
		dao := &exptTemplateDAOImpl{db: provider}
		templates, count, err := dao.List(context.Background(), 2, 5, &entity.ExptTemplateListFilter{
			Includes: &entity.ExptTemplateFilterFields{EvaluatorIDs: []int64{1001}},
		}, []*entity.OrderBy{{Field: &field, IsAsc: &isAsc}}, 100)
		assert.NoError(t, err)
		assert.Empty(t, templates)
		assert.Equal(t, int64(1), count)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
