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
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// mock DenyReason 实现

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
			name: "正常流程",
			prepare: func() {
				mockMetric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any())
				mockMetric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{SpaceID: 1},
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Online,
					},
				},
				ExptTurnRunResult: &entity.ExptTurnRunResult{},
			},
			wantErr: false,
		},
		{
			name: "调用目标失败",
			prepare: func() {
				mockMetric.EXPECT().EmitTurnExecEval(gomock.Any(), gomock.Any())
				mockMetric.EXPECT().EmitTurnExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Event: &entity.ExptItemEvalEvent{},
					Expt:  &entity.Experiment{},
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
			name:    "在线实验-跳过调用",
			prepare: func() {},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{
						ExptType: entity.ExptType_Online,
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
			name:    "已有成功结果",
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
					Expt: &entity.Experiment{},
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
			name: "权益检查失败",
			prepare: func() {
				mockBenefitService.EXPECT().CheckAndDeductEvalBenefit(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			etec: &entity.ExptTurnEvalCtx{
				ExptItemEvalCtx: &entity.ExptItemEvalCtx{
					Expt: &entity.Experiment{},
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
			name: "正常流程-真正调用callTarget",
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
			name: "正常流程",
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
			name: "检查失败",
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
			name: "拒绝原因存在",
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
			name: "正常流程",
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
			name: "权益检查失败",
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
			name: "正常-json",
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
			name: "正常-嵌套json",
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
			name: "正常-返回整个json",
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
			name:    "异常-content为nil",
			args:    args{content: nil, jsonPath: "$.key"},
			want:    nil,
			wantErr: false,
		},
		{
			name: "异常-contentType为nil",
			args: args{
				content:  &entity.Content{ContentType: nil, Text: gptr.Of(`{"key": "value"}`)},
				jsonPath: "$.key",
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "异常-contentType非text",
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
			name: "正常-json字符串",
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
			} else if tt.name == "正常-返回整个json" && tt.want.Text != nil && got != nil && got.Text != nil {
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
