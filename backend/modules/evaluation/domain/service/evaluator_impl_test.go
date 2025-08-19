// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	mqmocks "github.com/coze-dev/coze-loop/backend/infra/mq/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entitymocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestNewEvaluatorServiceImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockMQ := mqmocks.NewMockIFactory(ctrl)
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockIdem := idemmocks.NewMockIdempotentService(ctrl)
	mockConfiger := confmocks.NewMockIConfiger(ctrl)
	mockSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockSourceService.EXPECT().EvaluatorType().Return(entity.EvaluatorTypePrompt)

	// 这里需要传递一个 EvaluatorSourceService 的 slice
	service := NewEvaluatorServiceImpl(
		mockIdgen,
		mockLimiter,
		mockMQ,
		mockEvaluatorRepo,
		mockEvaluatorRecordRepo,
		mockIdem,
		mockConfiger,
		[]EvaluatorSourceService{mockSourceService},
	)

	assert.IsType(t, &EvaluatorServiceImpl{}, service)
}

// TestEvaluatorServiceImpl_ListEvaluator 使用 gomock 对 ListEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_ListEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()

	// 定义测试用例
	testCases := []struct {
		name          string
		request       *entity.ListEvaluatorRequest // 注意：这里的 ListEvaluatorRequest 是 service 包内的，不是 repo 包内的
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedList  []*entity.Evaluator
		expectedTotal int64
		expectedErr   error
	}{
		{
			name: "成功 - 不带版本信息 (WithVersion = false)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				PageSize:    10,
				PageNum:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				// buildListEvaluatorRequest 会将 service.ListEvaluatorRequest 转换为 repo.ListEvaluatorRequest
				// 这里我们模拟 repo.ListEvaluator 的行为，其输入参数是转换后的 repo.ListEvaluatorRequest
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					PageSize:      10,
					PageNum:       1,
					EvaluatorType: []entity.EvaluatorType{}, // 假设 request.EvaluatorType 为空
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{ID: 1, Name: "Eval1", SpaceID: 1, Description: "Desc1"},
							{ID: 2, Name: "Eval2", SpaceID: 1, Description: "Desc2"},
						},
						TotalCount: 2,
					}, nil)
			},
			expectedList: []*entity.Evaluator{
				{ID: 1, Name: "Eval1", SpaceID: 1, Description: "Desc1"},
				{ID: 2, Name: "Eval2", SpaceID: 1, Description: "Desc2"},
			},
			expectedTotal: 2,
			expectedErr:   nil,
		},
		{
			name: "成功 - 带版本信息 (WithVersion = true)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				PageSize:    10,
				PageNum:     1,
				WithVersion: true,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					PageSize:      10,
					PageNum:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				// 模拟 ListEvaluator 返回结果 (元数据)
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{
							{ID: 101, Name: "Eval101", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v1", Description: "Meta Desc 101", BaseInfo: &entity.BaseInfo{UpdatedAt: ptr.Of(int64(1))}},
							{ID: 102, Name: "Eval102", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v2", Description: "Meta Desc 102", BaseInfo: &entity.BaseInfo{UpdatedAt: ptr.Of(int64(2))}},
						},
						TotalCount: 2,
					}, nil)

				// 模拟 BatchGetEvaluatorVersionsByEvaluatorIDs 返回结果 (版本详情)
				evaluatorIDs := []int64{101, 102}
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), gomock.Eq(evaluatorIDs), false).Return(
					[]*entity.Evaluator{
						{ // 版本信息属于 Evaluator 101
							EvaluatorType: entity.EvaluatorTypePrompt, // 必须与元数据中的 EvaluatorType 一致
							PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ // 实际版本数据
								EvaluatorID: 101, Version: "v1.0-version", Description: "Version specific desc 1",
							},
						},
						{ // 版本信息属于 Evaluator 102
							EvaluatorType: entity.EvaluatorTypePrompt,
							PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
								EvaluatorID: 102, Version: "v2.0-version", Description: "Version specific desc 2",
							},
						},
					}, nil)
			},
			expectedList: []*entity.Evaluator{
				{ // 结果是元数据和版本详情的合并
					ID: 101, Name: "Eval101", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v1",
					Description:    "Meta Desc 101",                               // 来自 ListEvaluator 的元数据
					BaseInfo:       &entity.BaseInfo{UpdatedAt: ptr.Of(int64(1))}, // 来自 ListEvaluator 的元数据
					DraftSubmitted: false,                                         // 默认值或来自 ListEvaluator
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ // 来自 BatchGet
						EvaluatorID: 101, Version: "v1.0-version", Description: "Version specific desc 1",
					},
				},
				{
					ID: 102, Name: "Eval102", SpaceID: 1, EvaluatorType: entity.EvaluatorTypePrompt, LatestVersion: "v2",
					Description:    "Meta Desc 102",
					BaseInfo:       &entity.BaseInfo{UpdatedAt: ptr.Of(int64(2))},
					DraftSubmitted: false,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						EvaluatorID: 102, Version: "v2.0-version", Description: "Version specific desc 2",
					},
				},
			},
			expectedTotal: 2, // 当 WithVersion 为 true 时，返回的是版本数量
			expectedErr:   nil,
		},
		{
			name: "失败 - evaluatorRepo.ListEvaluator 返回错误",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(nil, errors.New("db error from ListEvaluator"))
			},
			expectedList:  nil,
			expectedTotal: 0,
			expectedErr:   errors.New("db error from ListEvaluator"),
		},
		{
			name: "失败 - WithVersion=true 时 evaluatorRepo.BatchGetEvaluatorVersionsByEvaluatorIDs 返回错误",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: true,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{{ID: 1, EvaluatorType: entity.EvaluatorTypePrompt}}, // 提供基础数据
						TotalCount: 1,
					}, nil)
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), gomock.Eq([]int64{1}), false).Return(
					nil, errors.New("db error from BatchGetVersions"))
			},
			expectedList:  nil,
			expectedTotal: 0,
			expectedErr:   errors.New("db error from BatchGetVersions"),
		},
		{
			name: "成功 - ListEvaluator 返回空列表 (WithVersion = false)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: false,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
			},
			expectedList:  []*entity.Evaluator{},
			expectedTotal: 0,
			expectedErr:   nil,
		},
		{
			name: "成功 - ListEvaluator 返回空列表 (WithVersion = true)",
			request: &entity.ListEvaluatorRequest{
				SpaceID:     1,
				WithVersion: true,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedRepoReq := &repo.ListEvaluatorRequest{
					SpaceID:       1,
					EvaluatorType: []entity.EvaluatorType{},
					OrderBy:       []*entity.OrderBy{{Field: ptr.Of("updated_at"), IsAsc: ptr.Of(false)}},
				}
				mockRepo.EXPECT().ListEvaluator(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorResponse{
						Evaluators: []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
				// BatchGetEvaluatorVersionsByEvaluatorIDs 应该传入空 evaluatorIDs
				mockRepo.EXPECT().BatchGetEvaluatorVersionsByEvaluatorIDs(gomock.Any(), gomock.Eq([]int64{}), false).Return(
					[]*entity.Evaluator{}, nil)
			},
			expectedList:  []*entity.Evaluator{},
			expectedTotal: 0,
			expectedErr:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo)

			list, total, err := s.ListEvaluator(ctx, tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedList, list)
			assert.Equal(t, tc.expectedTotal, total)
		})
	}
}

// TestEvaluatorServiceImpl_BatchGetEvaluator 使用 gomock 对 BatchGetEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_BatchGetEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	// 被测服务实例
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		// 其他依赖项对于 BatchGetEvaluator 方法不是必需的
	}

	ctx := context.Background()

	// 定义测试用例
	testCases := []struct {
		name             string
		spaceID          int64
		evaluatorIDs     []int64
		includeDeleted   bool
		setupMock        func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedResponse []*entity.Evaluator
		expectedErr      error
	}{
		{
			name:           "成功 - 返回多个评估器",
			spaceID:        1,
			evaluatorIDs:   []int64{10, 20},
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), []int64{10, 20}, false).Return(
					[]*entity.Evaluator{
						{ID: 10, Name: "Evaluator10"},
						{ID: 20, Name: "Evaluator20"},
					}, nil)
			},
			expectedResponse: []*entity.Evaluator{
				{ID: 10, Name: "Evaluator10"},
				{ID: 20, Name: "Evaluator20"},
			},
			expectedErr: nil,
		},
		{
			name:           "成功 - 返回空列表",
			spaceID:        2,
			evaluatorIDs:   []int64{30},
			includeDeleted: true,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(2), []int64{30}, true).Return(
					[]*entity.Evaluator{}, nil)
			},
			expectedResponse: []*entity.Evaluator{},
			expectedErr:      nil,
		},
		{
			name:           "成功 - evaluatorIDs 为空",
			spaceID:        3,
			evaluatorIDs:   []int64{},
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(3), []int64{}, false).Return(
					[]*entity.Evaluator{}, nil)
			},
			expectedResponse: []*entity.Evaluator{},
			expectedErr:      nil,
		},
		{
			name:           "失败 - evaluatorRepo 返回错误",
			spaceID:        4,
			evaluatorIDs:   []int64{40},
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(4), []int64{40}, false).Return(
					nil, errors.New("database error"))
			},
			expectedResponse: nil,
			expectedErr:      errors.New("database error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo)

			actualResponse, actualErr := s.BatchGetEvaluator(ctx, tc.spaceID, tc.evaluatorIDs, tc.includeDeleted)

			if tc.expectedErr != nil {
				assert.Error(t, actualErr)
				assert.Equal(t, tc.expectedErr, actualErr)
			} else {
				assert.NoError(t, actualErr)
			}
			assert.Equal(t, tc.expectedResponse, actualResponse)
		})
	}
}

// TestEvaluatorServiceImpl_GetEvaluator 使用gomock 对 GetEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_GetEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}
	ctx := context.Background()

	testCases := []struct {
		name              string
		spaceID           int64
		evaluatorID       int64
		includeDeleted    bool
		setupMock         func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedEvaluator *entity.Evaluator
		expectedErr       error
		expectedErrCode   int32 // 用于校验 errorx 错误码
	}{
		{
			name:           "失败 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回错误",
			spaceID:        1,
			evaluatorID:    100,
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{100}), false).
					Return(nil, errors.New("db error"))
			},
			expectedErr: errors.New("db error"),
		},
		{
			name:           "成功 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回空列表",
			spaceID:        1,
			evaluatorID:    101,
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{101}), false).
					Return([]*entity.Evaluator{}, nil)
			},
			expectedEvaluator: nil,
			expectedErr:       nil,
		},
		{
			name:           "成功 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回一个 evaluator",
			spaceID:        1,
			evaluatorID:    102,
			includeDeleted: false,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{102}), false).
					Return([]*entity.Evaluator{{ID: 102, Name: "Test Eval"}}, nil)
			},
			expectedEvaluator: &entity.Evaluator{ID: 102, Name: "Test Eval"},
			expectedErr:       nil,
		},
		{
			name:           "成功 - evaluatorRepo.BatchGetEvaluatorDraftByEvaluatorID 返回多个 evaluators, 取第一个",
			spaceID:        1,
			evaluatorID:    103,
			includeDeleted: true,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchGetEvaluatorDraftByEvaluatorID(gomock.Any(), int64(1), gomock.Eq([]int64{103}), true).
					Return([]*entity.Evaluator{
						{ID: 103, Name: "First Eval"},
						{ID: 10301, Name: "Second Eval"},
					}, nil)
			},
			expectedEvaluator: &entity.Evaluator{ID: 103, Name: "First Eval"},
			expectedErr:       nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo)

			evaluator, err := s.GetEvaluator(ctx, tc.spaceID, tc.evaluatorID, tc.includeDeleted)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				if tc.expectedErrCode != 0 {
					e, ok := err.(interface{ GetCode() int32 })
					assert.True(t, ok)
					assert.Equal(t, tc.expectedErrCode, e.GetCode())
				}
				assert.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedEvaluator, evaluator)
		})
	}
}

// TestEvaluatorServiceImpl_CreateEvaluator 使用 gomock 对 CreateEvaluator 方法进行单元测试
func TestEvaluatorServiceImpl_CreateEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockIdemService := idemmocks.NewMockIdempotentService(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		idem:          mockIdemService,
	}

	ctx := context.Background()
	fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	testUserID := "test_user_123"

	// 准备一个基础的 evaluatorDO 用于测试
	baseEvaluatorDO := func() *entity.Evaluator {
		return &entity.Evaluator{
			SpaceID:       int64(1),
			Name:          "Test Evaluator",
			EvaluatorType: entity.EvaluatorTypePrompt,
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				MessageList: []*entity.Message{},
				ModelConfig: &entity.ModelConfig{},
			},
		}
	}

	testCases := []struct {
		name            string
		evaluatorDO     *entity.Evaluator
		cid             string
		setupMock       func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo)
		expectedID      int64
		expectedErr     error
		expectedErrCode int32
	}{
		{
			name: "失败 - validateCreateEvaluatorRequest - CheckNameExist 返回错误",
			evaluatorDO: func() *entity.Evaluator {
				e := baseEvaluatorDO()
				e.Name = "check_name_err_eval"
				return e
			}(),
			cid: "validate_checkname_err_cid",
			setupMock: func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedKey := "create_evaluator_idem" + cid
				mockIdem.EXPECT().Set(gomock.Any(), expectedKey, time.Second*10).Return(nil)
				mockRepo.EXPECT().CheckNameExist(gomock.Any(), evaluatorDO.SpaceID, int64(-1), evaluatorDO.Name).
					Return(false, errors.New("db check name error"))
			},
			expectedID:  0,
			expectedErr: errors.New("db check name error"),
		},
		{
			name:        "失败 - evaluatorRepo.CreateEvaluator 返回错误",
			evaluatorDO: baseEvaluatorDO(),
			cid:         "repo_create_err_cid",
			setupMock: func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedKey := "create_evaluator_idem" + cid
				mockIdem.EXPECT().Set(gomock.Any(), expectedKey, time.Second*10).Return(nil)
				if evaluatorDO.Name != "" {
					mockEvaluatorRepo.EXPECT().CheckNameExist(gomock.Any(), evaluatorDO.SpaceID, int64(-1), evaluatorDO.Name).
						Return(false, nil)
				}
				session.WithCtxUser(ctx, &session.User{ID: testUserID})

				expectedInjectedDO := *evaluatorDO
				expectedInjectedDO.BaseInfo = &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					CreatedAt: ptr.Of(fixedTime.UnixMilli()),
					UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
				}
				if expectedInjectedDO.PromptEvaluatorVersion != nil {
					expectedInjectedDO.PromptEvaluatorVersion.BaseInfo = &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						CreatedAt: ptr.Of(fixedTime.UnixMilli()),
						UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
					}
				}

				mockRepo.EXPECT().CreateEvaluator(gomock.Any(), gomock.Any()).
					Return(int64(0), errors.New("db create error"))
			},
			expectedID:  int64(0),
			expectedErr: errors.New("db create error"),
		},
		{
			name:        "成功 - 创建 Evaluator",
			evaluatorDO: baseEvaluatorDO(),
			cid:         "success_cid",
			setupMock: func(evaluatorDO *entity.Evaluator, cid string, mockIdem *idemmocks.MockIdempotentService, mockRepo *repomocks.MockIEvaluatorRepo) {
				expectedKey := "create_evaluator_idem" + cid
				mockIdem.EXPECT().Set(gomock.Any(), expectedKey, time.Second*10).Return(nil)
				if evaluatorDO.Name != "" {
					mockEvaluatorRepo.EXPECT().CheckNameExist(gomock.Any(), evaluatorDO.SpaceID, int64(-1), evaluatorDO.Name).
						Return(false, nil)
				}
				session.WithCtxUser(ctx, &session.User{ID: testUserID})

				expectedInjectedDO := *evaluatorDO
				expectedInjectedDO.BaseInfo = &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
					CreatedAt: ptr.Of(fixedTime.UnixMilli()),
					UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
				}
				if expectedInjectedDO.PromptEvaluatorVersion != nil {
					expectedInjectedDO.PromptEvaluatorVersion.BaseInfo = &entity.BaseInfo{
						CreatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						UpdatedBy: &entity.UserInfo{UserID: ptr.Of(testUserID)},
						CreatedAt: ptr.Of(fixedTime.UnixMilli()),
						UpdatedAt: ptr.Of(fixedTime.UnixMilli()),
					}
				}

				mockRepo.EXPECT().CreateEvaluator(gomock.Any(), gomock.Any()).
					Return(int64(12345), nil)
			},
			expectedID:  int64(12345),
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(tc.evaluatorDO, tc.cid, mockIdemService, mockEvaluatorRepo)

			id, err := s.CreateEvaluator(ctx, tc.evaluatorDO, tc.cid)

			if tc.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedID, id)
		})
	}
}

func TestEvaluatorServiceImpl_UpdateEvaluatorMeta(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	// 创建被测服务实例，并注入 mock 依赖
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		// 其他依赖项对于此方法不是必需的，可以省略或设为 nil
	}

	ctx := context.Background()

	// 定义测试用例
	tests := []struct {
		name        string
		id          int64
		spaceID     int64
		evalName    string // 对应 UpdateEvaluatorMeta 的 name 参数
		description string
		userID      string
		setupMock   func(repoMock *repomocks.MockIEvaluatorRepo) // 用于设置 mock 期望
		wantErr     bool
		expectedErr error // 期望的错误，用于更精确的错误断言
	}{
		{
			name:        "成功 - 名称为空字符串，不校验名称是否存在，更新成功",
			id:          1,
			spaceID:     100,
			evalName:    "", // 名称为空
			description: "new description",
			userID:      "user123",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				// CheckNameExist 不应该被调用
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), int64(1), "", "new description", "user123").Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "成功 - 名称不为空，且名称不存在，更新成功",
			id:          2,
			spaceID:     101,
			evalName:    "newName",
			description: "another description",
			userID:      "user456",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(101), int64(2), "newName").Return(false, nil)
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), int64(2), "newName", "another description", "user456").Return(nil)
			},
			wantErr: false,
		},
		{
			name:        "失败 - 名称不为空，CheckNameExist 返回错误",
			id:          3,
			spaceID:     102,
			evalName:    "checkFailName",
			description: "desc",
			userID:      "user789",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(102), int64(3), "checkFailName").Return(false, errors.New("db check error"))
				// UpdateEvaluatorMeta 不应该被调用
			},
			wantErr:     true,
			expectedErr: errors.New("db check error"),
		},
		{
			name:        "失败 - 名称不为空，且名称已存在",
			id:          4,
			spaceID:     103,
			evalName:    "existingName",
			description: "desc",
			userID:      "userABC",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(103), int64(4), "existingName").Return(true, nil)
				// UpdateEvaluatorMeta 不应该被调用
			},
			wantErr:     true,
			expectedErr: errorx.NewByCode(errno.EvaluatorNameExistCode), // 假设 errorx 和 errno 包已正确导入
		},
		{
			name:        "失败 - 名称为空，但 UpdateEvaluatorMeta 返回错误",
			id:          5,
			spaceID:     104,
			evalName:    "",
			description: "update fail desc",
			userID:      "userDEF",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				// CheckNameExist 不应该被调用
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), int64(5), "", "update fail desc", "userDEF").Return(errors.New("db update error"))
			},
			wantErr:     true,
			expectedErr: errors.New("db update error"),
		},
		{
			name:        "失败 - 名称不为空且不存在，但 UpdateEvaluatorMeta 返回错误",
			id:          6,
			spaceID:     105,
			evalName:    "validNameButFailUpdate",
			description: "desc",
			userID:      "userGHI",
			setupMock: func(repoMock *repomocks.MockIEvaluatorRepo) {
				repoMock.EXPECT().CheckNameExist(gomock.Any(), int64(105), int64(6), "validNameButFailUpdate").Return(false, nil)
				repoMock.EXPECT().UpdateEvaluatorMeta(gomock.Any(), int64(6), "validNameButFailUpdate", "desc", "userGHI").Return(errors.New("db update error after check"))
			},
			wantErr:     true,
			expectedErr: errors.New("db update error after check"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个子测试重置/设置 mock 期望
			if tt.setupMock != nil {
				tt.setupMock(mockEvaluatorRepo)
			}

			err := s.UpdateEvaluatorMeta(ctx, tt.id, tt.spaceID, tt.evalName, tt.description, tt.userID)

			if tt.wantErr {
				assert.Error(t, err, "期望得到一个错误")
			} else {
				assert.NoError(t, err, "不期望得到错误")
			}
		})
	}
}

// TestEvaluatorServiceImpl_UpdateEvaluatorDraft 测试 UpdateEvaluatorDraft 方法
func TestEvaluatorServiceImpl_UpdateEvaluatorDraft(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()
	testEvaluator := &entity.Evaluator{
		ID:      1,
		SpaceID: 100,
		Name:    "Test Evaluator",
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:       10,
			BaseInfo: &entity.BaseInfo{},
		},
		BaseInfo: &entity.BaseInfo{},
	}

	tests := []struct {
		name          string
		evaluatorDO   *entity.Evaluator
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedError error
	}{
		{
			name:        "成功更新评估器草稿",
			evaluatorDO: testEvaluator,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:        "更新评估器草稿失败 - repo返回错误",
			evaluatorDO: testEvaluator,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().UpdateEvaluatorDraft(gomock.Any(), testEvaluator).Return(errors.New("repo error"))
			},
			expectedError: errors.New("repo error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockEvaluatorRepo)
			err := s.UpdateEvaluatorDraft(ctx, tt.evaluatorDO)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorServiceImpl_DeleteEvaluator 测试 DeleteEvaluator 方法
func TestEvaluatorServiceImpl_DeleteEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()
	testEvaluatorIDs := []int64{1, 2, 3}
	testUserID := "test_user_id"

	tests := []struct {
		name          string
		evaluatorIDs  []int64
		userID        string
		setupMock     func(mockRepo *repomocks.MockIEvaluatorRepo)
		expectedError error
	}{
		{
			name:         "成功删除评估器",
			evaluatorIDs: testEvaluatorIDs,
			userID:       testUserID,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchDeleteEvaluator(gomock.Any(), testEvaluatorIDs, testUserID).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:         "删除评估器失败 - repo返回错误",
			evaluatorIDs: testEvaluatorIDs,
			userID:       testUserID,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchDeleteEvaluator(gomock.Any(), testEvaluatorIDs, testUserID).Return(errors.New("repo delete error"))
			},
			expectedError: errors.New("repo delete error"),
		},
		{
			name:         "删除评估器 - 空ID列表",
			evaluatorIDs: []int64{},
			userID:       testUserID,
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo) {
				mockRepo.EXPECT().BatchDeleteEvaluator(gomock.Any(), []int64{}, testUserID).Return(nil) // 假设repo层允许空列表
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock(mockEvaluatorRepo)
			err := s.DeleteEvaluator(ctx, tt.evaluatorIDs, tt.userID)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEvaluatorServiceImpl_ListEvaluatorVersion 测试 ListEvaluatorVersion 方法
func TestEvaluatorServiceImpl_ListEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)

	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}

	ctx := context.Background()

	// 辅助函数，用于创建 entity.Evaluator 实例
	newEvaluator := func(id int64, version string) *entity.Evaluator {
		return &entity.Evaluator{
			PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
				ID:      id,
				Version: version,
			},
		}
	}

	tests := []struct {
		name              string
		request           *entity.ListEvaluatorVersionRequest // service 包内的 ListEvaluatorVersionRequest
		setupMock         func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest)
		expectedVersions  []*entity.Evaluator
		expectedTotal     int64
		expectedError     error
		expectedErrorMsg  string // 用于 errorx 类型的错误消息比较
		expectedErrorCode int    // 用于 errorx 类型的错误码比较
	}{
		{
			name: "成功获取评估器版本列表 - 带自定义排序",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 1,
				PageSize:    10,
				PageNum:     1,
				OrderBys: []*entity.OrderBy{
					{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(true)},
				},
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				// 预期 buildListEvaluatorVersionRequest 会转换的 repo.ListEvaluatorVersionRequest
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID:   serviceReq.EvaluatorID,
					QueryVersions: serviceReq.QueryVersions,
					PageSize:      serviceReq.PageSize,
					PageNum:       serviceReq.PageNum,
					OrderBy: []*entity.OrderBy{ // 确保 OrderBySet 包含 OrderByCreatedAt
						{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(true)},
					},
				}
				// Mock entity.OrderBySet 使得自定义排序生效
				// 注意：直接修改全局变量 entity.OrderBySet 可能会影响其他测试，更好的方式是确保其已正确初始化
				// 这里假设 entity.OrderBySet["created_at"] 存在

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{newEvaluator(101, "v1.0"), newEvaluator(102, "v1.1")},
						TotalCount: 2,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{newEvaluator(101, "v1.0"), newEvaluator(102, "v1.1")},
			expectedTotal:    2,
			expectedError:    nil,
		},
		{
			name: "成功获取评估器版本列表 - 默认排序 (updated_at desc)",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 2,
				PageSize:    5,
				PageNum:     2,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID:   serviceReq.EvaluatorID,
					QueryVersions: serviceReq.QueryVersions,
					PageSize:      serviceReq.PageSize,
					PageNum:       serviceReq.PageNum,
					OrderBy: []*entity.OrderBy{ // 默认排序
						{Field: ptr.Of(entity.OrderByUpdatedAt), IsAsc: ptr.Of(false)},
					},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{newEvaluator(201, "v2.0")},
						TotalCount: 1,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{newEvaluator(201, "v2.0")},
			expectedTotal:    1,
			expectedError:    nil,
		},
		{
			name: "成功获取评估器版本列表 - repo返回空列表",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 3,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID: serviceReq.EvaluatorID,
					OrderBy:     []*entity.OrderBy{{Field: ptr.Of(entity.OrderByUpdatedAt), IsAsc: ptr.Of(false)}},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{},
						TotalCount: 0,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{},
			expectedTotal:    0,
			expectedError:    nil,
		},
		{
			name: "获取评估器版本列表失败 - repo返回错误",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 4,
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID: serviceReq.EvaluatorID,
					OrderBy:     []*entity.OrderBy{{Field: ptr.Of(entity.OrderByUpdatedAt), IsAsc: ptr.Of(false)}},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					nil, errors.New("db query error"))
			},
			expectedVersions: nil,
			expectedTotal:    0,
			expectedError:    errors.New("db query error"),
		},
		{
			name: "获取评估器版本列表失败 - 无效的OrderBy字段 (buildListEvaluatorVersionRequest内部过滤)",
			request: &entity.ListEvaluatorVersionRequest{
				EvaluatorID: 1,
				OrderBys: []*entity.OrderBy{
					{Field: ptr.Of("invalid_field"), IsAsc: ptr.Of(true)}, // 这个字段会被过滤掉
					{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(false)},
				},
			},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, serviceReq *entity.ListEvaluatorVersionRequest) {
				// 预期 buildListEvaluatorVersionRequest 会过滤掉 "invalid_field"
				expectedRepoReq := &repo.ListEvaluatorVersionRequest{
					EvaluatorID: serviceReq.EvaluatorID,
					OrderBy: []*entity.OrderBy{
						{Field: ptr.Of(entity.OrderByCreatedAt), IsAsc: ptr.Of(false)},
					},
				}

				mockRepo.EXPECT().ListEvaluatorVersion(gomock.Any(), gomock.Eq(expectedRepoReq)).Return(
					&repo.ListEvaluatorVersionResponse{
						Versions:   []*entity.Evaluator{newEvaluator(101, "v1.0")},
						TotalCount: 1,
					}, nil)
			},
			expectedVersions: []*entity.Evaluator{newEvaluator(101, "v1.0")},
			expectedTotal:    1,
			expectedError:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 在每个 PatchConvey 内部设置 mock，以确保隔离
			// 对于依赖全局变量的 buildListEvaluatorVersionRequest，需要在这里 mock entity.OrderBySet
			// 如果 entity.OrderBySet 是在包初始化时就固定的，则不需要每次都 mock
			// 但为了测试的确定性，这里显式 mock
			originalOrderBySet := entity.OrderBySet                   // 备份原始值
			defer func() { entity.OrderBySet = originalOrderBySet }() // 恢复原始值

			tt.setupMock(mockEvaluatorRepo, tt.request)

			versions, total, err := s.ListEvaluatorVersion(ctx, tt.request)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Equal(t, tt.expectedError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectedVersions, versions)
			assert.Equal(t, tt.expectedTotal, total)
		})
	}
}

// TestEvaluatorServiceImpl_GetEvaluatorVersion 测试 GetEvaluatorVersion 方法
func TestEvaluatorServiceImpl_GetEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	// 被测服务实例
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
		// 其他依赖项如果该方法不需要，可以为 nil 或默认值
	}
	ctx := context.Background() // 标准的上下文

	// 定义输入参数结构体
	type args struct {
		evaluatorVersionID int64
		includeDeleted     bool
	}
	// 定义测试用例表格
	testCases := []struct {
		name      string                                                  // 测试用例名称
		args      args                                                    // 输入参数
		setupMock func(mockRepo *repomocks.MockIEvaluatorRepo, args args) // mock设置函数
		want      *entity.Evaluator                                       // 期望得到的评估器实体
		wantErr   error                                                   // 期望得到的错误
	}{
		{
			name: "成功 - 找到评估器版本",
			args: args{evaluatorVersionID: 1, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				// 期望 evaluatorRepo.BatchGetEvaluatorByVersionID 被调用一次
				// 参数为：任意上下文, ID切片, 是否包含删除
				// 返回预设的评估器列表和nil错误
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted).
					Return([]*entity.Evaluator{{ID: 1, Name: "Test Evaluator Version 1"}}, nil)
			},
			want:    &entity.Evaluator{ID: 1, Name: "Test Evaluator Version 1"},
			wantErr: nil,
		},
		{
			name: "成功 - 未找到评估器版本 (repo返回空列表)",
			args: args{evaluatorVersionID: 2, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted).
					Return([]*entity.Evaluator{}, nil) // Repo返回空列表表示未找到
			},
			want:    nil, // 期望返回nil实体
			wantErr: nil, // 期望返回nil错误
		},
		{
			name: "失败 - evaluatorRepo.BatchGetEvaluatorByVersionID 返回错误",
			args: args{evaluatorVersionID: 3, includeDeleted: true},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted).
					Return(nil, errors.New("repo database error")) // Repo返回错误
			},
			want:    nil,
			wantErr: errors.New("repo database error"), // 期望透传错误
		},
		{
			name: "成功 - repo返回多个评估器版本 (应返回第一个)",
			args: args{evaluatorVersionID: 4, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq([]int64{args.evaluatorVersionID}), args.includeDeleted).
					Return([]*entity.Evaluator{
						{ID: 4, Name: "First Evaluator Version"},
						{ID: 5, Name: "Second Evaluator Version"}, // 即使返回多个，方法也只取第一个
					}, nil)
			},
			want:    &entity.Evaluator{ID: 4, Name: "First Evaluator Version"},
			wantErr: nil,
		},
	}

	// 遍历执行测试用例
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo, tc.args)

			got, err := s.GetEvaluatorVersion(ctx, tc.args.evaluatorVersionID, tc.args.includeDeleted)

			// 断言错误
			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
			// 断言返回的实体
			assert.Equal(t, tc.want, got) // ShouldResemble用于比较结构体内容
		})
	}
}

// TestEvaluatorServiceImpl_BatchGetEvaluatorVersion 测试 BatchGetEvaluatorVersion 方法
func TestEvaluatorServiceImpl_BatchGetEvaluatorVersion(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo: mockEvaluatorRepo,
	}
	ctx := context.Background()

	type args struct {
		evaluatorVersionIDs []int64
		includeDeleted      bool
	}
	testCases := []struct {
		name      string
		args      args
		setupMock func(mockRepo *repomocks.MockIEvaluatorRepo, args args)
		want      []*entity.Evaluator
		wantErr   error
	}{
		{
			name: "成功 - 找到多个评估器版本",
			args: args{evaluatorVersionIDs: []int64{10, 20}, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted).
					Return([]*entity.Evaluator{
						{ID: 10, Name: "Evaluator Version 10"},
						{ID: 20, Name: "Evaluator Version 20"},
					}, nil)
			},
			want: []*entity.Evaluator{
				{ID: 10, Name: "Evaluator Version 10"},
				{ID: 20, Name: "Evaluator Version 20"},
			},
			wantErr: nil,
		},
		{
			name: "成功 - 传入空ID列表 (repo应返回空列表)",
			args: args{evaluatorVersionIDs: []int64{}, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted).
					Return([]*entity.Evaluator{}, nil) // 期望repo对于空ID列表返回空列表
			},
			want:    []*entity.Evaluator{},
			wantErr: nil,
		},
		{
			name: "成功 - 未找到任何评估器版本 (repo返回空列表)",
			args: args{evaluatorVersionIDs: []int64{999}, includeDeleted: true},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted).
					Return([]*entity.Evaluator{}, nil)
			},
			want:    []*entity.Evaluator{},
			wantErr: nil,
		},
		{
			name: "失败 - evaluatorRepo.BatchGetEvaluatorByVersionID 返回错误",
			args: args{evaluatorVersionIDs: []int64{30}, includeDeleted: false},
			setupMock: func(mockRepo *repomocks.MockIEvaluatorRepo, args args) {
				mockRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), gomock.Eq(args.evaluatorVersionIDs), args.includeDeleted).
					Return(nil, errors.New("batch repo database error"))
			},
			want:    nil,
			wantErr: errors.New("batch repo database error"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mockEvaluatorRepo, tc.args)

			got, err := s.BatchGetEvaluatorVersion(ctx, nil, tc.args.evaluatorVersionIDs, tc.args.includeDeleted)

			if tc.wantErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.wantErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.want, got) // ShouldResemble用于比较slice内容
		})
	}
}

// TestEvaluatorServiceImpl_SubmitEvaluatorVersion 使用 gomock 对 SubmitEvaluatorVersion 方法进行单元测试
func TestEvaluatorServiceImpl_SubmitEvaluatorVersion(t *testing.T) {
	mockUserID := "test-user-id"
	mockGeneratedVersionID := int64(12345)
	mockEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt, // 确保 GetEvaluatorVersion 能工作
		// PromptEvaluatorVersion 将在 setupMocks 中被 mock 的 IEvaluatorVersion "替换"其行为
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: 1,
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}

	// Test cases
	testCases := []struct {
		name            string
		evaluatorDO     *entity.Evaluator // 输入的 Evaluator 实体
		version         string
		description     string
		cid             string
		setupMocks      func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, mockEvalVersion *entitymocks.MockIEvaluatorVersion, inputEvaluatorDO *entity.Evaluator)
		expectedEvalDO  *entity.Evaluator // 期望返回的 Evaluator 实体
		expectedErrCode int32             // 期望的错误码，0表示无错误
		expectedErrMsg  string            // 期望的错误信息中的特定子串
		expectPanic     bool
	}{
		{
			name:        "成功提交新版本",
			evaluatorDO: mockEvaluatorDO,
			version:     "v1.0.0",
			description: "Initial version",
			cid:         "client-id-1",
			setupMocks: func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, mockEvalVersion *entitymocks.MockIEvaluatorVersion, inputEvaluatorDO *entity.Evaluator) {
				// 1. Mock idem.Set
				mockIdem.EXPECT().Set(gomock.Any(), consts.IdemKeySubmitEvaluator+"client-id-1", time.Second*10).Return(nil)
				// 2. Mock idgen.GenID
				mockIdgen.EXPECT().GenID(gomock.Any()).Return(mockGeneratedVersionID, nil)
				session.WithCtxUser(context.Background(), &session.User{ID: mockUserID})

				mockRepo.EXPECT().CheckVersionExist(gomock.Any(), inputEvaluatorDO.ID, "v1.0.0").Return(false, nil)
				// 7. Mock time.Now
				// 10. Mock repo.SubmitEvaluatorVersion
				mockRepo.EXPECT().SubmitEvaluatorVersion(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, submittedDO *entity.Evaluator) error {
						return nil
					})
			},
			expectedEvalDO:  mockEvaluatorDO,
			expectedErrCode: 0,
		},
		{
			name: "失败 - 幂等性检查失败",
			evaluatorDO: &entity.Evaluator{
				ID:            101,
				EvaluatorType: entity.EvaluatorTypePrompt,
			},
			version:     "v1.0.0",
			description: "Desc",
			cid:         "client-id-2",
			setupMocks: func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, mockEvalVersion *entitymocks.MockIEvaluatorVersion, inputEvaluatorDO *entity.Evaluator) {
				mockIdem.EXPECT().Set(gomock.Any(), consts.IdemKeySubmitEvaluator+"client-id-2", time.Second*10).Return(errors.New("idem set error"))
			},
			expectedErrCode: errno.ActionRepeatedCode,
			expectedErrMsg:  "idempotent error",
		},
		{
			name:        "失败 - ID生成失败",
			evaluatorDO: mockEvaluatorDO,
			version:     "v1.0.0",
			description: "Desc",
			cid:         "client-id-3",
			setupMocks: func(ctrl *gomock.Controller, mockIdem *idemmocks.MockIdempotentService, mockIdgen *idgenmocks.MockIIDGenerator, mockRepo *repomocks.MockIEvaluatorRepo, mockEvalVersion *entitymocks.MockIEvaluatorVersion, inputEvaluatorDO *entity.Evaluator) {
				mockIdem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockIdgen.EXPECT().GenID(gomock.Any()).Return(int64(1), errors.New("gen id error"))
			},
			expectedErrCode: -1, // 函数直接返回 err，不是 errorx 类型
			expectedErrMsg:  "gen id error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockIdemService := idemmocks.NewMockIdempotentService(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockEvalRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
			mockEvalVersion := entitymocks.NewMockIEvaluatorVersion(ctrl)

			s := &EvaluatorServiceImpl{
				evaluatorRepo: mockEvalRepo,
				idem:          mockIdemService,
				idgen:         mockIDGen,
			}

			if tc.setupMocks != nil {
				tc.setupMocks(ctrl, mockIdemService, mockIDGen, mockEvalRepo, mockEvalVersion, tc.evaluatorDO)
			}

			returnedEvalDO, err := s.SubmitEvaluatorVersion(context.Background(), tc.evaluatorDO, tc.version, tc.description, tc.cid)

			if tc.expectedErrCode != 0 {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tc.expectedEvalDO != nil {
				assert.Equal(t, returnedEvalDO.ID, tc.expectedEvalDO.ID)
				assert.Equal(t, returnedEvalDO.LatestVersion, tc.expectedEvalDO.LatestVersion)
				assert.Equal(t, returnedEvalDO.DraftSubmitted, tc.expectedEvalDO.DraftSubmitted)
				if tc.expectedEvalDO.BaseInfo != nil && returnedEvalDO.BaseInfo != nil {
					if tc.expectedEvalDO.BaseInfo.UpdatedBy != nil && returnedEvalDO.BaseInfo.UpdatedBy != nil {
						assert.Equal(t, *returnedEvalDO.BaseInfo.UpdatedBy.UserID, *tc.expectedEvalDO.BaseInfo.UpdatedBy.UserID)
					}
				}
			}
		})
	}
}

func TestEvaluatorServiceImpl_RunEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	s := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		// mqFactory, idem, configer 可以为 nil 或根据需要 mock
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService, // 假设这是一个 mock 的 PromptEvaluatorSourceService
		},
	}

	ctx := context.Background()

	defaultRequest := &entity.RunEvaluatorRequest{
		SpaceID:            1,
		EvaluatorVersionID: 101,
		InputData:          &entity.EvaluatorInputData{ /* ... */ },
		ExperimentID:       201,
		ItemID:             301,
		TurnID:             401,
		Ext:                map[string]string{"key": "value"},
	}

	defaultEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt, // 确保 GetEvaluatorVersion 能工作
		// PromptEvaluatorVersion 将在 setupMocks 中被 mock 的 IEvaluatorVersion "替换"其行为
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: 1,
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}

	defaultOutputData := &entity.EvaluatorOutputData{ /* ... */ }
	defaultRunStatus := entity.EvaluatorRunStatusSuccess
	defaultRecordID := int64(999)
	defaultUserID := "user-test-id"
	defaultLogID := "log-id-abc"

	testCases := []struct {
		name            string
		request         *entity.RunEvaluatorRequest
		setupMocks      func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService)
		expectedRecord  *entity.EvaluatorRecord
		expectedErr     error
		expectedErrCode int32 // 用于校验 errorx 类型的错误
	}{
		{
			name:    "成功运行评估器",
			request: defaultRequest,
			setupMocks: func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), []int64{defaultRequest.EvaluatorVersionID}, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), defaultRequest.SpaceID).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				session.WithCtxUser(ctx, &session.User{ID: defaultUserID})
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, defaultRequest.InputData).Return(defaultOutputData, defaultRunStatus, "trace-id-123")

				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).DoAndReturn(
					func(ctx context.Context, record *entity.EvaluatorRecord) error {
						assert.Equal(t, record.ID, defaultRecordID)
						assert.Equal(t, record.SpaceID, defaultRequest.SpaceID)
						assert.Equal(t, record.EvaluatorVersionID, defaultRequest.EvaluatorVersionID)
						assert.Equal(t, record.Status, defaultRunStatus)
						return nil
					})
			},
			expectedRecord: &entity.EvaluatorRecord{
				ID:                  defaultRecordID,
				SpaceID:             defaultRequest.SpaceID,
				ExperimentID:        defaultRequest.ExperimentID,
				ExperimentRunID:     defaultRequest.ExperimentRunID,
				ItemID:              defaultRequest.ItemID,
				TurnID:              defaultRequest.TurnID,
				EvaluatorVersionID:  defaultRequest.EvaluatorVersionID,
				LogID:               defaultLogID,
				EvaluatorInputData:  defaultRequest.InputData,
				EvaluatorOutputData: defaultOutputData,
				Status:              defaultRunStatus,
				Ext:                 defaultRequest.Ext,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: gptr.Of(defaultUserID)},
				},
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks(mockEvaluatorSourceService)
			}

			record, err := s.RunEvaluator(ctx, tc.request)

			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}

			// 使用 ShouldResemble 比较结构体，它会递归比较字段值
			assert.Equal(t, tc.expectedRecord.ID, record.ID)
			assert.Equal(t, tc.expectedRecord.SpaceID, record.SpaceID)
			assert.Equal(t, tc.expectedRecord.EvaluatorVersionID, record.EvaluatorVersionID)
			assert.Equal(t, tc.expectedRecord.Status, record.Status)
			assert.Equal(t, tc.expectedRecord.Ext, record.Ext)
		})
	}
}

func Test_EvaluatorServiceImpl_DebugEvaluator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockService := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		// mqFactory, idem, configer 可以为 nil 或根据需要 mock
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService, // 假设这是一个 mock 的 PromptEvaluatorSourceService
		},
	}

	defaultOutputData := &entity.EvaluatorOutputData{ /* ... */ }
	mockEvaluator := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt, // 确保 GetEvaluatorVersion 能工作
		// PromptEvaluatorVersion 将在 setupMocks 中被 mock 的 IEvaluatorVersion "替换"其行为
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			ID:                100,
			EvaluatorID:       100,
			SpaceID:           1,
			PromptTemplateKey: "test-template-key",
			PromptSuffix:      "test-prompt-suffix",
			ModelConfig: &entity.ModelConfig{
				ModelID: 1,
			},
			ParseType: entity.ParseTypeFunctionCall,
			MessageList: []*entity.Message{
				{
					Role: entity.RoleSystem,
					Content: &entity.Content{
						ContentType: ptr.Of(entity.ContentTypeText),
						Text:        ptr.Of("test-content"),
					},
				},
			},
			InputSchemas: []*entity.ArgsSchema{
				{
					Key:        ptr.Of("test-input-key"),
					JsonSchema: ptr.Of("test-json-schema"),
					SupportContentTypes: []entity.ContentType{
						entity.ContentTypeText,
					},
				},
			},
		},
	}
	testCases := []struct {
		name            string
		request         *entity.RunEvaluatorRequest
		setupMocks      func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService)
		expectedErr     error
		expectedErrCode int32 // 用于校验 errorx 类型的错误
	}{
		{
			name: "成功调试评估器",
			request: &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{ /*... */ },
				ExperimentID:       201,
				ItemID:             301,
				TurnID:             401,
				Ext:                map[string]string{"key": "value"},
			},
			setupMocks: func(mockEvaluatorSourceService *mocks.MockEvaluatorSourceService) {
				mockEvaluatorSourceService.EXPECT().PreHandle(ctx, mockEvaluator).Return(nil)
				mockEvaluatorSourceService.EXPECT().Debug(ctx, mockEvaluator, gomock.Any()).Return(defaultOutputData, nil)
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks(mockEvaluatorSourceService)
			}
			outputData, err := mockService.DebugEvaluator(ctx, mockEvaluator, tc.request.InputData)
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NotNil(t, outputData)
		})
	}
}

func Test_EvaluatorServiceImpl_injectUserInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.Background()
	mockEvaluatorRepo := repomocks.NewMockIEvaluatorRepo(ctrl)
	mockLimiter := repomocks.NewMockRateLimiter(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repomocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockEvaluatorSourceService := mocks.NewMockEvaluatorSourceService(ctrl)
	mockService := &EvaluatorServiceImpl{
		evaluatorRepo:       mockEvaluatorRepo,
		limiter:             mockLimiter,
		idgen:               mockIDGen,
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		// mqFactory, idem, configer 可以为 nil 或根据需要 mock
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService, // 假设这是一个 mock 的 PromptEvaluatorSourceService
		},
	}
	mockEvaluator := &entity.Evaluator{
		EvaluatorType: entity.EvaluatorTypePrompt,
		PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
			BaseInfo: nil,
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{UserID: gptr.Of("user-test-id")},
			UpdatedBy: &entity.UserInfo{UserID: gptr.Of("user-test-id")},
			UpdatedAt: gptr.Of(time.Now().UnixMilli()),
			CreatedAt: gptr.Of(time.Now().UnixMilli()),
		},
	}
	mockService.injectUserInfo(ctx, mockEvaluator)
	assert.NotNil(t, mockEvaluator.BaseInfo.CreatedBy.UserID)
	assert.NotNil(t, mockEvaluator.BaseInfo.UpdatedBy.UserID)
	assert.NotNil(t, mockEvaluator.BaseInfo.UpdatedAt)
	assert.NotNil(t, mockEvaluator.BaseInfo.CreatedAt)
}
