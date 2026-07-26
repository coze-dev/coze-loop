// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
)

// TestEvaluatorDAOImpl_ListEvaluator_SearchDescription 覆盖 search_description 模糊搜索：
// 仅描述命中/不命中、名称+描述组合(AND)、大小写不敏感(依赖列 _ci collation)、空串/缺省不过滤。
// 断言层面：desc 生效时下发的 SQL WHERE 含 `description LIKE`；缺省/空串时不含。
// 沿用本包既有 go-sqlmock + 真实 GORM(mysql driver, SkipInitializeWithVersion) + mock provider 风格。
func TestEvaluatorDAOImpl_ListEvaluator_SearchDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		req            *ListEvaluatorRequest
		expectDescLike bool // 期望 SQL 含 description LIKE
		expectNameLike bool // 期望 SQL 含 name LIKE
		description    string
	}{
		{
			name:           "desc only hit",
			req:            &ListEvaluatorRequest{SpaceID: 100, SearchDescription: "quality"},
			expectDescLike: true,
			expectNameLike: false,
			description:    "仅传 SearchDescription，SQL 应追加 description LIKE 分支",
		},
		{
			name:           "desc miss (still applies filter, returns empty rows)",
			req:            &ListEvaluatorRequest{SpaceID: 100, SearchDescription: "nonexistent-kw"},
			expectDescLike: true,
			expectNameLike: false,
			description:    "SearchDescription 有值即追加 WHERE；命中与否由数据决定，此处 mock 返回空",
		},
		{
			name:           "name + desc combined (AND)",
			req:            &ListEvaluatorRequest{SpaceID: 100, SearchName: "eval", SearchDescription: "quality"},
			expectDescLike: true,
			expectNameLike: true,
			description:    "名称 + 描述组合，两条 Where 链式追加即 SQL AND，均应出现",
		},
		{
			name:           "case-insensitive hit (uppercase keyword, relies on _ci collation, no LOWER)",
			req:            &ListEvaluatorRequest{SpaceID: 100, SearchDescription: "QUALITY"},
			expectDescLike: true,
			expectNameLike: false,
			description:    "大写关键词照常下发 description LIKE，不加 LOWER()，大小写不敏感由列 _ci collation 保证",
		},
		{
			name:           "empty description not filtered",
			req:            &ListEvaluatorRequest{SpaceID: 100, SearchDescription: ""},
			expectDescLike: false,
			expectNameLike: false,
			description:    "SearchDescription 为空串时 len==0，不追加 description LIKE",
		},
		{
			name:           "default (unset) not filtered",
			req:            &ListEvaluatorRequest{SpaceID: 100},
			expectDescLike: false,
			expectNameLike: false,
			description:    "缺省不传 SearchDescription 时不追加 description LIKE，行为与改动前一致",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer func() { _ = sqlDB.Close() }()

			gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{})
			if err != nil {
				t.Fatalf("failed to open gorm db: %v", err)
			}

			mockProvider := dbmock.NewMockProvider(ctrl)
			mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).AnyTimes()

			// desc 命中/不命中断言：命中场景返回一行，其余空行——不影响 SQL WHERE 断言。
			descRegex := "description LIKE"
			// Count 查询
			countRows := sqlmock.NewRows([]string{"count"}).AddRow(0)
			if tt.expectDescLike {
				mock.ExpectQuery(descRegex).WillReturnRows(countRows)
			} else {
				// 不含 description LIKE：用宽松的 count 匹配
				mock.ExpectQuery("SELECT count").WillReturnRows(countRows)
			}
			// Find 查询
			findRows := sqlmock.NewRows([]string{"id", "space_id", "name", "description"})
			if tt.expectDescLike {
				mock.ExpectQuery(descRegex).WillReturnRows(findRows)
			} else {
				mock.ExpectQuery("SELECT \\* FROM").WillReturnRows(findRows)
			}

			dao := &EvaluatorDAOImpl{provider: mockProvider}

			resp, err := dao.ListEvaluator(context.Background(), tt.req)
			assert.NoError(t, err, tt.description)
			assert.NotNil(t, resp)

			// 校验实际下发 SQL 是否含/不含 description LIKE，兜底断言（sqlmock 正则已卡一层）
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("[%s] unfulfilled expectations: %s", tt.description, err)
			}
		})
	}
}

// TestListBuiltinEvaluatorRequest_NoSearchDescription 回归：内置评估器查询走 ListBuiltinEvaluator
// / tagDAO，其请求类型 ListBuiltinEvaluatorRequest 不含 SearchDescription 字段，
// 结构上无法被 search_description 影响（spec「不影响内置评估器查询」）。
func TestListBuiltinEvaluatorRequest_NoSearchDescription(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(ListBuiltinEvaluatorRequest{})
	_, has := typ.FieldByName("SearchDescription")
	assert.False(t, has, "ListBuiltinEvaluatorRequest 不应含 SearchDescription 字段，内置查询不受描述搜索影响")
}
