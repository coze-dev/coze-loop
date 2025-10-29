// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	dbmocks "github.com/coze-dev/coze-loop/backend/infra/db/mocks"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	platestwritemocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	evaluatormocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/mocks"
)

func TestEvaluatorRepoImpl_SubmitEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name          string
		evaluator     *entity.Evaluator
		mockSetup     func()
		expectedError error
	}{
		{
			name: "成功提交评估器版本",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						// 创建一个模拟的 gorm.DB 实例
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置更新评估器最新版本的期望
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorLatestVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 设置创建评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					CreateEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "更新评估器最新版本失败",
			evaluator: &entity.Evaluator{
				ID: 1,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						// 创建一个模拟的 gorm.DB 实例
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置更新评估器最新版本的期望
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorLatestVersion(gomock.Any(), int64(1), "1.0.0", "test_user", gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置 mock 期望
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			// 执行测试
			err := repo.SubmitEvaluatorVersion(context.Background(), tt.evaluator)

			// 验证结果
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEvaluatorRepoImpl_UpdateEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name          string
		evaluator     *entity.Evaluator
		mockSetup     func()
		expectedError error
	}{
		{
			name: "成功更新评估器草稿",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置更新评估器草稿状态的期望
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorDraftSubmitted(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 设置更新评估器草稿的期望
				mockEvaluatorVersionDAO.EXPECT().
					UpdateEvaluatorDraft(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "更新评估器草稿状态失败",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorDraftSubmitted(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			err := repo.UpdateEvaluatorDraft(context.Background(), tt.evaluator)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEvaluatorRepoImpl_BatchGetEvaluatorMetaByID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		ids            []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.Evaluator
		expectedError  error
	}{
		{
			name:           "成功批量获取评估器元数据",
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.Evaluator{
						{
							ID:            1,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test1"),
						},
						{
							ID:            2,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test2"),
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test1",
				},
				{
					ID:            2,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test2",
				},
			},
			expectedError: nil,
		},
		{
			name:           "获取评估器元数据失败",
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.BatchGetEvaluatorMetaByID(context.Background(), tt.ids, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i := range result {
					assert.Equal(t, tt.expectedResult[i].ID, result[i].ID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorType, result[i].EvaluatorType)
					assert.Equal(t, tt.expectedResult[i].Name, result[i].Name)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_BatchGetEvaluatorByVersionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		ids            []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.Evaluator
		expectedError  error
	}{
		{
			name:           "成功批量获取评估器版本",
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				// 设置获取评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            1,
							EvaluatorID:   1,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
						{
							ID:            2,
							EvaluatorID:   2,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
					}, nil)

				// 设置获取评估器的期望
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*model.Evaluator{
						{
							ID:            1,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test1"),
						},
						{
							ID:            2,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test2"),
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test1",
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
				{
					ID:            2,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test2",
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
			},
			expectedError: nil,
		},
		{
			name:           "成功批量获取code评估器版本",
			ids:            []int64{3},
			includeDeleted: false,
			mockSetup: func() {
				// 设置获取评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gomock.Any(), []int64{3}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            3,
							EvaluatorID:   3,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypeCode)),
							Version:       "1.0.0",
						},
					}, nil)

				// 设置获取评估器的期望
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]*model.Evaluator{
						{
							ID:            3,
							EvaluatorType: int32(entity.EvaluatorTypeCode),
							Name:          gptr.Of("code-test"),
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            3,
					EvaluatorType: entity.EvaluatorTypeCode,
					Name:          "code-test",
					CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
						Version: "1.0.0",
					},
				},
			},
			expectedError: nil,
		},
		{
			name:           "获取评估器版本失败",
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gomock.Any(), []int64{1, 2}, false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.BatchGetEvaluatorByVersionID(context.Background(), nil, tt.ids, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i := range result {
					assert.Equal(t, tt.expectedResult[i].ID, result[i].ID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorType, result[i].EvaluatorType)
					assert.Equal(t, tt.expectedResult[i].Name, result[i].Name)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_BatchGetEvaluatorDraftByEvaluatorID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		spaceID        int64
		ids            []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.Evaluator
		expectedError  error
	}{
		{
			name:           "成功批量获取评估器草稿",
			spaceID:        1,
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				// 设置检查写入标志的期望
				mockLWT.EXPECT().
					CheckWriteFlagBySearchParam(gomock.Any(), platestwrite.ResourceTypeEvaluator, "1").
					Return(false)

				// 设置获取评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            1,
							EvaluatorID:   1,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
						{
							ID:            2,
							EvaluatorID:   2,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
					}, nil)

				// 设置获取评估器的期望
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.Evaluator{
						{
							ID:            1,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test1"),
						},
						{
							ID:            2,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test2"),
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test1",
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
				{
					ID:            2,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test2",
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
			},
			expectedError: nil,
		},
		{
			name:           "获取评估器草稿失败",
			spaceID:        1,
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockLWT.EXPECT().
					CheckWriteFlagBySearchParam(gomock.Any(), platestwrite.ResourceTypeEvaluator, "1").
					Return(false)

				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), []int64{1, 2}, false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.BatchGetEvaluatorDraftByEvaluatorID(context.Background(), tt.spaceID, tt.ids, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i := range result {
					assert.Equal(t, tt.expectedResult[i].ID, result[i].ID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorType, result[i].EvaluatorType)
					assert.Equal(t, tt.expectedResult[i].Name, result[i].Name)
					assert.Equal(t, tt.expectedResult[i].PromptEvaluatorVersion.Version, result[i].PromptEvaluatorVersion.Version)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_BatchGetEvaluatorVersionsByEvaluatorIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		evaluatorIDs   []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.Evaluator
		expectedError  error
	}{
		{
			name:           "成功批量获取评估器版本",
			evaluatorIDs:   []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            1,
							EvaluatorID:   1,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
						{
							ID:            2,
							EvaluatorID:   2,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
				{
					ID:            2,
					EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
			},
			expectedError: nil,
		},
		{
			name:           "获取评估器版本失败",
			evaluatorIDs:   []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), []int64{1, 2}, false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.BatchGetEvaluatorVersionsByEvaluatorIDs(context.Background(), tt.evaluatorIDs, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i := range result {
					assert.Equal(t, tt.expectedResult[i].EvaluatorType, result[i].EvaluatorType)
					assert.Equal(t, tt.expectedResult[i].PromptEvaluatorVersion.Version, result[i].PromptEvaluatorVersion.Version)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_ListEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	mockTagDAO := evaluatormocks.NewMockEvaluatorTagDAO(ctrl)
	evaluatorRepo := NewEvaluatorRepo(mockIDGen, mockDBProvider, mockEvaluatorDAO, mockEvaluatorVersionDAO, mockTagDAO, mockLWT)

	tests := []struct {
		name           string
		request        *entity.ListEvaluatorVersionRequest
		mockSetup      func()
		expectedResult *repo.ListEvaluatorVersionResponse
		expectedError  error
	}{
		{
			name: "成功获取评估器版本列表",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 1,
				PageSize:    10,
				PageNum:     1,
				OrderBys: []*entity.OrderBy{
					{
						Field: gptr.Of("updated_at"),
						IsAsc: gptr.Of(false),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					ListEvaluatorVersion(gomock.Any(), &mysql.ListEvaluatorVersionRequest{
						EvaluatorID: 1,
						PageSize:    10,
						PageNum:     1,
						OrderBy: []*mysql.OrderBy{
							{
								Field:  "updated_at",
								ByDesc: true,
							},
						},
					}).
					Return(&mysql.ListEvaluatorVersionResponse{
						TotalCount: 1,
						Versions: []*model.EvaluatorVersion{
							{
								ID:            1,
								EvaluatorID:   1,
								Version:       "1.0.0",
								EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							},
						},
					}, nil)
			},
			expectedResult: &repo.ListEvaluatorVersionResponse{
				TotalCount: 1,
				Versions: []*entity.Evaluator{
					{
						ID:            1,
						EvaluatorType: entity.EvaluatorTypePrompt,
						PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
							Version: "1.0.0",
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "获取评估器版本列表失败",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 1,
				PageSize:    10,
				PageNum:     1,
				OrderBys: []*entity.OrderBy{
					{
						Field: gptr.Of("updated_at"),
						IsAsc: gptr.Of(false),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					ListEvaluatorVersion(gomock.Any(), &mysql.ListEvaluatorVersionRequest{
						EvaluatorID: 1,
						PageSize:    10,
						PageNum:     1,
						OrderBy: []*mysql.OrderBy{
							{
								Field:  "updated_at",
								ByDesc: true,
							},
						},
					}).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			// 这里需要将 entity.ListEvaluatorVersionRequest 转换为 repoeval.ListEvaluatorVersionRequest
			req := &repo.ListEvaluatorVersionRequest{
				EvaluatorID:   tt.request.EvaluatorID,
				QueryVersions: tt.request.QueryVersions,
				PageSize:      tt.request.PageSize,
				PageNum:       tt.request.PageNum,
				OrderBy:       tt.request.OrderBys,
			}
			_, err := evaluatorRepo.ListEvaluatorVersion(context.Background(), req)
			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluatorRepoImpl_CheckVersionExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		evaluatorID    int64
		version        string
		mockSetup      func()
		expectedResult bool
		expectedError  error
	}{
		{
			name:        "版本存在",
			evaluatorID: 1,
			version:     "1.0.0",
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					CheckVersionExist(gomock.Any(), int64(1), "1.0.0").
					Return(true, nil)
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name:        "版本不存在",
			evaluatorID: 1,
			version:     "1.0.0",
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					CheckVersionExist(gomock.Any(), int64(1), "1.0.0").
					Return(false, nil)
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name:        "检查版本失败",
			evaluatorID: 1,
			version:     "1.0.0",
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					CheckVersionExist(gomock.Any(), int64(1), "1.0.0").
					Return(false, assert.AnError)
			},
			expectedResult: false,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.CheckVersionExist(context.Background(), tt.evaluatorID, tt.version)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestEvaluatorRepoImpl_CreateEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		evaluator      *entity.Evaluator
		mockSetup      func()
		expectedResult int64
		expectedError  error
	}{
		{
			name: "成功创建评估器",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置生成ID的期望
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 3).
					Return([]int64{1, 2, 3}, nil)

				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置创建评估器的期望
				mockEvaluatorDAO.EXPECT().
					CreateEvaluator(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 设置创建评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					CreateEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(2)

				// 设置写入标志的期望
				mockLWT.EXPECT().
					SetWriteFlag(gomock.Any(), platestwrite.ResourceTypeEvaluator, int64(1), gomock.Any()).
					Return()
			},
			expectedResult: 1,
			expectedError:  nil,
		},
		{
			name: "生成ID失败",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
				},
			},
			mockSetup: func() {
				mockIDGen.EXPECT().
					GenMultiIDs(gomock.Any(), 3).
					Return(nil, assert.AnError)
			},
			expectedResult: 0,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.CreateEvaluator(context.Background(), tt.evaluator)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestEvaluatorRepoImpl_BatchGetEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		ids            []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.Evaluator
		expectedError  error
	}{
		{
			name:           "成功批量获取评估器草稿",
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				// 设置获取评估器的期望
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.Evaluator{
						{
							ID:            1,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test1"),
						},
						{
							ID:            2,
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
							Name:          gptr.Of("test2"),
						},
					}, nil)

				// 设置获取评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gomock.Any(), []int64{1, 2}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            1,
							EvaluatorID:   1,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
						{
							ID:            2,
							EvaluatorID:   2,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test1",
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
				{
					ID:            2,
					EvaluatorType: entity.EvaluatorTypePrompt,
					Name:          "test2",
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						Version: "1.0.0",
					},
				},
			},
			expectedError: nil,
		},
		{
			name:           "获取评估器失败",
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), []int64{1, 2}, false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.BatchGetEvaluatorDraft(context.Background(), tt.ids, tt.includeDeleted)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i := range result {
					assert.Equal(t, tt.expectedResult[i].ID, result[i].ID)
					assert.Equal(t, tt.expectedResult[i].EvaluatorType, result[i].EvaluatorType)
					assert.Equal(t, tt.expectedResult[i].Name, result[i].Name)
					assert.Equal(t, tt.expectedResult[i].PromptEvaluatorVersion.Version, result[i].PromptEvaluatorVersion.Version)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_UpdateEvaluatorMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name          string
		id            int64
		evaluatorName string
		description   string
		userID        string
		mockSetup     func()
		expectedError error
	}{
		{
			name:          "成功更新评估器元数据",
			id:            1,
			evaluatorName: "test",
			description:   "test description",
			userID:        "test_user",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), &model.Evaluator{
						ID:          1,
						Name:        gptr.Of("test"),
						Description: gptr.Of("test description"),
						UpdatedBy:   "test_user",
					}).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name:          "更新评估器元数据失败",
			id:            1,
			evaluatorName: "test",
			description:   "test description",
			userID:        "test_user",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorMeta(gomock.Any(), &model.Evaluator{
						ID:          1,
						Name:        gptr.Of("test"),
						Description: gptr.Of("test description"),
						UpdatedBy:   "test_user",
					}).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			err := repo.UpdateEvaluatorMeta(context.Background(), tt.id, tt.evaluatorName, tt.description, tt.userID)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEvaluatorRepoImpl_BatchDeleteEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name          string
		ids           []int64
		userID        string
		mockSetup     func()
		expectedError error
	}{
		{
			name:   "成功批量删除评估器",
			ids:    []int64{1, 2},
			userID: "test_user",
			mockSetup: func() {
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				mockEvaluatorDAO.EXPECT().
					BatchDeleteEvaluator(gomock.Any(), []int64{1, 2}, "test_user", gomock.Any()).
					Return(nil)

				mockEvaluatorVersionDAO.EXPECT().
					BatchDeleteEvaluatorVersionByEvaluatorIDs(gomock.Any(), []int64{1, 2}, "test_user", gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "删除评估器失败",
			ids:    []int64{1, 2},
			userID: "test_user",
			mockSetup: func() {
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				mockEvaluatorDAO.EXPECT().
					BatchDeleteEvaluator(gomock.Any(), []int64{1, 2}, "test_user", gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			err := repo.BatchDeleteEvaluator(context.Background(), tt.ids, tt.userID)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestEvaluatorRepoImpl_CheckNameExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		spaceID        int64
		evaluatorID    int64
		evaluatorName  string
		mockSetup      func()
		expectedResult bool
		expectedError  error
	}{
		{
			name:          "名称已存在",
			spaceID:       1,
			evaluatorID:   1,
			evaluatorName: "test",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					CheckNameExist(gomock.Any(), int64(1), int64(1), "test").
					Return(true, nil)
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name:          "名称不存在",
			spaceID:       1,
			evaluatorID:   1,
			evaluatorName: "test",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					CheckNameExist(gomock.Any(), int64(1), int64(1), "test").
					Return(false, nil)
			},
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name:          "检查名称失败",
			spaceID:       1,
			evaluatorID:   1,
			evaluatorName: "test",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					CheckNameExist(gomock.Any(), int64(1), int64(1), "test").
					Return(false, assert.AnError)
			},
			expectedResult: false,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.CheckNameExist(context.Background(), tt.spaceID, tt.evaluatorID, tt.evaluatorName)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}

func TestEvaluatorRepoImpl_ListEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		request        *repo.ListEvaluatorRequest
		mockSetup      func()
		expectedResult *repo.ListEvaluatorResponse
		expectedError  error
	}{
		{
			name: "成功获取评估器列表",
			request: &repo.ListEvaluatorRequest{
				SpaceID:       1,
				SearchName:    "test",
				CreatorIDs:    []int64{1},
				EvaluatorType: []entity.EvaluatorType{entity.EvaluatorTypePrompt},
				PageSize:      10,
				PageNum:       1,
				OrderBy: []*entity.OrderBy{
					{
						Field: gptr.Of("updated_at"),
						IsAsc: gptr.Of(false),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					ListEvaluator(gomock.Any(), &mysql.ListEvaluatorRequest{
						SpaceID:       1,
						SearchName:    "test",
						CreatorIDs:    []int64{1},
						EvaluatorType: []int32{int32(entity.EvaluatorTypePrompt)},
						PageSize:      10,
						PageNum:       1,
						OrderBy: []*mysql.OrderBy{
							{
								Field:  "updated_at",
								ByDesc: true,
							},
						},
					}).
					Return(&mysql.ListEvaluatorResponse{
						TotalCount: 1,
						Evaluators: []*model.Evaluator{
							{
								ID:            1,
								EvaluatorType: int32(entity.EvaluatorTypePrompt),
								Name:          gptr.Of("test"),
							},
						},
					}, nil)
			},
			expectedResult: &repo.ListEvaluatorResponse{
				TotalCount: 1,
				Evaluators: []*entity.Evaluator{
					{
						ID:            1,
						EvaluatorType: entity.EvaluatorTypePrompt,
						Name:          "test",
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "获取评估器列表失败",
			request: &repo.ListEvaluatorRequest{
				SpaceID:       1,
				SearchName:    "test",
				CreatorIDs:    []int64{1},
				EvaluatorType: []entity.EvaluatorType{entity.EvaluatorTypePrompt},
				PageSize:      10,
				PageNum:       1,
				OrderBy: []*entity.OrderBy{
					{
						Field: gptr.Of("updated_at"),
						IsAsc: gptr.Of(false),
					},
				},
			},
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					ListEvaluator(gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.ListEvaluator(context.Background(), tt.request)
			assert.Equal(t, tt.expectedError, err)
			if err == nil {
				assert.Equal(t, tt.expectedResult.TotalCount, result.TotalCount)
				assert.Equal(t, len(tt.expectedResult.Evaluators), len(result.Evaluators))
				for i := range result.Evaluators {
					assert.Equal(t, tt.expectedResult.Evaluators[i].ID, result.Evaluators[i].ID)
					assert.Equal(t, tt.expectedResult.Evaluators[i].EvaluatorType, result.Evaluators[i].EvaluatorType)
					assert.Equal(t, tt.expectedResult.Evaluators[i].Name, result.Evaluators[i].Name)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_UpdateBuiltinEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockTagDAO := evaluatormocks.NewMockEvaluatorTagDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	repo := NewEvaluatorRepo(mockIDGen, mockDBProvider, mockEvaluatorDAO, mockEvaluatorVersionDAO, mockTagDAO, mockLWT)

	tests := []struct {
		name          string
		evaluator     *entity.Evaluator
		mockSetup     func()
		expectedError error
	}{
		{
			name: "成功更新内置评估器草稿，包含tag更新",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				Tags: map[entity.EvaluatorTagKey][]string{
					entity.EvaluatorTagKey_Category:  {"LLM", "Code"},
					entity.EvaluatorTagKey_Objective: {"Quality"},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置更新评估器草稿状态的期望
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorDraftSubmitted(gomock.Any(), int64(1), false, "test_user", gomock.Any()).
					Return(nil)

				// 设置更新评估器草稿的期望
				mockEvaluatorVersionDAO.EXPECT().
					UpdateEvaluatorDraft(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 设置获取草稿版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), []int64{1}, false, gomock.Any()).
					Return([]*model.EvaluatorVersion{
						{
							ID:          100, // 草稿版本ID
							EvaluatorID: 1,
							Version:     "draft",
						},
					}, nil)

				// 设置获取现有tags的期望
				mockTagDAO.EXPECT().
					BatchGetTagsBySourceIDsAndType(gomock.Any(), []int64{100}, int32(entity.EvaluatorTagKeyType_Evaluator), gomock.Any()).
					Return([]*model.EvaluatorTag{
						{
							ID:       1,
							SourceID: 100,
							TagKey:   "Category",
							TagValue: "LLM",
						},
						{
							ID:       2,
							SourceID: 100,
							TagKey:   "Objective",
							TagValue: "Performance",
						},
					}, nil)

				// 设置删除不需要的tags的期望
				mockTagDAO.EXPECT().
					DeleteEvaluatorTagsByConditions(gomock.Any(), int64(100), int32(entity.EvaluatorTagKeyType_Evaluator), gomock.Any(), gomock.Any()).
					Return(nil)

					// 设置新增tags的期望
				mockTagDAO.EXPECT().
					BatchCreateEvaluatorTags(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "成功更新内置评估器草稿，无tag更新",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置更新评估器草稿状态的期望
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorDraftSubmitted(gomock.Any(), int64(1), false, "test_user", gomock.Any()).
					Return(nil)

				// 设置更新评估器草稿的期望
				mockEvaluatorVersionDAO.EXPECT().
					UpdateEvaluatorDraft(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "失败 - 草稿版本不存在",
			evaluator: &entity.Evaluator{
				ID:            1,
				EvaluatorType: entity.EvaluatorTypePrompt,
				BaseInfo: &entity.BaseInfo{
					UpdatedBy: &entity.UserInfo{
						UserID: gptr.Of("test_user"),
					},
				},
				Tags: map[entity.EvaluatorTagKey][]string{
					entity.EvaluatorTagKey_Category: {"LLM"},
				},
				PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
					Version: "1.0.0",
					BaseInfo: &entity.BaseInfo{
						UpdatedBy: &entity.UserInfo{
							UserID: gptr.Of("test_user"),
						},
					},
				},
			},
			mockSetup: func() {
				// 设置数据库事务的期望
				mockDBProvider.EXPECT().
					Transaction(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, fn func(tx *gorm.DB) error, opts ...db.Option) error {
						mockTx := &gorm.DB{}
						return fn(mockTx)
					})

				// 设置更新评估器草稿状态的期望
				mockEvaluatorDAO.EXPECT().
					UpdateEvaluatorDraftSubmitted(gomock.Any(), int64(1), false, "test_user", gomock.Any()).
					Return(nil)

				// 设置更新评估器草稿的期望
				mockEvaluatorVersionDAO.EXPECT().
					UpdateEvaluatorDraft(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil)

				// 设置获取草稿版本的期望 - 返回空列表
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), []int64{1}, false, gomock.Any()).
					Return([]*model.EvaluatorVersion{}, nil)
			},
			expectedError: assert.AnError, // 期望返回错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := repo.UpdateBuiltinEvaluatorDraft(context.Background(), tt.evaluator)
			if tt.expectedError != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluatorRepoImpl_BatchGetBuiltinEvaluatorByVersionID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockTagDAO := evaluatormocks.NewMockEvaluatorTagDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name           string
		spaceID        *int64
		ids            []int64
		includeDeleted bool
		mockSetup      func()
		expectedResult []*entity.Evaluator
		expectedError  error
	}{
		{
			name:           "成功批量获取内置评估器版本，包含tag信息",
			spaceID:        gptr.Of(int64(1)),
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				// 设置获取评估器版本的期望
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gptr.Of(int64(1)), []int64{1, 2}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            1,
							EvaluatorID:   1,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
						{
							ID:            2,
							EvaluatorID:   2,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypeCode)),
							Version:       "1.0.0",
						},
					}, nil)

				// 设置获取评估器基本信息的期望
				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), []int64{1, 2}, false).
					Return([]*model.Evaluator{
						{
							ID:            1,
							Name:          gptr.Of("Test Evaluator 1"),
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
						},
						{
							ID:            2,
							Name:          gptr.Of("Test Evaluator 2"),
							EvaluatorType: int32(entity.EvaluatorTypeCode),
						},
					}, nil)

				// 设置获取tag信息的期望
				mockTagDAO.EXPECT().
					BatchGetTagsBySourceIDsAndType(gomock.Any(), []int64{1, 2}, int32(entity.EvaluatorTagKeyType_Evaluator)).
					Return([]*model.EvaluatorTag{
						{
							SourceID: 1,
							TagKey:   "category",
							TagValue: "test",
						},
						{
							SourceID: 2,
							TagKey:   "category",
							TagValue: "production",
						},
					}, nil)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					Name:          "Test Evaluator 1",
					EvaluatorType: entity.EvaluatorTypePrompt,
					Tags: map[entity.EvaluatorTagKey][]string{
						"category": {"test"},
					},
				},
				{
					ID:            2,
					Name:          "Test Evaluator 2",
					EvaluatorType: entity.EvaluatorTypeCode,
					Tags: map[entity.EvaluatorTagKey][]string{
						"category": {"production"},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:           "获取评估器版本失败",
			spaceID:        gptr.Of(int64(1)),
			ids:            []int64{1, 2},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gptr.Of(int64(1)), []int64{1, 2}, false).
					Return(nil, assert.AnError)
			},
			expectedResult: nil,
			expectedError:  assert.AnError,
		},
		{
			name:           "获取tag信息失败，但继续处理",
			spaceID:        gptr.Of(int64(1)),
			ids:            []int64{1},
			includeDeleted: false,
			mockSetup: func() {
				mockEvaluatorVersionDAO.EXPECT().
					BatchGetEvaluatorVersionByID(gomock.Any(), gptr.Of(int64(1)), []int64{1}, false).
					Return([]*model.EvaluatorVersion{
						{
							ID:            1,
							EvaluatorID:   1,
							EvaluatorType: gptr.Of(int32(entity.EvaluatorTypePrompt)),
							Version:       "1.0.0",
						},
					}, nil)

				mockEvaluatorDAO.EXPECT().
					BatchGetEvaluatorByID(gomock.Any(), []int64{1}, false).
					Return([]*model.Evaluator{
						{
							ID:            1,
							Name:          gptr.Of("Test Evaluator 1"),
							EvaluatorType: int32(entity.EvaluatorTypePrompt),
						},
					}, nil)

				// tag查询失败，但方法应该继续处理
				mockTagDAO.EXPECT().
					BatchGetTagsBySourceIDsAndType(gomock.Any(), []int64{1}, int32(entity.EvaluatorTagKeyType_Evaluator)).
					Return(nil, assert.AnError)
			},
			expectedResult: []*entity.Evaluator{
				{
					ID:            1,
					Name:          "Test Evaluator 1",
					EvaluatorType: entity.EvaluatorTypePrompt,
					Tags:          nil, // 没有tag信息
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				tagDAO:              mockTagDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			result, err := repo.BatchGetBuiltinEvaluatorByVersionID(context.Background(), tt.spaceID, tt.ids, tt.includeDeleted)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.expectedResult), len(result))
				for i, expected := range tt.expectedResult {
					assert.Equal(t, expected.ID, result[i].ID)
					assert.Equal(t, expected.Name, result[i].Name)
					assert.Equal(t, expected.EvaluatorType, result[i].EvaluatorType)
					assert.Equal(t, expected.Tags, result[i].Tags)
				}
			}
		})
	}
}

func TestEvaluatorRepoImpl_UpdateBuiltinEvaluatorMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorDAO := evaluatormocks.NewMockEvaluatorDAO(ctrl)
	mockEvaluatorVersionDAO := evaluatormocks.NewMockEvaluatorVersionDAO(ctrl)
	mockTagDAO := evaluatormocks.NewMockEvaluatorTagDAO(ctrl)
	mockDBProvider := dbmocks.NewMockProvider(ctrl)
	mockLWT := platestwritemocks.NewMockILatestWriteTracker(ctrl)

	tests := []struct {
		name          string
		id            int64
		benchmark     string
		vendor        string
		userID        string
		mockSetup     func()
		expectedError error
	}{
		{
			name:      "成功更新内置评估器元数据",
			id:        1,
			benchmark: "test_benchmark",
			vendor:    "test_vendor",
			userID:    "test_user",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					UpdateBuiltinEvaluatorMeta(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			expectedError: nil,
		},
		{
			name:      "更新内置评估器元数据失败",
			id:        2,
			benchmark: "test_benchmark",
			vendor:    "test_vendor",
			userID:    "test_user",
			mockSetup: func() {
				mockEvaluatorDAO.EXPECT().
					UpdateBuiltinEvaluatorMeta(gomock.Any(), gomock.Any()).
					Return(assert.AnError)
			},
			expectedError: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			repo := &EvaluatorRepoImpl{
				evaluatorDao:        mockEvaluatorDAO,
				evaluatorVersionDao: mockEvaluatorVersionDAO,
				tagDAO:              mockTagDAO,
				dbProvider:          mockDBProvider,
				idgen:               mockIDGen,
				lwt:                 mockLWT,
			}

			err := repo.UpdateBuiltinEvaluatorMeta(context.Background(), tt.id, "", "", tt.benchmark, tt.vendor, tt.userID)
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
