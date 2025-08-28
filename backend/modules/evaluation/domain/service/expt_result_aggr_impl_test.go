// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestExptAggrResultServiceImpl_CreateExptAggrResult(t *testing.T) {
	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		setup     func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric)
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name:    "正常创建聚合结果",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// 设置获取评估器结果引用的mock
				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{
							EvaluatorResultID:  1,
							EvaluatorVersionID: 1,
						},
					}, nil)

				// 设置获取评估器记录的mock
				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID: 1,
							EvaluatorOutputData: &entity.EvaluatorOutputData{
								EvaluatorResult: &entity.EvaluatorResult{
									Score: gptr.Of(0.8),
								},
							},
						},
					}, nil)

				// 设置创建聚合结果的mock
				mockExptAggrResultRepo.EXPECT().
					BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)

				// 设置指标统计的mock
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).
					Return()
			},
			wantErr: false,
		},
		{
			name:    "没有评估器结果时跳过创建",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).
					Return([]*entity.ExptTurnEvaluatorResultRef{}, nil)

				// 设置指标统计的mock
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).
					Return()
			},
			wantErr: false,
		},
		{
			name:    "获取评估器结果引用失败",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).
					Return(nil, errorx.NewByCode(500, errorx.WithExtraMsg("db error")))

				// 设置指标统计的mock
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), true, gomock.Any()).
					Return()
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(500), statusErr.Code())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
			mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
			mockMetric := metricsMocks.NewMockExptMetric(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptTurnResultRepo:     mockExptTurnResultRepo,
				exptAggrResultRepo:     mockExptAggrResultRepo,
				evaluatorRecordService: mockEvaluatorRecordService,
				metric:                 mockMetric,
			}

			tt.setup(mockExptTurnResultRepo, mockExptAggrResultRepo, mockEvaluatorRecordService, mockMetric)

			err := svc.CreateExptAggrResult(context.Background(), tt.spaceID, tt.exptID)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAggrResultServiceImpl_UpdateExptAggrResult(t *testing.T) {
	tests := []struct {
		name      string
		param     *entity.UpdateExptAggrResultParam
		setup     func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric)
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name: "正常更新聚合结果",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// 设置获取现有聚合结果的mock
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(&entity.ExptAggrResult{}, nil)

				// 设置更新版本号的mock
				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(int64(1), nil)

				// 设置获取评估器结果引用的mock
				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByEvaluatorVersionID(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{
							EvaluatorResultID: 1,
						},
					}, nil)

				// 设置获取评估器记录的mock
				mockEvaluatorRecordService.EXPECT().
					BatchGetEvaluatorRecord(gomock.Any(), []int64{1}, false).
					Return([]*entity.EvaluatorRecord{
						{
							ID: 1,
							EvaluatorOutputData: &entity.EvaluatorOutputData{
								EvaluatorResult: &entity.EvaluatorResult{
									Score: gptr.Of(0.8),
								},
							},
						},
					}, nil)

				// 设置更新聚合结果的mock
				mockExptAggrResultRepo.EXPECT().
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil)

				// 设置指标统计的mock
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).
					Return()
			},
			wantErr: false,
		},
		{
			name: "无效的字段类型",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Unknown,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// 设置指标统计的mock
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), true, gomock.Any()).
					Return()
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
			},
		},
		{
			name: "获取现有聚合结果失败",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// 设置获取现有聚合结果的mock
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(nil, errorx.NewByCode(500, errorx.WithExtraMsg("db error")))

				// 设置指标统计的mock
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), true, gomock.Any()).
					Return()
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(500), statusErr.Code())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockEvaluatorRecordService := svcMocks.NewMockEvaluatorRecordService(ctrl)
			mockMetric := metricsMocks.NewMockExptMetric(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAggrResultRepo:     mockExptAggrResultRepo,
				exptTurnResultRepo:     mockExptTurnResultRepo,
				evaluatorRecordService: mockEvaluatorRecordService,
				metric:                 mockMetric,
			}

			tt.setup(mockExptAggrResultRepo, mockExptTurnResultRepo, mockEvaluatorRecordService, mockMetric)

			err := svc.UpdateExptAggrResult(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAggrResultServiceImpl_BatchGetExptAggrResultByExperimentIDs(t *testing.T) {
	tests := []struct {
		name    string
		spaceID int64
		exptIDs []int64
		setup   func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockEvaluatorService *svcMocks.MockEvaluatorService,
			mockTagRPCAdapter *rpcmocks.MockITagRPCAdapter, mockAnnotateRepo *repoMocks.MockIExptAnnotateRepo)
		want      []*entity.ExptAggregateResult
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name:    "正常批量获取聚合结果",
			spaceID: 100,
			exptIDs: []int64{1},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockEvaluatorService *svcMocks.MockEvaluatorService,
				mockTagRPCAdapter *rpcmocks.MockITagRPCAdapter, mockAnnotateRepo *repoMocks.MockIExptAnnotateRepo,
			) {
				// 设置获取聚合结果的mock
				aggrResult := &entity.AggregateResult{
					AggregatorResults: []*entity.AggregatorResult{
						{
							AggregatorType: entity.Average,
							Data: &entity.AggregateData{
								DataType: entity.Double,
								Value:    gptr.Of(0.8),
							},
						},
					},
				}
				aggrResultBytes, _ := json.Marshal(aggrResult)
				mockExptAggrResultRepo.EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), []int64{1}).
					Return([]*entity.ExptAggrResult{
						{
							ExperimentID: 1,
							FieldType:    int32(entity.FieldType_EvaluatorScore),
							FieldKey:     "1",
							AggrResult:   aggrResultBytes,
						},
						{
							ExperimentID: 1,
							FieldType:    int32(entity.FieldType_Annotation),
							FieldKey:     "1",
							AggrResult:   aggrResultBytes,
						},
					}, nil)

				// 设置获取评估器引用的mock
				mockExperimentRepo.EXPECT().
					GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(100)).
					Return([]*entity.ExptEvaluatorRef{
						{
							EvaluatorVersionID: 1,
							EvaluatorID:        1,
						},
					}, nil)

				// 设置获取评估器版本的mock
				evaluator := &entity.Evaluator{
					ID:            1,
					Name:          "test evaluator",
					EvaluatorType: entity.EvaluatorTypePrompt,
					PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
						ID:      1,
						Version: "1.0",
					},
				}
				mockEvaluatorService.EXPECT().
					BatchGetEvaluatorVersion(gomock.Any(), gomock.Any(), []int64{1}, true).
					Return([]*entity.Evaluator{evaluator}, nil)
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					map[int64]*entity.TagInfo{1: {
						TagKeyId:       1,
						TagKeyName:     "123",
						Description:    "123",
						InActive:       false,
						TagContentType: "",
						TagValues:      nil,
						TagContentSpec: nil,
						TagStatus:      "",
					}}, nil)
				mockAnnotateRepo.EXPECT().BatchGetExptTurnAnnotateRecordRefs(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					[]*entity.ExptTurnAnnotateRecordRef{
						{
							ID:               1,
							TagKeyID:         1,
							ExptID:           1,
							AnnotateRecordID: 1,
						},
					}, nil)
			},
			want: []*entity.ExptAggregateResult{
				{
					ExperimentID: 1,
					EvaluatorResults: map[int64]*entity.EvaluatorAggregateResult{
						1: {
							EvaluatorVersionID: 1,
							AggregatorResults: []*entity.AggregatorResult{
								{
									AggregatorType: entity.Average,
									Data: &entity.AggregateData{
										DataType: entity.Double,
										Value:    gptr.Of(0.8),
									},
								},
							},
							Name:    gptr.Of("test evaluator"),
							Version: gptr.Of("1.0"),
						},
					},
					AnnotationResults: map[int64]*entity.AnnotationAggregateResult{
						1: {
							TagKeyID: 1,
							Name:     ptr.Of("123"),
							AggregatorResults: []*entity.AggregatorResult{
								{
									AggregatorType: entity.Average,
									Data: &entity.AggregateData{
										DataType: entity.Double,
										Value:    gptr.Of(0.8),
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "获取聚合结果失败",
			spaceID: 100,
			exptIDs: []int64{1},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockEvaluatorService *svcMocks.MockEvaluatorService,
				mockTagRPCAdapter *rpcmocks.MockITagRPCAdapter, mockAnnotateRepo *repoMocks.MockIExptAnnotateRepo,
			) {
				mockExptAggrResultRepo.EXPECT().
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), []int64{1}).
					Return(nil, errorx.NewByCode(500, errorx.WithExtraMsg("db error")))
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(500), statusErr.Code())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
			mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			mockEvaluatorService := svcMocks.NewMockEvaluatorService(ctrl)
			mockTagRPCAdapter := rpcmocks.NewMockITagRPCAdapter(ctrl)
			mockAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAggrResultRepo: mockExptAggrResultRepo,
				experimentRepo:     mockExperimentRepo,
				evaluatorService:   mockEvaluatorService,
				tagRPCAdapter:      mockTagRPCAdapter,
				exptAnnotateRepo:   mockAnnotateRepo,
			}

			tt.setup(mockExptAggrResultRepo, mockExperimentRepo, mockEvaluatorService, mockTagRPCAdapter, mockAnnotateRepo)

			got, err := svc.BatchGetExptAggrResultByExperimentIDs(context.Background(), tt.spaceID, tt.exptIDs)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExptAggrResultServiceImpl_CreateAnnotationAggrResult(t *testing.T) {
	tests := []struct {
		name      string
		param     *entity.CreateSpecificFieldAggrResultParam
		setup     func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric)
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name: "创建连续数值类型标注聚合结果成功",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), int64(1)).
					Return([]*entity.ExptTurnAnnotateRecordRef{{AnnotateRecordID: 1}}, nil)

				mockExptAnnotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.AnnotateRecord{{
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeContinuousNumber,
							Score:          gptr.Of(0.8),
						},
					}}, nil)

				mockExptAggrResultRepo.EXPECT().
					CreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "创建布尔类型标注聚合结果成功",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), int64(1)).
					Return([]*entity.ExptTurnAnnotateRecordRef{{AnnotateRecordID: 1}}, nil)

				mockExptAnnotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.AnnotateRecord{{
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeBoolean,
						},
						TagValueID: 1,
					}}, nil)

				mockExptAggrResultRepo.EXPECT().CreateExptAggrResult(gomock.Any(), gomock.Any()).Return(nil)
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "创建分类类型标注聚合结果成功",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), int64(1)).
					Return([]*entity.ExptTurnAnnotateRecordRef{{AnnotateRecordID: 1}}, nil)

				mockExptAnnotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.AnnotateRecord{{
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeCategorical,
						},
						TagValueID: 1,
					}}, nil)

				mockExptAggrResultRepo.EXPECT().CreateExptAggrResult(gomock.Any(), gomock.Any()).Return(nil)
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "无效字段类型返回错误",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), true, gomock.Any()).Return()
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
			},
		},
		{
			name: "无标注记录时跳过创建",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), int64(1)).
					Return([]*entity.ExptTurnAnnotateRecordRef{}, nil)
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
			mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
			mockMetric := metricsMocks.NewMockExptMetric(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAnnotateRepo:   mockExptAnnotateRepo,
				exptAggrResultRepo: mockExptAggrResultRepo,
				metric:             mockMetric,
			}

			tt.setup(mockExptAnnotateRepo, mockExptAggrResultRepo, mockMetric)

			err := svc.CreateAnnotationAggrResult(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptAggrResultServiceImpl_UpdateAnnotationAggrResult(t *testing.T) {
	tests := []struct {
		name      string
		param     *entity.UpdateExptAggrResultParam
		setup     func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockMetric *metricsMocks.MockExptMetric)
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name: "更新数值标注聚合结果成功",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(&entity.ExptAggrResult{}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(int64(1), nil)

				tagKeyID := int64(1)
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), tagKeyID).
					Return([]*entity.ExptTurnAnnotateRecordRef{{AnnotateRecordID: 1}}, nil)

				mockExptAnnotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.AnnotateRecord{{
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeContinuousNumber,
							Score:          gptr.Of(0.8),
						},
					}}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil)

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "更新分类标注聚合结果成功",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(&entity.ExptAggrResult{}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(int64(1), nil)

				tagKeyID := int64(1)
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), tagKeyID).
					Return([]*entity.ExptTurnAnnotateRecordRef{{AnnotateRecordID: 1}}, nil)

				mockExptAnnotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.AnnotateRecord{{
						TagValueID: 1,
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeCategorical,
						},
					}}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil)

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "更新布尔标注聚合结果成功",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(&entity.ExptAggrResult{}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(int64(1), nil)

				tagKeyID := int64(1)
				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), tagKeyID).
					Return([]*entity.ExptTurnAnnotateRecordRef{{AnnotateRecordID: 1}}, nil)

				mockExptAnnotateRepo.EXPECT().
					GetAnnotateRecordsByIDs(gomock.Any(), int64(100), []int64{1}).
					Return([]*entity.AnnotateRecord{{
						TagValueID: 2,
						AnnotateData: &entity.AnnotateData{
							TagContentType: entity.TagContentTypeBoolean,
						},
					}}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil)

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "无效字段类型返回错误",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), true, gomock.Any()).Return()
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.CommonInvalidParamCode), statusErr.Code())
			},
		},
		{
			name: "聚合结果不存在且实验未结束时跳过更新",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))

				mockExperimentRepo.EXPECT().
					GetByID(gomock.Any(), int64(1), int64(100)).
					Return(&entity.Experiment{Status: entity.ExptStatus_Processing}, nil)

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)
			mockExptAnnotateRepo := repoMocks.NewMockIExptAnnotateRepo(ctrl)
			mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)
			mockMetric := metricsMocks.NewMockExptMetric(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAggrResultRepo: mockExptAggrResultRepo,
				exptAnnotateRepo:   mockExptAnnotateRepo,
				experimentRepo:     mockExperimentRepo,
				metric:             mockMetric,
			}

			tt.setup(mockExptAggrResultRepo, mockExptAnnotateRepo, mockExperimentRepo, mockMetric)

			err := svc.UpdateAnnotationAggrResult(context.Background(), tt.param)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
