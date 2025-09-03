// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// Phase 3: Runtime param validation integration tests
func TestExptMangerImpl_checkTargetConnector_WithRuntimeParam(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "1"}

	tests := []struct {
		name    string
		expt    *entity.Experiment
		setup   func()
		wantErr bool
	}{
		{
			name: "valid_runtime_param_success",
			expt: &entity.Experiment{
				ID:         1,
				TargetType: entity.EvalTargetTypeLoopPrompt,
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						OutputSchema: []*entity.ArgsSchema{{Key: gptr.Of("output_field")}},
					},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{{Name: "input_field"}},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 1,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{{FromField: "input_field"}},
								},
								CustomConf: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
											Value:     `{"model_config":{"model_id":"test_model"}}`,
										},
									},
								},
							},
						},
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 1,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FromField: "input_field"}},
										},
										TargetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FromField: "output_field"}},
										},
									},
								},
							},
						},
					},
				},
			},
			setup: func() {
				mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).
					EXPECT().
					ValidateRuntimeParam(ctx, entity.EvalTargetTypeLoopPrompt, `{"model_config":{"model_id":"test_model"}}`).
					Return(nil)
			},
			wantErr: false,
		},
		{
			name: "invalid_runtime_param_format_error",
			expt: &entity.Experiment{
				ID:         1,
				TargetType: entity.EvalTargetTypeLoopPrompt,
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						OutputSchema: []*entity.ArgsSchema{{Key: gptr.Of("output_field")}},
					},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{{Name: "input_field"}},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 1,
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{{FromField: "input_field"}},
								},
								CustomConf: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: consts.FieldAdapterBuiltinFieldNameRuntimeParam,
											Value:     `invalid_json`,
										},
									},
								},
							},
						},
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 1,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FromField: "input_field"}},
										},
										TargetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{{FromField: "output_field"}},
										},
									},
								},
							},
						},
					},
				},
			},
			setup: func() {
				mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).
					EXPECT().
					ValidateRuntimeParam(ctx, entity.EvalTargetTypeLoopPrompt, "invalid_json").
					Return(errors.New("invalid JSON format"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			err := mgr.checkTargetConnector(ctx, tt.expt, session)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkTargetConnector() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
