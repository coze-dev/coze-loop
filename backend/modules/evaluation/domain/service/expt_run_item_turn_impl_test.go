// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefitmocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
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

	eval := NewExptTurnEvaluation(mockMetric, mockEvalTargetService, mockEvaluatorService, mockBenefitService)
	assert.NotNil(t, eval)
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
					Event: &entity.ExptItemEvalEvent{SpaceID: 1},
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
					Event: &entity.ExptItemEvalEvent{SpaceID: 1},
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
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{SpaceID: 1},
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
						EvalConf: &entity.EvaluationConfiguration{
							ConnectorConf: entity.Connector{
								TargetConf: &entity.TargetConf{
									TargetVersionID: 1,
								},
							},
						},
					},
					Event: &entity.ExptItemEvalEvent{
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
						ExptType: entity.ExptType_Offline,
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

func TestDefaultExptTurnEvaluationImpl_CallEvaluators(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockEvaluatorService := svcmocks.NewMockEvaluatorService(ctrl)
	mockBenefitService := benefitmocks.NewMockIBenefitService(ctrl)

	service := &DefaultExptTurnEvaluationImpl{
		metric:           mockMetric,
		evaluatorService: mockEvaluatorService,
		benefitService:   mockBenefitService,
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
