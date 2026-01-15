// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
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
		setup     func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric, mockLocker *lockMocks.MockILocker)
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name:    "Create aggregation result successfully",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric, mockLocker *lockMocks.MockILocker) {
				// Mock GetTurnEvaluatorResultRefByExptID
				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{
							EvaluatorResultID:  1,
							EvaluatorVersionID: 1,
						},
					}, nil)

				// Mock BatchGetEvaluatorRecord
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

				// Mock GetExptAggrResultByExperimentID
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResultByExperimentID(gomock.Any(), int64(1)).
					Return([]*entity.ExptAggrResult{}, nil)

				// Mock ScanTurnResults
				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(100)).
					Return([]*entity.ExptTurnResult{}, int64(0), nil)

				// Mock BatchCreateExptAggrResult
				mockExptAggrResultRepo.EXPECT().
					BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)

				// Mock EmitCalculateExptAggrResult
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).
					Return()

				mockLocker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
			},
			wantErr: false,
		},
		{
			name:    "Skip creation when no evaluator results",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric, mockLocker *lockMocks.MockILocker) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResultByExperimentID(gomock.Any(), int64(1)).
					Return([]*entity.ExptAggrResult{}, nil)

				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByExptID(gomock.Any(), int64(100), int64(1)).
					Return([]*entity.ExptTurnEvaluatorResultRef{}, nil)

				// Mock ScanTurnResults
				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(100)).
					Return([]*entity.ExptTurnResult{}, int64(0), nil)

				// Mock BatchCreateExptAggrResult for target metrics
				mockExptAggrResultRepo.EXPECT().
					BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)

				// Mock EmitCalculateExptAggrResult
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAllFields), false, gomock.Any()).
					Return()

				mockLocker.EXPECT().Unlock(gomock.Any()).Return(true, nil)
			},
			wantErr: false,
		},
		{
			name:    "Failed to get evaluator result refs",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric, mockLocker *lockMocks.MockILocker) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResultByExperimentID(gomock.Any(), int64(1)).
					Return(nil, errorx.NewByCode(500, errorx.WithExtraMsg("db error")))

				// Mock EmitCalculateExptAggrResult
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
			mockEvalTargetSvc := svcMocks.NewMockIEvalTargetService(ctrl)
			mockLocker := lockMocks.NewMockILocker(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptTurnResultRepo:     mockExptTurnResultRepo,
				exptAggrResultRepo:     mockExptAggrResultRepo,
				evaluatorRecordService: mockEvaluatorRecordService,
				metric:                 mockMetric,
				evalTargetSvc:          mockEvalTargetSvc,
				locker:                 mockLocker,
			}

			tt.setup(mockExptTurnResultRepo, mockExptAggrResultRepo, mockEvaluatorRecordService, mockMetric, mockLocker)

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
			name: "Update aggregation result successfully",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// Mock GetExptAggrResult
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(&entity.ExptAggrResult{}, nil)

				// Mock UpdateAndGetLatestVersion
				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(int64(1), nil)

				// Mock GetTurnEvaluatorResultRefByEvaluatorVersionID
				mockExptTurnResultRepo.EXPECT().
					GetTurnEvaluatorResultRefByEvaluatorVersionID(gomock.Any(), int64(100), int64(1), int64(1)).
					Return([]*entity.ExptTurnEvaluatorResultRef{
						{
							EvaluatorResultID: 1,
						},
					}, nil)

				// Mock BatchGetEvaluatorRecord
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

				// Mock UpdateExptAggrResultByVersion
				mockExptAggrResultRepo.EXPECT().
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil)

				// Mock EmitCalculateExptAggrResult
				mockMetric.EXPECT().
					EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).
					Return()
			},
			wantErr: false,
		},
		{
			name: "Invalid field type",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Unknown,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// Mock EmitCalculateExptAggrResult
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
			name: "Failed to get existing aggregation result",
			param: &entity.UpdateExptAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvaluatorRecordService *svcMocks.MockEvaluatorRecordService, mockMetric *metricsMocks.MockExptMetric) {
				// Mock GetExptAggrResult
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(nil, errorx.NewByCode(500, errorx.WithExtraMsg("db error")))

				// Mock EmitCalculateExptAggrResult
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
			name:    "Batch get aggregation results successfully",
			spaceID: 100,
			exptIDs: []int64{1},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockEvaluatorService *svcMocks.MockEvaluatorService,
				mockTagRPCAdapter *rpcmocks.MockITagRPCAdapter, mockAnnotateRepo *repoMocks.MockIExptAnnotateRepo,
			) {
				// Mock experiments
				mockExperimentRepo.EXPECT().MGetBasicByID(gomock.Any(), []int64{1}).Return([]*entity.Experiment{{ID: 1, TargetID: 10, TargetVersionID: 20}}, nil)

				// Mock aggregation results
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
							UpdateAt:     gptr.Of(time.Unix(1000, 0)),
						},
						{
							ExperimentID: 1,
							FieldType:    int32(entity.FieldType_Annotation),
							FieldKey:     "1",
							AggrResult:   aggrResultBytes,
							UpdateAt:     gptr.Of(time.Unix(1000, 0)),
						},
						{
							ExperimentID: 1,
							FieldType:    int32(entity.FieldType_TargetLatency),
							FieldKey:     entity.AggrResultFieldKey_TargetLatency,
							AggrResult:   aggrResultBytes,
							UpdateAt:     gptr.Of(time.Unix(1000, 0)),
						},
					}, nil)

				// Mock evaluator refs
				mockExperimentRepo.EXPECT().
					GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(100)).
					Return([]*entity.ExptEvaluatorRef{
						{
							EvaluatorVersionID: 1,
							EvaluatorID:        1,
						},
					}, nil)

				// Mock evaluator versions
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

				// Mock tag info
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), gomock.Any(), gomock.Any()).Return(
					map[int64]*entity.TagInfo{1: {
						TagKeyId:   1,
						TagKeyName: "123",
					}}, nil)

				// Mock annotate refs
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
							EvaluatorID:        1,
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
					TargetResults: &entity.EvalTargetMtrAggrResult{
						TargetID:        10,
						TargetVersionID: 20,
						LatencyAggrResults: []*entity.AggregatorResult{
							{
								AggregatorType: entity.Average,
								Data: &entity.AggregateData{
									DataType: entity.Double,
									Value:    gptr.Of(0.8),
								},
							},
						},
					},
					UpdateTime: gptr.Of(time.Unix(1000, 0)),
				},
			},
			wantErr: false,
		},
		{
			name:    "Batch get aggregation results successfully with all target metrics",
			spaceID: 100,
			exptIDs: []int64{2},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockEvaluatorService *svcMocks.MockEvaluatorService,
				mockTagRPCAdapter *rpcmocks.MockITagRPCAdapter, mockAnnotateRepo *repoMocks.MockIExptAnnotateRepo,
			) {
				mockExperimentRepo.EXPECT().MGetBasicByID(gomock.Any(), []int64{2}).Return([]*entity.Experiment{{ID: 2, TargetID: 10, TargetVersionID: 20}}, nil)

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
					BatchGetExptAggrResultByExperimentIDs(gomock.Any(), []int64{2}).
					Return([]*entity.ExptAggrResult{
						{ExperimentID: 2, FieldType: int32(entity.FieldType_TargetLatency), FieldKey: entity.AggrResultFieldKey_TargetLatency, AggrResult: aggrResultBytes},
						{ExperimentID: 2, FieldType: int32(entity.FieldType_TargetInputTokens), FieldKey: entity.AggrResultFieldKey_TargetInputTokens, AggrResult: aggrResultBytes},
						{ExperimentID: 2, FieldType: int32(entity.FieldType_TargetOutputTokens), FieldKey: entity.AggrResultFieldKey_TargetOutputTokens, AggrResult: aggrResultBytes},
						{ExperimentID: 2, FieldType: int32(entity.FieldType_TargetTotalTokens), FieldKey: entity.AggrResultFieldKey_TargetTotalTokens, AggrResult: aggrResultBytes},
					}, nil)

				mockExperimentRepo.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{2}, int64(100)).Return([]*entity.ExptEvaluatorRef{}, nil)
				mockEvaluatorService.EXPECT().BatchGetEvaluatorVersion(gomock.Any(), gomock.Nil(), []int64{}, true).Return([]*entity.Evaluator{}, nil)
				mockTagRPCAdapter.EXPECT().BatchGetTagInfo(gomock.Any(), int64(100), []int64{}).Return(map[int64]*entity.TagInfo{}, nil)
				mockAnnotateRepo.EXPECT().BatchGetExptTurnAnnotateRecordRefs(gomock.Any(), []int64{2}, int64(100)).Return([]*entity.ExptTurnAnnotateRecordRef{}, nil)
			},
			want: []*entity.ExptAggregateResult{
				{
					ExperimentID:      2,
					EvaluatorResults:  map[int64]*entity.EvaluatorAggregateResult{},
					AnnotationResults: map[int64]*entity.AnnotationAggregateResult{},
					TargetResults: &entity.EvalTargetMtrAggrResult{
						TargetID:        10,
						TargetVersionID: 20,
						LatencyAggrResults: []*entity.AggregatorResult{
							{AggregatorType: entity.Average, Data: &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(0.8)}},
						},
						InputTokensAggrResults: []*entity.AggregatorResult{
							{AggregatorType: entity.Average, Data: &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(0.8)}},
						},
						OutputTokensAggrResults: []*entity.AggregatorResult{
							{AggregatorType: entity.Average, Data: &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(0.8)}},
						},
						TotalTokensAggrResults: []*entity.AggregatorResult{
							{AggregatorType: entity.Average, Data: &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(0.8)}},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "Failed to get aggregation results",
			spaceID: 100,
			exptIDs: []int64{1},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockExperimentRepo *repoMocks.MockIExperimentRepo, mockEvaluatorService *svcMocks.MockEvaluatorService,
				mockTagRPCAdapter *rpcmocks.MockITagRPCAdapter, mockAnnotateRepo *repoMocks.MockIExptAnnotateRepo,
			) {
				mockExperimentRepo.EXPECT().MGetBasicByID(gomock.Any(), []int64{1}).Return([]*entity.Experiment{{ID: 1}}, nil)
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
			mockEvalTargetSvc := svcMocks.NewMockIEvalTargetService(ctrl)
			mockLocker := lockMocks.NewMockILocker(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAggrResultRepo: mockExptAggrResultRepo,
				experimentRepo:     mockExperimentRepo,
				evaluatorService:   mockEvaluatorService,
				tagRPCAdapter:      mockTagRPCAdapter,
				exptAnnotateRepo:   mockAnnotateRepo,
				evalTargetSvc:      mockEvalTargetSvc,
				locker:             mockLocker,
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
			name: "Create continuous number annotation aggregation result successfully",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				// 先检查是否已有聚合结果，返回 ResourceNotFound 走创建逻辑
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))

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

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAnnotationFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "Create boolean annotation aggregation result successfully",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))

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
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAnnotationFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "Create categorical annotation aggregation result successfully",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))

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
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAnnotationFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "Invalid field type for annotation",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_EvaluatorScore,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAnnotationFields), true, gomock.Any()).Return()
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
			name: "Skip creation when no annotate records",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode))

				mockExptAnnotateRepo.EXPECT().
					GetExptTurnAnnotateRecordRefsByTagKeyID(gomock.Any(), int64(1), int64(100), int64(1)).
					Return([]*entity.ExptTurnAnnotateRecordRef{}, nil)
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAnnotationFields), false, gomock.Any()).Return()
			},
			wantErr: false,
		},
		{
			name: "GetExptAggrResult returns non-ResourceNotFound error",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(nil, errors.New("repo error"))
				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.CreateAnnotationFields), true, gomock.Any()).Return()
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "repo error")
			},
		},
		{
			name: "CreateAnnotationAggrResult when record already exists should call UpdateAnnotationAggrResult",
			param: &entity.CreateSpecificFieldAggrResultParam{
				SpaceID:      100,
				ExperimentID: 1,
				FieldType:    entity.FieldType_Annotation,
				FieldKey:     "1",
			},
			setup: func(mockExptAnnotateRepo *repoMocks.MockIExptAnnotateRepo, mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo, mockMetric *metricsMocks.MockExptMetric) {
				// Mock GetExptAggrResult to return existing record
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(&entity.ExptAggrResult{
						ExperimentID: 1,
						FieldType:    int32(entity.FieldType_Annotation),
						FieldKey:     "1",
					}, nil)

				// Mock UpdateAnnotationAggrResult calls
				mockExptAggrResultRepo.EXPECT().
					GetExptAggrResult(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(&entity.ExptAggrResult{}, nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_Annotation), "1").
					Return(int64(1), nil)

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
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(1)).
					Return(nil)

				mockMetric.EXPECT().EmitCalculateExptAggrResult(int64(100), int64(entity.UpdateSpecificField), false, gomock.Any()).Return()
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
			mockExperimentRepo := repoMocks.NewMockIExperimentRepo(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAnnotateRepo:   mockExptAnnotateRepo,
				exptAggrResultRepo: mockExptAggrResultRepo,
				metric:             mockMetric,
				experimentRepo:     mockExperimentRepo,
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

func TestExptAggrResultServiceImpl_buildExptTargetMtrAggregatorGroup(t *testing.T) {
	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		setup     func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvalTargetSvc *svcMocks.MockIEvalTargetService)
		wantErr   bool
		checkFunc func(t *testing.T, result *targetMtrAggrGroup)
	}{
		{
			name:    "Build target metric aggregator group successfully",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvalTargetSvc *svcMocks.MockIEvalTargetService) {
				// First round scan
				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(100)).
					Return([]*entity.ExptTurnResult{
						{TargetResultID: 1},
						{TargetResultID: 2},
					}, int64(2), nil)

				// Second round scan
				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(2), int64(50), int64(100)).
					Return([]*entity.ExptTurnResult{}, int64(0), nil)

				// Batch get target records
				mockEvalTargetSvc.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1, 2}).
					Return([]*entity.EvalTargetRecord{
						{
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								TimeConsumingMS: gptr.Of(int64(100)),
								EvalTargetUsage: &entity.EvalTargetUsage{
									InputTokens:  10,
									OutputTokens: 20,
									TotalTokens:  30,
								},
							},
						},
						{
							EvalTargetOutputData: &entity.EvalTargetOutputData{
								TimeConsumingMS: gptr.Of(int64(200)),
								EvalTargetUsage: &entity.EvalTargetUsage{
									InputTokens:  15,
									OutputTokens: 25,
									TotalTokens:  40,
								},
							},
						},
					}, nil)
			},
			wantErr: false,
			checkFunc: func(t *testing.T, result *targetMtrAggrGroup) {
				assert.NotNil(t, result)
				assert.NotNil(t, result.latency)
				assert.NotNil(t, result.inputTokens)
				assert.NotNil(t, result.outputTokens)
				assert.NotNil(t, result.totalTokens)
			},
		},
		{
			name:    "Failed to scan turn results",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvalTargetSvc *svcMocks.MockIEvalTargetService) {
				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(100)).
					Return(nil, int64(0), errorx.NewByCode(500, errorx.WithExtraMsg("db error")))
			},
			wantErr: true,
		},
		{
			name:    "Failed to batch get target records",
			spaceID: 100,
			exptID:  1,
			setup: func(mockExptTurnResultRepo *repoMocks.MockIExptTurnResultRepo, mockEvalTargetSvc *svcMocks.MockIEvalTargetService) {
				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(0), int64(50), int64(100)).
					Return([]*entity.ExptTurnResult{
						{TargetResultID: 1},
					}, int64(1), nil)

				mockExptTurnResultRepo.EXPECT().
					ScanTurnResults(gomock.Any(), int64(1), gomock.Any(), int64(1), int64(50), int64(100)).
					Return([]*entity.ExptTurnResult{}, int64(0), nil)

				mockEvalTargetSvc.EXPECT().
					BatchGetRecordByIDs(gomock.Any(), int64(100), []int64{1}).
					Return(nil, errorx.NewByCode(500, errorx.WithExtraMsg("db error")))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
			mockEvalTargetSvc := svcMocks.NewMockIEvalTargetService(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptTurnResultRepo: mockExptTurnResultRepo,
				evalTargetSvc:      mockEvalTargetSvc,
			}

			tt.setup(mockExptTurnResultRepo, mockEvalTargetSvc)

			result, err := svc.buildExptTargetMtrAggregatorGroup(context.Background(), tt.spaceID, tt.exptID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, result)
				}
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
			name: "Update continuous number annotation aggregation result successfully",
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
			name: "Update categorical annotation aggregation result successfully",
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
			name: "Update boolean annotation aggregation result successfully",
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
			name: "Invalid field type for annotation update",
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
			name: "Skip update when aggregation result not found and experiment not finished",
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

func TestGetTopNScores(t *testing.T) {
	score2Count := map[float64]int64{
		1.0: 10,
		2.0: 20,
		3.0: 30,
		4.0: 40,
		5.0: 50,
	}
	res := GetTopNScores(score2Count, 3)
	assert.Len(t, res, 4) // n=3, but 5 items, so 3+1 (Other)
	assert.Equal(t, "5.00", res[0].Score)
	assert.Equal(t, int64(50), res[0].Count)
	assert.Equal(t, "4.00", res[1].Score)
	assert.Equal(t, int64(40), res[1].Count)
	assert.Equal(t, "3.00", res[2].Score)
	assert.Equal(t, int64(30), res[2].Count)
	assert.Equal(t, "Other", res[3].Score)

	res = GetTopNScores(score2Count, 10)
	assert.Len(t, res, 5)
}

func TestGetTopNOptions(t *testing.T) {
	option2Count := map[string]int64{
		"a": 10,
		"b": 20,
		"c": 30,
		"d": 40,
		"e": 50,
	}
	res := GetTopNOptions(option2Count, 3)
	assert.Len(t, res, 4) // n=3, but 5 items, so 3+1 (Other)
	assert.Equal(t, "e", res[0].Option)
	assert.Equal(t, int64(50), res[0].Count)
	assert.Equal(t, "d", res[1].Option)
	assert.Equal(t, int64(40), res[1].Count)
	assert.Equal(t, "c", res[2].Option)
	assert.Equal(t, int64(30), res[2].Count)
	assert.Equal(t, "Other", res[3].Option)

	res = GetTopNOptions(option2Count, 10)
	assert.Len(t, res, 5)
}

func TestExptAggrResultServiceImpl_CreateOrUpdateExptAggrResult(t *testing.T) {
	tests := []struct {
		name                               string
		spaceID                            int64
		exptID                             int64
		evaluatorVersionID2AggregatorGroup map[int64]*AggregatorGroup
		tmag                               *targetMtrAggrGroup
		existedAggrResults                 []*entity.ExptAggrResult
		setup                              func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo)
		wantErr                            bool
	}{
		{
			name:    "Create new aggregation results",
			spaceID: 100,
			exptID:  1,
			evaluatorVersionID2AggregatorGroup: map[int64]*AggregatorGroup{
				1: func() *AggregatorGroup {
					ag := NewAggregatorGroup()
					ag.Append(0.8)
					return ag
				}(),
			},
			tmag: func() *targetMtrAggrGroup {
				tg := &targetMtrAggrGroup{
					latency:      NewAggregatorGroup(WithScoreDistributionAggregator()),
					inputTokens:  NewAggregatorGroup(WithScoreDistributionAggregator()),
					outputTokens: NewAggregatorGroup(WithScoreDistributionAggregator()),
					totalTokens:  NewAggregatorGroup(WithScoreDistributionAggregator()),
				}
				tg.latency.Append(100)
				tg.inputTokens.Append(10)
				tg.outputTokens.Append(20)
				tg.totalTokens.Append(30)
				return tg
			}(),
			existedAggrResults: []*entity.ExptAggrResult{},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo) {
				mockExptAggrResultRepo.EXPECT().
					BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "Update existing aggregation results",
			spaceID: 100,
			exptID:  1,
			evaluatorVersionID2AggregatorGroup: map[int64]*AggregatorGroup{
				1: func() *AggregatorGroup {
					ag := NewAggregatorGroup()
					ag.Append(0.9)
					return ag
				}(),
			},
			tmag: &targetMtrAggrGroup{
				latency:      NewAggregatorGroup(),
				inputTokens:  NewAggregatorGroup(),
				outputTokens: NewAggregatorGroup(),
				totalTokens:  NewAggregatorGroup(),
			},
			existedAggrResults: []*entity.ExptAggrResult{
				{
					ExperimentID: 1,
					FieldType:    int32(entity.FieldType_EvaluatorScore),
					FieldKey:     "1",
					Score:        0.8,
					AggrResult:   []byte(`{"aggregator_results":[{"aggregator_type":1,"data":{"data_type":0,"value":0.8}}]}`),
				},
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo) {
				mockExptAggrResultRepo.EXPECT().
					UpdateAndGetLatestVersion(gomock.Any(), int64(1), int32(entity.FieldType_EvaluatorScore), "1").
					Return(int64(2), nil)

				mockExptAggrResultRepo.EXPECT().
					UpdateExptAggrResultByVersion(gomock.Any(), gomock.Any(), int64(2)).
					Return(nil)

				// For target metrics which are newly created in this test case
				mockExptAggrResultRepo.EXPECT().
					BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name:    "Skip update when aggregation results are identical",
			spaceID: 100,
			exptID:  1,
			evaluatorVersionID2AggregatorGroup: map[int64]*AggregatorGroup{
				1: func() *AggregatorGroup {
					ag := NewAggregatorGroup()
					ag.Append(0.8)
					return ag
				}(),
			},
			tmag: &targetMtrAggrGroup{
				latency:      NewAggregatorGroup(),
				inputTokens:  NewAggregatorGroup(),
				outputTokens: NewAggregatorGroup(),
				totalTokens:  NewAggregatorGroup(),
			},
			existedAggrResults: []*entity.ExptAggrResult{
				{
					SpaceID:      100,
					ExperimentID: 1,
					FieldType:    int32(entity.FieldType_EvaluatorScore),
					FieldKey:     "1",
					Score:        0.8,
					AggrResult:   []byte(`{"AggregatorResults":[{"AggregatorType":1,"Data":{"DataType":0,"Value":0.8,"ScoreDistribution":null,"OptionDistribution":null,"BooleanDistribution":null}},{"AggregatorType":2,"Data":{"DataType":0,"Value":0.8,"ScoreDistribution":null,"OptionDistribution":null,"BooleanDistribution":null}},{"AggregatorType":3,"Data":{"DataType":0,"Value":0.8,"ScoreDistribution":null,"OptionDistribution":null,"BooleanDistribution":null}},{"AggregatorType":4,"Data":{"DataType":0,"Value":0.8,"ScoreDistribution":null,"OptionDistribution":null,"BooleanDistribution":null}}]}`),
				},
			},
			setup: func(mockExptAggrResultRepo *repoMocks.MockIExptAggrResultRepo) {
				// Target metrics will still be created
				mockExptAggrResultRepo.EXPECT().
					BatchCreateExptAggrResult(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockExptAggrResultRepo := repoMocks.NewMockIExptAggrResultRepo(ctrl)

			svc := &ExptAggrResultServiceImpl{
				exptAggrResultRepo: mockExptAggrResultRepo,
			}

			tt.setup(mockExptAggrResultRepo)

			err := svc.CreateOrUpdateExptAggrResult(context.Background(), tt.spaceID, tt.exptID, tt.evaluatorVersionID2AggregatorGroup, tt.tmag, tt.existedAggrResults)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTargetMtrAggrGroup_calcRecord(t *testing.T) {
	tests := []struct {
		name      string
		records   []*entity.EvalTargetRecord
		setup     func(tg *targetMtrAggrGroup)
		checkFunc func(t *testing.T, tg *targetMtrAggrGroup)
	}{
		{
			name: "Calculate records successfully",
			records: []*entity.EvalTargetRecord{
				{
					EvalTargetOutputData: &entity.EvalTargetOutputData{
						TimeConsumingMS: gptr.Of(int64(100)),
						EvalTargetUsage: &entity.EvalTargetUsage{
							InputTokens:  10,
							OutputTokens: 20,
							TotalTokens:  30,
						},
					},
				},
				{
					EvalTargetOutputData: &entity.EvalTargetOutputData{
						TimeConsumingMS: gptr.Of(int64(200)),
						EvalTargetUsage: &entity.EvalTargetUsage{
							InputTokens:  15,
							OutputTokens: 25,
							TotalTokens:  40,
						},
					},
				},
			},
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = NewAggregatorGroup()
				tg.inputTokens = NewAggregatorGroup()
				tg.outputTokens = NewAggregatorGroup()
				tg.totalTokens = NewAggregatorGroup()
			},
			checkFunc: func(t *testing.T, tg *targetMtrAggrGroup) {
				assert.NotNil(t, tg.latency)
				assert.NotNil(t, tg.inputTokens)
				assert.NotNil(t, tg.outputTokens)
				assert.NotNil(t, tg.totalTokens)
			},
		},
		{
			name:    "Empty records",
			records: []*entity.EvalTargetRecord{},
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = NewAggregatorGroup()
				tg.inputTokens = NewAggregatorGroup()
				tg.outputTokens = NewAggregatorGroup()
				tg.totalTokens = NewAggregatorGroup()
			},
			checkFunc: func(t *testing.T, tg *targetMtrAggrGroup) {
				assert.NotNil(t, tg.latency)
				assert.NotNil(t, tg.inputTokens)
				assert.NotNil(t, tg.outputTokens)
				assert.NotNil(t, tg.totalTokens)
			},
		},
		{
			name:    "Nil records",
			records: []*entity.EvalTargetRecord{nil},
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = NewAggregatorGroup()
				tg.inputTokens = NewAggregatorGroup()
				tg.outputTokens = NewAggregatorGroup()
				tg.totalTokens = NewAggregatorGroup()
			},
			checkFunc: func(t *testing.T, tg *targetMtrAggrGroup) {
				assert.NotNil(t, tg.latency)
				assert.NotNil(t, tg.inputTokens)
				assert.NotNil(t, tg.outputTokens)
				assert.NotNil(t, tg.totalTokens)
			},
		},
		{
			name: "Nil EvalTargetOutputData",
			records: []*entity.EvalTargetRecord{
				{EvalTargetOutputData: nil},
			},
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = NewAggregatorGroup()
				tg.inputTokens = NewAggregatorGroup()
				tg.outputTokens = NewAggregatorGroup()
				tg.totalTokens = NewAggregatorGroup()
			},
			checkFunc: func(t *testing.T, tg *targetMtrAggrGroup) {
				assert.NotNil(t, tg.latency)
				assert.NotNil(t, tg.inputTokens)
				assert.NotNil(t, tg.outputTokens)
				assert.NotNil(t, tg.totalTokens)
			},
		},
		{
			name: "Nil EvalTargetUsage",
			records: []*entity.EvalTargetRecord{
				{
					EvalTargetOutputData: &entity.EvalTargetOutputData{
						TimeConsumingMS: gptr.Of(int64(100)),
						EvalTargetUsage: nil,
					},
				},
			},
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = NewAggregatorGroup()
				tg.inputTokens = NewAggregatorGroup()
				tg.outputTokens = NewAggregatorGroup()
				tg.totalTokens = NewAggregatorGroup()
			},
			checkFunc: func(t *testing.T, tg *targetMtrAggrGroup) {
				assert.NotNil(t, tg.latency)
				assert.NotNil(t, tg.inputTokens)
				assert.NotNil(t, tg.outputTokens)
				assert.NotNil(t, tg.totalTokens)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg := &targetMtrAggrGroup{}
			tt.setup(tg)
			tg.calcRecord(tt.records)
			if tt.checkFunc != nil {
				tt.checkFunc(t, tg)
			}
		})
	}
}

func TestTargetMtrAggrGroup_buildAggrResult(t *testing.T) {
	tests := []struct {
		name      string
		spaceID   int64
		exptID    int64
		setup     func(tg *targetMtrAggrGroup)
		wantErr   bool
		checkFunc func(t *testing.T, results []*entity.ExptAggrResult)
	}{
		{
			name:    "Build aggregation results successfully",
			spaceID: 100,
			exptID:  1,
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = NewAggregatorGroup()
				tg.latency.Append(100)
				tg.inputTokens = NewAggregatorGroup()
				tg.inputTokens.Append(10)
				tg.outputTokens = NewAggregatorGroup()
				tg.outputTokens.Append(20)
				tg.totalTokens = NewAggregatorGroup()
				tg.totalTokens.Append(30)
			},
			wantErr: false,
			checkFunc: func(t *testing.T, results []*entity.ExptAggrResult) {
				assert.Len(t, results, 4)
				for _, result := range results {
					assert.Equal(t, int64(100), result.SpaceID)
					assert.Equal(t, int64(1), result.ExperimentID)
					assert.NotEmpty(t, result.AggrResult)
				}
			},
		},
		{
			name:    "Aggregation group is nil",
			spaceID: 100,
			exptID:  1,
			setup: func(tg *targetMtrAggrGroup) {
				tg.latency = nil
				tg.inputTokens = NewAggregatorGroup()
				tg.outputTokens = NewAggregatorGroup()
				tg.totalTokens = NewAggregatorGroup()
			},
			wantErr: false,
			checkFunc: func(t *testing.T, results []*entity.ExptAggrResult) {
				assert.Len(t, results, 3) // latency is nil, so only 3 results
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tg := &targetMtrAggrGroup{}
			tt.setup(tg)

			results, err := tg.buildAggrResult(tt.spaceID, tt.exptID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, results)
				}
			}
		})
	}
}

func TestExptAggrResultServiceImpl_PublishExptAggrResultEvent(t *testing.T) {
	tests := []struct {
		name      string
		event     *entity.AggrCalculateEvent
		duration  *time.Duration
		setup     func(mockLocker *lockMocks.MockILocker, mockPublisher *eventsMocks.MockExptEventPublisher)
		wantErr   bool
		checkFunc func(t *testing.T, err error)
	}{
		{
			name: "Publish aggregation result event successfully",
			event: &entity.AggrCalculateEvent{
				ExperimentID: 1,
				SpaceID:      100,
			},
			duration: nil,
			setup: func(mockLocker *lockMocks.MockILocker, mockPublisher *eventsMocks.MockExptEventPublisher) {
				mockLocker.EXPECT().
					Lock(gomock.Any(), "calc_expt_result_aggr:1", time.Minute*10).
					Return(true, nil)

				mockPublisher.EXPECT().
					PublishExptAggrCalculateEvent(gomock.Any(), []*entity.AggrCalculateEvent{
						{ExperimentID: 1, SpaceID: 100},
					}, nil).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "Failed to acquire lock",
			event: &entity.AggrCalculateEvent{
				ExperimentID: 1,
				SpaceID:      100,
			},
			duration: nil,
			setup: func(mockLocker *lockMocks.MockILocker, mockPublisher *eventsMocks.MockExptEventPublisher) {
				mockLocker.EXPECT().
					Lock(gomock.Any(), "calc_expt_result_aggr:1", time.Minute*10).
					Return(false, nil)
			},
			wantErr: true,
			checkFunc: func(t *testing.T, err error) {
				statusErr, ok := errorx.FromStatusError(err)
				assert.True(t, ok)
				assert.Equal(t, int32(errno.DuplicateCalcExptAggrResultErrorCode), statusErr.Code())
			},
		},
		{
			name: "Error occurred while acquiring lock",
			event: &entity.AggrCalculateEvent{
				ExperimentID: 1,
				SpaceID:      100,
			},
			duration: nil,
			setup: func(mockLocker *lockMocks.MockILocker, mockPublisher *eventsMocks.MockExptEventPublisher) {
				mockLocker.EXPECT().
					Lock(gomock.Any(), "calc_expt_result_aggr:1", time.Minute*10).
					Return(false, errorx.NewByCode(500, errorx.WithExtraMsg("lock error")))
			},
			wantErr: true,
		},
		{
			name: "Failed to publish event",
			event: &entity.AggrCalculateEvent{
				ExperimentID: 1,
				SpaceID:      100,
			},
			duration: nil,
			setup: func(mockLocker *lockMocks.MockILocker, mockPublisher *eventsMocks.MockExptEventPublisher) {
				mockLocker.EXPECT().
					Lock(gomock.Any(), "calc_expt_result_aggr:1", time.Minute*10).
					Return(true, nil)

				mockPublisher.EXPECT().
					PublishExptAggrCalculateEvent(gomock.Any(), gomock.Any(), nil).
					Return(errorx.NewByCode(500, errorx.WithExtraMsg("publish error")))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockLocker := lockMocks.NewMockILocker(ctrl)
			mockPublisher := eventsMocks.NewMockExptEventPublisher(ctrl)

			svc := &ExptAggrResultServiceImpl{
				locker:    mockLocker,
				publisher: mockPublisher,
			}

			tt.setup(mockLocker, mockPublisher)

			err := svc.PublishExptAggrResultEvent(context.Background(), tt.event, tt.duration)
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

func TestExptAggrResultServiceImpl_MakeCalcExptAggrResultLockKey(t *testing.T) {
	tests := []struct {
		name   string
		exptID int64
		want   string
	}{
		{
			name:   "Generate lock key normally",
			exptID: 123,
			want:   "calc_expt_result_aggr:123",
		},
		{
			name:   "Generate lock key for 0",
			exptID: 0,
			want:   "calc_expt_result_aggr:0",
		},
		{
			name:   "Generate lock key for negative number",
			exptID: -1,
			want:   "calc_expt_result_aggr:-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ExptAggrResultServiceImpl{}
			got := svc.MakeCalcExptAggrResultLockKey(tt.exptID)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewBucketScoreDistributionAggregator(t *testing.T) {
	tests := []struct {
		name       string
		numBuckets int
		want       int
	}{
		{
			name:       "Valid number of buckets",
			numBuckets: 50,
			want:       50,
		},
		{
			name:       "Zero buckets defaults to 20",
			numBuckets: 0,
			want:       20,
		},
		{
			name:       "Negative buckets defaults to 20",
			numBuckets: -1,
			want:       20,
		},
		{
			name:       "Single bucket",
			numBuckets: 1,
			want:       1,
		},
		{
			name:       "Large number of buckets",
			numBuckets: 1000,
			want:       1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewBucketScoreDistributionAggregator(tt.numBuckets)
			assert.NotNil(t, agg)
			assert.Equal(t, tt.want, agg.NumBuckets)
			assert.NotNil(t, agg.Scores)
			assert.Len(t, agg.Scores, 0)
			assert.Equal(t, int64(0), agg.Total)
		})
	}
}

func TestBucketScoreDistributionAggregator_Append(t *testing.T) {
	tests := []struct {
		name       string
		numBuckets int
		scores     []float64
		checkFunc  func(t *testing.T, agg *BucketScoreDistributionAggregator)
	}{
		{
			name:       "Append first score initializes min and max",
			numBuckets: 10,
			scores:     []float64{5.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, 5.0, agg.Min)
				assert.Equal(t, 5.0, agg.Max)
				assert.Equal(t, int64(1), agg.Total)
				assert.Len(t, agg.Scores, 1)
				assert.Equal(t, 5.0, agg.Scores[0])
			},
		},
		{
			name:       "Append multiple scores updates min and max",
			numBuckets: 10,
			scores:     []float64{1.0, 5.0, 3.0, 9.0, 2.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, 1.0, agg.Min)
				assert.Equal(t, 9.0, agg.Max)
				assert.Equal(t, int64(5), agg.Total)
				assert.Len(t, agg.Scores, 5)
			},
		},
		{
			name:       "Append scores updates min",
			numBuckets: 10,
			scores:     []float64{5.0, 3.0, 1.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, 1.0, agg.Min)
				assert.Equal(t, 5.0, agg.Max)
				assert.Equal(t, int64(3), agg.Total)
			},
		},
		{
			name:       "Append scores updates max",
			numBuckets: 10,
			scores:     []float64{1.0, 3.0, 5.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, 1.0, agg.Min)
				assert.Equal(t, 5.0, agg.Max)
				assert.Equal(t, int64(3), agg.Total)
			},
		},
		{
			name:       "All scores are the same",
			numBuckets: 10,
			scores:     []float64{5.0, 5.0, 5.0, 5.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, 5.0, agg.Min)
				assert.Equal(t, 5.0, agg.Max)
				assert.Equal(t, int64(4), agg.Total)
				assert.Len(t, agg.Scores, 4)
			},
		},
		{
			name:       "Scores distributed across buckets",
			numBuckets: 5,
			scores:     []float64{0.0, 0.5, 1.0, 1.5, 2.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, 0.0, agg.Min)
				assert.Equal(t, 2.0, agg.Max)
				assert.Equal(t, int64(5), agg.Total)
			},
		},
		{
			name:       "Negative scores",
			numBuckets: 10,
			scores:     []float64{-10.0, -5.0, 0.0, 5.0, 10.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, -10.0, agg.Min)
				assert.Equal(t, 10.0, agg.Max)
				assert.Equal(t, int64(5), agg.Total)
			},
		},
		{
			name:       "Large number of scores",
			numBuckets: 50,
			scores:     make([]float64, 1000),
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator) {
				assert.Equal(t, int64(1000), agg.Total)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewBucketScoreDistributionAggregator(tt.numBuckets)
			if len(tt.scores) > 0 && tt.scores[0] == 0.0 && len(tt.scores) == 1000 {
				for i := range tt.scores {
					tt.scores[i] = float64(i%100) * 0.1
				}
			}
			for _, score := range tt.scores {
				agg.Append(score)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, agg)
			}
		})
	}
}

func TestBucketScoreDistributionAggregator_Result(t *testing.T) {
	tests := []struct {
		name       string
		numBuckets int
		scores     []float64
		checkFunc  func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData)
	}{
		{
			name:       "Empty aggregator returns all buckets with zero count",
			numBuckets: 10,
			scores:     []float64{},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				assert.NotNil(t, result)
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				assert.Equal(t, entity.ScoreDistribution, data.DataType)
				assert.NotNil(t, data.ScoreDistribution)
				// All buckets should be included, even when empty
				assert.Equal(t, 10, len(data.ScoreDistribution.ScoreDistributionItems))
				// All buckets should have count 0
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					assert.Equal(t, int64(0), item.Count)
					assert.Equal(t, 0.0, item.Percentage)
				}
			},
		},
		{
			name:       "Single score returns all buckets with one non-empty",
			numBuckets: 10,
			scores:     []float64{5.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				// All buckets should be included
				assert.Equal(t, 10, len(data.ScoreDistribution.ScoreDistributionItems))
				// One bucket should have count 1, others should have count 0
				nonEmptyCount := 0
				totalCount := int64(0)
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					totalCount += item.Count
					if item.Count > 0 {
						nonEmptyCount++
						assert.Equal(t, int64(1), item.Count)
						assert.Equal(t, 1.0, item.Percentage)
					} else {
						assert.Equal(t, int64(0), item.Count)
						assert.Equal(t, 0.0, item.Percentage)
					}
				}
				assert.Equal(t, 1, nonEmptyCount, "Should have exactly one non-empty bucket")
				assert.Equal(t, int64(1), totalCount, "Total count should be 1")
			},
		},
		{
			name:       "Multiple scores distributed across buckets",
			numBuckets: 5,
			scores:     []float64{0.0, 1.0, 2.0, 3.0, 4.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				assert.GreaterOrEqual(t, len(data.ScoreDistribution.ScoreDistributionItems), 1)
				totalCount := int64(0)
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					totalCount += item.Count
				}
				assert.Equal(t, int64(5), totalCount)
			},
		},
		{
			name:       "Empty buckets are included",
			numBuckets: 10,
			scores:     []float64{0.0, 10.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				// All buckets should be included, even empty ones
				assert.Equal(t, 10, len(data.ScoreDistribution.ScoreDistributionItems))
				// Verify that empty buckets have count 0
				emptyBucketCount := 0
				nonEmptyBucketCount := 0
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					if item.Count == 0 {
						emptyBucketCount++
						assert.Equal(t, 0.0, item.Percentage, "Empty bucket should have 0 percentage")
					} else {
						nonEmptyBucketCount++
						assert.Greater(t, item.Count, int64(0))
					}
				}
				assert.Greater(t, emptyBucketCount, 0, "Should have at least one empty bucket")
				assert.Greater(t, nonEmptyBucketCount, 0, "Should have at least one non-empty bucket")
			},
		},
		{
			name:       "Result items are in bucket index order",
			numBuckets: 5,
			scores:     []float64{4.0, 1.0, 3.0, 2.0, 0.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				items := data.ScoreDistribution.ScoreDistributionItems
				// Verify that items are returned (order is by bucket index, not sorted by score)
				assert.Greater(t, len(items), 0, "Should have at least one item")
				// Verify all items have valid counts
				for _, item := range items {
					assert.Greater(t, item.Count, int64(0), "Item should have positive count")
					assert.GreaterOrEqual(t, item.Percentage, 0.0, "Percentage should be >= 0")
					assert.LessOrEqual(t, item.Percentage, 1.0, "Percentage should be <= 1")
				}
			},
		},
		{
			name:       "Percentages sum to 1.0",
			numBuckets: 10,
			scores:     []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				totalPercentage := 0.0
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					totalPercentage += item.Percentage
				}
				assert.InDelta(t, 1.0, totalPercentage, 0.0001)
			},
		},
		{
			name:       "All scores same value",
			numBuckets: 10,
			scores:     []float64{5.0, 5.0, 5.0, 5.0, 5.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				// All buckets should be included
				assert.Equal(t, 10, len(data.ScoreDistribution.ScoreDistributionItems))
				// One bucket should have count 5, others should have count 0
				nonEmptyCount := 0
				totalCount := int64(0)
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					totalCount += item.Count
					if item.Count > 0 {
						nonEmptyCount++
						assert.Equal(t, int64(5), item.Count)
						assert.Equal(t, 1.0, item.Percentage)
					} else {
						assert.Equal(t, int64(0), item.Count)
						assert.Equal(t, 0.0, item.Percentage)
					}
				}
				assert.Equal(t, 1, nonEmptyCount, "Should have exactly one non-empty bucket")
				assert.Equal(t, int64(5), totalCount, "Total count should be 5")
			},
		},
		{
			name:       "Min and max values in correct buckets",
			numBuckets: 10,
			scores:     []float64{0.0, 1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				items := data.ScoreDistribution.ScoreDistributionItems
				assert.Greater(t, len(items), 0)
				firstItem := items[0]
				assert.Contains(t, firstItem.Score, "0.00")
				lastItem := items[len(items)-1]
				assert.Contains(t, lastItem.Score, "10.00")
			},
		},
		{
			name:       "Large number of buckets",
			numBuckets: 100,
			scores:     []float64{0.0, 50.0, 100.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				assert.LessOrEqual(t, len(data.ScoreDistribution.ScoreDistributionItems), 100)
			},
		},
		{
			name:       "Negative scores handled correctly",
			numBuckets: 10,
			scores:     []float64{-10.0, -5.0, 0.0, 5.0, 10.0},
			checkFunc: func(t *testing.T, result map[entity.AggregatorType]*entity.AggregateData) {
				data := result[entity.Distribution]
				assert.NotNil(t, data)
				totalCount := int64(0)
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					totalCount += item.Count
				}
				assert.Equal(t, int64(5), totalCount)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewBucketScoreDistributionAggregator(tt.numBuckets)
			for _, score := range tt.scores {
				agg.Append(score)
			}
			result := agg.Result()
			if tt.checkFunc != nil {
				tt.checkFunc(t, result)
			}
		})
	}
}

func TestBucketScoreDistributionAggregator_getBucketIndex(t *testing.T) {
	tests := []struct {
		name       string
		numBuckets int
		scores     []float64
		testScore  float64
		want       int
	}{
		{
			name:       "Score at minimum goes to first bucket",
			numBuckets: 10,
			scores:     []float64{0.0, 10.0},
			testScore:  0.0,
			want:       0,
		},
		{
			name:       "Score at maximum goes to last bucket",
			numBuckets: 10,
			scores:     []float64{0.0, 10.0},
			testScore:  10.0,
			want:       9,
		},
		{
			name:       "Score below minimum clamped to first bucket",
			numBuckets: 10,
			scores:     []float64{5.0, 10.0},
			testScore:  0.0,
			want:       0,
		},
		{
			name:       "Score above maximum clamped to last bucket",
			numBuckets: 10,
			scores:     []float64{0.0, 10.0},
			testScore:  20.0,
			want:       9,
		},
		{
			name:       "Score in middle goes to middle bucket",
			numBuckets: 10,
			scores:     []float64{0.0, 10.0},
			testScore:  5.0,
			want:       5,
		},
		{
			name:       "All scores same returns bucket 0",
			numBuckets: 10,
			scores:     []float64{5.0},
			testScore:  5.0,
			want:       0,
		},
		{
			name:       "Single bucket always returns 0",
			numBuckets: 1,
			scores:     []float64{0.0, 10.0},
			testScore:  5.0,
			want:       0,
		},
		{
			name:       "Uninitialized aggregator returns 0",
			numBuckets: 10,
			scores:     []float64{},
			testScore:  5.0,
			want:       0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewBucketScoreDistributionAggregator(tt.numBuckets)
			for _, score := range tt.scores {
				agg.Append(score)
			}
			bucketIndex := agg.getBucketIndex(tt.testScore)
			assert.Equal(t, tt.want, bucketIndex)
		})
	}
}

func TestBucketScoreDistributionAggregator_getBucketRange(t *testing.T) {
	tests := []struct {
		name        string
		numBuckets  int
		scores      []float64
		bucketIndex int
		checkFunc   func(t *testing.T, start, end float64)
	}{
		{
			name:        "First bucket range",
			numBuckets:  10,
			scores:      []float64{0.0, 10.0},
			bucketIndex: 0,
			checkFunc: func(t *testing.T, start, end float64) {
				assert.Equal(t, 0.0, start)
				assert.Greater(t, end, start)
			},
		},
		{
			name:        "Last bucket includes max value",
			numBuckets:  10,
			scores:      []float64{0.0, 10.0},
			bucketIndex: 9,
			checkFunc: func(t *testing.T, start, end float64) {
				assert.Equal(t, 10.0, end)
				assert.Less(t, start, end)
			},
		},
		{
			name:        "Middle bucket range",
			numBuckets:  10,
			scores:      []float64{0.0, 10.0},
			bucketIndex: 5,
			checkFunc: func(t *testing.T, start, end float64) {
				assert.GreaterOrEqual(t, start, 0.0)
				assert.LessOrEqual(t, end, 10.0)
				assert.Greater(t, end, start)
			},
		},
		{
			name:        "All scores same returns min and max",
			numBuckets:  10,
			scores:      []float64{5.0, 5.0},
			bucketIndex: 0,
			checkFunc: func(t *testing.T, start, end float64) {
				assert.Equal(t, 5.0, start)
				assert.Equal(t, 5.0, end)
			},
		},
		{
			name:        "Single bucket returns full range",
			numBuckets:  1,
			scores:      []float64{0.0, 10.0},
			bucketIndex: 0,
			checkFunc: func(t *testing.T, start, end float64) {
				assert.Equal(t, 0.0, start)
				assert.Equal(t, 10.0, end)
			},
		},
		{
			name:        "Negative scores handled correctly",
			numBuckets:  10,
			scores:      []float64{-10.0, 10.0},
			bucketIndex: 0,
			checkFunc: func(t *testing.T, start, end float64) {
				assert.Equal(t, -10.0, start)
				assert.Greater(t, end, start)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewBucketScoreDistributionAggregator(tt.numBuckets)
			for _, score := range tt.scores {
				agg.Append(score)
			}
			var bucketWidth float64
			if len(agg.Scores) > 0 && agg.Max != agg.Min {
				bucketWidth = (agg.Max - agg.Min) / float64(agg.NumBuckets)
			}
			start, end := agg.getBucketRange(tt.bucketIndex, bucketWidth)
			if tt.checkFunc != nil {
				tt.checkFunc(t, start, end)
			}
		})
	}
}

func TestWithBucketScoreDistributionAggregator(t *testing.T) {
	tests := []struct {
		name       string
		numBuckets int
		checkFunc  func(t *testing.T, ag *AggregatorGroup)
	}{
		{
			name:       "Add bucket aggregator to group",
			numBuckets: 50,
			checkFunc: func(t *testing.T, ag *AggregatorGroup) {
				assert.NotNil(t, ag)
				assert.Greater(t, len(ag.Aggregators), 1)
				found := false
				for _, agg := range ag.Aggregators {
					if bucketAgg, ok := agg.(*BucketScoreDistributionAggregator); ok {
						found = true
						assert.Equal(t, 50, bucketAgg.NumBuckets)
					}
				}
				assert.True(t, found)
			},
		},
		{
			name:       "Invalid buckets defaults to 20",
			numBuckets: 0,
			checkFunc: func(t *testing.T, ag *AggregatorGroup) {
				assert.NotNil(t, ag)
				found := false
				for _, agg := range ag.Aggregators {
					if bucketAgg, ok := agg.(*BucketScoreDistributionAggregator); ok {
						found = true
						assert.Equal(t, 20, bucketAgg.NumBuckets)
					}
				}
				assert.True(t, found)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ag := NewAggregatorGroup(WithBucketScoreDistributionAggregator(tt.numBuckets))
			if tt.checkFunc != nil {
				tt.checkFunc(t, ag)
			}
		})
	}
}

func TestBucketScoreDistributionAggregator_Integration(t *testing.T) {
	tests := []struct {
		name       string
		numBuckets int
		scores     []float64
		checkFunc  func(t *testing.T, agg *BucketScoreDistributionAggregator, result map[entity.AggregatorType]*entity.AggregateData)
	}{
		{
			name:       "Full integration test with various scores",
			numBuckets: 20,
			scores:     []float64{0.0, 0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.5, 4.0, 4.5, 5.0},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator, result map[entity.AggregatorType]*entity.AggregateData) {
				assert.Equal(t, 0.0, agg.Min)
				assert.Equal(t, 5.0, agg.Max)
				assert.Equal(t, int64(11), agg.Total)

				data := result[entity.Distribution]
				assert.NotNil(t, data)
				// All buckets should be included
				assert.Equal(t, 20, len(data.ScoreDistribution.ScoreDistributionItems))

				totalCount := int64(0)
				nonEmptyCount := 0
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					totalCount += item.Count
					assert.GreaterOrEqual(t, item.Count, int64(0), "Bucket count should be >= 0")
					assert.GreaterOrEqual(t, item.Percentage, 0.0)
					assert.LessOrEqual(t, item.Percentage, 1.0)
					if item.Count > 0 {
						nonEmptyCount++
					}
				}
				assert.Equal(t, int64(11), totalCount, "Total count should match number of scores")
				assert.Greater(t, nonEmptyCount, 0, "Should have at least one non-empty bucket")
			},
		},
		{
			name:       "Integration test with empty aggregator",
			numBuckets: 10,
			scores:     []float64{},
			checkFunc: func(t *testing.T, agg *BucketScoreDistributionAggregator, result map[entity.AggregatorType]*entity.AggregateData) {
				assert.Equal(t, int64(0), agg.Total)
				assert.Len(t, agg.Scores, 0)

				data := result[entity.Distribution]
				assert.NotNil(t, data)
				// All buckets should be included, even when empty
				assert.Equal(t, 10, len(data.ScoreDistribution.ScoreDistributionItems))
				// All buckets should have count 0
				for _, item := range data.ScoreDistribution.ScoreDistributionItems {
					assert.Equal(t, int64(0), item.Count)
					assert.Equal(t, 0.0, item.Percentage)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agg := NewBucketScoreDistributionAggregator(tt.numBuckets)
			for _, score := range tt.scores {
				agg.Append(score)
			}
			result := agg.Result()
			if tt.checkFunc != nil {
				tt.checkFunc(t, agg, result)
			}
		})
	}
}

func TestBucketScoreDistributionAggregator_CompleteFlow_1To157_20Buckets(t *testing.T) {
	const numBuckets = 20
	const minScore = 1.0
	const maxScore = 157.0
	const totalScores = 157

	// Setup: create aggregator and append scores
	agg := NewBucketScoreDistributionAggregator(numBuckets)
	for i := 1; i <= totalScores; i++ {
		agg.Append(float64(i))
	}

	// Get result
	result := agg.Result()
	data := result[entity.Distribution]

	// Calculate expected bucket distribution
	bucketCounts := make([]int64, numBuckets)
	for _, score := range agg.Scores {
		bucketIndex := agg.getBucketIndex(score)
		bucketCounts[bucketIndex]++
	}

	// Calculate bucket width
	var bucketWidth float64
	if len(agg.Scores) > 0 && agg.Max != agg.Min {
		bucketWidth = (agg.Max - agg.Min) / float64(agg.NumBuckets)
	}

	// Build a map of bucket index to result item for verification
	bucketIndexToItem := make(map[int]*entity.ScoreDistributionItem)
	for _, item := range data.ScoreDistribution.ScoreDistributionItems {
		var start, end float64
		n, err := fmt.Sscanf(item.Score, "%f-%f", &start, &end)
		assert.NoError(t, err, "Failed to parse score range: %s", item.Score)
		assert.Equal(t, 2, n, "Should parse two values from range: %s", item.Score)

		// Find which bucket index this range corresponds to
		bucketIndex := -1
		for i := 0; i < numBuckets; i++ {
			expectedStart := minScore + float64(i)*bucketWidth
			if start >= expectedStart-0.1 && start <= expectedStart+0.1 {
				bucketIndex = i
				break
			}
		}
		assert.GreaterOrEqual(t, bucketIndex, 0, "Could not determine bucket index for range %s", item.Score)
		bucketIndexToItem[bucketIndex] = item
	}

	// Table-driven tests for basic properties
	t.Run("BasicProperties", func(t *testing.T) {
		tests := []struct {
			name     string
			actual   interface{}
			expected interface{}
			msg      string
		}{
			{"Total", agg.Total, int64(totalScores), "Total should match total scores"},
			{"Min", agg.Min, minScore, "Min should match minScore"},
			{"Max", agg.Max, maxScore, "Max should match maxScore"},
			{"ScoresLength", len(agg.Scores), totalScores, "Scores length should match total scores"},
			{"ResultNotNull", result != nil, true, "Result should not be nil"},
			{"DataNotNull", data != nil, true, "Data should not be nil"},
			{"DataType", data.DataType, entity.ScoreDistribution, "DataType should be ScoreDistribution"},
			{"ScoreDistributionNotNull", data.ScoreDistribution != nil, true, "ScoreDistribution should not be nil"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, tt.actual, tt.msg)
			})
		}

		// Verify all scores are stored correctly
		for i := 0; i < totalScores; i++ {
			assert.Equal(t, float64(i+1), agg.Scores[i], "Score at index %d should be %d", i, i+1)
		}
	})

	// Table-driven tests for bucket width calculation
	t.Run("BucketWidth", func(t *testing.T) {
		expectedBucketWidth := (maxScore - minScore) / float64(numBuckets) // 156.0 / 20.0 = 7.8
		assert.InDelta(t, expectedBucketWidth, bucketWidth, 0.0001, "Bucket width should be 7.8")
	})

	// Table-driven tests for bucket distribution
	t.Run("BucketDistribution", func(t *testing.T) {
		// Verify total count matches
		totalCount := int64(0)
		for _, count := range bucketCounts {
			totalCount += count
		}
		assert.Equal(t, int64(totalScores), totalCount, "Total count should match total scores")

		// Verify bucket indices are valid
		for _, score := range agg.Scores {
			bucketIndex := agg.getBucketIndex(score)
			assert.GreaterOrEqual(t, bucketIndex, 0, "Bucket index should be >= 0 for score %f", score)
			assert.Less(t, bucketIndex, numBuckets, "Bucket index should be < %d for score %f", numBuckets, score)
		}
	})

	// Table-driven tests for bucket items verification
	t.Run("BucketItems", func(t *testing.T) {
		totalPercentage := 0.0

		for i := 0; i < numBuckets; i++ {
			item, exists := bucketIndexToItem[i]
			assert.True(t, exists, "Bucket %d should have a result item", i)

			// Parse score range
			var start, end float64
			n, err := fmt.Sscanf(item.Score, "%f-%f", &start, &end)
			assert.NoError(t, err, "Failed to parse score range: %s", item.Score)
			assert.Equal(t, 2, n, "Should parse two values from range: %s", item.Score)

			// Verify display range is valid
			assert.LessOrEqual(t, start, end, "Start should be <= end for range %s", item.Score)
			assert.GreaterOrEqual(t, start, minScore, "Start should be >= minScore for range %s", item.Score)
			assert.LessOrEqual(t, end, maxScore, "End should be <= maxScore for range %s", item.Score)

			// Verify display ranges don't overlap
			if i > 0 {
				for j := i - 1; j >= 0; j-- {
					if bucketCounts[j] > 0 {
						prevItem := bucketIndexToItem[j]
						var prevStart, prevEndVal float64
						n, err := fmt.Sscanf(prevItem.Score, "%f-%f", &prevStart, &prevEndVal)
						assert.NoError(t, err, "Failed to parse previous score range: %s", prevItem.Score)
						assert.Equal(t, 2, n, "Should parse two values from previous range: %s", prevItem.Score)
						assert.Less(t, prevEndVal, start, "Display ranges should not overlap: bucket %d end %f should be < bucket %d start %f", j, prevEndVal, i, start)
						break
					}
				}
			} else {
				assert.InDelta(t, minScore, start, 0.01, "First bucket should start at minScore")
			}

			// Verify actual bucket range
			var actualBucketWidth float64
			if len(agg.Scores) > 0 && agg.Max != agg.Min {
				actualBucketWidth = (agg.Max - agg.Min) / float64(agg.NumBuckets)
			}
			actualStart, actualEnd := agg.getBucketRange(i, actualBucketWidth)
			assert.InDelta(t, actualStart, start, 0.01, "Display start should match actual start for bucket %d", i)
			if i < numBuckets-1 {
				expectedDisplayEnd := math.Floor((actualEnd-0.01)*100) / 100
				assert.InDelta(t, expectedDisplayEnd, end, 0.01, "Display end should be actualEnd - 0.01 for bucket %d", i)
			} else {
				assert.InDelta(t, actualEnd, end, 0.01, "Last bucket display end should match actual end")
			}

			// Verify count and percentage
			assert.Equal(t, bucketCounts[i], item.Count, "Count mismatch for bucket %d, range %s", i, item.Score)
			assert.GreaterOrEqual(t, item.Count, int64(0), "Bucket count should be >= 0 for range %s", item.Score)
			expectedPercentage := 0.0
			if totalScores > 0 {
				expectedPercentage = float64(item.Count) / float64(totalScores)
			}
			assert.InDelta(t, expectedPercentage, item.Percentage, 0.0001, "Percentage mismatch for range %s", item.Score)

			totalPercentage += item.Percentage
		}

		// Verify total percentage
		assert.InDelta(t, 1.0, totalPercentage, 0.0001, "Total percentage should be 1.0, got %f", totalPercentage)
	})

	// Table-driven tests for boundary cases
	t.Run("BoundaryCases", func(t *testing.T) {
		tests := []struct {
			name           string
			score          float64
			expectedBucket int
			expectedRange  struct {
				min int
				max int
			}
		}{
			{"Score1", 1.0, 0, struct{ min, max int }{0, 0}},
			{"Score157", 157.0, numBuckets - 1, struct{ min, max int }{numBuckets - 1, numBuckets - 1}},
			{"MiddleScore", (minScore + maxScore) / 2.0, -1, struct{ min, max int }{9, 11}}, // 79.0, should be around bucket 10
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				bucketIndex := agg.getBucketIndex(tt.score)
				if tt.expectedBucket >= 0 {
					assert.Equal(t, tt.expectedBucket, bucketIndex, "Score %f should be in bucket %d", tt.score, tt.expectedBucket)
				} else {
					assert.GreaterOrEqual(t, bucketIndex, tt.expectedRange.min, "Score %f bucket index should be >= %d", tt.score, tt.expectedRange.min)
					assert.LessOrEqual(t, bucketIndex, tt.expectedRange.max, "Score %f bucket index should be <= %d", tt.score, tt.expectedRange.max)
				}
				assert.GreaterOrEqual(t, bucketIndex, 0, "Bucket index should be >= 0")
				assert.Less(t, bucketIndex, numBuckets, "Bucket index should be < %d", numBuckets)
			})
		}
	})

	// Table-driven tests for last bucket
	t.Run("LastBucket", func(t *testing.T) {
		lastBucketIndex := numBuckets - 1
		if bucketCounts[lastBucketIndex] > 0 {
			lastItem := bucketIndexToItem[lastBucketIndex]
			var lastStart, lastEnd float64
			n, err := fmt.Sscanf(lastItem.Score, "%f-%f", &lastStart, &lastEnd)
			assert.NoError(t, err, "Failed to parse last bucket score range: %s", lastItem.Score)
			assert.Equal(t, 2, n, "Should parse two values from last bucket range: %s", lastItem.Score)
			assert.InDelta(t, maxScore, lastEnd, 0.01, "Last bucket should end at maxScore")
		}
	})

	// Table-driven tests for distribution evenness
	t.Run("DistributionEvenness", func(t *testing.T) {
		expectedAvgCount := float64(totalScores) / float64(numBuckets)
		nonEmptyBuckets := 0
		for i := 0; i < numBuckets; i++ {
			if bucketCounts[i] > 0 {
				nonEmptyBuckets++
				assert.GreaterOrEqual(t, bucketCounts[i], int64(expectedAvgCount-2), "Bucket %d count %d too low (expected ~%f)", i, bucketCounts[i], expectedAvgCount)
				assert.LessOrEqual(t, bucketCounts[i], int64(expectedAvgCount+2), "Bucket %d count %d too high (expected ~%f)", i, bucketCounts[i], expectedAvgCount)
			}
		}
		assert.GreaterOrEqual(t, nonEmptyBuckets, 15, "Should have at least 15 non-empty buckets out of 20")
	})

	// Verify result items count
	t.Run("ResultItemsCount", func(t *testing.T) {
		assert.Greater(t, len(data.ScoreDistribution.ScoreDistributionItems), 0, "Should have at least one bucket")
	})
}

func TestBucketScoreDistributionAggregator_BoundaryValueHandling(t *testing.T) {
	const numBuckets = 20
	const minScore = 1.0
	const maxScore = 157.0

	// Setup: create aggregator and establish range
	agg := NewBucketScoreDistributionAggregator(numBuckets)
	agg.Append(minScore)
	agg.Append(maxScore)

	// Calculate bucket width
	bucketWidth := (maxScore - minScore) / float64(numBuckets) // 7.8

	// Table-driven tests for boundary values
	t.Run("BoundaryScores", func(t *testing.T) {
		tests := []struct {
			name           string
			score          float64
			expectedBucket int
			description    string
		}{
			{
				name:           "Score equals first bucket end",
				score:          minScore + bucketWidth, // 1.0 + 7.8 = 8.8
				expectedBucket: 1,
				description:    "Score 8.8 equals bucket 0 end, should belong to bucket 1",
			},
			{
				name:           "Score slightly less than bucket end",
				score:          minScore + bucketWidth - 0.0001, // 8.7999
				expectedBucket: 0,
				description:    "Score 8.7999 is in bucket 0 [1.0, 8.8)",
			},
			{
				name:           "Score slightly greater than bucket start",
				score:          minScore + bucketWidth + 0.0001, // 8.8001
				expectedBucket: 1,
				description:    "Score 8.8001 is in bucket 1 [8.8, 16.6)",
			},
			{
				name:           "Score equals middle bucket end",
				score:          minScore + 10*bucketWidth, // 1.0 + 78.0 = 79.0
				expectedBucket: 10,
				description:    "Score 79.0 equals bucket 9 end, should belong to bucket 10",
			},
			{
				name:           "Score equals last bucket start",
				score:          minScore + 19*bucketWidth, // 1.0 + 148.2 = 149.2
				expectedBucket: 19,
				description:    "Score 149.2 equals bucket 18 end, should belong to bucket 19",
			},
			{
				name:           "Score equals max",
				score:          maxScore, // 157.0
				expectedBucket: 19,
				description:    "Score 157.0 equals max, should belong to last bucket",
			},
			{
				name:           "Score equals min",
				score:          minScore, // 1.0
				expectedBucket: 0,
				description:    "Score 1.0 equals min, should belong to first bucket",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				bucketIndex := agg.getBucketIndex(tt.score)
				assert.Equal(t, tt.expectedBucket, bucketIndex, "%s: score %f should be in bucket %d, got %d", tt.description, tt.score, tt.expectedBucket, bucketIndex)

				// Verify the bucket range includes this score correctly
				var bucketWidth float64
				if len(agg.Scores) > 0 && agg.Max != agg.Min {
					bucketWidth = (agg.Max - agg.Min) / float64(agg.NumBuckets)
				}
				start, end := agg.getBucketRange(bucketIndex, bucketWidth)

				// For left-closed right-open intervals [start, end)
				// Score should be >= start and < end (or == end only for last bucket's max)
				if bucketIndex == agg.NumBuckets-1 {
					assert.GreaterOrEqual(t, tt.score, start, "Score should be >= bucket start")
					assert.LessOrEqual(t, tt.score, end, "Score should be <= bucket end (last bucket includes max)")
				} else {
					assert.GreaterOrEqual(t, tt.score, start, "Score should be >= bucket start")
					assert.Less(t, tt.score, end, "Score should be < bucket end (left-closed right-open)")
				}
			})
		}
	})

	// Table-driven test for all boundary values distribution
	t.Run("AllBoundaryValuesDistribution", func(t *testing.T) {
		agg2 := NewBucketScoreDistributionAggregator(numBuckets)
		for i := 0; i < numBuckets; i++ {
			boundaryScore := minScore + float64(i+1)*bucketWidth
			if i == numBuckets-1 {
				boundaryScore = maxScore
			}
			agg2.Append(boundaryScore)
		}

		// Verify all boundary scores are distributed correctly
		result := agg2.Result()
		data := result[entity.Distribution]
		assert.NotNil(t, data, "Result data should not be nil")

		// Verify that boundary scores don't cause duplicate counts
		totalCount := int64(0)
		for _, item := range data.ScoreDistribution.ScoreDistributionItems {
			totalCount += item.Count
		}
		assert.Equal(t, int64(numBuckets), totalCount, "Total count should equal number of boundary scores")
	})
}
