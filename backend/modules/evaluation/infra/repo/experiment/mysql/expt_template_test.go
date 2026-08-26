// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestExptTemplateDAOImpl_toConditions_OrderByColumnWhitelist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		field     string
		wantOrder string
	}{
		{
			name:      "合法列按请求排序",
			field:     "updated_at",
			wantOrder: "ORDER BY updated_at desc",
		},
		{
			name:      "注入 payload 被忽略，回落默认排序",
			field:     "(SELECT UPDATEXML(1,CONCAT(0x7e,database(),0x7e),1))",
			wantOrder: "ORDER BY created_at desc",
		},
		{
			name:      "未知列被忽略，回落默认排序",
			field:     "not_a_column",
			wantOrder: "ORDER BY created_at desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sqlDB, _, err := sqlmock.New()
			assert.NoError(t, err)
			defer func() { _ = sqlDB.Close() }()

			gormDB, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{DryRun: true})
			assert.NoError(t, err)

			d := &exptTemplateDAOImpl{}
			conds, ok := d.toConditions(nil, []*entity.OrderBy{{Field: gptr.Of(tt.field)}})
			assert.True(t, ok)

			tx := gormDB.Model(&model.ExptTemplate{})
			for _, cond := range conds {
				tx = cond(tx)
			}
			var templates []*model.ExptTemplate
			tx.Find(&templates)

			sql := tx.Statement.SQL.String()
			assert.Contains(t, sql, tt.wantOrder)
			assert.NotContains(t, sql, "UPDATEXML")
		})
	}
}
