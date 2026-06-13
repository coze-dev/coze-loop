// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
)

func TestEvalConfConvert_ConvertToEntity_TargetConfAlwaysCreated(t *testing.T) {
	tests := []struct {
		name                string
		request             *expt.CreateExperimentRequest
		expectedTargetConf  bool
		expectedVersionID   int64
		expectedEvalSetConf bool
	}{
		{
			name: "nil_target_field_mapping_should_create_target_conf",
			request: &expt.CreateExperimentRequest{
				TargetVersionID:       gptr.Of(int64(123)),
				TargetFieldMapping:    nil,
				EvaluatorFieldMapping: nil,
			},
			expectedTargetConf:  true,
			expectedVersionID:   123,
			expectedEvalSetConf: false,
		},
		{
			name: "valid_target_field_mapping_should_create_target_conf",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(456)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
							ConstValue:    gptr.Of(""),
						},
					},
				},
				EvaluatorFieldMapping: nil,
			},
			expectedTargetConf:  true,
			expectedVersionID:   456,
			expectedEvalSetConf: false,
		},
		{
			name: "with_evaluator_field_mapping_should_create_both_confs",
			request: &expt.CreateExperimentRequest{
				TargetVersionID: gptr.Of(int64(789)),
				TargetFieldMapping: &domain_expt.TargetFieldMapping{
					FromEvalSet: []*domain_expt.FieldMapping{
						{
							FieldName:     gptr.Of("input"),
							FromFieldName: gptr.Of("question"),
						},
					},
				},
				EvaluatorFieldMapping: []*domain_expt.EvaluatorFieldMapping{
					{
						EvaluatorVersionID: 999,
						FromEvalSet: []*domain_expt.FieldMapping{
							{
								FieldName:     gptr.Of("eval_input"),
								FromFieldName: gptr.Of("question"),
							},
						},
					},
				},
			},
			expectedTargetConf:  true,
			expectedVersionID:   789,
			expectedEvalSetConf: true,
		},
	}

	converter := &EvalConfConvert{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := converter.ConvertToEntity(tt.request, nil)

			assert.NoError(t, err)
			assert.NotNil(t, result)

			// TargetConf should always be created
			if tt.expectedTargetConf {
				assert.NotNil(t, result.ConnectorConf.TargetConf)
				assert.Equal(t, tt.expectedVersionID, result.ConnectorConf.TargetConf.TargetVersionID)
				assert.NotNil(t, result.ConnectorConf.TargetConf.IngressConf)
			} else {
				assert.Nil(t, result.ConnectorConf.TargetConf)
			}

			// EvaluatorsConf should only be created when evaluator mapping exists
			if tt.expectedEvalSetConf {
				assert.NotNil(t, result.ConnectorConf.EvaluatorsConf)
			} else {
				assert.Nil(t, result.ConnectorConf.EvaluatorsConf)
			}
		})
	}
}

func TestConvertCreateReq_OldPath(t *testing.T) {
	// 老路径: EvalSetConfigs 为空时，走 EvalConfConvert，ExptConf 非 nil，EvalSetConfigs 为空
	req := &expt.CreateExperimentRequest{
		WorkspaceID:     1001,
		TargetVersionID: gptr.Of(int64(10)),
		Name:            gptr.Of("test-expt"),
	}
	param, err := ConvertCreateReq(req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, param)
	// 老路径：ExptConf 由 EvalConfConvert 生成，不为 nil
	assert.NotNil(t, param.ExptConf)
	// 老路径：EvalSetConfigs 为空
	assert.Empty(t, param.EvalSetConfigs)
	// 基础字段映射正确
	assert.Equal(t, int64(1001), param.WorkspaceID)
	assert.Equal(t, "test-expt", param.Name)
}

func TestConvertCreateReq_EvalSetConfigs(t *testing.T) {
	// 新路径: 有 EvalSetConfigs 时，EvalSetConfigs 非空，ExptConf 为 nil
	req := &expt.CreateExperimentRequest{
		WorkspaceID: 2002,
		Name:        gptr.Of("multi-set-expt"),
		EvalSetConfigs: []*domain_expt.EvalSetConfig{
			{
				EvalSetID:        111,
				EvalSetVersionID: 222,
				EvaluatorConfs: []*domain_expt.ExptEvaluatorConf{
					{
						EvaluatorID:        10,
						EvaluatorVersionID: 20,
						Alias:              gptr.Of("judge_A"),
					},
				},
			},
		},
	}
	param, err := ConvertCreateReq(req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, param)
	// 新路径：ExptConf 为 nil（不走 EvalConfConvert）
	assert.Nil(t, param.ExptConf)
	// 新路径：EvalSetConfigs 非空
	assert.Len(t, param.EvalSetConfigs, 1)
	// EvalSetID / EvalSetVersionID 正确
	assert.Equal(t, int64(111), param.EvalSetConfigs[0].EvalSetID)
	assert.Equal(t, int64(222), param.EvalSetConfigs[0].EvalSetVersionID)
	// EvaluatorConfs[0].Alias 正确
	assert.Len(t, param.EvalSetConfigs[0].EvaluatorConfs, 1)
	assert.Equal(t, "judge_A", param.EvalSetConfigs[0].EvaluatorConfs[0].Alias)
}

func TestConvertEvalSetConfigsDTOToDO(t *testing.T) {
	// 构造 2 个 EvalSetConfig，每个有不同 EvalSetID 和多个 ExptEvaluatorConf
	dtos := []*domain_expt.EvalSetConfig{
		{
			EvalSetID:        100,
			EvalSetVersionID: 101,
			EvaluatorConfs: []*domain_expt.ExptEvaluatorConf{
				{
					EvaluatorID:        1,
					EvaluatorVersionID: 11,
					Alias:              gptr.Of("alpha"),
				},
				{
					EvaluatorID:        2,
					EvaluatorVersionID: 22,
					Alias:              gptr.Of("beta"),
				},
			},
		},
		{
			EvalSetID:        200,
			EvalSetVersionID: 201,
			EvaluatorConfs: []*domain_expt.ExptEvaluatorConf{
				{
					EvaluatorID:        3,
					EvaluatorVersionID: 33,
					Alias:              gptr.Of("gamma"),
				},
			},
		},
	}

	// 通过 ConvertCreateReq 的新路径间接触发 convertEvalSetConfigsDTOToDO
	req := &expt.CreateExperimentRequest{
		WorkspaceID:    999,
		EvalSetConfigs: dtos,
	}
	param, err := ConvertCreateReq(req, nil)
	assert.NoError(t, err)
	assert.NotNil(t, param)

	// 长度断言
	assert.Len(t, param.EvalSetConfigs, 2)

	// 第一个 EvalSetConfig
	cfg0 := param.EvalSetConfigs[0]
	assert.Equal(t, int64(100), cfg0.EvalSetID)
	assert.Equal(t, int64(101), cfg0.EvalSetVersionID)
	assert.Len(t, cfg0.EvaluatorConfs, 2)
	assert.Equal(t, "alpha", cfg0.EvaluatorConfs[0].Alias)
	assert.Equal(t, "beta", cfg0.EvaluatorConfs[1].Alias)

	// 第二个 EvalSetConfig
	cfg1 := param.EvalSetConfigs[1]
	assert.Equal(t, int64(200), cfg1.EvalSetID)
	assert.Equal(t, int64(201), cfg1.EvalSetVersionID)
	assert.Len(t, cfg1.EvaluatorConfs, 1)
	assert.Equal(t, "gamma", cfg1.EvaluatorConfs[0].Alias)
}

func TestToTargetFieldMappingDO_AlwaysReturnsValidConf(t *testing.T) {
	tests := []struct {
		name                  string
		mapping               *domain_expt.TargetFieldMapping
		runtimeParam          *common.RuntimeParam
		expectedEvalSetFields int
		expectedCustomConf    bool
	}{
		{
			name:                  "nil_mapping_should_return_valid_conf_with_empty_adapter",
			mapping:               nil,
			runtimeParam:          nil,
			expectedEvalSetFields: 0,
			expectedCustomConf:    false,
		},
		{
			name: "valid_mapping_should_populate_field_configs",
			mapping: &domain_expt.TargetFieldMapping{
				FromEvalSet: []*domain_expt.FieldMapping{
					{
						FieldName:     gptr.Of("input"),
						FromFieldName: gptr.Of("question"),
						ConstValue:    gptr.Of(""),
					},
					{
						FieldName:     gptr.Of("role"),
						FromFieldName: gptr.Of("user_role"),
						ConstValue:    gptr.Of("user"),
					},
				},
			},
			runtimeParam:          nil,
			expectedEvalSetFields: 2,
			expectedCustomConf:    false,
		},
		{
			name:    "nil_mapping_with_runtime_param_should_create_custom_conf",
			mapping: nil,
			runtimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(`{"model":"test"}`),
			},
			expectedEvalSetFields: 0,
			expectedCustomConf:    true,
		},
		{
			name: "valid_mapping_with_runtime_param_should_create_both",
			mapping: &domain_expt.TargetFieldMapping{
				FromEvalSet: []*domain_expt.FieldMapping{
					{
						FieldName:     gptr.Of("input"),
						FromFieldName: gptr.Of("question"),
					},
				},
			},
			runtimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(`{"temperature":0.7}`),
			},
			expectedEvalSetFields: 1,
			expectedCustomConf:    true,
		},
		{
			name: "runtime_param_with_empty_json_should_not_create_custom_conf",
			mapping: &domain_expt.TargetFieldMapping{
				FromEvalSet: []*domain_expt.FieldMapping{
					{
						FieldName:     gptr.Of("input"),
						FromFieldName: gptr.Of("question"),
					},
				},
			},
			runtimeParam: &common.RuntimeParam{
				JSONValue: gptr.Of(""),
			},
			expectedEvalSetFields: 1,
			expectedCustomConf:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toTargetFieldMappingDO(tt.mapping, tt.runtimeParam)

			// Should always return a valid TargetIngressConf
			assert.NotNil(t, result)
			assert.NotNil(t, result.EvalSetAdapter)

			// Check EvalSetAdapter field configurations
			assert.Equal(t, tt.expectedEvalSetFields, len(result.EvalSetAdapter.FieldConfs))

			// Check CustomConf creation
			if tt.expectedCustomConf {
				assert.NotNil(t, result.CustomConf)
				assert.Equal(t, 1, len(result.CustomConf.FieldConfs))
				assert.Equal(t, consts.FieldAdapterBuiltinFieldNameRuntimeParam, result.CustomConf.FieldConfs[0].FieldName)
			} else {
				assert.Nil(t, result.CustomConf)
			}

			// Verify field mapping content when mapping is provided
			if tt.mapping != nil && len(tt.mapping.FromEvalSet) > 0 {
				for i, expectedMapping := range tt.mapping.FromEvalSet {
					actualField := result.EvalSetAdapter.FieldConfs[i]
					assert.Equal(t, gptr.Indirect(expectedMapping.FieldName), actualField.FieldName)
					assert.Equal(t, gptr.Indirect(expectedMapping.FromFieldName), actualField.FromField)
					assert.Equal(t, gptr.Indirect(expectedMapping.ConstValue), actualField.Value)
				}
			}
		})
	}
}
