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

// TestEvaluatorDAOImpl_ListEvaluator_SearchDescription 验证 SearchDescription
// 透传到 DAL 后会追加 description LIKE 过滤，并与 name LIKE 以 AND 组合。
func TestEvaluatorDAOImpl_ListEvaluator_SearchDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		searchName        string
		searchDescription string
		wantDescLike      bool
		wantNameLike      bool
	}{
		{
			name:              "only search description",
			searchDescription: "accuracy",
			wantDescLike:      true,
			wantNameLike:      false,
		},
		{
			name:              "search name and description combined",
			searchName:        "judge",
			searchDescription: "quality",
			wantDescLike:      true,
			wantNameLike:      true,
		},
		{
			name:              "empty description omits like",
			searchName:        "judge",
			searchDescription: "",
			wantDescLike:      false,
			wantNameLike:      true,
		},
		{
			name:              "both empty omits both likes",
			searchDescription: "",
			wantDescLike:      false,
			wantNameLike:      false,
		},
		{
			name:              "uppercase description keyword passed as-is without lower",
			searchDescription: "ACCURACY",
			wantDescLike:      true,
			wantNameLike:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 自定义 matcher：捕获实际下发的 SQL，同时放行所有期望以便断言 SQL 片段。
			var capturedQueries []string
			matcher := sqlmock.QueryMatcherFunc(func(expectedSQL, actualSQL string) error {
				capturedQueries = append(capturedQueries, actualSQL)
				return nil
			})

			sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
			if err != nil {
				t.Fatalf("failed to create sqlmock: %v", err)
			}
			defer func() { _ = sqlDB.Close() }()

			gormDB, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{})
			if err != nil {
				t.Fatalf("failed to open gorm db: %v", err)
			}

			mockProvider := dbmock.NewMockProvider(ctrl)
			mockProvider.EXPECT().NewSession(gomock.Any(), gomock.Any()).Return(gormDB).AnyTimes()

			// ListEvaluator 会先 Count 再 Find，均返回空结果。
			countRows := sqlmock.NewRows([]string{"count(*)"}).AddRow(int64(0))
			findRows := sqlmock.NewRows([]string{"id"})
			mock.ExpectQuery("").WillReturnRows(countRows)
			mock.ExpectQuery("").WillReturnRows(findRows)

			dao := &EvaluatorDAOImpl{provider: mockProvider}
			resp, err := dao.ListEvaluator(context.Background(), &ListEvaluatorRequest{
				SpaceID:           100,
				SearchName:        tt.searchName,
				SearchDescription: tt.searchDescription,
				PageSize:          0,
				PageNum:           0,
			})
			assert.NoError(t, err)
			assert.NotNil(t, resp)

			joined := strings.Join(capturedQueries, "\n")

			assert.Equal(t, tt.wantDescLike, strings.Contains(joined, "description LIKE"),
				"description LIKE presence mismatch, sql:\n%s", joined)
			assert.Equal(t, tt.wantNameLike, strings.Contains(joined, "name LIKE"),
				"name LIKE presence mismatch, sql:\n%s", joined)

			// 大写关键词应原样下发，不应被 LOWER() 包裹。
			if tt.searchDescription != "" {
				assert.NotContains(t, joined, "LOWER(", "should not wrap description with LOWER()")
			}

			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
