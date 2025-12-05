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

	"github.com/coze-dev/coze-loop/backend/infra/external/audit"
	auditMocks "github.com/coze-dev/coze-loop/backend/infra/external/audit/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

type testExptManager = ExptMangerImpl

func TestExptMangerImpl_checkTargetConnector_VersionIDValidation(t *testing.T) {
	tests := []struct {
		name              string
		expt              *entity.Experiment
		setupMocks        func(mgr *testExptManager)
		expectedError     bool
		expectedErrorCode string
		expectedErrorMsg  string
	}{
		{
			name: "target_version_id_mismatch_should_return_error",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "question"},
								{Name: "expected"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456, // Different from experiment's TargetVersionID
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: "input",
											FromField: "question",
										},
									},
								},
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// No mocks needed for this test case
			},
			expectedError:     true,
			expectedErrorCode: "601204001",
			expectedErrorMsg:  "target config's version id not match",
		},
		{
			name: "target_version_id_match_should_pass_validation",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				TargetType:      entity.EvalTargetTypeLoopPrompt,
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "question"},
								{Name: "expected"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 123, // Matches experiment's TargetVersionID
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{
											FieldName: "input",
											FromField: "question",
										},
									},
								},
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// Mock TargetConf.Valid to return no error
				// Note: In real implementation, this would be called on the entity
			},
			expectedError: false,
		},
		{
			name: "loop_trace_target_should_fail_validation", // 修正测试名称以符合实际行为
			expt: &entity.Experiment{
				TargetVersionID: 123,
				TargetType:      entity.EvalTargetTypeLoopPrompt, // 添加TargetType以触发fixTargetConf
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopTrace,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "field1"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456, // Different version ID should cause error
							IngressConf: &entity.TargetIngressConf{
								EvalSetAdapter: &entity.FieldAdapter{
									FieldConfs: []*entity.FieldConf{
										{FromField: "field1"},
									},
								},
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// No mocks needed
			},
			expectedError:     true,
			expectedErrorCode: "601204001",
			expectedErrorMsg:  "target config's version id not match",
		},
		{
			name: "nil_target_should_skip_validation",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				Target:          nil,
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						TargetConf: &entity.TargetConf{
							TargetVersionID: 456,
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// No mocks needed as validation should be skipped
			},
			expectedError: false,
		},
		{
			name: "runtime_param_validation_error_should_return_error",
			expt: &entity.Experiment{
				TargetVersionID: 123,
				TargetType:      entity.EvalTargetTypeLoopPrompt,
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "question"},
								{Name: "expected"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
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
											Value:     "invalid_json",
										},
									},
								},
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				mgr.evalTargetService.(*svcMocks.MockIEvalTargetService).
					EXPECT().
					ValidateRuntimeParam(gomock.Any(), entity.EvalTargetTypeLoopPrompt, "invalid_json").
					Return(errors.New("invalid JSON format"))
			},
			expectedError:     true,
			expectedErrorCode: "601204001",
			expectedErrorMsg:  "invalid runtime param",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mgr := newTestExptManager(ctrl)
			tt.setupMocks(mgr)

			ctx := context.Background()
			session := &entity.Session{UserID: "test_user"}

			err := mgr.checkTargetConnector(ctx, tt.expt, session)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorCode != "" {
					// Check if error contains expected code
					assert.Contains(t, err.Error(), tt.expectedErrorCode)
				}
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptMangerImpl_checkEvaluatorsConnector_VersionIDValidation(t *testing.T) {
	tests := []struct {
		name              string
		expt              *entity.Experiment
		setupMocks        func(mgr *testExptManager)
		expectedError     bool
		expectedErrorCode string
		expectedErrorMsg  string
	}{
		{
			name: "evaluator_version_id_not_found_should_return_error",
			expt: &entity.Experiment{
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorVersionID: 123},
					{EvaluatorVersionID: 456},
				},
				Evaluators: []*entity.Evaluator{
					{ID: 1},
				},
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						OutputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("result")},
						},
					},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "question"},
								{Name: "expected"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 123,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{},
									},
								},
								{
									EvaluatorVersionID: 789, // Not in EvaluatorVersionRef
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{},
									},
								},
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// Mock EvaluatorsConf.Valid to return no error first
				// This would be handled by the entity validation
			},
			expectedError:     true,
			expectedErrorCode: "601204001",
			expectedErrorMsg:  "evaluator version id not found 789",
		},
		{
			name: "all_evaluator_version_ids_found_should_pass",
			expt: &entity.Experiment{
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorVersionID: 123},
					{EvaluatorVersionID: 456},
				},
				Evaluators: []*entity.Evaluator{
					{ID: 1},
				},
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
					EvalTargetVersion: &entity.EvalTargetVersion{
						OutputSchema: []*entity.ArgsSchema{
							{Key: gptr.Of("result")},
						},
					},
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "question"},
								{Name: "expected"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 123,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "input",
													FromField: "question",
												},
											},
										},
										TargetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "output",
													FromField: "result",
												},
											},
										},
									},
								},
								{
									EvaluatorVersionID: 456,
									IngressConf: &entity.EvaluatorIngressConf{
										EvalSetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "reference",
													FromField: "expected",
												},
											},
										},
										TargetAdapter: &entity.FieldAdapter{
											FieldConfs: []*entity.FieldConf{
												{
													FieldName: "output",
													FromField: "result",
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
			setupMocks: func(mgr *testExptManager) {
				// Mock would be handled by entity validation
			},
			expectedError: false,
		},
		{
			name: "no_evaluators_should_skip_validation",
			expt: &entity.Experiment{
				Evaluators: []*entity.Evaluator{},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: nil,
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// No mocks needed as validation should be skipped
			},
			expectedError: false,
		},
		{
			name: "evaluators_conf_validation_error_should_return_error",
			expt: &entity.Experiment{
				EvaluatorVersionRef: []*entity.ExptEvaluatorVersionRef{
					{EvaluatorVersionID: 123},
				},
				Evaluators: []*entity.Evaluator{
					{ID: 1},
				},
				EvalConf: &entity.EvaluationConfiguration{
					ConnectorConf: entity.Connector{
						EvaluatorsConf: &entity.EvaluatorsConf{
							EvaluatorConf: []*entity.EvaluatorConf{
								{
									EvaluatorVersionID: 0, // Invalid version ID
									IngressConf:        nil,
								},
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// This would be handled by entity validation
			},
			expectedError:     true,
			expectedErrorCode: "601204001",
			expectedErrorMsg:  "invalid evaluator connector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mgr := newTestExptManager(ctrl)
			tt.setupMocks(mgr)

			ctx := context.Background()
			session := &entity.Session{UserID: "test_user"}
			err := mgr.checkEvaluatorsConnector(ctx, tt.expt, session)
			if tt.expectedError {
				assert.Error(t, err)
				if tt.expectedErrorCode != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorCode)
				}
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExptMangerImpl_CheckRun_RemovedCheckers(t *testing.T) {
	tests := []struct {
		name          string
		expt          *entity.Experiment
		setupMocks    func(mgr *testExptManager)
		expectedError bool
	}{
		{
			name: "check_run_should_not_include_removed_checkers",
			expt: &entity.Experiment{
				ExptType:         entity.ExptType_Offline,
				EvalSetVersionID: 456,
				TargetVersionID:  123,
				Target: &entity.EvalTarget{
					EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				},
				EvalSet: &entity.EvaluationSet{
					EvaluationSetVersion: &entity.EvaluationSetVersion{
						ItemCount: 10,
						EvaluationSetSchema: &entity.EvaluationSetSchema{
							FieldSchemas: []*entity.FieldSchema{
								{Name: "question"},
								{Name: "expected"},
							},
						},
					},
				},
				EvalConf: &entity.EvaluationConfiguration{
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
							},
						},
					},
				},
			},
			setupMocks: func(mgr *testExptManager) {
				// Mock audit service
				mgr.audit.(*auditMocks.MockIAuditService).
					EXPECT().
					Audit(gomock.Any(), gomock.Any()).
					Return(audit.AuditRecord{AuditStatus: audit.AuditStatus_Approved}, nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mgr := newTestExptManager(ctrl)
			tt.setupMocks(mgr)

			ctx := context.Background()
			session := &entity.Session{UserID: "test_user"}

			err := mgr.CheckRun(ctx, tt.expt, 123, session)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
