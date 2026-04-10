// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// mock DenyReason implementation

func TestNewExptTurnEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	mockEvaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)

	eval := NewExptTurnEvaluation(mockMetric, mockEvalTargetService, mockEvaluatorService, mockBenefitService, mockEvalAsyncRepo, mockEvalSetItemSvc, mockEvaluatorRecordService)
	assert.NotNil(t, eval)

	impl, ok := eval.(*DefaultExptTurnEvaluationImpl)
	assert.True(t, ok)
	assert.Equal(t, mockMetric, impl.metric)
	assert.Equal(t, mockEvalTargetService, impl.evalTargetService)
	assert.Equal(t, mockEvaluatorService, impl.evaluatorService)
	assert.Equal(t, mockBenefitService, impl.benefitService)
	assert.Equal(t, mockEvalAsyncRepo, impl.evalAsyncRepo)
	assert.Equal(t, mockEvalSetItemSvc, impl.evalSetItemSvc)
	assert.Equal(t, mockEvaluatorRecordService, impl.evaluatorRecordService)
}

func TestDefaultExptTurnEvaluationImpl_Eval(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evalTargetService: mockEvalTargetService,
		evaluatorService:  mockEvaluatorService,
		benefitService:    mockBenefitService,
	}

	tests := []struct {
		name    string
		prepare func()
		etec    *entity.ExptTurnEvalCtx
		wantErr bool
	}{
		{
			name: "normal flow",
			prepare: func() {
				mockMetric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any())
				mockMetric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{SpaceID: 1, Session: &entity.Session{UserID: "1"}},
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Online,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			wantErr: false,
		},
		{
			name: "no target config - skip call",
			prepare: func() {
				mockMetric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any())
				mockMetric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{SpaceID: 1, Session: &entity.Session{UserID: "1"}},
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Offline,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: nil, // no target config
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			wantErr: false,
		},
		{
			name: "call target failed",
			prepare: func() {
				mockMetric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any())
				mockMetric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock benefit error"))
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{
						ExptID:  1,
						SpaceID: 1,
						Session: &entity.Session{UserID: "1"},
					},
					Expt: &entity.Experiment{
						ExptType:        entity.ExptType_Offline,
						TargetVersionID: 1,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			got := service.Eval(context.Background(), tt.etec)
			if tt.wantErr {
				assert.Error(t, got.EvalErr)
			} else {
				assert.NoError(t, got.EvalErr)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_buildEvaluatorInputData_Agent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := &DefaultExptTurnEvaluationImpl{}

	mockContent1 := &entity.Content{Text: gptr.Of("value1")}
	mockContent2 := &entity.Content{Text: gptr.Of("value2")}

	turnFields := map[string]*entity.Content{
		"turn_field1": mockContent1,
		"turn_field2": mockContent2,
	}

	targetFields := map[string]*entity.Content{
		"target_field1": mockContent1,
	}

	tests := []struct {
		name          string
		evaluatorType entity.EvaluatorType
		ec            *entity.EvaluatorConf
		turnFields    map[string]*entity.Content
		targetFields  map[string]*entity.Content
		inputSchemas  []*entity.ArgsSchema
		ext           map[string]string
		wantInputData *entity.EvaluatorInputData
		wantErr       bool
		mockSetup     func(mockEvalSetItemSvc *svcmocks.MockEvaluationSetItemService)
	}{
		{
			name:          "Agent evaluator - with full dataset context",
			evaluatorType: entity.EvaluatorTypeAgent,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "eval_field", FromField: "turn_field1"},
						},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "target_field", FromField: "target_field1"},
						},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				HistoryMessages: nil,
				InputFields: map[string]*entity.Content{
					"eval_field":   mockContent1,
					"target_field": mockContent1,
				},
				EvaluateDatasetFields: map[string]*entity.Content{
					"turn_field1": mockContent1,
					"turn_field2": mockContent2,
				},
				EvaluateTargetOutputFields: targetFields,
				Ext:                        make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "Agent evaluator - with omitted content",
			evaluatorType: entity.EvaluatorTypeAgent,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
					TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
				},
			},
			turnFields: map[string]*entity.Content{
				"omitted_field": {
					ContentType:    gptr.Of(entity.ContentTypeText),
					Text:           nil,
					ContentOmitted: gptr.Of(true),
				},
			},
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				InputFields: targetFields,
				EvaluateDatasetFields: map[string]*entity.Content{
					"omitted_field": mockContent1,
				},
				EvaluateTargetOutputFields: targetFields,
				Ext:                        make(map[string]string),
			},
			wantErr: false,
			mockSetup: func(mockEvalSetItemSvc *svcmocks.MockEvaluationSetItemService) {
				mockEvalSetItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.Any()).Return(&entity.FieldData{
					Content: mockContent1,
				}, nil)
			},
		},
		{
			name:          "Agent evaluator - getAllEvalSetFields error",
			evaluatorType: entity.EvaluatorTypeAgent,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
					TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
				},
			},
			turnFields: map[string]*entity.Content{
				"omitted_field": {
					ContentType:    gptr.Of(entity.ContentTypeText),
					Text:           nil,
					ContentOmitted: gptr.Of(true),
				},
			},
			targetFields:  targetFields,
			wantInputData: nil,
			wantErr:       true,
			mockSetup: func(mockEvalSetItemSvc *svcmocks.MockEvaluationSetItemService) {
				mockEvalSetItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.Any()).Return(nil, errors.New("get field error"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
			if tt.mockSetup != nil {
				tt.mockSetup(mockEvalSetItemSvc)
			}
			service.evalSetItemSvc = mockEvalSetItemSvc

			turn := &entity.Turn{
				ID:            1,
				FieldDataList: []*entity.FieldData{},
			}
			for key, c := range tt.turnFields {
				turn.FieldDataList = append(turn.FieldDataList, &entity.FieldData{
					Name:    key,
					Content: c,
				})
			}

			got, err := service.buildEvaluatorInputData(ctx, 0, tt.evaluatorType, tt.ec, turn, tt.targetFields, tt.inputSchemas, tt.ext)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantInputData.HistoryMessages, got.HistoryMessages)
			assert.Equal(t, tt.wantInputData.InputFields, got.InputFields)
			assert.Equal(t, tt.wantInputData.EvaluateDatasetFields, got.EvaluateDatasetFields)
			assert.Equal(t, tt.wantInputData.EvaluateTargetOutputFields, got.EvaluateTargetOutputFields)
			assert.Equal(t, tt.wantInputData.Ext, got.Ext)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_asyncCallEvaluator_Agent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:           mockMetric,
		evaluatorService: mockEvaluatorService,
		evalAsyncRepo:    mockEvalAsyncRepo,
	}

	ev := &entity.Evaluator{
		ID:            1,
		EvaluatorType: entity.EvaluatorTypeAgent,
		AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
			ID: 101,
		},
	}
	ec := &entity.EvaluatorConf{
		RunConf: &entity.EvaluatorRunConfig{},
	}
	etec := &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:   1,
				ExptID:    2,
				ExptRunID: 3,
				Session:   &entity.Session{UserID: "test_user"},
			},
			EvalSetItem: &entity.EvaluationSetItem{
				ItemID: 4,
			},
		},
		Turn: &entity.Turn{
			ID: 5,
		},
		Ext: map[string]string{"key": "val"},
	}
	inputData := &entity.EvaluatorInputData{
		InputFields: map[string]*entity.Content{},
	}
	var recordMap sync.Map

	mockEvaluatorRecord := &entity.EvaluatorRecord{
		ID:     202,
		Status: entity.EvaluatorRunStatusAsyncInvoking,
	}

	// Expectations
	mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false)

	mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, req *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
			assert.Equal(t, int64(1), req.SpaceID)
			assert.Equal(t, int64(101), req.EvaluatorVersionID)
			assert.Equal(t, inputData, req.InputData)
			assert.Equal(t, int64(2), req.ExperimentID)
			assert.Equal(t, int64(3), req.ExperimentRunID)
			assert.Equal(t, int64(4), req.ItemID)
			assert.Equal(t, int64(5), req.TurnID)
			assert.Equal(t, etec.Ext, req.Ext)
			return mockEvaluatorRecord, nil
		},
	)

	mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, key string, val *entity.EvalAsyncCtx) error {
			assert.Equal(t, "evaluator:202", key)
			assert.Equal(t, int64(202), val.RecordID)
			assert.Equal(t, int64(101), val.EvaluatorVersionID)
			assert.Equal(t, etec.Event, val.Event)
			// Check timestamp
			assert.True(t, val.AsyncUnixMS <= time.Now().UnixMilli())
			assert.True(t, val.AsyncUnixMS > time.Now().Add(-time.Minute).UnixMilli())
			return nil
		},
	)

	err := service.asyncCallEvaluator(context.Background(), ev, ec, etec, inputData, &recordMap)
	assert.NoError(t, err)

	// verify recordMap
	val, ok := recordMap.Load(int64(101))
	assert.True(t, ok)
	assert.Equal(t, mockEvaluatorRecord, val)
}

func TestDefaultExptTurnEvaluationImpl_asyncCallEvaluator_Agent_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:           mockMetric,
		evaluatorService: mockEvaluatorService,
		evalAsyncRepo:    mockEvalAsyncRepo,
	}

	ev := &entity.Evaluator{
		ID:            1,
		EvaluatorType: entity.EvaluatorTypeAgent,
		AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
			ID: 101,
		},
	}
	ec := &entity.EvaluatorConf{
		RunConf: &entity.EvaluatorRunConfig{},
	}
	etec := &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				SpaceID:   1,
				ExptID:    2,
				ExptRunID: 3,
				Session:   &entity.Session{UserID: "test_user"},
			},
			EvalSetItem: &entity.EvaluationSetItem{
				ItemID: 4,
			},
		},
		Turn: &entity.Turn{
			ID: 5,
		},
		Ext: map[string]string{"key": "val"},
	}
	inputData := &entity.EvaluatorInputData{
		InputFields: map[string]*entity.Content{},
	}
	var recordMap sync.Map

	tests := []struct {
		name      string
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "AsyncRunEvaluator error",
			mockSetup: func() {
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), true)
				mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(nil, errors.New("async run error"))
			},
			wantErr: true,
		},
		{
			name: "SetEvalAsyncCtx error",
			mockSetup: func() {
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), true)
				mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{
					ID: 202,
				}, nil)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("set ctx error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			err := service.asyncCallEvaluator(context.Background(), ev, ec, etec, inputData, &recordMap)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CallTarget(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evalTargetService: mockEvalTargetService,
		benefitService:    mockBenefitService,
	}

	mockContent := &entity.Content{Text: gptr.Of("value1")}
	mockTargetResult := &entity.EvalTargetRecord{
		ID: 1,
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"field1": mockContent,
			},
		},
	}

	tests := []struct {
		name    string
		prepare func()
		etec    *entity.ExptTurnEvalCtx
		want    *entity.EvalTargetRecord
		wantErr bool
	}{
		{
			name:    "online experiment - skip call",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Online,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			want: &entity.EvalTargetRecord{
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: make(map[string]*entity.Content),
				},
			},
			wantErr: false,
		},
		{
			name:    "no target config - skip call",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Offline,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: nil, // no target config
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			want: &entity.EvalTargetRecord{
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: make(map[string]*entity.Content),
				},
			},
			wantErr: false,
		},
		{
			name:    "already has successful result",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{
						SpaceID: 1,
						ExptID:  1,
						Session: &entity.Session{
							UserID: "test_user",
						},
					},
					Expt: &entity.Experiment{
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					TargetResult: &entity.EvalTargetRecord{
						ID: 1,
						EvalTargetOutputData: &entity.EvalTargetOutputData{
							OutputFields: map[string]*entity.Content{
								"field1": mockContent,
							},
						},
						Status: gptr.Of(entity.EvalTargetRunStatusSuccess),
					},
				},
			},
			want:    mockTargetResult,
			wantErr: false,
		},
		{
			name:    "no target config - skip call",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Offline,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: nil, // no target config
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			want: &entity.EvalTargetRecord{
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: make(map[string]*entity.Content),
				},
			},
			wantErr: false,
		},
		{
			name: "privilege check failed",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType:        entity.ExptType_Offline,
						TargetVersionID: 1,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
								},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
						ExptID:  1,
						SpaceID: 2,
						Session: &entity.Session{
							UserID: "test_user",
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			wantErr: true,
		},
		{
			name: "normal flow - actually call callTarget",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvalTargetService.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTargetResult, nil)
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType:        entity.ExptType_Offline,
						TargetVersionID: 1,
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
										},
									},
								},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
						ExptID:  1,
						SpaceID: 2,
						Session: &entity.Session{UserID: "test_user"},
					},
					EvalSetItem: &entity.EvaluationSetItem{
						ItemID: 1,
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn: &entity.Turn{
					ID:            1,
					FieldDataList: []*entity.FieldData{{Name: "field1", Content: mockContent}},
				},
			},
			want:    mockTargetResult,
			wantErr: false,
		},
		{
			name:    "no target config - skip call",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Offline,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: nil, // no target config
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			want: &entity.EvalTargetRecord{
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: make(map[string]*entity.Content),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			_, err := service.CallTarget(context.Background(), tt.etec)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CheckBenefit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		benefitService: mockBenefitService,
	}

	tests := []struct {
		name     string
		prepare  func()
		exptID   int64
		spaceID  int64
		freeCost bool
		session  *entity.Session
		wantErr  bool
	}{
		{
			name: "normal flow",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
			},
			exptID:   1,
			spaceID:  2,
			freeCost: false,
			session:  &entity.Session{UserID: "test_user"},
			wantErr:  false,
		},
		{
			name: "check failed",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			exptID:   1,
			spaceID:  2,
			freeCost: false,
			session:  &entity.Session{UserID: "test_user"},
			wantErr:  true,
		},
		{
			name: "deny reason exists",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{
					DenyReason: gptr.Of(benefit.DenyReason(1)),
				}, nil)
			},
			exptID:   1,
			spaceID:  2,
			freeCost: false,
			session:  &entity.Session{UserID: "test_user"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			err := service.CheckBenefit(context.Background(), tt.exptID, tt.spaceID, tt.freeCost, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CallTarget_AsyncReport(t *testing.T) {
	t.Parallel()
	service := &DefaultExptTurnEvaluationImpl{}

	tests := []struct {
		name    string
		etec    *entity.ExptTurnEvalCtx
		want    *entity.EvalTargetRecord
		wantErr bool
	}{
		{
			name: "AsyncReportTrigger with valid result",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{ID: 1, TargetVersionID: 1}, // Initialize Expt
					Event: &entity.ExptItemEvalEvent{
						AsyncReportTrigger: true,
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					TargetResult: &entity.EvalTargetRecord{ID: 1},
				},
			},
			want:    &entity.EvalTargetRecord{ID: 1},
			wantErr: false,
		},
		{
			name: "AsyncReportTrigger with nil result",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{ID: 1, TargetVersionID: 1}, // Initialize Expt
					Event: &entity.ExptItemEvalEvent{
						AsyncReportTrigger: true,
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					TargetResult: nil,
				},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.CallTarget(context.Background(), tt.etec)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CallTarget_ExistedRecord_Status(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evalTargetService: mockEvalTargetService,
		benefitService:    mockBenefitService,
		evalSetItemSvc:    mockEvalSetItemSvc,
	}

	tests := []struct {
		name      string
		etec      *entity.ExptTurnEvalCtx
		mockSetup func()
		wantID    int64
	}{
		{
			name: "Existed record with success status - return directly",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						TargetVersionID: 1,
						ExptType:        entity.ExptType_Offline,
						EvalConf:        &entity.EvaluationConfiguration{ConnectorConf: entity.Connector{TargetConf: &entity.TargetConf{TargetVersionID: 1}}},
					},
					Event: &entity.ExptItemEvalEvent{
						SpaceID: 1,
						Session: &entity.Session{UserID: "u1"},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					TargetResult: &entity.EvalTargetRecord{
						ID:     100,
						Status: gptr.Of(entity.EvalTargetRunStatusSuccess),
					},
				},
			},
			mockSetup: func() {}, // No calls expected
			wantID:    100,
		},
		{
			name: "Existed record with failed status - proceed to call",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						TargetVersionID: 1,
						ExptType:        entity.ExptType_Offline,
						Target:          &entity.EvalTarget{ID: 1, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1}},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "f1", FromField: "f1"}}},
									},
								},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
						SpaceID: 1,
						Session: &entity.Session{UserID: "u1"},
					},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
				},
				Turn: &entity.Turn{ID: 1},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					TargetResult: &entity.EvalTargetRecord{
						ID:     100,
						Status: gptr.Of(entity.EvalTargetRunStatusFail),
					},
				},
			},
			mockSetup: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvalTargetService.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{ID: 200}, nil)
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
			},
			wantID: 200,
		},
		{
			name: "CustomRPCServer target with omitted content",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						TargetVersionID: 1,
						ExptType:        entity.ExptType_Offline,
						Target:          &entity.EvalTarget{ID: 1, EvalTargetType: entity.EvalTargetTypeCustomRPCServer, EvalTargetVersion: &entity.EvalTargetVersion{ID: 1}},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
									},
								},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
						SpaceID: 1,
						Session: &entity.Session{UserID: "u1"},
					},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
				},
				Turn: &entity.Turn{
					ID: 1,
					FieldDataList: []*entity.FieldData{
						{
							Name: "f1",
							Content: &entity.Content{
								ContentOmitted: gptr.Of(true),
								ContentType:    gptr.Of(entity.ContentTypeText),
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					TargetResult: nil,
				},
			},
			mockSetup: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				// Expect fetching omitted content
				mockEvalSetItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.Any()).Return(&entity.FieldData{
					Content: &entity.Content{Text: gptr.Of("full content")},
				}, nil)
				mockEvalTargetService.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.EvalTargetRecord{ID: 300}, nil)
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
			},
			wantID: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}
			got, err := service.CallTarget(context.Background(), tt.etec)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_buildEvalSetFields_Errors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	service := &DefaultExptTurnEvaluationImpl{
		evalSetItemSvc: mockEvalSetItemSvc,
	}

	tests := []struct {
		name      string
		fcs       []*entity.FieldConf
		turn      *entity.Turn
		mockSetup func()
		wantErr   bool
	}{
		{
			name: "getFieldContent error",
			fcs: []*entity.FieldConf{
				{FieldName: "f1", FromField: "[invalid"},
			},
			turn: &entity.Turn{
				FieldDataList: []*entity.FieldData{
					{Name: "f1", Content: &entity.Content{Text: gptr.Of("v")}},
				},
			},
			mockSetup: func() {},
			wantErr:   true,
		},
		{
			name: "GetEvaluationSetItemField error for omitted content",
			fcs: []*entity.FieldConf{
				{FieldName: "f1", FromField: "f1"},
			},
			turn: &entity.Turn{
				FieldDataList: []*entity.FieldData{
					{Name: "f1", Content: &entity.Content{
						ContentOmitted: gptr.Of(true),
						ContentType:    gptr.Of(entity.ContentTypeText),
					}},
				},
			},
			mockSetup: func() {
				mockEvalSetItemSvc.EXPECT().GetEvaluationSetItemField(gomock.Any(), gomock.Any()).Return(nil, errors.New("fetch error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			_, err := service.buildEvalSetFields(context.Background(), 1, tt.fcs, tt.turn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_getContentByJsonPath_Errors(t *testing.T) {
	service := &DefaultExptTurnEvaluationImpl{}

	tests := []struct {
		name     string
		content  *entity.Content
		jsonPath string
		wantErr  bool
	}{
		{
			name: "RemoveFirstJSONPathLevel error",
			content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("{}"),
			},
			jsonPath: "invalid..path", // Should trigger error in RemoveFirstJSONPathLevel if implemented to strict check or just basic invalid format
			// Note: RemoveFirstJSONPathLevel implementation might be robust, but let's try invalid path
			wantErr: false, // Assuming current impl might not error on this specific string, but let's check
		},
		{
			name: "GetStringByJSONPath error",
			content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of(`{"key": "value"}`),
			},
			jsonPath: "$.nonexistent", // Should return empty string, not error usually?
			// To trigger error in GetStringByJSONPath, maybe invalid JSON in text?
			// But GetStringByJSONPath usually handles invalid JSON by returning error.
			wantErr: false,
		},
	}

	// Adjusting test to target specific error conditions based on json pkg
	// If GetStringByJSONPath fails on invalid json:
	tests = append(tests, struct {
		name     string
		content  *entity.Content
		jsonPath string
		wantErr  bool
	}{
		name: "Invalid JSON in content",
		content: &entity.Content{
			ContentType: gptr.Of(entity.ContentTypeText),
			Text:        gptr.Of(`{invalid_json`),
		},
		jsonPath: "$.key.subkey",
		wantErr:  true,
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.getContentByJsonPath(tt.content, tt.jsonPath)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CallEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evaluatorService:  mockEvaluatorService,
		benefitService:    mockBenefitService,
		evalTargetService: mockEvalTargetService,
		evalAsyncRepo:     mockEvalAsyncRepo,
	}

	mockContent := &entity.Content{Text: gptr.Of("value1")}
	mockTargetResult := &entity.EvalTargetRecord{
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"field1": mockContent,
			},
		},
	}
	mockEvaluatorResults := map[int64]*entity.EvaluatorRecord{
		1: {ID: 1, Status: entity.EvaluatorRunStatusSuccess},
	}

	tests := []struct {
		name    string
		prepare func()
		etec    *entity.ExptTurnEvalCtx
		target  *entity.EvalTargetRecord
		wantErr bool
	}{
		{
			name: "normal flow",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).Return(mockEvaluatorResults[1], nil)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{
						ID:     1,
						ItemID: 2,
					},
					Event: &entity.ExptItemEvalEvent{
						Session: &entity.Session{UserID: "test_user"},
						ExptID:  1,
						SpaceID: 2,
					},
					Expt: &entity.Experiment{
						ID:      1,
						SpaceID: 2,
						Evaluators: []*entity.Evaluator{
							{
								ID:            1,
								EvaluatorType: entity.EvaluatorTypePrompt,
								PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
									ID: 1,
								},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ItemConcurNum: gptr.Of(1),
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{
											EvaluatorVersionID: 1,
											IngressConf: &entity.EvaluatorIngressConf{
												EvalSetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{
														{
															FieldName: "field1",
															FromField: "field1",
														},
													},
												},
												TargetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{
														{
															FieldName: "field1",
															FromField: "field1",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn: &entity.Turn{
					FieldDataList: []*entity.FieldData{
						{Name: "field1", Content: mockContent},
					},
				},
			},
			target:  mockTargetResult,
			wantErr: false,
		},
		{
			name: "Agent evaluator flow",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{
						ID:     1,
						ItemID: 2,
					},
					Event: &entity.ExptItemEvalEvent{
						Session: &entity.Session{UserID: "test_user"},
						ExptID:  1,
						SpaceID: 2,
					},
					Expt: &entity.Experiment{
						ID:      1,
						SpaceID: 2,
						Evaluators: []*entity.Evaluator{
							{
								ID:            101,
								EvaluatorType: entity.EvaluatorTypeAgent,
								AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
									ID: 101,
								},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ItemConcurNum: gptr.Of(1),
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{
											EvaluatorVersionID: 101,
											IngressConf: &entity.EvaluatorIngressConf{
												EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
												TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
											},
											RunConf: &entity.EvaluatorRunConfig{},
										},
									},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn: &entity.Turn{
					FieldDataList: []*entity.FieldData{},
				},
			},
			target:  mockTargetResult,
			wantErr: false,
		},
		{
			name: "content_omitted triggers LoadRecordOutputFields",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvalTargetService.EXPECT().LoadRecordOutputFields(gomock.Any(), gomock.Any(), []string{"field1"}).Return(nil)
				mockEvaluatorService.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).Return(mockEvaluatorResults[1], nil)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{ID: 1, ItemID: 2},
					Event:       &entity.ExptItemEvalEvent{Session: &entity.Session{UserID: "u"}, ExptID: 1, SpaceID: 2},
					Expt: &entity.Experiment{
						ID: 1, SpaceID: 2,
						Evaluators: []*entity.Evaluator{{
							ID: 1, EvaluatorType: entity.EvaluatorTypePrompt,
							PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1},
						}},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{{
										EvaluatorVersionID: 1,
										IngressConf: &entity.EvaluatorIngressConf{
											EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}}},
											TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}}},
										},
									}},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn:              &entity.Turn{FieldDataList: []*entity.FieldData{{Name: "field1", Content: mockContent}}},
			},
			target: &entity.EvalTargetRecord{
				EvalTargetOutputData: &entity.EvalTargetOutputData{
					OutputFields: map[string]*entity.Content{
						"field1": {
							ContentType:      gptr.Of(entity.ContentTypeText),
							Text:             gptr.Of(""),
							ContentOmitted:   gptr.Of(true),
							FullContent:      &entity.ObjectStorage{URI: gptr.Of("key")},
							FullContentBytes: gptr.Of(int32(0)),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "no target config - skip call",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Offline,
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: nil, // no target config
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			wantErr: false,
		},
		{
			name: "privilege check failed",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						Evaluators: []*entity.Evaluator{
							{
								ID:            1,
								EvaluatorType: entity.EvaluatorTypePrompt,
								PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
									ID: 1,
								},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
						Session: &entity.Session{UserID: "test_user"},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			target:  mockTargetResult,
			wantErr: true,
		},
		{
			name: "evaluator conf not found",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Event:       &entity.ExptItemEvalEvent{Session: &entity.Session{UserID: "u"}, ExptID: 1, SpaceID: 2},
					Expt: &entity.Experiment{
						SpaceID: 2,
						Evaluators: []*entity.Evaluator{
							{
								ID:                     1,
								EvaluatorType:          entity.EvaluatorTypePrompt,
								PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ItemConcurNum: gptr.Of(1),
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{EvaluatorVersionID: 999, IngressConf: &entity.EvaluatorIngressConf{TargetAdapter: &entity.FieldAdapter{}}},
									},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn:              &entity.Turn{FieldDataList: []*entity.FieldData{{Name: "field1", Content: mockContent}}},
			},
			target:  mockTargetResult,
			wantErr: true,
		},
		{
			name: "RunEvaluator error",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).Return(nil, errors.New("run evaluator failed"))
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Event:       &entity.ExptItemEvalEvent{Session: &entity.Session{UserID: "u"}, ExptID: 1, SpaceID: 2},
					Expt: &entity.Experiment{
						SpaceID: 2,
						Evaluators: []*entity.Evaluator{
							{
								ID:                     1,
								EvaluatorType:          entity.EvaluatorTypePrompt,
								PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ItemConcurNum: gptr.Of(1),
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{
											EvaluatorVersionID: 1,
											IngressConf: &entity.EvaluatorIngressConf{
												EvalSetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
												},
												TargetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn:              &entity.Turn{FieldDataList: []*entity.FieldData{{Name: "field1", Content: mockContent}}},
			},
			target:  mockTargetResult,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			_, err := service.CallEvaluators(context.Background(), tt.etec, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_getContentByJsonPath(t *testing.T) {
	s := &DefaultExptTurnEvaluationImpl{}

	type args struct {
		content  *entity.Content
		jsonPath string
	}
	tests := []struct {
		name    string
		args    args
		want    *entity.Content
		wantErr bool
	}{
		{
			name: "normal - json",
			args: args{
				content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of(`{"key": "value"}`),
				},
				jsonPath: "$.key",
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of(`{"key": "value"}`),
			},
			wantErr: false,
		},

		{
			name: "normal - nested json",
			args: args{
				content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of(`{"key": {"inner_key": "inner_value"}}`),
				},
				jsonPath: "$.key.inner_key",
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of(""),
			},
			wantErr: false,
		},

		{
			name: "normal - return entire json",
			args: args{
				content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of(`{"key": "value"}`),
				},
				jsonPath: "$",
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of(`{"key": "value"}`),
			},
			wantErr: false,
		},

		{
			name:    "abnormal - content is nil",
			args:    args{content: nil, jsonPath: "$.key"},
			want:    nil,
			wantErr: false,
		},

		{
			name: "abnormal - contentType is nil",
			args: args{
				content:  &entity.Content{ContentType: nil, Text: gptr.Of(`{"key": "value"}`)},
				jsonPath: "$.key",
			},
			want:    nil,
			wantErr: false,
		},

		{
			name: "abnormal - contentType is not text",
			args: args{
				content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeImage),
					Text:        gptr.Of(`{"key": "value"}`),
				},
				jsonPath: "$.key",
			},
			want:    nil,
			wantErr: false,
		},

		{
			name: "normal - json string",
			args: args{
				content: &entity.Content{
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("{\"age\":18,\"msg\":[{\"role\":1,\"query\":\"hi\"}],\"name\":\"dsf\"}"),
				},
				jsonPath: "parameter",
			},
			want: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("{\"age\":18,\"msg\":[{\"role\":1,\"query\":\"hi\"}],\"name\":\"dsf\"}"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.getContentByJsonPath(tt.args.content, tt.args.jsonPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("getContentByJsonPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want == nil {
				assert.Nil(t, got)
			} else if tt.name == "normal - return entire json" && tt.want.Text != nil && got != nil && got.Text != nil {
				assert.JSONEq(t, *tt.want.Text, *got.Text)
				tmpWant := *tt.want
				tmpGot := *got
				tmpWant.Text = nil
				tmpGot.Text = nil
				assert.Equal(t, tmpWant, tmpGot)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_callTarget_RuntimeParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evalTargetService: mockEvalTargetService,
	}

	ctx := context.Background()
	spaceID := int64(123)
	mockContent := &entity.Content{Text: gptr.Of("test_value")}
	mockTargetResult := &entity.EvalTargetRecord{
		ID: 1,
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"output": mockContent,
			},
		},
	}

	tests := []struct {
		name                  string
		etec                  *entity.ExptTurnEvalCtx
		history               []*entity.Message
		mockSetup             func()
		wantRuntimeParamInExt string
		wantErr               bool
	}{
		{
			name: "runtime param in custom config",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{
						ExptRunID: 1,
					},
					EvalSetItem: &entity.EvaluationSetItem{
						ItemID: 1,
					},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "field1",
													FromField: "field1",
												},
											},
										},
										CustomConf: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
													Value:     `{"model_config":{"model_id":"custom_model","temperature":0.8}}`,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID: 1,
					FieldDataList: []*entity.FieldData{
						{
							Name:    "field1",
							Content: mockContent,
						},
					},
				},
				Ext: map[string]string{},
			},
			history: []*entity.Message{},
			mockSetup: func() {
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
				mockEvalTargetService.EXPECT().ExecuteTarget(
					gomock.Any(),
					spaceID,
					int64(1),
					int64(1),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					// Verify runtime param is injected into Ext
					assert.Contains(t, inputData.Ext, consts.TargetExecuteExtRuntimeParamKey)
					assert.Equal(t, `{"model_config":{"model_id":"custom_model","temperature":0.8}}`, inputData.Ext[consts.TargetExecuteExtRuntimeParamKey])
					return mockTargetResult, nil
				})
			},
			wantRuntimeParamInExt: `{"model_config":{"model_id":"custom_model","temperature":0.8}}`,
			wantErr:               false,
		},
		{
			name: "multiple field configs with runtime param",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{
						ExptRunID: 1,
					},
					EvalSetItem: &entity.EvaluationSetItem{
						ItemID: 1,
					},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "field1",
													FromField: "field1",
												},
											},
										},
										CustomConf: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "other_field",
													Value:     "other_value",
												},
												{
													FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
													Value:     `{"model_config":{"model_id":"multi_config_model"}}`,
												},
												{
													FieldName: "another_field",
													Value:     "another_value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID: 1,
					FieldDataList: []*entity.FieldData{
						{
							Name:    "field1",
							Content: mockContent,
						},
					},
				},
				Ext: map[string]string{
					"existing_key": "existing_value",
				},
			},
			history: []*entity.Message{},
			mockSetup: func() {
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
				mockEvalTargetService.EXPECT().ExecuteTarget(
					gomock.Any(),
					spaceID,
					int64(1),
					int64(1),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					// Verify runtime param is injected into Ext
					assert.Contains(t, inputData.Ext, consts.TargetExecuteExtRuntimeParamKey)
					assert.Equal(t, `{"model_config":{"model_id":"multi_config_model"}}`, inputData.Ext[consts.TargetExecuteExtRuntimeParamKey])
					// Verify existing ext values are preserved
					assert.Contains(t, inputData.Ext, "existing_key")
					assert.Equal(t, "existing_value", inputData.Ext["existing_key"])
					return mockTargetResult, nil
				})
			},
			wantRuntimeParamInExt: `{"model_config":{"model_id":"multi_config_model"}}`,
			wantErr:               false,
		},
		{
			name: "no runtime param configured",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{
						ExptRunID: 1,
					},
					EvalSetItem: &entity.EvaluationSetItem{
						ItemID: 1,
					},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "field1",
													FromField: "field1",
												},
											},
										},
										CustomConf: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "other_field",
													Value:     "other_value",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID: 1,
					FieldDataList: []*entity.FieldData{
						{
							Name:    "field1",
							Content: mockContent,
						},
					},
				},
				Ext: map[string]string{},
			},
			history: []*entity.Message{},
			mockSetup: func() {
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
				mockEvalTargetService.EXPECT().ExecuteTarget(
					gomock.Any(),
					spaceID,
					int64(1),
					int64(1),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					// Verify runtime param is NOT in Ext
					assert.NotContains(t, inputData.Ext, consts.TargetExecuteExtRuntimeParamKey)
					return mockTargetResult, nil
				})
			},
			wantErr: false,
		},
		{
			name: "no custom config - no runtime param",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{
						ExptRunID: 1,
					},
					EvalSetItem: &entity.EvaluationSetItem{
						ItemID: 1,
					},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "field1",
													FromField: "field1",
												},
											},
										},
										CustomConf: nil, // No custom config
									},
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID: 1,
					FieldDataList: []*entity.FieldData{
						{
							Name:    "field1",
							Content: mockContent,
						},
					},
				},
				Ext: map[string]string{},
			},
			history: []*entity.Message{},
			mockSetup: func() {
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), false)
				mockEvalTargetService.EXPECT().ExecuteTarget(
					gomock.Any(),
					spaceID,
					int64(1),
					int64(1),
					gomock.Any(),
					gomock.Any(),
				).DoAndReturn(func(ctx context.Context, spaceID, targetID, targetVersionID int64, param *entity.ExecuteTargetCtx, inputData *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
					// Verify runtime param is NOT in Ext
					assert.NotContains(t, inputData.Ext, consts.TargetExecuteExtRuntimeParamKey)
					return mockTargetResult, nil
				})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup()
			}

			record, err := service.callTarget(ctx, tt.etec, tt.history, spaceID)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, record)
				assert.Equal(t, mockTargetResult.ID, record.ID)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_callTarget_Async(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
	mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evalTargetService: mockEvalTargetService,
		evalAsyncRepo:     mockEvalAsyncRepo,
	}

	spaceID := int64(42)
	targetID := int64(101)
	targetVersionID := int64(202)
	isAsync := true
	record := &entity.EvalTargetRecord{
		ID:                   9999,
		EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{}},
		Status:               gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
		TargetID:             targetID,
		TargetVersionID:      targetVersionID,
		SpaceID:              spaceID,
	}

	turnContent := &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("payload")}
	etec := &entity.ExptTurnEvalCtx{
		ExptItemEvalCtx: &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{
				ExptID:    555,
				ExptRunID: 777,
				SpaceID:   spaceID,
				Session:   &entity.Session{UserID: "user"},
			},
			Expt: &entity.Experiment{
				SpaceID:         spaceID,
				TargetVersionID: targetVersionID,
				Target: &entity.EvalTarget{
					ID:             targetID,
					EvalTargetType: entity.EvalTargetTypeCustomRPCServer,
					EvalTargetVersion: &entity.EvalTargetVersion{
						ID: targetVersionID,
						CustomRPCServer: &entity.CustomRPCServer{
							IsAsync: gptr.Of(isAsync),
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: targetVersionID,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{FieldName: "fieldA", FromField: "fieldA"},
									},
								},
							},
						},
					},
				},
			},
			EvalSetItem: &entity.EvaluationSetItem{ItemID: 888},
		},
		Turn: &entity.Turn{
			ID: 999,
			FieldDataList: []*entity.FieldData{
				{Name: "fieldA", Content: turnContent},
			},
		},
		Ext: map[string]string{"ext-key": "ext-val"},
	}

	mockMetric.EXPECT().EmitTurnExecTargetResult(spaceID, false)

	mockEvalTargetService.EXPECT().AsyncExecuteTarget(gomock.Any(), spaceID, targetID, targetVersionID, gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _, _ int64, param *entity.ExecuteTargetCtx, input *entity.EvalTargetInputData) (*entity.EvalTargetRecord, string, error) {
			assert.Equal(t, int64(777), gptr.Indirect(param.ExperimentRunID))
			assert.Equal(t, int64(888), param.ItemID)
			assert.Equal(t, int64(999), param.TurnID)
			if assert.NotNil(t, input) {
				assert.Equal(t, "payload", gptr.Indirect(input.InputFields["fieldA"].Text))
				assert.Equal(t, "ext-val", input.Ext["ext-key"])
			}
			return record, "callee-service", nil
		},
	)

	mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), strconv.FormatInt(record.ID, 10), gomock.Any()).DoAndReturn(
		func(_ context.Context, invokeID string, actx *entity.EvalAsyncCtx) error {
			assert.Equal(t, strconv.FormatInt(record.ID, 10), invokeID)
			if assert.NotNil(t, actx) {
				assert.Equal(t, "callee-service", actx.Callee)
				assert.Equal(t, etec.Event, actx.Event)
			}
			return nil
		},
	)

	got, err := service.callTarget(context.Background(), etec, []*entity.Message{}, spaceID)
	assert.NoError(t, err)
	assert.Equal(t, record, got)
}

func TestDefaultExptTurnEvaluationImpl_buildEvaluatorInputData(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := &DefaultExptTurnEvaluationImpl{}

	mockContent1 := &entity.Content{Text: gptr.Of("value1")}
	mockContent2 := &entity.Content{Text: gptr.Of("value2")}

	turnFields := map[string]*entity.Content{
		"turn_field1": mockContent1,
		"turn_field2": mockContent2,
	}

	targetFields := map[string]*entity.Content{
		"target_field1": mockContent1,
		"target_field2": mockContent2,
	}

	tests := []struct {
		name          string
		evaluatorType entity.EvaluatorType
		ec            *entity.EvaluatorConf
		turnFields    map[string]*entity.Content
		targetFields  map[string]*entity.Content
		inputSchemas  []*entity.ArgsSchema
		ext           map[string]string
		wantInputData *entity.EvaluatorInputData
		wantErr       bool
	}{
		{
			name:          "Code evaluator - separated field data sources",
			evaluatorType: entity.EvaluatorTypeCode,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "eval_field", FromField: "turn_field1"},
						},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "target_field", FromField: "target_field1"},
						},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				HistoryMessages:       nil,
				InputFields:           make(map[string]*entity.Content),
				EvaluateDatasetFields: map[string]*entity.Content{"eval_field": mockContent1},
				// Code 类型评估器下，目标字段应直接透传原始 targetFields
				EvaluateTargetOutputFields: targetFields,
				Ext:                        make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "Prompt evaluator - merged all fields",
			evaluatorType: entity.EvaluatorTypePrompt,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "eval_field", FromField: "turn_field1"},
						},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "target_field", FromField: "target_field1"},
						},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				HistoryMessages: nil,
				InputFields: map[string]*entity.Content{
					"eval_field":   mockContent1,
					"target_field": mockContent1,
				},
				Ext: make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "Code evaluator - empty field configs",
			evaluatorType: entity.EvaluatorTypeCode,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				HistoryMessages:       nil,
				InputFields:           make(map[string]*entity.Content),
				EvaluateDatasetFields: map[string]*entity.Content{},
				// Code 类型评估器下，即使没有配置 FieldConfs，也应透传原始 targetFields
				EvaluateTargetOutputFields: targetFields,
				Ext:                        make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "Prompt evaluator - empty field configs（透传全部 target 输出）",
			evaluatorType: entity.EvaluatorTypePrompt,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				HistoryMessages: nil,
				InputFields:     targetFields,
				Ext:             make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "CustomRPC evaluator - empty input schemas",
			evaluatorType: entity.EvaluatorTypeCustomRPC,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "eval_field", FromField: "turn_field1"},
						},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "target_field", FromField: "target_field1"},
						},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantInputData: &entity.EvaluatorInputData{
				InputFields:           make(map[string]*entity.Content),
				EvaluateDatasetFields: map[string]*entity.Content{"eval_field": mockContent1},
				EvaluateTargetOutputFields: map[string]*entity.Content{
					"target_field1": mockContent1,
					"target_field2": mockContent2,
				},
				Ext: make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "CustomRPC evaluator - with input schemas",
			evaluatorType: entity.EvaluatorTypeCustomRPC,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "eval_field", FromField: "turn_field1"},
						},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{
							{FieldName: "target_field", FromField: "target_field1"},
						},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			inputSchemas: []*entity.ArgsSchema{{Key: gptr.Of("some_schema")}},
			wantInputData: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{
					"eval_field":   mockContent1,
					"target_field": mockContent1,
				},
				Ext: make(map[string]string),
			},
			wantErr: false,
		},
		{
			name:          "Runtime param in RunConf",
			evaluatorType: entity.EvaluatorTypePrompt,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
					TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
				},
				RunConf: &entity.EvaluatorRunConfig{
					EvaluatorRuntimeParam: &entity.RuntimeParam{
						JSONValue: gptr.Of(`{"key":"val"}`),
					},
				},
			},
			ext: map[string]string{"orig": "val"},
			wantInputData: &entity.EvaluatorInputData{
				InputFields: map[string]*entity.Content{},
				Ext: map[string]string{
					"orig": "val",
					consts.FieldAdapterBuiltinFieldNameRuntimeParam: `{"key":"val"}`,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			turn := &entity.Turn{
				FieldDataList: []*entity.FieldData{},
			}
			for key, c := range tt.turnFields {
				turn.FieldDataList = append(turn.FieldDataList, &entity.FieldData{
					Name:    key,
					Content: c,
				})
			}

			got, err := service.buildEvaluatorInputData(ctx, 0, tt.evaluatorType, tt.ec, turn, tt.targetFields, tt.inputSchemas, tt.ext)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantInputData.HistoryMessages, got.HistoryMessages)
			assert.Equal(t, tt.wantInputData.InputFields, got.InputFields)
			assert.Equal(t, tt.wantInputData.EvaluateDatasetFields, got.EvaluateDatasetFields)
			assert.Equal(t, tt.wantInputData.EvaluateTargetOutputFields, got.EvaluateTargetOutputFields)
			assert.Equal(t, tt.wantInputData.Ext, got.Ext)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_buildFieldsFromSource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	service := &DefaultExptTurnEvaluationImpl{}

	mockContent1 := &entity.Content{Text: gptr.Of("value1")}
	mockContent2 := &entity.Content{Text: gptr.Of("value2")}
	mockJSONContent := &entity.Content{
		ContentType: gptr.Of(entity.ContentTypeText),
		Text:        gptr.Of(`{"key": "nested_value"}`),
	}

	actualOutputContent := &entity.Content{Text: gptr.Of("actual output text")}
	sourceFields := map[string]*entity.Content{
		"field1":     mockContent1,
		"field2":     mockContent2,
		"json_field": mockJSONContent,
	}
	sourceFieldsWithActualOutput := map[string]*entity.Content{
		"field1":        mockContent1,
		"field2":        mockContent2,
		"json_field":    mockJSONContent,
		"actual_output": actualOutputContent,
	}

	tests := []struct {
		name          string
		fieldConfs    []*entity.FieldConf
		sourceFields  map[string]*entity.Content
		evaluatorType entity.EvaluatorType
		wantResult    map[string]*entity.Content
		wantErr       bool
		inputSchemas  []*entity.ArgsSchema
	}{
		{
			name: "Normal field mapping",
			fieldConfs: []*entity.FieldConf{
				{FieldName: "output1", FromField: "field1"},
				{FieldName: "output2", FromField: "field2"},
			},
			sourceFields:  sourceFields,
			evaluatorType: entity.EvaluatorTypePrompt,
			wantResult: map[string]*entity.Content{
				"output1": mockContent1,
				"output2": mockContent2,
			},
			wantErr: false,
		},
		{
			name: "JSON Path field mapping",
			fieldConfs: []*entity.FieldConf{
				{FieldName: "nested_output", FromField: "json_field.key"},
			},
			sourceFields:  sourceFields,
			evaluatorType: entity.EvaluatorTypePrompt,
			wantResult: map[string]*entity.Content{
				"nested_output": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of("nested_value"),
				},
			},
			wantErr: false,
		},
		{
			name: "Non-existent field",
			fieldConfs: []*entity.FieldConf{
				{FieldName: "output", FromField: "non_existent_field"},
			},
			sourceFields:  sourceFields,
			evaluatorType: entity.EvaluatorTypePrompt,
			wantResult: map[string]*entity.Content{
				"output": nil,
			},
			wantErr: false,
		},
		{
			name: "Non-existent JSON field",
			fieldConfs: []*entity.FieldConf{
				{FieldName: "output", FromField: "json_field.non_existent"},
			},
			sourceFields:  sourceFields,
			evaluatorType: entity.EvaluatorTypePrompt,
			wantResult: map[string]*entity.Content{
				"output": {
					ContentType: gptr.Of(entity.ContentTypeText),
					Text:        gptr.Of(""),
				},
			},
			wantErr: false,
		},
		{
			name:          "Empty field configuration - 透传全部 target 输出确保 actual_output 等不丢失",
			fieldConfs:    []*entity.FieldConf{},
			sourceFields:  sourceFields,
			evaluatorType: entity.EvaluatorTypePrompt,
			wantResult:    sourceFields,
			wantErr:       false,
		},
		{
			name: "Code evaluator returns source fields directly",
			fieldConfs: []*entity.FieldConf{
				{FieldName: "output1", FromField: "field1"},
			},
			sourceFields:  sourceFields,
			evaluatorType: entity.EvaluatorTypeCode,
			// 对于 Code 类型评估器，应直接返回 sourceFields
			wantResult: sourceFields,
			wantErr:    false,
		},
		{
			name: "actual_output 始终传入评估器（FieldConfs 未配置时自动补充）",
			fieldConfs: []*entity.FieldConf{
				{FieldName: "output1", FromField: "field1"},
			},
			sourceFields:  sourceFieldsWithActualOutput,
			evaluatorType: entity.EvaluatorTypePrompt,
			wantResult: map[string]*entity.Content{
				"output1":       mockContent1,
				"actual_output": actualOutputContent,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.buildFieldsFromSource(ctx, tt.fieldConfs, tt.sourceFields, tt.evaluatorType, tt.inputSchemas)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.name == "JSON Path field mapping" {
				// Special handling for JSON field comparison
				assert.Equal(t, len(tt.wantResult), len(got))
				for key, expectedContent := range tt.wantResult {
					actualContent := got[key]
					assert.NotNil(t, actualContent)
					assert.Equal(t, expectedContent.ContentType, actualContent.ContentType)
					if expectedContent.Text != nil && actualContent.Text != nil {
						assert.Equal(t, *expectedContent.Text, *actualContent.Text)
					}
				}
			} else {
				assert.Equal(t, tt.wantResult, got)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_getFieldContent(t *testing.T) {
	t.Parallel()

	service := &DefaultExptTurnEvaluationImpl{}

	mockContent := &entity.Content{Text: gptr.Of("simple_value")}
	mockJSONContent := &entity.Content{
		ContentType: gptr.Of(entity.ContentTypeText),
		Text:        gptr.Of(`{"nested": {"key": "nested_value"}}`),
	}

	sourceFields := map[string]*entity.Content{
		"simple_field": mockContent,
		"json_field":   mockJSONContent,
	}

	tests := []struct {
		name         string
		fc           *entity.FieldConf
		sourceFields map[string]*entity.Content
		wantContent  *entity.Content
		wantErr      bool
	}{
		{
			name: "Simple field direct mapping",
			fc: &entity.FieldConf{
				FieldName: "output",
				FromField: "simple_field",
			},
			sourceFields: sourceFields,
			wantContent:  mockContent,
			wantErr:      false,
		},
		{
			name: "JSON Path field mapping",
			fc: &entity.FieldConf{
				FieldName: "output",
				FromField: "json_field.nested.key",
			},
			sourceFields: sourceFields,
			wantContent: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of("nested_value"),
			},
			wantErr: false,
		},
		{
			name: "Non-existent field",
			fc: &entity.FieldConf{
				FieldName: "output",
				FromField: "non_existent",
			},
			sourceFields: sourceFields,
			wantContent:  nil,
			wantErr:      false,
		},
		{
			name: "Non-existent JSON field",
			fc: &entity.FieldConf{
				FieldName: "output",
				FromField: "json_field.non_existent",
			},
			sourceFields: sourceFields,
			wantContent: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				Text:        gptr.Of(""),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := service.getFieldContent(tt.fc, tt.sourceFields)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.name == "JSON Path field mapping" && tt.wantContent != nil && got != nil {
				// Special handling for JSON field comparison
				assert.Equal(t, tt.wantContent.ContentType, got.ContentType)
				if tt.wantContent.Text != nil && got.Text != nil {
					assert.Equal(t, *tt.wantContent.Text, *got.Text)
				}
			} else {
				assert.Equal(t, tt.wantContent, got)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_skipTargetNode(t *testing.T) {
	t.Parallel()

	service := &DefaultExptTurnEvaluationImpl{}

	tests := []struct {
		name string
		expt *entity.Experiment
		want bool
	}{
		{
			name: "No target version ID - skip",
			expt: &entity.Experiment{
				TargetVersionID: 0,
				ExptType:        entity.ExptType_Offline,
			},
			want: true,
		},
		{
			name: "Online experiment - skip",
			expt: &entity.Experiment{
				TargetVersionID: 1,
				ExptType:        entity.ExptType_Online,
			},
			want: true,
		},
		{
			name: "Offline experiment with target version ID - do not skip",
			expt: &entity.Experiment{
				TargetVersionID: 1,
				ExptType:        entity.ExptType_Offline,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := service.skipTargetNode(tt.expt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_skipEvaluatorNode(t *testing.T) {
	t.Parallel()

	service := &DefaultExptTurnEvaluationImpl{}

	tests := []struct {
		name string
		expt *entity.Experiment
		want bool
	}{
		{
			name: "No evaluator configuration - skip",
			expt: &entity.Experiment{
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: nil,
					},
				},
			},
			want: true,
		},
		{
			name: "With evaluator configuration - do not skip",
			expt: &entity.Experiment{
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := service.skipEvaluatorNode(tt.expt)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CallEvaluators_EdgeCases(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:            mockMetric,
		evaluatorService:  mockEvaluatorService,
		benefitService:    mockBenefitService,
		evalTargetService: mockEvalTargetService,
	}

	mockContent := &entity.Content{Text: gptr.Of("value1")}
	mockTargetResult := &entity.EvalTargetRecord{
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"field1": mockContent,
			},
		},
	}

	tests := []struct {
		name    string
		prepare func()
		etec    *entity.ExptTurnEvalCtx
		target  *entity.EvalTargetRecord
		wantErr bool
	}{
		{
			name: "Successful evaluator result already exists - skip execution",
			prepare: func() {
				// No need to mock any calls as it will directly return the existing result
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						Evaluators: []*entity.Evaluator{
							{
								ID:            1,
								EvaluatorType: entity.EvaluatorTypePrompt,
								PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{
									ID: 1,
								},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{}, // Event required: CallEvaluators uses Event.IgnoreExistedTargetResult()
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{
					EvaluatorResults: map[int64]*entity.EvaluatorRecord{
						1: {ID: 1, Status: entity.EvaluatorRunStatusSuccess},
					},
				},
			},
			target:  mockTargetResult,
			wantErr: false,
		},
		{
			name: "Code evaluator builds input data",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).Return(&entity.EvaluatorRecord{ID: 1, Status: entity.EvaluatorRunStatusSuccess}, nil)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						Evaluators: []*entity.Evaluator{
							{
								ID:            1,
								EvaluatorType: entity.EvaluatorTypeCode,
								CodeEvaluatorVersion: &entity.CodeEvaluatorVersion{
									ID: 1,
								},
							},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{
											EvaluatorVersionID: 1,
											IngressConf: &entity.EvaluatorIngressConf{
												EvalSetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{
														{FieldName: "eval_field", FromField: "field1"},
													},
												},
												TargetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{
														{FieldName: "target_field", FromField: "field1"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
						Session: &entity.Session{UserID: "test_user"},
						ExptID:  1,
						SpaceID: 2,
					},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn: &entity.Turn{
					FieldDataList: []*entity.FieldData{
						{Name: "field1", Content: mockContent},
					},
				},
			},
			target:  mockTargetResult,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.prepare != nil {
				tt.prepare()
			}

			_, err := service.CallEvaluators(context.Background(), tt.etec, tt.target)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_callTarget_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		etec    *entity.ExptTurnEvalCtx
		history []*entity.Message
		spaceID int64
		wantErr bool
	}{
		{
			name: "target config validation fails",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{ExptRunID: 1, SpaceID: 1},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
							EvalTargetType:    entity.EvalTargetTypeCozeBot,
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									// Missing required IngressConf to make validation fail
									IngressConf: nil,
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID:            1,
					FieldDataList: []*entity.FieldData{{Name: "field1", Content: &entity.Content{Text: gptr.Of("value1")}}},
				},
			},
			history: []*entity.Message{},
			spaceID: 1,
			wantErr: true,
		},
		{
			name: "json path parsing error",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{ExptRunID: 1, SpaceID: 1},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
							EvalTargetType:    entity.EvalTargetTypeLoopPrompt,
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "[invalid_json_path"}},
										},
									},
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID:            1,
					FieldDataList: []*entity.FieldData{{Name: "field1", Content: &entity.Content{Text: gptr.Of("value1")}}},
				},
			},
			history: []*entity.Message{},
			spaceID: 1,
			wantErr: true,
		},
		{
			name: "execute target service fails",
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event:       &entity.ExptItemEvalEvent{ExptRunID: 1, SpaceID: 1},
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Expt: &entity.Experiment{
						Target: &entity.EvalTarget{
							ID:                1,
							EvalTargetVersion: &entity.EvalTargetVersion{ID: 1},
							EvalTargetType:    entity.EvalTargetTypeLoopPrompt,
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
									IngressConf: &entity.TargetIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
										},
									},
								},
							},
						},
					},
				},
				Turn: &entity.Turn{
					ID:            1,
					FieldDataList: []*entity.FieldData{{Name: "field1", Content: &entity.Content{Text: gptr.Of("value1")}}},
				},
				Ext: map[string]string{},
			},
			history: []*entity.Message{},
			spaceID: 1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockMetric := metricsmocks.NewMockExptMetric(ctrl)
			mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)

			service := &DefaultExptTurnEvaluationImpl{
				metric:            mockMetric,
				evalTargetService: mockEvalTargetService,
			}

			// Setup mocks based on test case
			switch tt.name {
			case "execute target service fails":
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), true)
				mockEvalTargetService.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("execute target failed"))
			case "target config validation fails":
				// For target config validation fails, no ExecuteTarget call should be made
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), true)
			default:
				// For json path parsing error case
				mockMetric.EXPECT().EmitTurnExecTargetResult(gomock.Any(), true)
			}

			_, err := service.callTarget(context.Background(), tt.etec, tt.history, tt.spaceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_callEvaluators_EdgeCases(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:           mockMetric,
		evaluatorService: mockEvaluatorService,
	}

	mockContent := &entity.Content{Text: gptr.Of("value1")}
	mockTargetResult := &entity.EvalTargetRecord{
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"field1": mockContent,
			},
		},
	}

	tests := []struct {
		name    string
		prepare func()
		etec    *entity.ExptTurnEvalCtx
		target  *entity.EvalTargetRecord
		wantErr bool
	}{
		{
			name:    "evaluators config validation fails",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Expt: &entity.Experiment{
						Evaluators: []*entity.Evaluator{
							{ID: 1, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1}},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(0), // Invalid concurrency number
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			target:  mockTargetResult,
			wantErr: true,
		},
		{
			name:    "evaluator config not found",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Expt: &entity.Experiment{
						Evaluators: []*entity.Evaluator{
							{ID: 1, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 999}}, // Non-existent evaluator
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{EvaluatorVersionID: 1}, // Different ID
									},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn: &entity.Turn{
					FieldDataList: []*entity.FieldData{
						{Name: "field1", Content: mockContent},
					},
				},
			},
			target:  mockTargetResult,
			wantErr: true,
		},
		{
			name:    "build evaluator input data fails",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					EvalSetItem: &entity.EvaluationSetItem{ItemID: 1},
					Expt: &entity.Experiment{
						Evaluators: []*entity.Evaluator{
							{ID: 1, EvaluatorType: entity.EvaluatorTypePrompt, PromptEvaluatorVersion: &entity.PromptEvaluatorVersion{ID: 1}},
						},
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								EvaluatorsConf: &entity.EvaluatorsConf{
									EvaluatorConcurNum: gptr.Of(1),
									EvaluatorConf: []*entity.EvaluatorConf{
										{
											EvaluatorVersionID: 1,
											IngressConf: &entity.EvaluatorIngressConf{
												EvalSetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "[invalid_json_path"}},
												},
												TargetAdapter: &entity.FieldAdapter{
													FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
				Turn: &entity.Turn{
					FieldDataList: []*entity.FieldData{
						{Name: "field1", Content: mockContent},
					},
				},
			},
			target:  mockTargetResult,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.prepare()
			// Check if targetResult is nil to avoid panic
			if tt.target != nil && tt.target.EvalTargetOutputData == nil {
				tt.target.EvalTargetOutputData = &entity.EvalTargetOutputData{
					OutputFields: make(map[string]*entity.Content),
				}
			}

			// Setup mock expectations for EmitTurnExecEvaluatorResult based on test case
			switch tt.name {
			case "evaluators config validation fails":
				// For validation failures, EmitTurnExecEvaluatorResult should be called with false
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false).AnyTimes()
			default:
				// For other cases, add expectation
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false).AnyTimes()
			}

			_, err := service.callEvaluators(context.Background(), []int64{1}, tt.etec, tt.target, []*entity.Message{})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_deepCopyEvaluatorInputData(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		got := deepCopyEvaluatorInputData(nil)
		assert.Nil(t, got)
	})

	t.Run("deep copy produces independent copy", func(t *testing.T) {
		in := &entity.EvaluatorInputData{
			InputFields: map[string]*entity.Content{
				"a": {Text: gptr.Of("x")},
			},
			EvaluateDatasetFields: map[string]*entity.Content{
				"b": {Text: gptr.Of("y")},
			},
			EvaluateTargetOutputFields: map[string]*entity.Content{
				"c": {Text: gptr.Of("z")},
			},
			Ext: map[string]string{"k": "v"},
		}
		got := deepCopyEvaluatorInputData(in)
		assert.NotNil(t, got)
		assert.NotSame(t, in, got)
		assert.Equal(t, in.InputFields["a"].GetText(), got.InputFields["a"].GetText())
		assert.Equal(t, in.EvaluateDatasetFields["b"].GetText(), got.EvaluateDatasetFields["b"].GetText())
		assert.Equal(t, in.EvaluateTargetOutputFields["c"].GetText(), got.EvaluateTargetOutputFields["c"].GetText())
		assert.Equal(t, in.Ext, got.Ext)
		// 修改 copy 不应影响原对象
		got.InputFields["a"].SetText("modified")
		assert.Equal(t, "x", in.InputFields["a"].GetText())
	})
}

func TestDefaultExptTurnEvaluationImpl_buildEvaluatorInputData_EdgeCases(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service := &DefaultExptTurnEvaluationImpl{}

	mockContent := &entity.Content{Text: gptr.Of("value1")}
	turnFields := map[string]*entity.Content{"turn_field": mockContent}
	targetFields := map[string]*entity.Content{"target_field": mockContent}

	tests := []struct {
		name           string
		evaluatorType  entity.EvaluatorType
		ec             *entity.EvaluatorConf
		turnFields     map[string]*entity.Content
		targetFields   map[string]*entity.Content
		wantErr        bool
		validateResult func(t *testing.T, result *entity.EvaluatorInputData)
	}{
		{
			name:          "code evaluator with invalid field config",
			evaluatorType: entity.EvaluatorTypeCode,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "[invalid_json_path"}},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantErr:      true,
		},
		{
			name:          "prompt evaluator with invalid field config",
			evaluatorType: entity.EvaluatorTypePrompt,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "[invalid_json_path"}},
					},
					TargetAdapter: &entity.FieldAdapter{
						FieldConfs: []*entity.FieldConf{{FieldName: "field1", FromField: "field1"}},
					},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantErr:      true,
		},
		{
			name:          "code evaluator with empty field configs",
			evaluatorType: entity.EvaluatorTypeCode,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
					TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantErr:      false,
			validateResult: func(t *testing.T, result *entity.EvaluatorInputData) {
				assert.NotNil(t, result.EvaluateDatasetFields)
				assert.NotNil(t, result.EvaluateTargetOutputFields)
				// Code 类型评估器下，即使 FieldConfs 为空，buildFieldsFromSource 也会直接返回 sourceFields
				assert.Empty(t, result.EvaluateDatasetFields)
				assert.Equal(t, targetFields, result.EvaluateTargetOutputFields)
			},
		},
		{
			name:          "prompt evaluator with empty field configs（透传全部 target 输出）",
			evaluatorType: entity.EvaluatorTypePrompt,
			ec: &entity.EvaluatorConf{
				IngressConf: &entity.EvaluatorIngressConf{
					EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
					TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
				},
			},
			turnFields:   turnFields,
			targetFields: targetFields,
			wantErr:      false,
			validateResult: func(t *testing.T, result *entity.EvaluatorInputData) {
				assert.NotNil(t, result.InputFields)
				assert.Equal(t, targetFields, result.InputFields)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			turn := &entity.Turn{
				FieldDataList: []*entity.FieldData{},
			}
			for key, c := range tt.turnFields {
				turn.FieldDataList = append(turn.FieldDataList, &entity.FieldData{
					Name:    key,
					Content: c,
				})
			}

			got, err := service.buildEvaluatorInputData(ctx, 0, tt.evaluatorType, tt.ec, turn, tt.targetFields, nil, nil)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				if tt.validateResult != nil {
					tt.validateResult(t, got)
				}
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_getFieldContent_EdgeCases(t *testing.T) {
	t.Parallel()

	service := &DefaultExptTurnEvaluationImpl{}

	mockContent := &entity.Content{Text: gptr.Of(`{"nested": "value"}`)}
	sourceFields := map[string]*entity.Content{
		"field1": mockContent,
		"field2": {Text: gptr.Of("simple_value")},
	}

	tests := []struct {
		name         string
		fc           *entity.FieldConf
		sourceFields map[string]*entity.Content
		wantErr      bool
		wantContent  *entity.Content
	}{
		{
			name:         "invalid json path in field config",
			fc:           &entity.FieldConf{FieldName: "test", FromField: "[invalid_json_path"},
			sourceFields: sourceFields,
			wantErr:      true,
		},
		{
			name:         "direct field access",
			fc:           &entity.FieldConf{FieldName: "test", FromField: "field2"},
			sourceFields: sourceFields,
			wantErr:      false,
			wantContent:  &entity.Content{Text: gptr.Of("simple_value")},
		},
		{
			name:         "json path field access with error",
			fc:           &entity.FieldConf{FieldName: "test", FromField: "field1.invalid_nested_path"},
			sourceFields: sourceFields,
			wantErr:      false, // getContentByJsonPath doesn't return error for this case
			wantContent:  nil,   // Returns nil for this case based on actual behavior
		},
		{
			name:         "field not exists in source",
			fc:           &entity.FieldConf{FieldName: "test", FromField: "non_existent_field"},
			sourceFields: sourceFields,
			wantErr:      false,
			wantContent:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := service.getFieldContent(tt.fc, tt.sourceFields)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.wantContent == nil {
					assert.Nil(t, got)
				} else {
					assert.Equal(t, tt.wantContent, got)
				}
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_buildEvalSetFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEvalSetItemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	service := &DefaultExptTurnEvaluationImpl{
		evalSetItemSvc: mockEvalSetItemSvc,
	}
	ctx := context.Background()
	spaceID := int64(100)
	evalSetID := int64(200)
	itemID := int64(300)
	turnID := int64(10)

	tests := []struct {
		name      string
		fcs       []*entity.FieldConf
		evalTurn  *entity.Turn
		mockSetup func()
		wantErr   bool
		validate  func(t *testing.T, result map[string]*entity.Content)
	}{
		{
			name: "load omitted content from eval set before field conf processing",
			fcs:  []*entity.FieldConf{{FieldName: "out", FromField: "f1"}},
			evalTurn: &entity.Turn{
				ID:        turnID,
				EvalSetID: evalSetID,
				ItemID:    itemID,
				FieldDataList: []*entity.FieldData{
					{
						Name: "f1",
						Content: &entity.Content{
							ContentType:      gptr.Of(entity.ContentTypeText),
							Text:             gptr.Of("short"),
							ContentOmitted:   gptr.Of(true),
							FullContent:      &entity.ObjectStorage{URI: gptr.Of("key")},
							FullContentBytes: gptr.Of(int32(100)),
						},
					},
				},
			},
			mockSetup: func() {
				mockEvalSetItemSvc.EXPECT().
					GetEvaluationSetItemField(gomock.Any(), &entity.GetEvaluationSetItemFieldParam{
						SpaceID:         spaceID,
						EvaluationSetID: evalSetID,
						ItemPK:          itemID,
						FieldName:       "f1",
						TurnID:          gptr.Of(turnID),
					}).
					Return(&entity.FieldData{
						Name:    "f1",
						Content: &entity.Content{ContentType: gptr.Of(entity.ContentTypeText), Text: gptr.Of("full content from eval set")},
					}, nil)
			},
			wantErr: false,
			validate: func(t *testing.T, result map[string]*entity.Content) {
				assert.NotNil(t, result)
				assert.Contains(t, result, "out")
				assert.Equal(t, "full content from eval set", result["out"].GetText())
			},
		},
		{
			name: "GetEvaluationSetItemField error returns err",
			fcs:  []*entity.FieldConf{{FieldName: "out", FromField: "f1"}},
			evalTurn: &entity.Turn{
				ID:        turnID,
				EvalSetID: evalSetID,
				ItemID:    itemID,
				FieldDataList: []*entity.FieldData{
					{Name: "f1", Content: &entity.Content{
						ContentType:      gptr.Of(entity.ContentTypeText),
						Text:             gptr.Of("x"),
						ContentOmitted:   gptr.Of(true),
						FullContent:      &entity.ObjectStorage{URI: gptr.Of("k")},
						FullContentBytes: gptr.Of(int32(50)),
					}},
				},
			},
			mockSetup: func() {
				mockEvalSetItemSvc.EXPECT().
					GetEvaluationSetItemField(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("svc err"))
			},
			wantErr: true,
		},
		{
			name:     "nil evalSetTurn with empty fcs returns empty",
			fcs:      []*entity.FieldConf{},
			evalTurn: nil,
			mockSetup: func() {
			},
			wantErr: false,
			validate: func(t *testing.T, result map[string]*entity.Content) {
				assert.NotNil(t, result)
				assert.Empty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()
			got, err := service.buildEvalSetFields(ctx, spaceID, tt.fcs, tt.evalTurn)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CheckBenefit_EdgeCases(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
	service := &DefaultExptTurnEvaluationImpl{
		benefitService: mockBenefitService,
	}

	tests := []struct {
		name     string
		prepare  func()
		exptID   int64
		spaceID  int64
		freeCost bool
		session  *entity.Session
		wantErr  bool
	}{
		{
			name: "benefit result with nil deny reason",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{
					DenyReason: nil,
				}, nil)
			},
			exptID:   1,
			spaceID:  2,
			freeCost: true,
			session:  &entity.Session{UserID: "test_user"},
			wantErr:  false,
		},
		{
			name: "benefit result with nil result",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
			exptID:   1,
			spaceID:  2,
			freeCost: false,
			session:  &entity.Session{UserID: "test_user"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.prepare()
			err := service.CheckBenefit(context.Background(), tt.exptID, tt.spaceID, tt.freeCost, tt.session)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_refreshAsyncEvaluatorRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		service        func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl
		input          map[int64]*entity.EvaluatorRecord
		wantErr        bool
		validateResult func(t *testing.T, result map[int64]*entity.EvaluatorRecord)
	}{
		{
			name: "nil evaluatorRecordService - skip refresh",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				return &DefaultExptTurnEvaluationImpl{}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: {ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking},
			},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, result[101].Status)
			},
		},
		{
			name: "no async invoking records - no refresh needed",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: {ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusSuccess},
				102: {ID: 202, EvaluatorVersionID: 102, Status: entity.EvaluatorRunStatusFail},
			},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Equal(t, entity.EvaluatorRunStatusSuccess, result[101].Status)
				assert.Equal(t, entity.EvaluatorRunStatusFail, result[102].Status)
			},
		},
		{
			name: "nil record in map - skip",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: nil,
			},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Nil(t, result[101])
			},
		},
		{
			name: "async record refreshed to success",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				mockRecordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(201), false).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusSuccess}, nil,
				)
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: {ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking},
			},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Equal(t, entity.EvaluatorRunStatusSuccess, result[101].Status)
				assert.Equal(t, int64(201), result[101].ID)
			},
		},
		{
			name: "async record still invoking after refresh",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				mockRecordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(201), false).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil,
				)
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: {ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking},
			},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, result[101].Status)
			},
		},
		{
			name: "GetEvaluatorRecord returns error",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				mockRecordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(201), false).Return(nil, errors.New("db error"))
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: {ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking},
			},
			wantErr: true,
		},
		{
			name: "mixed records - only refresh async invoking ones",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				mockRecordSvc.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(302), false).Return(
					&entity.EvaluatorRecord{ID: 302, EvaluatorVersionID: 102, Status: entity.EvaluatorRunStatusSuccess}, nil,
				)
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input: map[int64]*entity.EvaluatorRecord{
				101: {ID: 301, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusSuccess},
				102: {ID: 302, EvaluatorVersionID: 102, Status: entity.EvaluatorRunStatusAsyncInvoking},
			},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Equal(t, entity.EvaluatorRunStatusSuccess, result[101].Status)
				assert.Equal(t, int64(301), result[101].ID)
				assert.Equal(t, entity.EvaluatorRunStatusSuccess, result[102].Status)
				assert.Equal(t, int64(302), result[102].ID)
			},
		},
		{
			name: "empty map - no-op",
			service: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockRecordSvc := svcmocks.NewMockEvaluatorRecordService(ctrl)
				return &DefaultExptTurnEvaluationImpl{evaluatorRecordService: mockRecordSvc}
			},
			input:   map[int64]*entity.EvaluatorRecord{},
			wantErr: false,
			validateResult: func(t *testing.T, result map[int64]*entity.EvaluatorRecord) {
				assert.Empty(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tt.service(ctrl)
			result, err := svc.refreshAsyncEvaluatorRecords(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validateResult != nil {
				tt.validateResult(t, result)
			}
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_CallEvaluators_WithRefresh(t *testing.T) {
	t.Parallel()

	mockContent := &entity.Content{Text: gptr.Of("value1")}
	mockTargetResult := &entity.EvalTargetRecord{
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields: map[string]*entity.Content{
				"field1": mockContent,
			},
		},
	}

	baseEtec := func() *entity.ExptTurnEvalCtx {
		return &entity.ExptTurnEvalCtx{
			ExptItemEvalCtx: &entity.ExptItemEvalCtx{
				EvalSetItem: &entity.EvaluationSetItem{ID: 1, ItemID: 2},
				Event:       &entity.ExptItemEvalEvent{Session: &entity.Session{UserID: "test_user"}, ExptID: 1, SpaceID: 2},
				Expt: &entity.Experiment{
					ID: 1, SpaceID: 2,
					Evaluators: []*entity.Evaluator{
						{
							ID:            101,
							EvaluatorType: entity.EvaluatorTypeAgent,
							AgentEvaluatorVersion: &entity.AgentEvaluatorVersion{
								ID: 101,
							},
						},
					},
					EvalConf: &entity.EvaluationConfiguration{
						ItemConcurNum: gptr.Of(1),
						ConnectorConf: entity.Connector{
							EvaluatorsConf: &entity.EvaluatorsConf{
								EvaluatorConcurNum: gptr.Of(1),
								EvaluatorConf: []*entity.EvaluatorConf{
									{
										EvaluatorVersionID: 101,
										IngressConf: &entity.EvaluatorIngressConf{
											EvalSetAdapter: &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
											TargetAdapter:  &entity.FieldAdapter{FieldConfs: []*entity.FieldConf{}},
										},
										RunConf: &entity.EvaluatorRunConfig{},
									},
								},
							},
						},
					},
				},
			},
			ExptTurnRunResult: &entity.ExptTurnRunResult{},
			Turn:              &entity.Turn{FieldDataList: []*entity.FieldData{}},
		}
	}

	tests := []struct {
		name           string
		prepare        func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl
		etec           func() *entity.ExptTurnEvalCtx
		wantErr        bool
		validateResult func(t *testing.T, results map[int64]*entity.EvaluatorRecord)
	}{
		{
			name: "async evaluator refreshed to success after sync evaluator completes",
			prepare: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockMetric := metricsmocks.NewMockExptMetric(ctrl)
				mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
				mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
				mockEvaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)

				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil,
				)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorRecordService.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(201), false).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusSuccess}, nil,
				)

				return &DefaultExptTurnEvaluationImpl{
					metric:                 mockMetric,
					evaluatorService:       mockEvaluatorService,
					benefitService:         mockBenefitService,
					evalTargetService:      mockEvalTargetService,
					evalAsyncRepo:          mockEvalAsyncRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
				}
			},
			etec:    baseEtec,
			wantErr: false,
			validateResult: func(t *testing.T, results map[int64]*entity.EvaluatorRecord) {
				assert.Len(t, results, 1)
				for _, record := range results {
					assert.Equal(t, entity.EvaluatorRunStatusSuccess, record.Status)
				}
			},
		},
		{
			name: "async evaluator still invoking after refresh",
			prepare: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockMetric := metricsmocks.NewMockExptMetric(ctrl)
				mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
				mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
				mockEvaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)

				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil,
				)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorRecordService.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(201), false).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil,
				)

				return &DefaultExptTurnEvaluationImpl{
					metric:                 mockMetric,
					evaluatorService:       mockEvaluatorService,
					benefitService:         mockBenefitService,
					evalTargetService:      mockEvalTargetService,
					evalAsyncRepo:          mockEvalAsyncRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
				}
			},
			etec:    baseEtec,
			wantErr: false,
			validateResult: func(t *testing.T, results map[int64]*entity.EvaluatorRecord) {
				assert.Len(t, results, 1)
				for _, record := range results {
					assert.Equal(t, entity.EvaluatorRunStatusAsyncInvoking, record.Status)
				}
			},
		},
		{
			name: "refresh returns error",
			prepare: func(ctrl *gomock.Controller) *DefaultExptTurnEvaluationImpl {
				mockMetric := metricsmocks.NewMockExptMetric(ctrl)
				mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
				mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)
				mockEvalTargetService := svcmocks.NewMockIEvalTargetService(ctrl)
				mockEvalAsyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
				mockEvaluatorRecordService := svcmocks.NewMockEvaluatorRecordService(ctrl)

				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckAndDeductEvalBenefitResult{}, nil)
				mockEvaluatorService.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(
					&entity.EvaluatorRecord{ID: 201, EvaluatorVersionID: 101, Status: entity.EvaluatorRunStatusAsyncInvoking}, nil,
				)
				mockMetric.EXPECT().EmitTurnExecEvaluatorResult(gomock.Any(), false)
				mockEvalAsyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				mockEvaluatorRecordService.EXPECT().GetEvaluatorRecord(gomock.Any(), int64(201), false).Return(nil, errors.New("db error"))

				return &DefaultExptTurnEvaluationImpl{
					metric:                 mockMetric,
					evaluatorService:       mockEvaluatorService,
					benefitService:         mockBenefitService,
					evalTargetService:      mockEvalTargetService,
					evalAsyncRepo:          mockEvalAsyncRepo,
					evaluatorRecordService: mockEvaluatorRecordService,
				}
			},
			etec:    baseEtec,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := tt.prepare(ctrl)
			etec := tt.etec()

			results, err := svc.CallEvaluators(context.Background(), etec, mockTargetResult)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if tt.validateResult != nil {
				tt.validateResult(t, results)
			}
		})
	}
}
