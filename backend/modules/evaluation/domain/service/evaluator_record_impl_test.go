// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert" // 新增 testify/assert
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	userinfo_mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/userinfo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks" // 假设gomock生成的mock在此路径
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// TestEvaluatorRecordServiceImpl_CorrectEvaluatorRecord 用于测试 CorrectEvaluatorRecord 方法
func TestEvaluatorRecordServiceImpl_CorrectEvaluatorRecord(t *testing.T) {
	// 定义固定的时间，用于mock time.Now()
	// fixedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC) // 不再需要 mock time.Now()
	// fixedTimeMillis := fixedTime.UnixMilli() // 不再需要
	testUserID := "test_user_id_123"

	// 定义测试用例结构体
	type fields struct {
		mockEvaluatorRecordRepo     *repo_mocks.MockIEvaluatorRecordRepo // evaluatorRecordRepo 的 mock 对象
		mockExptEventPublisher      *mocks.MockExptEventPublisher        // exptPublisher 的 mock 对象
		mockEvaluatorEventPublisher *mocks.MockEvaluatorEventPublisher   // evaluatorPublisher 的 mock 对象
		mockExptRepo                *repo_mocks.MockIExperimentRepo      // exptRepo 的 mock 对象
	}
	type args struct {
		ctx               context.Context
		evaluatorRecordDO *entity.EvaluatorRecord
		correctionDO      *entity.Correction
	}
	tests := []struct {
		name             string                                   // 测试用例名称
		args             args                                     // 输入参数
		prepareMock      func(t *testing.T, f *fields, args args) // mock准备函数，增加 t *testing.T
		wantErr          bool                                     // 是否期望错误
		expectedErr      error                                    // 期望的错误类型
		checkSideEffects func(t *testing.T, args args)            // 检查副作用的函数，增加 t *testing.T
	}{
		{
			name: "成功修正评估记录 - 所有字段都已初始化",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}), // 直接在ctx中设置用户
				evaluatorRecordDO: &entity.EvaluatorRecord{
					ID:                 1,
					SpaceID:            100,
					ExperimentID:       200,
					EvaluatorVersionID: 300,
					EvaluatorOutputData: &entity.EvaluatorOutputData{
						EvaluatorResult: &entity.EvaluatorResult{},
					},
					BaseInfo: &entity.BaseInfo{
						CreatedAt: gptr.Of(time.Now().UnixMilli() - 1000), // 时间不再固定
					},
					Ext: map[string]string{"key": "value"},
				},
				correctionDO: &entity.Correction{
					Score:   gptr.Of(0.9),
					Explain: "Looks good",
				},
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				// Mock session.UserIDInCtxOrEmpty - 已通过 session.WithCtxUser 处理
				// Mock time.Now() - 已移除
				f.mockEvaluatorRecordRepo.EXPECT().CorrectEvaluatorRecord(gomock.Any(), gomock.AssignableToTypeOf(&entity.EvaluatorRecord{})).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockEvaluatorEventPublisher.EXPECT().PublishEvaluatorRecordCorrection(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
					ID:       200,
					ExptType: entity.ExptType_Online,
				}, nil).Times(1)
				// 补充 PublishExptTurnResultFilterEvent 的模拟调用
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Nil()).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), ptr.Of(10*time.Second)).Return(nil).Times(1)
			},
			wantErr: false,
			checkSideEffects: func(t *testing.T, args args) {
				assert.Equal(t, testUserID, args.correctionDO.UpdatedBy)
				assert.EqualValues(t, args.correctionDO, args.evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult.Correction) // 使用 EqualValues 比较结构体
				assert.NotNil(t, args.evaluatorRecordDO.BaseInfo.UpdatedBy)
				assert.Equal(t, testUserID, *args.evaluatorRecordDO.BaseInfo.UpdatedBy.UserID)
				assert.NotZero(t, *args.evaluatorRecordDO.BaseInfo.UpdatedAt) // 验证时间已更新，但不比较具体值
			},
		},
		{
			name: "成功修正评估记录 - EvaluatorOutputData 和 BaseInfo 为 nil",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				evaluatorRecordDO: &entity.EvaluatorRecord{
					ID:                 2,
					SpaceID:            101,
					ExperimentID:       201,
					EvaluatorVersionID: 301,
				},
				correctionDO: &entity.Correction{
					Score:   gptr.Of(0.8),
					Explain: "Needs improvement",
				},
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				f.mockEvaluatorRecordRepo.EXPECT().CorrectEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockEvaluatorEventPublisher.EXPECT().PublishEvaluatorRecordCorrection(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
					ID:       201,
					ExptType: entity.ExptType_Online,
				}, nil).Times(1)
				// 补充 PublishExptTurnResultFilterEvent 的模拟调用
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Nil()).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), ptr.Of(10*time.Second)).Return(nil).Times(1)
			},
			wantErr: false,
			checkSideEffects: func(t *testing.T, args args) {
				assert.Equal(t, testUserID, args.correctionDO.UpdatedBy)
				assert.NotNil(t, args.evaluatorRecordDO.EvaluatorOutputData)
				assert.NotNil(t, args.evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult)
				assert.EqualValues(t, args.correctionDO, args.evaluatorRecordDO.EvaluatorOutputData.EvaluatorResult.Correction)
				assert.NotNil(t, args.evaluatorRecordDO.BaseInfo)
				assert.NotNil(t, args.evaluatorRecordDO.BaseInfo.UpdatedBy)
				assert.Equal(t, testUserID, *args.evaluatorRecordDO.BaseInfo.UpdatedBy.UserID)
				assert.NotZero(t, *args.evaluatorRecordDO.BaseInfo.UpdatedAt)
			},
		},
		{
			name: "失败 - evaluatorRecordRepo.CorrectEvaluatorRecord 返回错误",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				evaluatorRecordDO: &entity.EvaluatorRecord{
					ID: 3,
				},
				correctionDO: &entity.Correction{Score: gptr.Of(0.5)},
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				f.mockEvaluatorRecordRepo.EXPECT().CorrectEvaluatorRecord(gomock.Any(), gomock.Any()).Return(errors.New("db error")).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				f.mockEvaluatorEventPublisher.EXPECT().PublishEvaluatorRecordCorrection(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				// 因为 CorrectEvaluatorRecord 失败，不会调用 PublishExptTurnResultFilterEvent
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr:     true,
			expectedErr: errors.New("db error"),
		},
		{
			name: "失败 - exptPublisher.PublishExptAggrCalculateEvent 返回错误 (日志记录)",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				evaluatorRecordDO: &entity.EvaluatorRecord{
					ID:                 4,
					SpaceID:            102,
					ExperimentID:       202,
					EvaluatorVersionID: 302,
				},
				correctionDO: &entity.Correction{Score: gptr.Of(0.7)},
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				f.mockEvaluatorRecordRepo.EXPECT().CorrectEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish aggr error")).Times(1)
				f.mockEvaluatorEventPublisher.EXPECT().PublishEvaluatorRecordCorrection(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
					ID:       202,
					ExptType: entity.ExptType_Online,
				}, nil).Times(1)
				// 补充 PublishExptTurnResultFilterEvent 的模拟调用
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Nil()).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), ptr.Of(10*time.Second)).Return(nil).Times(1)
			},
			wantErr: false, // PublishExptAggrCalculateEvent 错误被捕获并记录日志，不向上层返回错误
			checkSideEffects: func(t *testing.T, args args) {
				// 无法直接验证 logs.CtxError 调用次数，除非有特定的 mock 机制
			},
		},
		{
			name: "失败 - evaluatorPublisher.PublishEvaluatorRecordCorrection 返回错误",
			args: args{
				ctx: session.WithCtxUser(context.Background(), &session.User{ID: testUserID}),
				evaluatorRecordDO: &entity.EvaluatorRecord{
					ID:                 5,
					SpaceID:            103,
					ExperimentID:       203,
					EvaluatorVersionID: 303,
					BaseInfo:           &entity.BaseInfo{CreatedAt: gptr.Of(time.Now().Add(-time.Hour).UnixMilli())},
				},
				correctionDO: &entity.Correction{Score: gptr.Of(0.6)},
			},
			prepareMock: func(t *testing.T, f *fields, args args) {
				f.mockEvaluatorRecordRepo.EXPECT().CorrectEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockExptEventPublisher.EXPECT().PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
				f.mockEvaluatorEventPublisher.EXPECT().PublishEvaluatorRecordCorrection(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("publish correction error")).Times(1)
				f.mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.Experiment{
					ID:       202,
					ExptType: entity.ExptType_Online,
				}, nil).Times(1)
				// 因为 PublishEvaluatorRecordCorrection 失败，后面的 PublishExptTurnResultFilterEvent 不会调用
				f.mockExptEventPublisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantErr:     true,
			expectedErr: errors.New("publish correction error"),
		},
	}

	for _, tt := range tests {
		tt := tt                            // capture range variable
		t.Run(tt.name, func(t *testing.T) { // 使用 t.Run
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			f := fields{
				mockEvaluatorRecordRepo:     repo_mocks.NewMockIEvaluatorRecordRepo(ctrl),
				mockExptEventPublisher:      mocks.NewMockExptEventPublisher(ctrl),
				mockEvaluatorEventPublisher: mocks.NewMockEvaluatorEventPublisher(ctrl),
				mockExptRepo:                repo_mocks.NewMockIExperimentRepo(ctrl),
			}

			if tt.prepareMock != nil {
				tt.prepareMock(t, &f, tt.args) // 传递 t
			}

			s := &EvaluatorRecordServiceImpl{
				evaluatorRecordRepo: f.mockEvaluatorRecordRepo,
				exptPublisher:       f.mockExptEventPublisher,
				evaluatorPublisher:  f.mockEvaluatorEventPublisher,
				exptRepo:            f.mockExptRepo,
			}

			err := s.CorrectEvaluatorRecord(tt.args.ctx, tt.args.evaluatorRecordDO, tt.args.correctionDO)

			if tt.wantErr {
				assert.Error(t, err) // 使用 testify 断言
				if tt.expectedErr != nil {
					assert.EqualError(t, err, tt.expectedErr.Error())
				}
			} else {
				assert.NoError(t, err) // 使用 testify 断言
			}

			if tt.checkSideEffects != nil {
				tt.checkSideEffects(t, tt.args) // 传递 t
			}
		})
	}
}

// TestNewEvaluatorRecordServiceImpl 测试构造函数
func TestNewEvaluatorRecordServiceImpl(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIdgen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockEvaluatorRecordRepo := repo_mocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockExptEventPublisher := mocks.NewMockExptEventPublisher(ctrl)
	mockEvaluatorEventPublisher := mocks.NewMockEvaluatorEventPublisher(ctrl)
	mockUserInfoService := userinfo_mocks.NewMockUserInfoService(ctrl)

	mockExptRepo := repo_mocks.NewMockIExperimentRepo(ctrl)

	service := NewEvaluatorRecordServiceImpl(
		mockIdgen,
		mockEvaluatorRecordRepo,
		mockExptEventPublisher,
		mockEvaluatorEventPublisher,
		mockUserInfoService,
		mockExptRepo,
	)
	assert.NotNil(t, service)
}

// TestEvaluatorRecordServiceImpl_GetEvaluatorRecord 测试 GetEvaluatorRecord 方法
func TestEvaluatorRecordServiceImpl_GetEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRecordRepo := repo_mocks.NewMockIEvaluatorRecordRepo(ctrl)
	s := &EvaluatorRecordServiceImpl{
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
	}
	ctx := context.Background()

	t.Run("成功获取评估记录", func(t *testing.T) {
		mockRecord := &entity.EvaluatorRecord{ID: 1, SpaceID: 100}
		mockEvaluatorRecordRepo.EXPECT().GetEvaluatorRecord(ctx, int64(1), false).Return(mockRecord, nil)
		record, err := s.GetEvaluatorRecord(ctx, 1, false)
		assert.NoError(t, err)
		assert.Equal(t, mockRecord, record)
	})

	t.Run("获取评估记录失败", func(t *testing.T) {
		mockEvaluatorRecordRepo.EXPECT().GetEvaluatorRecord(ctx, int64(2), true).Return(nil, errors.New("db error"))
		record, err := s.GetEvaluatorRecord(ctx, 2, true)
		assert.Error(t, err)
		assert.Nil(t, record)
	})
}

// TestEvaluatorRecordServiceImpl_BatchGetEvaluatorRecord 测试 BatchGetEvaluatorRecord 方法
func TestEvaluatorRecordServiceImpl_BatchGetEvaluatorRecord(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvaluatorRecordRepo := repo_mocks.NewMockIEvaluatorRecordRepo(ctrl)
	mockUserInfoService := userinfo_mocks.NewMockUserInfoService(ctrl)
	s := &EvaluatorRecordServiceImpl{
		evaluatorRecordRepo: mockEvaluatorRecordRepo,
		userInfoService:     mockUserInfoService,
	}
	ctx := context.Background()

	t.Run("成功批量获取评估记录", func(t *testing.T) {
		records := []*entity.EvaluatorRecord{
			{ID: 1, SpaceID: 100},
			{ID: 2, SpaceID: 101},
		}
		mockEvaluatorRecordRepo.EXPECT().BatchGetEvaluatorRecord(ctx, []int64{1, 2}, false).Return(records, nil)
		mockUserInfoService.EXPECT().PackUserInfo(ctx, gomock.Any()).Return()
		result, err := s.BatchGetEvaluatorRecord(ctx, []int64{1, 2}, false)
		assert.NoError(t, err)
		assert.Equal(t, records, result)
	})

	t.Run("批量获取评估记录失败", func(t *testing.T) {
		mockEvaluatorRecordRepo.EXPECT().BatchGetEvaluatorRecord(ctx, []int64{3, 4}, true).Return(nil, errors.New("db error"))
		result, err := s.BatchGetEvaluatorRecord(ctx, []int64{3, 4}, true)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
