// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	benefitMocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

func TestDefaultExptTurnEvaluationImpl_skipTargetNode_TargetVersionIDCheck(t *testing.T) {
	tests := []struct {
		name               string
		expt               *entity.Experiment
		expectedSkipTarget bool
	}{
		{
			name: "target_version_id_zero_should_skip_target",
			expt: &entity.Experiment{
				TargetVersionID: 0,
				ExptType:        entity.ExptType_Offline,
			},
			expectedSkipTarget: true,
		},
		{
			name: "target_version_id_nonzero_offline_should_not_skip_target",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				ExptType:        entity.ExptType_Offline,
			},
			expectedSkipTarget: false,
		},
		{
			name: "target_version_id_nonzero_online_should_skip_target",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				ExptType:        entity.ExptType_Online,
			},
			expectedSkipTarget: true,
		},
		{
			name: "target_version_id_zero_online_should_skip_target",
			expt: &entity.Experiment{
				TargetVersionID: 0,
				ExptType:        entity.ExptType_Online,
			},
			expectedSkipTarget: true,
		},
		{
			name: "large_target_version_id_offline_should_not_skip_target",
			expt: &entity.Experiment{
				TargetVersionID: 9223372036854775807, // max int64
				ExptType:        entity.ExptType_Offline,
			},
			expectedSkipTarget: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// Create the service implementation
			impl := &DefaultExptTurnEvaluationImpl{
				metric:            metricsMocks.NewMockExptMetric(ctrl),
				evalTargetService: svcMocks.NewMockIEvalTargetService(ctrl),
				evaluatorService:  svcMocks.NewMockEvaluatorService(ctrl),
				benefitService:    benefitMocks.NewMockIBenefitService(ctrl),
			}

			result := impl.skipTargetNode(tt.expt)
			assert.Equal(t, tt.expectedSkipTarget, result)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_skipTargetNode_ExptTypeCheck(t *testing.T) {
	tests := []struct {
		name               string
		exptType           entity.ExptType
		targetVersionID    int64
		expectedSkipTarget bool
	}{
		{
			name:               "offline_experiment_with_valid_target_should_not_skip",
			exptType:           entity.ExptType_Offline,
			targetVersionID:    123,
			expectedSkipTarget: false,
		},
		{
			name:               "online_experiment_with_valid_target_should_skip",
			exptType:           entity.ExptType_Online,
			targetVersionID:    123,
			expectedSkipTarget: true,
		},
		{
			name:               "offline_experiment_with_zero_target_should_skip",
			exptType:           entity.ExptType_Offline,
			targetVersionID:    0,
			expectedSkipTarget: true,
		},
		{
			name:               "online_experiment_with_zero_target_should_skip",
			exptType:           entity.ExptType_Online,
			targetVersionID:    0,
			expectedSkipTarget: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			impl := &DefaultExptTurnEvaluationImpl{
				metric:            metricsMocks.NewMockExptMetric(ctrl),
				evalTargetService: svcMocks.NewMockIEvalTargetService(ctrl),
				evaluatorService:  svcMocks.NewMockEvaluatorService(ctrl),
				benefitService:    benefitMocks.NewMockIBenefitService(ctrl),
			}

			expt := &entity.Experiment{
				TargetVersionID: tt.targetVersionID,
				ExptType:        tt.exptType,
			}

			result := impl.skipTargetNode(expt)
			assert.Equal(t, tt.expectedSkipTarget, result)
		})
	}
}

func TestDefaultExptTurnEvaluationImpl_skipTargetNode_EdgeCases(t *testing.T) {
	tests := []struct {
		name               string
		expt               *entity.Experiment
		expectedSkipTarget bool
	}{
		{
			name: "negative_target_version_id_should_skip",
			expt: &entity.Experiment{
				TargetVersionID: -1,
				ExptType:        entity.ExptType_Offline,
			},
			expectedSkipTarget: false,
		},
		{
			name: "min_positive_target_version_id_offline_should_not_skip",
			expt: &entity.Experiment{
				TargetVersionID: 1,
				ExptType:        entity.ExptType_Offline,
			},
			expectedSkipTarget: false,
		},
		{
			name: "min_positive_target_version_id_online_should_skip",
			expt: &entity.Experiment{
				TargetVersionID: 1,
				ExptType:        entity.ExptType_Online,
			},
			expectedSkipTarget: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			impl := &DefaultExptTurnEvaluationImpl{
				metric:            metricsMocks.NewMockExptMetric(ctrl),
				evalTargetService: svcMocks.NewMockIEvalTargetService(ctrl),
				evaluatorService:  svcMocks.NewMockEvaluatorService(ctrl),
				benefitService:    benefitMocks.NewMockIBenefitService(ctrl),
			}

			result := impl.skipTargetNode(tt.expt)
			assert.Equal(t, tt.expectedSkipTarget, result)
		})
	}
}

// Test to ensure the logic change from TargetConf to TargetVersionID
func TestDefaultExptTurnEvaluationImpl_skipTargetNode_LogicChange(t *testing.T) {
	tests := []struct {
		name               string
		expt               *entity.Experiment
		expectedSkipTarget bool
		description        string
	}{
		{
			name: "experiment_with_target_conf_but_zero_version_id_should_skip",
			expt: &entity.Experiment{
				TargetVersionID: 0,
				ExptType:        entity.ExptType_Offline,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 123, // This should be ignored
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{},
							},
						},
					},
				},
			},
			expectedSkipTarget: true,
			description:        "Should check experiment's TargetVersionID, not TargetConf",
		},
		{
			name: "experiment_with_nil_target_conf_but_valid_version_id_should_not_skip",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				ExptType:        entity.ExptType_Offline,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: nil, // This should be ignored
					},
				},
			},
			expectedSkipTarget: false,
			description:        "Should check experiment's TargetVersionID, not TargetConf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			impl := &DefaultExptTurnEvaluationImpl{
				metric:            metricsMocks.NewMockExptMetric(ctrl),
				evalTargetService: svcMocks.NewMockIEvalTargetService(ctrl),
				evaluatorService:  svcMocks.NewMockEvaluatorService(ctrl),
				benefitService:    benefitMocks.NewMockIBenefitService(ctrl),
			}

			result := impl.skipTargetNode(tt.expt)
			assert.Equal(t, tt.expectedSkipTarget, result, tt.description)
		})
	}
}