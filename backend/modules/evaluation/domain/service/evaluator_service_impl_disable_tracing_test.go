// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// TestEvaluatorServiceImpl_RunEvaluator_DisableTracing 测试EvaluatorServiceImpl.RunEvaluator中DisableTracing参数传递
func TestEvaluatorServiceImpl_RunEvaluator_DisableTracing(t *testing.T) {
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
		evaluatorSourceServices: map[entity.EvaluatorType]EvaluatorSourceService{
			entity.EvaluatorTypePrompt: mockEvaluatorSourceService,
		},
	}

	ctx := context.Background()
	session.WithCtxUser(ctx, &session.User{ID: "test-user"})

	defaultEvaluatorDO := &entity.Evaluator{
		ID:            100,
		SpaceID:       1,
		Name:          "Test Evaluator",
		EvaluatorType: entity.EvaluatorTypePrompt,
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
		},
	}

	defaultOutputData := &entity.EvaluatorOutputData{
		EvaluatorResult: &entity.EvaluatorResult{
			Score:     gptr.Of(0.85),
			Reasoning: "Test reasoning",
		},
	}
	defaultRunStatus := entity.EvaluatorRunStatusSuccess
	defaultRecordID := int64(999)

	tests := []struct {
		name           string
		disableTracing bool
		setupMocks     func()
	}{
		{
			name:           "DisableTracing为true时正确传递给EvaluatorSourceService.Run",
			disableTracing: true,
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), []int64{101}, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(1)).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				// 关键验证：确保DisableTracing参数正确传递
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, gomock.Any(), true).Return(defaultOutputData, defaultRunStatus, "trace-id-123")
				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
		{
			name:           "DisableTracing为false时正确传递给EvaluatorSourceService.Run",
			disableTracing: false,
			setupMocks: func() {
				mockEvaluatorRepo.EXPECT().BatchGetEvaluatorByVersionID(gomock.Any(), gomock.Any(), []int64{101}, false).Return([]*entity.Evaluator{defaultEvaluatorDO}, nil)
				mockLimiter.EXPECT().AllowInvoke(gomock.Any(), int64(1)).Return(true)
				mockIDGen.EXPECT().GenID(gomock.Any()).Return(defaultRecordID, nil)
				mockEvaluatorSourceService.EXPECT().PreHandle(gomock.Any(), defaultEvaluatorDO).Return(nil)
				// 关键验证：确保DisableTracing参数正确传递
				mockEvaluatorSourceService.EXPECT().Run(gomock.Any(), defaultEvaluatorDO, gomock.Any(), false).Return(defaultOutputData, defaultRunStatus, "trace-id-123")
				mockEvaluatorRecordRepo.EXPECT().CreateEvaluatorRecord(gomock.Any(), gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			request := &entity.RunEvaluatorRequest{
				SpaceID:            1,
				EvaluatorVersionID: 101,
				InputData:          &entity.EvaluatorInputData{},
				DisableTracing:     tt.disableTracing,
			}

			record, err := s.RunEvaluator(ctx, request)

			assert.NoError(t, err)
			assert.NotNil(t, record)
			assert.Equal(t, defaultRecordID, record.ID)
			assert.Equal(t, defaultRunStatus, record.Status)
		})
	}
}