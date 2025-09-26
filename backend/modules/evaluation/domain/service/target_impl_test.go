// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestEvalTargetServiceImpl_CreateEvalTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		setupMocks          func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, *idgenmocks.MockIIDGenerator, *mocks.MockEvalTargetMetrics, *servicemocks.MockISourceEvalTargetOperateService)
		spaceID             int64
		sourceTargetID      string
		sourceTargetVersion string
		targetType          entity.EvalTargetType
		wantID              int64
		wantVersionID       int64
		wantErr             bool
	}{
		{
			name: "成功创建评估目标",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *idgenmocks.MockIIDGenerator, *mocks.MockEvalTargetMetrics, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				evalTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockOperator.EXPECT().BuildBySource(gomock.Any(), int64(1), "target_123", "v1.0").Return(evalTarget, nil)
				mockRepo.EXPECT().CreateEvalTarget(gomock.Any(), evalTarget).Return(int64(123), int64(456), nil)
				mockMetric.EXPECT().EmitCreate(int64(1), nil)

				return mockRepo, mockIDGen, mockMetric, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			wantID:              123,
			wantVersionID:       456,
			wantErr:             false,
		},
		{
			name: "不支持的目标类型",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *idgenmocks.MockIIDGenerator, *mocks.MockEvalTargetMetrics, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				mockMetric.EXPECT().EmitCreate(int64(1), gomock.Any())

				return mockRepo, mockIDGen, mockMetric, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          99, // 不支持的类型
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, mockIDGen, mockMetric, mockOperator := tt.setupMocks(ctrl)

			typedOperators := make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			if tt.targetType == entity.EvalTargetTypeCozeBot {
				typedOperators[entity.EvalTargetTypeCozeBot] = mockOperator
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			id, versionID, err := service.CreateEvalTarget(context.Background(), tt.spaceID, tt.sourceTargetID, tt.sourceTargetVersion, tt.targetType)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
				assert.Equal(t, tt.wantVersionID, versionID)
			}
		})
	}
}

func TestEvalTargetServiceImpl_GetEvalTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(*gomock.Controller) *repomocks.MockIEvalTargetRepo
		targetID   int64
		want       *entity.EvalTarget
		wantErr    bool
	}{
		{
			name: "成功获取评估目标",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				
				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTarget(gomock.Any(), int64(123)).Return(expectedTarget, nil)
				return mockRepo
			},
			targetID: 123,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
			},
			wantErr: false,
		},
		{
			name: "获取失败",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().GetEvalTarget(gomock.Any(), int64(999)).Return(nil, errors.New("not found"))
				return mockRepo
			},
			targetID: 999,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, nil)

			result, err := service.GetEvalTarget(context.Background(), tt.targetID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestEvalTargetServiceImpl_GenerateMockOutputData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		outputSchemas []*entity.ArgsSchema
		want          map[string]string
		wantErr       bool
	}{
		{
			name:          "空schema列表",
			outputSchemas: []*entity.ArgsSchema{},
			want:          map[string]string{},
			wantErr:       false,
		},
		{
			name: "有效schema生成mock数据",
			outputSchemas: []*entity.ArgsSchema{
				{
					Key:        gptr.Of("output1"),
					JsonSchema: gptr.Of(`{"type": "string"}`),
				},
				{
					Key:        gptr.Of("output2"),
					JsonSchema: gptr.Of(`{"type": "number"}`),
				},
			},
			want:    map[string]string{}, // 实际内容由jsonmock生成，这里只验证不为空
			wantErr: false,
		},
		{
			name: "无效schema使用默认值",
			outputSchemas: []*entity.ArgsSchema{
				{
					Key:        gptr.Of("invalid_output"),
					JsonSchema: gptr.Of(`invalid json`),
				},
			},
			want: map[string]string{
				"invalid_output": "{}",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, nil)

			result, err := service.GenerateMockOutputData(tt.outputSchemas)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, len(tt.outputSchemas), len(result))
				
				// 对于有效的schema，验证生成的数据不为空
				for _, schema := range tt.outputSchemas {
					if schema.Key != nil {
						value, exists := result[*schema.Key]
						assert.True(t, exists)
						assert.NotEmpty(t, value)
					}
				}
			}
		})
	}
}

func TestNewEvalTargetServiceImpl(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
	mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
	mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)
	typedOperators := make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)

	service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

	assert.NotNil(t, service)
	
	impl, ok := service.(*EvalTargetServiceImpl)
	assert.True(t, ok)
	assert.Equal(t, mockRepo, impl.evalTargetRepo)
	assert.Equal(t, mockIDGen, impl.idgen)
	assert.Equal(t, mockMetric, impl.metric)
	assert.Equal(t, typedOperators, impl.typedOperators)
}