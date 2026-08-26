// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	dbmock "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
)

// capturingMatcher 记录所有实际执行的 SQL，并对任意期望都放行（返回 nil）。
type capturingMatcher struct{ captured *[]string }

func (m capturingMatcher) Match(expectedSQL, actualSQL string) error {
	*m.captured = append(*m.captured, actualSQL)
	return nil
}

// TestEvaluatorDAOImpl_ListEvaluator_SearchDescription 断言 SearchDescription 被翻译为
// description LIKE 条件，空值时不追加该条件（与既有 SearchName 口径一致）。
func TestEvaluatorDAOImpl_ListEvaluator_SearchDescription(t *testing.T) {
	tests := []struct {
		name         string
		req          *ListEvaluatorRequest
		wantDescLike bool
		wantNameLike bool
	}{
		{
			name:         "有描述关键词_生成 description LIKE",
			req:          &ListEvaluatorRequest{SpaceID: 1, SearchDescription: "ceshi"},
			wantDescLike: true,
		},
		{
			name:         "空描述关键词_不生成 description 条件",
			req:          &ListEvaluatorRequest{SpaceID: 1},
			wantDescLike: false,
		},
		{
			name:         "名称与描述叠加_两个 LIKE 均生成",
			req:          &ListEvaluatorRequest{SpaceID: 1, SearchName: "foo", SearchDescription: "bar"},
			wantDescLike: true,
			wantNameLike: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var captured []string
			sqlDB, mock, err := sqlmock.New(
				sqlmock.QueryMatcherOption(capturingMatcher{captured: &captured}))
			assert.NoError(t, err)
			defer func() { _ = sqlDB.Close() }()

			gormDB, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{})
			assert.NoError(t, err)

			mockProvider := dbmock.NewMockProvider(ctrl)
			mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).AnyTimes()

			// ListEvaluator 先 Count 再 Find，两条查询都放行。
			mock.ExpectQuery(".*").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(".*").WillReturnRows(sqlmock.NewRows([]string{"id"}))

			dao := &EvaluatorDAOImpl{provider: mockProvider}
			_, err = dao.ListEvaluator(context.Background(), tt.req)
			assert.NoError(t, err)

			joined := strings.ToLower(strings.Join(captured, " | "))
			assert.Contains(t, joined, "space_id", "查询必须受 space_id 收敛")

			if tt.wantDescLike {
				assert.Contains(t, joined, "description like", "应生成 description LIKE 条件")
			} else {
				assert.NotContains(t, joined, "description like", "空描述不应生成 description 条件")
			}
			if tt.wantNameLike {
				assert.Contains(t, joined, "name like", "应保留 name LIKE 条件")
			}
		})
	}
}
