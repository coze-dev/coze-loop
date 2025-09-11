// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluatorsConf_Valid_NilCheck(t *testing.T) {
	tests := []struct {
		name           string
		evaluatorsConf *EvaluatorsConf
		expectedError  bool
		errorMessage   string
	}{
		{
			name:           "nil_evaluators_conf_should_return_error",
			evaluatorsConf: nil,
			expectedError:  true,
			errorMessage:   "nil EvaluatorConf",
		},
		{
			name: "empty_evaluator_conf_slice_should_pass",
			evaluatorsConf: &EvaluatorsConf{
				EvaluatorConf: []*EvaluatorConf{},
			},
			expectedError: false,
		},
		{
			name: "valid_evaluator_conf_should_pass",
			evaluatorsConf: &EvaluatorsConf{
				EvaluatorConf: []*EvaluatorConf{
					{
						EvaluatorVersionID: 123,
						IngressConf: &EvaluatorIngressConf{
							EvalSetAdapter: &FieldAdapter{
								FieldConfs: []*FieldConf{
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
			expectedError: false,
		},
		{
			name: "invalid_evaluator_conf_should_return_error",
			evaluatorsConf: &EvaluatorsConf{
				EvaluatorConf: []*EvaluatorConf{
					{
						EvaluatorVersionID: 0, // Invalid version ID
						IngressConf:        nil,
					},
				},
			},
			expectedError: true,
		},
		{
			name: "mixed_valid_and_invalid_evaluator_conf_should_return_error",
			evaluatorsConf: &EvaluatorsConf{
				EvaluatorConf: []*EvaluatorConf{
					{
						EvaluatorVersionID: 123,
						IngressConf: &EvaluatorIngressConf{
							EvalSetAdapter: &FieldAdapter{
								FieldConfs: []*FieldConf{
									{
										FieldName: "input",
										FromField: "question",
									},
								},
							},
						},
					},
					{
						EvaluatorVersionID: 0, // Invalid version ID
						IngressConf:        nil,
					},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := tt.evaluatorsConf.Valid(ctx)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorMessage != "" {
					assert.Contains(t, err.Error(), tt.errorMessage)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluatorConf_Valid_NewValidation(t *testing.T) {
	tests := []struct {
		name           string
		evaluatorConf  *EvaluatorConf
		expectedError  bool
		errorContains  string
	}{
		{
			name:          "nil_evaluator_conf_should_return_error",
			evaluatorConf: nil,
			expectedError: true,
			errorContains: "invalid EvaluatorConf",
		},
		{
			name: "zero_evaluator_version_id_should_return_error",
			evaluatorConf: &EvaluatorConf{
				EvaluatorVersionID: 0,
				IngressConf: &EvaluatorIngressConf{
					EvalSetAdapter: &FieldAdapter{},
				},
			},
			expectedError: true,
			errorContains: "invalid EvaluatorConf",
		},
		{
			name: "nil_ingress_conf_should_return_error",
			evaluatorConf: &EvaluatorConf{
				EvaluatorVersionID: 123,
				IngressConf:        nil,
			},
			expectedError: true,
			errorContains: "invalid EvaluatorConf",
		},
		{
			name: "both_adapters_nil_should_return_error",
			evaluatorConf: &EvaluatorConf{
				EvaluatorVersionID: 123,
				IngressConf: &EvaluatorIngressConf{
					EvalSetAdapter: nil,
					TargetAdapter:  nil,
				},
			},
			expectedError: true,
			errorContains: "invalid EvaluatorConf",
		},
		{
			name: "valid_with_eval_set_adapter_only_should_pass",
			evaluatorConf: &EvaluatorConf{
				EvaluatorVersionID: 123,
				IngressConf: &EvaluatorIngressConf{
					EvalSetAdapter: &FieldAdapter{
						FieldConfs: []*FieldConf{
							{
								FieldName: "input",
								FromField: "question",
							},
						},
					},
					TargetAdapter: nil,
				},
			},
			expectedError: false,
		},
		{
			name: "valid_with_target_adapter_only_should_pass",
			evaluatorConf: &EvaluatorConf{
				EvaluatorVersionID: 123,
				IngressConf: &EvaluatorIngressConf{
					EvalSetAdapter: nil,
					TargetAdapter: &FieldAdapter{
						FieldConfs: []*FieldConf{
							{
								FieldName: "output",
								FromField: "result",
							},
						},
					},
				},
			},
			expectedError: false,
		},
		{
			name: "valid_with_both_adapters_should_pass",
			evaluatorConf: &EvaluatorConf{
				EvaluatorVersionID: 123,
				IngressConf: &EvaluatorIngressConf{
					EvalSetAdapter: &FieldAdapter{
						FieldConfs: []*FieldConf{
							{
								FieldName: "input",
								FromField: "question",
							},
						},
					},
					TargetAdapter: &FieldAdapter{
						FieldConfs: []*FieldConf{
							{
								FieldName: "output",
								FromField: "result",
							},
						},
					},
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := tt.evaluatorConf.Valid(ctx)

			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}