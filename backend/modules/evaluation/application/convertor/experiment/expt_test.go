// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
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

	target, evaluators := NewEvalConfConvert().ConvertEntityToDTO(conf)
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
