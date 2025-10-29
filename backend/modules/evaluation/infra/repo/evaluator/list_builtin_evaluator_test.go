// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/mocks"
)

func TestEvaluatorRepoImpl_ListBuiltinEvaluator(t *testing.T) {
	tests := []struct {
		name          string
		request       *repo.ListBuiltinEvaluatorRequest
		mockDaoResult *mysql.ListEvaluatorResponse
		mockDaoError  error
		mockTagResult []*model.EvaluatorTag
		mockTagError  error
		expectedError bool
		expectedCount int64
	}{
		{
			name: "successful query without filters",
			request: &repo.ListBuiltinEvaluatorRequest{
				SpaceID:        123,
				FilterOption:   nil,
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockDaoResult: &mysql.ListEvaluatorResponse{
				TotalCount: 2,
				Evaluators: []*model.Evaluator{
					{ID: 1, Name: gptr.Of("test1")},
					{ID: 2, Name: gptr.Of("test2")},
				},
			},
			mockDaoError: nil,
			mockTagResult: []*model.EvaluatorTag{
				{SourceID: 1, TagKey: "type", TagValue: "builtin"},
				{SourceID: 2, TagKey: "type", TagValue: "custom"},
			},
			mockTagError:  nil,
			expectedError: false,
			expectedCount: 2,
		},
		{
			name: "successful query with tags",
			request: &repo.ListBuiltinEvaluatorRequest{
				SpaceID:        123,
				FilterOption:   nil,
				PageSize:       10,
				PageNum:        1,
				IncludeDeleted: false,
			},
			mockDaoResult: &mysql.ListEvaluatorResponse{
				TotalCount: 1,
				Evaluators: []*model.Evaluator{
					{ID: 1, Name: gptr.Of("test1")},
				},
			},
			mockDaoError: nil,
			mockTagResult: []*model.EvaluatorTag{
				{SourceID: 1, TagKey: "type", TagValue: "builtin"},
				{SourceID: 1, TagKey: "category", TagValue: "performance"},
			},
			mockTagError:  nil,
			expectedError: false,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建mock controller
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 创建mock DAOs
			mockEvaluatorDao := mocks.NewMockEvaluatorDAO(ctrl)
			mockEvaluatorVersionDao := mocks.NewMockEvaluatorVersionDAO(ctrl)
			mockTagDao := mocks.NewMockEvaluatorTagDAO(ctrl)

			// 设置evaluatorDao的期望
			if tt.mockDaoResult != nil {
				mockEvaluatorDao.EXPECT().ListBuiltinEvaluator(gomock.Any(), gomock.Any()).Return(tt.mockDaoResult, tt.mockDaoError)
			}

			// 设置tagDAO的期望 - 使用批量查询
			if tt.mockDaoResult != nil && len(tt.mockDaoResult.Evaluators) > 0 {
				// 收集所有evaluator的ID
				evaluatorIDs := make([]int64, 0, len(tt.mockDaoResult.Evaluators))
				for _, evaluator := range tt.mockDaoResult.Evaluators {
					evaluatorIDs = append(evaluatorIDs, evaluator.ID)
				}

				mockTagDao.EXPECT().BatchGetTagsBySourceIDsAndType(
					gomock.Any(),
					evaluatorIDs,
					int32(entity.EvaluatorTagKeyType_Evaluator),
				).Return(tt.mockTagResult, tt.mockTagError).AnyTimes()
			}

			// 创建EvaluatorRepoImpl实例
			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDao,
				evaluatorVersionDao: mockEvaluatorVersionDao,
				tagDAO:              mockTagDao,
			}

			// 调用方法
			result, err := repo.ListBuiltinEvaluator(context.Background(), tt.request)

			// 验证结果
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedCount, result.TotalCount)
			}
		})
	}
}
