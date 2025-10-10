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

func TestEvaluatorTargetServiceImpl_CreateEvalTarget(t *testing.T) {
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
		{
			name: "BuildBySource返回错误",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *idgenmocks.MockIIDGenerator, *mocks.MockEvalTargetMetrics, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				mockOperator.EXPECT().BuildBySource(gomock.Any(), int64(1), "target_123", "v1.0").Return(nil, errors.New("build error"))
				mockMetric.EXPECT().EmitCreate(int64(1), gomock.Any())

				return mockRepo, mockIDGen, mockMetric, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			wantErr:             true,
		},
		{
			name: "BuildBySource返回nil",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *idgenmocks.MockIIDGenerator, *mocks.MockEvalTargetMetrics, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
				mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				mockOperator.EXPECT().BuildBySource(gomock.Any(), int64(1), "target_123", "v1.0").Return(nil, nil)
				mockMetric.EXPECT().EmitCreate(int64(1), gomock.Any())

				return mockRepo, mockIDGen, mockMetric, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			wantErr:             true,
		},
		{
			name: "CreateEvalTarget失败",
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
				mockRepo.EXPECT().CreateEvalTarget(gomock.Any(), evalTarget).Return(int64(0), int64(0), errors.New("repo error"))
				mockMetric.EXPECT().EmitCreate(int64(1), gomock.Any())

				return mockRepo, mockIDGen, mockMetric, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
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

func TestEvalTargetServiceImpl_GetEvalTargetVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService)
		spaceID        int64
		versionID      int64
		needSourceInfo bool
		want           *entity.EvalTarget
		wantErr        bool
	}{
		{
			name: "成功获取版本信息_不需要源信息",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(456)).Return(expectedTarget, nil)
				return mockRepo, mockOperator
			},
			spaceID:        1,
			versionID:      456,
			needSourceInfo: false,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
			},
			wantErr: false,
		},
		{
			name: "成功获取版本信息_需要源信息",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(456)).Return(expectedTarget, nil)
				mockOperator.EXPECT().PackSourceVersionInfo(gomock.Any(), int64(1), []*entity.EvalTarget{expectedTarget}).Return(nil)

				return mockRepo, mockOperator
			},
			spaceID:        1,
			versionID:      456,
			needSourceInfo: true,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
			},
			wantErr: false,
		},
		{
			name: "获取版本失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(999)).Return(nil, errors.New("version not found"))
				return mockRepo, mockOperator
			},
			spaceID:        1,
			versionID:      999,
			needSourceInfo: false,
			wantErr:        true,
		},
		{
			name: "打包源信息失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(456)).Return(expectedTarget, nil)
				mockOperator.EXPECT().PackSourceVersionInfo(gomock.Any(), int64(1), []*entity.EvalTarget{expectedTarget}).Return(errors.New("pack error"))

				return mockRepo, mockOperator
			},
			spaceID:        1,
			versionID:      456,
			needSourceInfo: true,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, mockOperator := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			typedOperators := map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetTypeCozeBot: mockOperator,
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.GetEvalTargetVersion(context.Background(), tt.spaceID, tt.versionID, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_GetEvalTargetVersionBySourceTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		setupMocks          func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService)
		spaceID             int64
		sourceTargetID      string
		sourceTargetVersion string
		targetType          entity.EvalTargetType
		needSourceInfo      bool
		want                *entity.EvalTarget
		wantErr             bool
	}{
		{
			name: "成功通过源目标获取版本_不需要源信息",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTargetVersionBySourceTarget(gomock.Any(), int64(1), "target_123", "v1.0", entity.EvalTargetTypeCozeBot).Return(expectedTarget, nil)
				return mockRepo, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			needSourceInfo:      false,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
			},
			wantErr: false,
		},
		{
			name: "成功通过源目标获取版本_需要源信息",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTargetVersionBySourceTarget(gomock.Any(), int64(1), "target_123", "v1.0", entity.EvalTargetTypeCozeBot).Return(expectedTarget, nil)
				mockOperator.EXPECT().PackSourceVersionInfo(gomock.Any(), int64(1), []*entity.EvalTarget{expectedTarget}).Return(nil)

				return mockRepo, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			needSourceInfo:      true,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
			},
			wantErr: false,
		},
		{
			name: "通过源目标获取失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, *servicemocks.MockISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				mockRepo.EXPECT().GetEvalTargetVersionBySourceTarget(gomock.Any(), int64(1), "invalid_target", "v1.0", entity.EvalTargetTypeCozeBot).Return(nil, errors.New("target not found"))
				return mockRepo, mockOperator
			},
			spaceID:             1,
			sourceTargetID:      "invalid_target",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			needSourceInfo:      false,
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, mockOperator := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			typedOperators := map[entity.EvalTargetType]ISourceEvalTargetOperateService{
				entity.EvalTargetTypeCozeBot: mockOperator,
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.GetEvalTargetVersionBySourceTarget(context.Background(), tt.spaceID, tt.sourceTargetID, tt.sourceTargetVersion, tt.targetType, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService)
		spaceID        int64
		versionID      int64
		needSourceInfo bool
		want           *entity.EvalTarget
		wantErr        bool
	}{
		{
			name: "成功获取版本信息_无需源信息",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  456,
						SourceTargetVersion: "v1.0",
					},
				}

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(456)).Return(expectedTarget, nil)
				return mockRepo, nil
			},
			spaceID:        1,
			versionID:      456,
			needSourceInfo: false,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  456,
					SourceTargetVersion: "v1.0",
				},
			},
			wantErr: false,
		},
		{
			name: "成功获取版本信息_需要源信息",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  456,
						SourceTargetVersion: "v1.0",
					},
				}

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(456)).Return(expectedTarget, nil)
				mockOperator.EXPECT().PackSourceVersionInfo(gomock.Any(), int64(1), []*entity.EvalTarget{expectedTarget}).Return(nil)

				typedOperators := map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeCozeBot: mockOperator,
				}
				return mockRepo, typedOperators
			},
			spaceID:        1,
			versionID:      456,
			needSourceInfo: true,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  456,
					SourceTargetVersion: "v1.0",
				},
			},
			wantErr: false,
		},
		{
			name: "获取版本失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(999)).Return(nil, errors.New("version not found"))
				return mockRepo, nil
			},
			spaceID:        1,
			versionID:      999,
			needSourceInfo: false,
			wantErr:        true,
		},
		{
			name: "包装源信息失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				}

				mockRepo.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(1), int64(456)).Return(expectedTarget, nil)
				mockOperator.EXPECT().PackSourceVersionInfo(gomock.Any(), int64(1), []*entity.EvalTarget{expectedTarget}).Return(errors.New("pack source info failed"))

				typedOperators := map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeCozeBot: mockOperator,
				}
				return mockRepo, typedOperators
			},
			spaceID:        1,
			versionID:      456,
			needSourceInfo: true,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, typedOperators := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			if typedOperators == nil {
				typedOperators = make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.GetEvalTargetVersion(context.Background(), tt.spaceID, tt.versionID, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_MoreEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		setupMocks          func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService)
		spaceID             int64
		sourceTargetID      string
		sourceTargetVersion string
		targetType          entity.EvalTargetType
		needSourceInfo      bool
		want                *entity.EvalTarget
		wantErr             bool
	}{
		{
			name: "成功获取源目标版本",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  456,
						SourceTargetVersion: "v1.0",
					},
				}

				mockRepo.EXPECT().GetEvalTargetVersionBySourceTarget(gomock.Any(), int64(1), "target_123", "v1.0", entity.EvalTargetTypeCozeBot).Return(expectedTarget, nil)
				return mockRepo, nil
			},
			spaceID:             1,
			sourceTargetID:      "target_123",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			needSourceInfo:      false,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_123",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  456,
					SourceTargetVersion: "v1.0",
				},
			},
			wantErr: false,
		},
		{
			name: "获取失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().GetEvalTargetVersionBySourceTarget(gomock.Any(), int64(1), "target_999", "v1.0", entity.EvalTargetTypeCozeBot).Return(nil, errors.New("not found"))
				return mockRepo, nil
			},
			spaceID:             1,
			sourceTargetID:      "target_999",
			sourceTargetVersion: "v1.0",
			targetType:          entity.EvalTargetTypeCozeBot,
			needSourceInfo:      false,
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, typedOperators := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			if typedOperators == nil {
				typedOperators = make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.GetEvalTargetVersionBySourceTarget(context.Background(), tt.spaceID, tt.sourceTargetID, tt.sourceTargetVersion, tt.targetType, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_GetEvalTargetVersionBySource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService)
		spaceID        int64
		targetID       int64
		sourceVersion  string
		needSourceInfo bool
		want           *entity.EvalTarget
		wantErr        bool
	}{
		{
			name: "成功找到匹配版本",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				versions := []*entity.EvalTarget{
					{
						ID:             123,
						SpaceID:        1,
						SourceTargetID: "456",
						EvalTargetType: entity.EvalTargetTypeCozeBot,
						EvalTargetVersion: &entity.EvalTargetVersion{
							ID:                  789,
							SourceTargetVersion: "v1.0",
						},
					},
				}

				mockRepo.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).Return(versions, nil)
				return mockRepo, nil
			},
			spaceID:        1,
			targetID:       456,
			sourceVersion:  "v1.0",
			needSourceInfo: false,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "456",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  789,
					SourceTargetVersion: "v1.0",
				},
			},
			wantErr: false,
		},
		{
			name: "未找到匹配版本",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				versions := []*entity.EvalTarget{
					{
						ID:             123,
						SpaceID:        1,
						SourceTargetID: "456",
						EvalTargetType: entity.EvalTargetTypeCozeBot,
						EvalTargetVersion: &entity.EvalTargetVersion{
							ID:                  789,
							SourceTargetVersion: "v2.0", // 不匹配
						},
					},
				}

				mockRepo.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).Return(versions, nil)
				return mockRepo, nil
			},
			spaceID:        1,
			targetID:       456,
			sourceVersion:  "v1.0",
			needSourceInfo: false,
			wantErr:        true,
		},
		{
			name: "批量查询失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).Return(nil, errors.New("query failed"))
				return mockRepo, nil
			},
			spaceID:        1,
			targetID:       456,
			sourceVersion:  "v1.0",
			needSourceInfo: false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, typedOperators := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			if typedOperators == nil {
				typedOperators = make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.GetEvalTargetVersionBySource(context.Background(), tt.spaceID, tt.targetID, tt.sourceVersion, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_GetEvalTargetVersionByTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		setupMocks          func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService)
		spaceID             int64
		targetID            int64
		sourceTargetVersion string
		needSourceInfo      bool
		want                *entity.EvalTarget
		wantErr             bool
	}{
		{
			name: "成功获取目标版本",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedTarget := &entity.EvalTarget{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_456",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  789,
						SourceTargetVersion: "v1.0",
					},
				}

				mockRepo.EXPECT().GetEvalTargetVersionByTarget(gomock.Any(), int64(1), int64(456), "v1.0").Return(expectedTarget, nil)
				return mockRepo, nil
			},
			spaceID:             1,
			targetID:            456,
			sourceTargetVersion: "v1.0",
			needSourceInfo:      false,
			want: &entity.EvalTarget{
				ID:             123,
				SpaceID:        1,
				SourceTargetID: "target_456",
				EvalTargetType: entity.EvalTargetTypeCozeBot,
				EvalTargetVersion: &entity.EvalTargetVersion{
					ID:                  789,
					SourceTargetVersion: "v1.0",
				},
			},
			wantErr: false,
		},
		{
			name: "获取失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().GetEvalTargetVersionByTarget(gomock.Any(), int64(1), int64(999), "v1.0").Return(nil, errors.New("not found"))
				return mockRepo, nil
			},
			spaceID:             1,
			targetID:            999,
			sourceTargetVersion: "v1.0",
			needSourceInfo:      false,
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, typedOperators := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			if typedOperators == nil {
				typedOperators = make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.GetEvalTargetVersionByTarget(context.Background(), tt.spaceID, tt.targetID, tt.sourceTargetVersion, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_BatchGetEvalTargetBySource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(*gomock.Controller) *repomocks.MockIEvalTargetRepo
		param      *entity.BatchGetEvalTargetBySourceParam
		want       []*entity.EvalTarget
		wantErr    bool
	}{
		{
			name: "成功批量获取",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedTargets := []*entity.EvalTarget{
					{
						ID:             123,
						SpaceID:        1,
						SourceTargetID: "target_123",
						EvalTargetType: entity.EvalTargetTypeCozeBot,
					},
				}

				mockRepo.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).Return(expectedTargets, nil)
				return mockRepo
			},
			param: &entity.BatchGetEvalTargetBySourceParam{
				SpaceID:        1,
				SourceTargetID: []string{"target_123"},
				TargetType:     entity.EvalTargetTypeCozeBot,
			},
			want: []*entity.EvalTarget{
				{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
				},
			},
			wantErr: false,
		},
		{
			name: "查询失败",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().BatchGetEvalTargetBySource(gomock.Any(), gomock.Any()).Return(nil, errors.New("query failed"))
				return mockRepo
			},
			param: &entity.BatchGetEvalTargetBySourceParam{
				SpaceID:        1,
				SourceTargetID: []string{"target_123"},
				TargetType:     entity.EvalTargetTypeCozeBot,
			},
			wantErr: true,
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

			result, err := service.BatchGetEvalTargetBySource(context.Background(), tt.param)

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

func TestEvalTargetServiceImpl_BatchGetEvalTargetVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		setupMocks     func(*gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService)
		spaceID        int64
		versionIDs     []int64
		needSourceInfo bool
		want           []*entity.EvalTarget
		wantErr        bool
	}{
		{
			name: "成功批量获取版本",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedVersions := []*entity.EvalTarget{
					{
						ID:             123,
						SpaceID:        1,
						SourceTargetID: "target_123",
						EvalTargetType: entity.EvalTargetTypeCozeBot,
						EvalTargetVersion: &entity.EvalTargetVersion{
							ID:                  456,
							SourceTargetVersion: "v1.0",
						},
					},
				}

				mockRepo.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(1), []int64{456}).Return(expectedVersions, nil)
				return mockRepo, nil
			},
			spaceID:        1,
			versionIDs:     []int64{456},
			needSourceInfo: false,
			want: []*entity.EvalTarget{
				{
					ID:             123,
					SpaceID:        1,
					SourceTargetID: "target_123",
					EvalTargetType: entity.EvalTargetTypeCozeBot,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID:                  456,
						SourceTargetVersion: "v1.0",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "查询失败",
			setupMocks: func(ctrl *gomock.Controller) (*repomocks.MockIEvalTargetRepo, map[entity.EvalTargetType]ISourceEvalTargetOperateService) {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().BatchGetEvalTargetVersion(gomock.Any(), int64(1), []int64{999}).Return(nil, errors.New("query failed"))
				return mockRepo, nil
			},
			spaceID:        1,
			versionIDs:     []int64{999},
			needSourceInfo: false,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRepo, typedOperators := tt.setupMocks(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			if typedOperators == nil {
				typedOperators = make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			}

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			result, err := service.BatchGetEvalTargetVersion(context.Background(), tt.spaceID, tt.versionIDs, tt.needSourceInfo)

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

func TestEvalTargetServiceImpl_GetRecordByID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(*gomock.Controller) *repomocks.MockIEvalTargetRepo
		spaceID    int64
		recordID   int64
		want       *entity.EvalTargetRecord
		wantErr    bool
	}{
		{
			name: "成功获取记录",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedRecord := &entity.EvalTargetRecord{
					ID:              123,
					SpaceID:         1,
					TargetID:        456,
					TargetVersionID: 789,
				}

				mockRepo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(1), int64(123)).Return(expectedRecord, nil)
				return mockRepo
			},
			spaceID:  1,
			recordID: 123,
			want: &entity.EvalTargetRecord{
				ID:              123,
				SpaceID:         1,
				TargetID:        456,
				TargetVersionID: 789,
			},
			wantErr: false,
		},
		{
			name: "获取失败",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().GetEvalTargetRecordByIDAndSpaceID(gomock.Any(), int64(1), int64(999)).Return(nil, errors.New("not found"))
				return mockRepo
			},
			spaceID:  1,
			recordID: 999,
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

			result, err := service.GetRecordByID(context.Background(), tt.spaceID, tt.recordID)

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

func TestEvalTargetServiceImpl_BatchGetRecordByIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setupMocks func(*gomock.Controller) *repomocks.MockIEvalTargetRepo
		spaceID    int64
		recordIDs  []int64
		want       []*entity.EvalTargetRecord
		wantErr    bool
	}{
		{
			name: "成功批量获取记录",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)

				expectedRecords := []*entity.EvalTargetRecord{
					{
						ID:              123,
						SpaceID:         1,
						TargetID:        456,
						TargetVersionID: 789,
					},
					{
						ID:              124,
						SpaceID:         1,
						TargetID:        457,
						TargetVersionID: 790,
					},
				}

				mockRepo.EXPECT().ListEvalTargetRecordByIDsAndSpaceID(gomock.Any(), int64(1), []int64{123, 124}).Return(expectedRecords, nil)
				return mockRepo
			},
			spaceID:   1,
			recordIDs: []int64{123, 124},
			want: []*entity.EvalTargetRecord{
				{
					ID:              123,
					SpaceID:         1,
					TargetID:        456,
					TargetVersionID: 789,
				},
				{
					ID:              124,
					SpaceID:         1,
					TargetID:        457,
					TargetVersionID: 790,
				},
			},
			wantErr: false,
		},
		{
			name: "spaceID为0",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				// 不应该调用仓储方法
				return mockRepo
			},
			spaceID:   0,
			recordIDs: []int64{123},
			wantErr:   true,
		},
		{
			name: "recordIDs为空",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				// 不应该调用仓储方法
				return mockRepo
			},
			spaceID:   1,
			recordIDs: []int64{},
			wantErr:   true,
		},
		{
			name: "查询失败",
			setupMocks: func(ctrl *gomock.Controller) *repomocks.MockIEvalTargetRepo {
				mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
				mockRepo.EXPECT().ListEvalTargetRecordByIDsAndSpaceID(gomock.Any(), int64(1), []int64{999}).Return(nil, errors.New("query failed"))
				return mockRepo
			},
			spaceID:   1,
			recordIDs: []int64{999},
			wantErr:   true,
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

			result, err := service.BatchGetRecordByIDs(context.Background(), tt.spaceID, tt.recordIDs)

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

func TestEvalTargetServiceImpl_ValidateRuntimeParam(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupMocks   func(*gomock.Controller) map[entity.EvalTargetType]ISourceEvalTargetOperateService
		targetType   entity.EvalTargetType
		runtimeParam string
		wantErr      bool
	}{
		{
			name: "空参数直接返回成功",
			setupMocks: func(ctrl *gomock.Controller) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				return make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			},
			targetType:   entity.EvalTargetTypeCozeBot,
			runtimeParam: "",
			wantErr:      false,
		},
		{
			name: "成功验证运行时参数",
			setupMocks: func(ctrl *gomock.Controller) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)
				mockRuntimeParam := &entity.DummyRuntimeParam{}

				mockOperator.EXPECT().RuntimeParam().Return(mockRuntimeParam)

				return map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeCozeBot: mockOperator,
				}
			},
			targetType:   entity.EvalTargetTypeCozeBot,
			runtimeParam: `{"timeout": 30}`,
			wantErr:      false,
		},
		{
			name: "不支持的目标类型",
			setupMocks: func(ctrl *gomock.Controller) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				return make(map[entity.EvalTargetType]ISourceEvalTargetOperateService)
			},
			targetType:   99, // 不存在的类型
			runtimeParam: `{"timeout": 30}`,
			wantErr:      true,
		},
		{
			name: "JSON解析失败",
			setupMocks: func(ctrl *gomock.Controller) map[entity.EvalTargetType]ISourceEvalTargetOperateService {
				// 创建一个自定义的RuntimeParam实现来模拟解析失败
				mockOperator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)

				// 使用真实的PromptRuntimeParam来测试解析错误
				mockOperator.EXPECT().RuntimeParam().Return(entity.NewPromptRuntimeParam(nil))

				return map[entity.EvalTargetType]ISourceEvalTargetOperateService{
					entity.EvalTargetTypeCozeBot: mockOperator,
				}
			},
			targetType:   entity.EvalTargetTypeCozeBot,
			runtimeParam: `invalid json syntax`,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			typedOperators := tt.setupMocks(ctrl)
			mockRepo := repomocks.NewMockIEvalTargetRepo(ctrl)
			mockIDGen := idgenmocks.NewMockIIDGenerator(ctrl)
			mockMetric := mocks.NewMockEvalTargetMetrics(ctrl)

			service := NewEvalTargetServiceImpl(mockRepo, mockIDGen, mockMetric, typedOperators)

			err := service.ValidateRuntimeParam(context.Background(), tt.targetType, tt.runtimeParam)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildPageByCursor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cursor   *string
		wantPage int32
		wantErr  bool
	}{
		{
			name:     "cursor为nil_返回第1页",
			cursor:   nil,
			wantPage: 1,
			wantErr:  false,
		},
		{
			name:     "cursor为有效数字",
			cursor:   gptr.Of("5"),
			wantPage: 5,
			wantErr:  false,
		},
		{
			name:    "cursor为无效字符串",
			cursor:  gptr.Of("invalid"),
			wantErr: true,
		},
		{
			name:     "cursor为0",
			cursor:   gptr.Of("0"),
			wantPage: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			page, err := buildPageByCursor(tt.cursor)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantPage, page)
			}
		})
	}
}

func TestConvert2TraceString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{
			name:  "nil输入",
			input: nil,
			want:  "",
		},
		{
			name:  "字符串输入",
			input: "test string",
			want:  `"test string"`,
		},
		{
			name:  "数字输入",
			input: 123,
			want:  "123",
		},
		{
			name: "对象输入",
			input: map[string]interface{}{
				"key": "value",
			},
			want: `{"key":"value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := Convert2TraceString(tt.input)
			assert.Equal(t, tt.want, result)
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
