// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestEvalConfConvert_ConvertEntityToDTO(t *testing.T) {
	raw := `{
    "ConnectorConf":
    {
        "TargetConf":
        {
            "TargetVersionID": 7486074365205872641,
            "IngressConf":
            {
                "EvalSetAdapter":
                {
                    "FieldConfs":
                    [
                        {
                            "FieldName": "role",
                            "FromField": "role",
                            "Value": ""
                        },
                        {
                            "FieldName": "question",
                            "FromField": "input",
                            "Value": ""
                        }
                    ]
                },
                "CustomConf": null
            }
        },
        "EvaluatorsConf":
        {
            "EvaluatorConcurNum": null,
            "EvaluatorConf":
            [
                {
                    "EvaluatorVersionID": 7486074365205823489,
                    "IngressConf":
                    {
                        "EvalSetAdapter":
                        {
                            "FieldConfs":
                            [
                                {
                                    "FieldName": "input",
                                    "FromField": "input",
                                    "Value": ""
                                },
                                {
                                    "FieldName": "reference_output",
                                    "FromField": "reference_output",
                                    "Value": ""
                                }
                            ]
                        },
                        "TargetAdapter":
                        {
                            "FieldConfs":
                            [
                                {
                                    "FieldName": "output",
                                    "FromField": "actual_output",
                                    "Value": ""
                                }
                            ]
                        },
                        "CustomConf": null
                    }
                }
            ]
        }
    },
    "ItemConcurNum": null
}`
	conf := &entity.EvaluationConfiguration{}
	err := json.Unmarshal([]byte(raw), &conf)
	assert.Nil(t, err)

	target, evaluators, _ := NewEvalConfConvert().ConvertEntityToDTO(conf)
	t.Logf("target: %v", json.Jsonify(target))
	t.Logf("evaluators: %v", json.Jsonify(evaluators))
}

func TestConvertExptTurnResultFilterAccelerator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name    string
		input   *domain_expt.ExperimentFilter
		want    *entity.ExptTurnResultFilterAccelerator
		wantErr bool
	}{
		{
			name: "有效输入",
			input: &domain_expt.ExperimentFilter{
				Filters: &domain_expt.Filters{
					FilterConditions: []*domain_expt.FilterCondition{
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_ItemID,
							},
							Operator:     domain_expt.FilterOperatorType_Equal,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_ItemRunState,
							},
							Operator:     domain_expt.FilterOperatorType_Greater,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_TurnRunState,
							},
							Operator:     domain_expt.FilterOperatorType_GreaterOrEqual,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_EvaluatorScore,
							},
							Operator:     domain_expt.FilterOperatorType_Less,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_ActualOutput,
							},
							Operator:     domain_expt.FilterOperatorType_LessOrEqual,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_Annotation,
							},
							Operator:     domain_expt.FilterOperatorType_Like,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_EvaluatorScoreCorrected,
							},
							Operator:     domain_expt.FilterOperatorType_NotIn,
							Value:        "1",
							SourceTarget: nil,
						},
						{
							Field: &domain_expt.FilterField{
								FieldType: domain_expt.FieldType_EvalSetColumn,
							},
							Operator:     domain_expt.FilterOperatorType_NotLike,
							Value:        "1",
							SourceTarget: nil,
						},
					},
					LogicOp: ptr.Of(domain_expt.FilterLogicOp_And),
				},
				KeywordSearch: &domain_expt.KeywordSearch{
					Keyword: ptr.Of("1"),
					FilterFields: []*domain_expt.FilterField{
						{
							FieldType: domain_expt.FieldType_ActualOutput,
						},
					},
				},
			},
			want: &entity.ExptTurnResultFilterAccelerator{
				ItemIDs: []*entity.FieldFilter{
					{
						Key:    "item_id",
						Op:     "=",
						Values: []any{"1"},
					},
				},
				ItemRunStatus: []*entity.FieldFilter{},
				TurnRunStatus: []*entity.FieldFilter{},
				MapCond: &entity.ExptTurnResultFilterMapCond{
					EvalTargetDataFilters:   []*entity.FieldFilter{},
					EvaluatorScoreFilters:   []*entity.FieldFilter{},
					AnnotationFloatFilters:  []*entity.FieldFilter{},
					AnnotationBoolFilters:   []*entity.FieldFilter{},
					AnnotationStringFilters: []*entity.FieldFilter{},
				},
				ItemSnapshotCond: &entity.ItemSnapshotFilter{
					BoolMapFilters:   []*entity.FieldFilter{},
					StringMapFilters: []*entity.FieldFilter{},
					IntMapFilters:    []*entity.FieldFilter{},
					FloatMapFilters:  []*entity.FieldFilter{},
				},
				KeywordSearch: &entity.KeywordFilter{
					EvalTargetDataFilters: []*entity.FieldFilter{
						{
							Key:    "actual_output",
							Op:     "LIKE",
							Values: []any{"%1%"},
						},
					},
					ItemSnapshotFilter: &entity.ItemSnapshotFilter{
						BoolMapFilters:   []*entity.FieldFilter{},
						StringMapFilters: []*entity.FieldFilter{},
						IntMapFilters:    []*entity.FieldFilter{},
						FloatMapFilters:  []*entity.FieldFilter{},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertExptTurnResultFilterAccelerator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertExptTurnResultFilterAccelerator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got.ItemIDs) != len(tt.want.ItemIDs) {
					t.Errorf("ConvertExptTurnResultFilterAccelerator() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestToTargetFieldMappingDO_RuntimeParam(t *testing.T) {
	tests := []struct {
		name           string
		request        *expt.CreateExperimentRequest
		wantCustomConf *entity.FieldAdapter
	}{
		{
			name: "正常运行时参数转换",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
							ConstValue:    gptr.Of(""),
						},
					},
				},
				TargetRuntimeParam: &common.RuntimeParam{
					JSONValue: gptr.Of(`{"model_config":{"model_id":"test_model","temperature":0.7}}`),
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{
					{
						FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
						Value:     `{"model_config":{"model_id":"test_model","temperature":0.7}}`,
					},
				},
			},
		},
		{
			name: "运行时参数为nil",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: nil,
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
		},
		{
			name: "运行时参数JSONValue为空",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: &common.RuntimeParam{
					JSONValue: nil,
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
		},
		{
			name: "mapping为nil",
			request: &expt.CreateExperimentRequest{
				TargetFieldMapping: nil,
				TargetRuntimeParam: &common.RuntimeParam{JSONValue: gptr.Of(`{"test":"value"}`)},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
		},
	}

	converter := NewEvalConfConvert()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToEntity(tt.request)
			assert.NoError(t, err)

			if tt.request.TargetFieldMapping == nil {
				if result.ConnectorConf.TargetConf != nil {
					assert.Nil(t, result.ConnectorConf.TargetConf.IngressConf)
				}
				return
			}

			assert.NotNil(t, result)
			assert.NotNil(t, result.ConnectorConf.TargetConf)
			assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf)
			assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf.EvalSetAdapter)

			if tt.wantCustomConf == nil {
				assert.Nil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
			} else {
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
				assert.Equal(t, len(tt.wantCustomConf.FieldConfs), len(result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs))
				if len(tt.wantCustomConf.FieldConfs) > 0 {
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].FieldName, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].FieldName)
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].Value, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].Value)
				}
			}
		})
	}
}

func TestEvalConfConvert_ConvertEntityToDTO_RuntimeParam(t *testing.T) {
	tests := []struct {
		name             string
		ec               *entity.EvaluationConfiguration
		wantRuntimeParam *common.RuntimeParam
	}{
		{
			name: "包含运行时参数的配置",
			ec: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					TargetConf: &entity.TargetConf{
						TargetVersionID: 123,
						IngressConf: &entity.TargetIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "input",
										FromField: "question",
									},
								},
							},
							CustomConf: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
										Value:     `{"model_config":{"model_id":"converted_model","temperature":0.5}}`,
									},
								},
							},
						},
					},
				},
			},
			wantRuntimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(`{"model_config":{"model_id":"converted_model","temperature":0.5}}`),
			},
		},
		{
			name: "无运行时参数的配置",
			ec: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					TargetConf: &entity.TargetConf{
						TargetVersionID: 123,
						IngressConf: &entity.TargetIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "input",
										FromField: "question",
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
			wantRuntimeParam: &common.RuntimeParam{},
		},
		{
			name: "CustomConf为nil",
			ec: &entity.EvaluationConfiguration{
				ConnectorConf: entity.Connector{
					TargetConf: &entity.TargetConf{
						TargetVersionID: 123,
						IngressConf: &entity.TargetIngressConf{
							EvalSetAdapter: &entity.FieldAdapter{
								FieldConfs: []*entity.FieldConf{
									{
										FieldName: "input",
										FromField: "question",
									},
								},
							},
							CustomConf: nil,
						},
					},
				},
			},
			wantRuntimeParam: &common.RuntimeParam{},
		},
		{
			name:             "配置为nil",
			ec:               nil,
			wantRuntimeParam: nil,
		},
	}

	converter := NewEvalConfConvert()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, runtimeParam := converter.ConvertEntityToDTO(tt.ec)

			if tt.wantRuntimeParam == nil {
				assert.Nil(t, runtimeParam)
			} else {
				assert.NotNil(t, runtimeParam)
				if tt.wantRuntimeParam.JSONValue == nil {
					assert.Nil(t, runtimeParam.JSONValue)
				} else {
					assert.NotNil(t, runtimeParam.JSONValue)
					assert.Equal(t, gptr.Indirect(tt.wantRuntimeParam.JSONValue), gptr.Indirect(runtimeParam.JSONValue))
				}
			}
		})
	}
}

func TestEvalConfConvert_ConvertToEntity_RuntimeParam(t *testing.T) {
	tests := []struct {
		name           string
		request        *expt.CreateExperimentRequest
		wantCustomConf *entity.FieldAdapter
		wantErr        bool
	}{
		{
			name: "包含运行时参数的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(123)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: &common.RuntimeParam{
					JSONValue: gptr.Of(`{"model_config":{"model_id":"request_model","max_tokens":200}}`),
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
						FromEvalSet: []*domain_expt.FieldMapping{
							{
								FieldName:     gptr.Of("input"),
								FromFieldName: gptr.Of("question"),
							},
						},
					},
				},
			},
			wantCustomConf: &entity.FieldAdapter{
				FieldConfs: []*entity.FieldConf{
					{
						FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
						Value:     `{"model_config":{"model_id":"request_model","max_tokens":200}}`,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "无运行时参数的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(123)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				TargetRuntimeParam: nil,
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 456,
					},
				},
			},
			wantCustomConf: nil,
			wantErr:        false,
		},
		{
			name: "EvaluatorFieldMapping为nil的请求",
			request: &expt.CreateExperimentRequest{
				TargetVersionID:       gptr.Of(int64(123)),
				EvaluatorFieldMapping: nil,
			},
			wantCustomConf: nil,
			wantErr:        false,
		},
	}

	converter := NewEvalConfConvert()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToEntity(tt.request)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.wantCustomConf == nil {
				if result.ConnectorConf.TargetConf != nil && result.ConnectorConf.TargetConf.IngressConf != nil {
					assert.Nil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
				}
			} else {
				assert.NotNil(t, result.ConnectorConf.TargetConf)
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf)
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf.CustomConf)
				assert.Equal(t, len(tt.wantCustomConf.FieldConfs), len(result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs))
				if len(tt.wantCustomConf.FieldConfs) > 0 {
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].FieldName, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].FieldName)
					assert.Equal(t, tt.wantCustomConf.FieldConfs[0].Value, result.ConnectorConf.TargetConf.IngressConf.CustomConf.FieldConfs[0].Value)
				}
			}
		})
	}
}

func TestToExptDTO_RuntimeParam(t *testing.T) {
	tests := []struct {
		name             string
		experiment       *entity.Experiment
		wantRuntimeParam bool
		wantJSONValue    string
	}{
		{
			name: "包含运行时参数的实验",
			experiment: &entity.Experiment{
				ID:       123,
				SourceID: "test_source",
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: "input",
											FromField: "question",
										},
									},
								},
								CustomConf: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
											Value:     `{"model_config":{"model_id":"dto_test_model"}}`,
										},
									},
								},
							},
						},
					},
				},
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{},
			},
			wantRuntimeParam: true,
			wantJSONValue:    `{"model_config":{"model_id":"dto_test_model"}}`,
		},
		{
			name: "无运行时参数的实验",
			experiment: &entity.Experiment{
				ID:       123,
				SourceID: "test_source",
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: "input",
											FromField: "question",
										},
									},
								},
								CustomConf: nil,
							},
						},
					},
				},
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{},
			},
			wantRuntimeParam: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ToExptDTO(tt.experiment)

			assert.NotNil(t, result)
			assert.Equal(t, tt.experiment.ID, gptr.Indirect(result.ID))
			assert.Equal(t, tt.experiment.SourceID, gptr.Indirect(result.SourceID))

			if tt.wantRuntimeParam {
				assert.NotNil(t, result.TargetRuntimeParam)
				assert.NotNil(t, result.TargetRuntimeParam.JSONValue)
				assert.Equal(t, tt.wantJSONValue, gptr.Indirect(result.TargetRuntimeParam.JSONValue))
			} else {
				// 当没有运行时参数时，应该返回空的RuntimeParam对象而不是nil
				assert.NotNil(t, result.TargetRuntimeParam)
				assert.Nil(t, result.TargetRuntimeParam.JSONValue)
			}
		})
	}
}
