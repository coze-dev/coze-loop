// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertBoxType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, evaluatordo.EvaluatorBoxTypeWhite, convertBoxTypeDTO2DO("White"))
	assert.Equal(t, evaluatordo.EvaluatorBoxTypeBlack, convertBoxTypeDTO2DO("Black"))
	assert.Equal(t, evaluatordo.EvaluatorBoxTypeWhite, convertBoxTypeDTO2DO(""))

	assert.Equal(t, "White", convertBoxTypeDO2DTO(evaluatordo.EvaluatorBoxTypeWhite))
	assert.Equal(t, "Black", convertBoxTypeDO2DTO(evaluatordo.EvaluatorBoxTypeBlack))
}

func TestNormalizeLanguageType(t *testing.T) {
	t.Parallel()
	assert.Equal(t, evaluatordo.LanguageTypePython, normalizeLanguageType(evaluatordo.LanguageType("python")))
	assert.Equal(t, evaluatordo.LanguageTypeJS, normalizeLanguageType(evaluatordo.LanguageType("js")))
	assert.Equal(t, evaluatordo.LanguageType("Java"), normalizeLanguageType(evaluatordo.LanguageType("java")))
}

func TestConvertEvaluatorDTO2DO_And_Back(t *testing.T) {
	t.Parallel()
	dto := &evaluatordto.Evaluator{
		EvaluatorID:           gptr.Of(int64(1)),
		WorkspaceID:           gptr.Of(int64(2)),
		Name:                  gptr.Of("n"),
		Description:           gptr.Of("d"),
		DraftSubmitted:        gptr.Of(true),
		EvaluatorType:         evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
		LatestVersion:         gptr.Of("1.0.0"),
		Builtin:               gptr.Of(false),
		BuiltinVisibleVersion: gptr.Of("1.0.0"),
		BoxType:               gptr.Of("Black"),
		BaseInfo:              &commondto.BaseInfo{},
		CurrentVersion: &evaluatordto.EvaluatorVersion{
			ID:      gptr.Of(int64(10)),
			Version: gptr.Of("1.0.0"),
			EvaluatorContent: &evaluatordto.EvaluatorContent{CodeEvaluator: &evaluatordto.CodeEvaluator{
				LanguageType:     gptr.Of(evaluatordto.LanguageTypePython),
				CodeTemplateKey:  gptr.Of("tk"),
				CodeTemplateName: gptr.Of("tn"),
				CodeContent:      gptr.Of("print(1)"),
			}},
		},
		Tags: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{
			evaluatordto.EvaluatorTagLangType("en"): {evaluatordto.EvaluatorTagKeyName: {"tag"}},
		},
	}
	do, err := ConvertEvaluatorDTO2DO(dto)
	if assert.NotNil(t, do) && assert.NoError(t, err) {
		assert.Equal(t, int64(1), do.ID)
		assert.Equal(t, int64(2), do.SpaceID)
		assert.Equal(t, "n", do.Name)
		assert.Equal(t, true, do.DraftSubmitted)
		assert.Equal(t, evaluatordo.EvaluatorBoxTypeBlack, do.BoxType)
		// EvaluatorInfo 字段非本用例重点
		// Code 版本
		if assert.NotNil(t, do.CodeEvaluatorVersion) {
			assert.Equal(t, "Python", string(do.CodeEvaluatorVersion.LanguageType))
			assert.Equal(t, "print(1)", do.CodeEvaluatorVersion.CodeContent)
		}
		// Tags
		assert.Equal(t, []string{"tag"}, do.Tags[evaluatordo.EvaluatorTagLangType("en")][evaluatordo.EvaluatorTagKey("Name")])
	}
	back := ConvertEvaluatorDO2DTO(do)
	if assert.NotNil(t, back) {
		assert.Equal(t, "Black", back.GetBoxType())
		if assert.NotNil(t, back.CurrentVersion) && assert.NotNil(t, back.CurrentVersion.EvaluatorContent) {
			assert.Equal(t, evaluatordto.LanguageTypePython, back.CurrentVersion.EvaluatorContent.CodeEvaluator.GetLanguageType())
		}
		// EvaluatorInfo 字段非本用例重点
	}
}

func TestConvertCodeEvaluatorVersionRoundTrip(t *testing.T) {
	t.Parallel()
	// DTO -> DO
	dto := &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(int64(11)),
		Version:     gptr.Of("0.1.0"),
		Description: gptr.Of("desc"),
		EvaluatorContent: &evaluatordto.EvaluatorContent{CodeEvaluator: &evaluatordto.CodeEvaluator{
			LanguageType:     gptr.Of(evaluatordto.LanguageTypeJS),
			CodeTemplateKey:  gptr.Of("k"),
			CodeTemplateName: gptr.Of("n"),
			CodeContent:      gptr.Of("console.log(1)"),
		}},
	}
	do := ConvertCodeEvaluatorVersionDTO2DO(1, 2, dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, int64(11), do.ID)
		assert.Equal(t, int64(2), do.SpaceID)
		assert.Equal(t, evaluatordo.LanguageTypeJS, do.LanguageType)
	}
	// DO -> DTO
	dtoBack := ConvertCodeEvaluatorVersionDO2DTO(do)
	if assert.NotNil(t, dtoBack) {
		assert.Equal(t, "0.1.0", dtoBack.GetVersion())
		if assert.NotNil(t, dtoBack.EvaluatorContent) && assert.NotNil(t, dtoBack.EvaluatorContent.CodeEvaluator) {
			assert.Equal(t, evaluatordto.LanguageTypeJS, dtoBack.EvaluatorContent.CodeEvaluator.GetLanguageType())
			assert.Equal(t, "console.log(1)", dtoBack.EvaluatorContent.CodeEvaluator.GetCodeContent())
		}
	}
}

func TestConvertPromptEvaluatorVersionRoundTrip(t *testing.T) {
	t.Parallel()
	// DTO -> DO
	dto := &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(int64(21)),
		Version:     gptr.Of("0.2.0"),
		Description: gptr.Of("desc"),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			ReceiveChatHistory: gptr.Of(true),
			InputSchemas:       []*commondto.ArgsSchema{{Key: gptr.Of("in")}},
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType_BuiltinTemplate),
				PromptTemplateKey: gptr.Of("ptk"),
				MessageList:       []*commondto.Message{{Content: &commondto.Content{Text: gptr.Of("t")}}},
			},
		},
	}
	do := ConvertPromptEvaluatorVersionDTO2DO(100, 200, dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, int64(21), do.ID)
		assert.True(t, gptr.Indirect(do.ReceiveChatHistory))
		assert.Equal(t, "ptk", do.PromptTemplateKey)
		assert.Len(t, do.InputSchemas, 1)
	}
	// DO -> DTO
	dtoBack := ConvertPromptEvaluatorVersionDO2DTO(do)
	if assert.NotNil(t, dtoBack) {
		assert.Equal(t, "0.2.0", dtoBack.GetVersion())
		if assert.NotNil(t, dtoBack.EvaluatorContent) && assert.NotNil(t, dtoBack.EvaluatorContent.PromptEvaluator) {
			assert.Equal(t, "ptk", dtoBack.EvaluatorContent.PromptEvaluator.GetPromptTemplateKey())
		}
	}
}

func TestConvertEvaluatorContent2DO(t *testing.T) {
	t.Parallel()
	// nil content
	e, err := ConvertEvaluatorContent2DO(nil, evaluatordto.EvaluatorType_Prompt)
	assert.Nil(t, e)
	assert.Error(t, err)

	// prompt missing content
	_, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{}, evaluatordto.EvaluatorType_Prompt)
	assert.Error(t, err)

	// code missing content
	_, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{}, evaluatordto.EvaluatorType_Code)
	assert.Error(t, err)

	// custom rpc missing content
	_, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{}, evaluatordto.EvaluatorType_CustomRPC)
	assert.Error(t, err)

	// code ok
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		CodeEvaluator: &evaluatordto.CodeEvaluator{LanguageType: gptr.Of(evaluatordto.LanguageTypePython), CodeTemplateKey: gptr.Of("k"), CodeTemplateName: gptr.Of("n"), CodeContent: gptr.Of("print(1)")},
	}, evaluatordto.EvaluatorType_Code)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.CodeEvaluatorVersion) {
		assert.Equal(t, evaluatordo.LanguageTypePython, e.CodeEvaluatorVersion.LanguageType)
	}

	// prompt ok
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		ReceiveChatHistory: gptr.Of(true),
		PromptEvaluator: &evaluatordto.PromptEvaluator{
			PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType_BuiltinTemplate),
			PromptTemplateKey: gptr.Of("key"),
			MessageList:       []*commondto.Message{{Content: &commondto.Content{Text: gptr.Of("t")}}},
			ModelConfig:       &commondto.ModelConfig{},
		},
		InputSchemas: []*commondto.ArgsSchema{{Key: gptr.Of("in")}},
	}, evaluatordto.EvaluatorType_Prompt)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.PromptEvaluatorVersion) {
		assert.True(t, gptr.Indirect(e.PromptEvaluatorVersion.ReceiveChatHistory))
		assert.Equal(t, "key", e.PromptEvaluatorVersion.PromptTemplateKey)
	}

	// custom rpc ok - basic fields
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
			ProviderEvaluatorCode: gptr.Of("CN:480"),
			AccessProtocol:        evaluatordto.EvaluatorAccessProtocolRPC,
			ServiceName:           gptr.Of("test-service"),
			Cluster:               gptr.Of("test-cluster"),
			Timeout:               gptr.Of(int64(5000)),
		},
	}, evaluatordto.EvaluatorType_CustomRPC)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.CustomRPCEvaluatorVersion) {
		assert.Equal(t, "CN:480", gptr.Indirect(e.CustomRPCEvaluatorVersion.ProviderEvaluatorCode))
		assert.Equal(t, evaluatordto.EvaluatorAccessProtocolRPC, e.CustomRPCEvaluatorVersion.AccessProtocol)
		assert.Equal(t, "test-service", gptr.Indirect(e.CustomRPCEvaluatorVersion.ServiceName))
		assert.Equal(t, "test-cluster", gptr.Indirect(e.CustomRPCEvaluatorVersion.Cluster))
		assert.Equal(t, int64(5000), gptr.Indirect(e.CustomRPCEvaluatorVersion.Timeout))
	}

	// custom rpc ok - with input schemas
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
			ProviderEvaluatorCode: gptr.Of("CN:480"),
			AccessProtocol:        evaluatordto.EvaluatorAccessProtocolRPC,
			ServiceName:           gptr.Of("test-service"),
		},
		InputSchemas: []*commondto.ArgsSchema{
			{Key: gptr.Of("input1"), SupportContentTypes: []string{"text"}},
			{Key: gptr.Of("input2"), SupportContentTypes: []string{"image"}},
		},
	}, evaluatordto.EvaluatorType_CustomRPC)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.CustomRPCEvaluatorVersion) {
		assert.NotNil(t, e.CustomRPCEvaluatorVersion.InputSchemas)
		assert.Equal(t, 2, len(e.CustomRPCEvaluatorVersion.InputSchemas))
		assert.Equal(t, "input1", gptr.Indirect(e.CustomRPCEvaluatorVersion.InputSchemas[0].Key))
		assert.Equal(t, "input2", gptr.Indirect(e.CustomRPCEvaluatorVersion.InputSchemas[1].Key))
		assert.Equal(t, 1, len(e.CustomRPCEvaluatorVersion.InputSchemas[0].SupportContentTypes))
		assert.Equal(t, evaluatordo.ContentType("text"), e.CustomRPCEvaluatorVersion.InputSchemas[0].SupportContentTypes[0])
	}

	// custom rpc ok - with output schemas
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
			ProviderEvaluatorCode: gptr.Of("CN:480"),
			AccessProtocol:        evaluatordto.EvaluatorAccessProtocolRPC,
			ServiceName:           gptr.Of("test-service"),
		},
		OutputSchemas: []*commondto.ArgsSchema{
			{Key: gptr.Of("output1"), SupportContentTypes: []string{"text"}},
		},
	}, evaluatordto.EvaluatorType_CustomRPC)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.CustomRPCEvaluatorVersion) {
		assert.NotNil(t, e.CustomRPCEvaluatorVersion.OutputSchemas)
		assert.Equal(t, 1, len(e.CustomRPCEvaluatorVersion.OutputSchemas))
		assert.Equal(t, "output1", gptr.Indirect(e.CustomRPCEvaluatorVersion.OutputSchemas[0].Key))
	}

	// custom rpc ok - with both input and output schemas
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
			ProviderEvaluatorCode: gptr.Of("CN:480"),
			AccessProtocol:        evaluatordto.EvaluatorAccessProtocolRPC,
			ServiceName:           gptr.Of("test-service"),
			Cluster:               gptr.Of("prod-cluster"),
			Timeout:               gptr.Of(int64(10000)),
		},
		InputSchemas: []*commondto.ArgsSchema{
			{Key: gptr.Of("input1"), SupportContentTypes: []string{"text"}},
		},
		OutputSchemas: []*commondto.ArgsSchema{
			{Key: gptr.Of("output1"), SupportContentTypes: []string{"text"}},
			{Key: gptr.Of("output2"), SupportContentTypes: []string{"json"}},
		},
	}, evaluatordto.EvaluatorType_CustomRPC)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.CustomRPCEvaluatorVersion) {
		assert.Equal(t, "CN:480", gptr.Indirect(e.CustomRPCEvaluatorVersion.ProviderEvaluatorCode))
		assert.Equal(t, evaluatordto.EvaluatorAccessProtocolRPC, e.CustomRPCEvaluatorVersion.AccessProtocol)
		assert.Equal(t, "test-service", gptr.Indirect(e.CustomRPCEvaluatorVersion.ServiceName))
		assert.Equal(t, "prod-cluster", gptr.Indirect(e.CustomRPCEvaluatorVersion.Cluster))
		assert.Equal(t, int64(10000), gptr.Indirect(e.CustomRPCEvaluatorVersion.Timeout))
		assert.NotNil(t, e.CustomRPCEvaluatorVersion.InputSchemas)
		assert.Equal(t, 1, len(e.CustomRPCEvaluatorVersion.InputSchemas))
		assert.NotNil(t, e.CustomRPCEvaluatorVersion.OutputSchemas)
		assert.Equal(t, 2, len(e.CustomRPCEvaluatorVersion.OutputSchemas))
		assert.Equal(t, "input1", gptr.Indirect(e.CustomRPCEvaluatorVersion.InputSchemas[0].Key))
		assert.Equal(t, "output1", gptr.Indirect(e.CustomRPCEvaluatorVersion.OutputSchemas[0].Key))
		assert.Equal(t, "output2", gptr.Indirect(e.CustomRPCEvaluatorVersion.OutputSchemas[1].Key))
	}

	// custom rpc ok - empty schemas
	e, err = ConvertEvaluatorContent2DO(&evaluatordto.EvaluatorContent{
		CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
			ProviderEvaluatorCode: gptr.Of("CN:480"),
			AccessProtocol:        evaluatordto.EvaluatorAccessProtocolRPC,
		},
		InputSchemas:  []*commondto.ArgsSchema{},
		OutputSchemas: []*commondto.ArgsSchema{},
	}, evaluatordto.EvaluatorType_CustomRPC)
	assert.NoError(t, err)
	if assert.NotNil(t, e) && assert.NotNil(t, e.CustomRPCEvaluatorVersion) {
		assert.Nil(t, e.CustomRPCEvaluatorVersion.InputSchemas)
		assert.Nil(t, e.CustomRPCEvaluatorVersion.OutputSchemas)
	}
}

func TestTagKeyConvert(t *testing.T) {
	t.Parallel()
	assert.Equal(t, evaluatordto.EvaluatorTagKeyCategory, ConvertEvaluatorTagKeyDO2DTO(evaluatordo.EvaluatorTagKey_Category))
	assert.Equal(t, evaluatordto.EvaluatorTagKeyTargetType, ConvertEvaluatorTagKeyDO2DTO(evaluatordo.EvaluatorTagKey_TargetType))
	assert.Equal(t, evaluatordto.EvaluatorTagKeyObjective, ConvertEvaluatorTagKeyDO2DTO(evaluatordo.EvaluatorTagKey_Objective))
	assert.Equal(t, evaluatordto.EvaluatorTagKeyBusinessScenario, ConvertEvaluatorTagKeyDO2DTO(evaluatordo.EvaluatorTagKey_BusinessScenario))
	assert.Equal(t, evaluatordto.EvaluatorTagKeyName, ConvertEvaluatorTagKeyDO2DTO(evaluatordo.EvaluatorTagKey_Name))
}

func TestConvertEvaluatorDOList2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		doList   []*evaluatordo.Evaluator
		expected int
	}{
		{
			name: "多个评估器转换",
			doList: []*evaluatordo.Evaluator{
				{
					ID:            123,
					SpaceID:       456,
					Name:          "Evaluator 1",
					EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				},
				{
					ID:            124,
					SpaceID:       456,
					Name:          "Evaluator 2",
					EvaluatorType: evaluatordo.EvaluatorTypeCode,
				},
			},
			expected: 2,
		},
		{
			name:     "空列表",
			doList:   []*evaluatordo.Evaluator{},
			expected: 0,
		},
		{
			name:     "nil列表",
			doList:   nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDOList2DTO(tt.doList)

			assert.Equal(t, tt.expected, len(result))

			for i, evaluatorDO := range tt.doList {
				if i < len(result) {
					assert.Equal(t, evaluatorDO.ID, result[i].GetEvaluatorID())
					assert.Equal(t, evaluatorDO.Name, result[i].GetName())
				}
			}
		})
	}
}

// 重复函数已移除，避免重复定义

func TestConvertLanguageTypeDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		doLangType evaluatordo.LanguageType
		expected   evaluatordto.LanguageType
	}{
		{
			name:       "Python类型",
			doLangType: evaluatordo.LanguageTypePython,
			expected:   "Python",
		},
		{
			name:       "JS类型",
			doLangType: evaluatordo.LanguageTypeJS,
			expected:   "JS",
		},
		{
			name:       "自定义类型",
			doLangType: "CustomLang",
			expected:   "CustomLang",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := convertLanguageTypeDO2DTO(tt.doLangType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorDTO2DO_WithCurrentVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		evaluatorDTO *evaluatordto.Evaluator
		validate     func(t *testing.T, result *evaluatordo.Evaluator, err error)
	}{
		{
			name: "Prompt评估器带版本信息",
			evaluatorDTO: &evaluatordto.Evaluator{
				EvaluatorID:    gptr.Of(int64(123)),
				WorkspaceID:    gptr.Of(int64(456)),
				Name:           gptr.Of("Test Prompt Evaluator"),
				Description:    gptr.Of("Test description"),
				DraftSubmitted: gptr.Of(true),
				EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				LatestVersion:  gptr.Of("1"),
				CurrentVersion: &evaluatordto.EvaluatorVersion{
					ID:          gptr.Of(int64(789)),
					Version:     gptr.Of("1"),
					Description: gptr.Of("Version description"),
					EvaluatorContent: &evaluatordto.EvaluatorContent{
						ReceiveChatHistory: gptr.Of(true),
						PromptEvaluator: &evaluatordto.PromptEvaluator{
							PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType_BuiltinTemplate),
							PromptTemplateKey: gptr.Of("test_template"),
						},
					},
				},
			},
			validate: func(t *testing.T, result *evaluatordo.Evaluator, err error) {
				assert.NoError(t, err)
				assert.Equal(t, int64(123), result.ID)
				assert.Equal(t, evaluatordo.EvaluatorTypePrompt, result.EvaluatorType)
				assert.NotNil(t, result.PromptEvaluatorVersion)
				assert.Equal(t, int64(789), result.PromptEvaluatorVersion.ID)
				assert.Equal(t, "test_template", result.PromptEvaluatorVersion.PromptTemplateKey)
			},
		},
		{
			name: "Code评估器带版本信息",
			evaluatorDTO: &evaluatordto.Evaluator{
				EvaluatorID:    gptr.Of(int64(124)),
				WorkspaceID:    gptr.Of(int64(457)),
				Name:           gptr.Of("Test Code Evaluator"),
				Description:    gptr.Of("Code test description"),
				DraftSubmitted: gptr.Of(false),
				EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
				LatestVersion:  gptr.Of("2"),
				CurrentVersion: &evaluatordto.EvaluatorVersion{
					ID:          gptr.Of(int64(890)),
					Version:     gptr.Of("2"),
					Description: gptr.Of("Code version description"),
					EvaluatorContent: &evaluatordto.EvaluatorContent{
						CodeEvaluator: &evaluatordto.CodeEvaluator{
							CodeTemplateKey:  gptr.Of("test_code_template"),
							CodeTemplateName: gptr.Of("Test Code Template"),
							CodeContent:      gptr.Of("print('hello world')"),
							LanguageType:     gptr.Of(evaluatordto.LanguageType("python")),
						},
					},
				},
			},
			validate: func(t *testing.T, result *evaluatordo.Evaluator, err error) {
				assert.NoError(t, err)
				assert.Equal(t, int64(124), result.ID)
				assert.Equal(t, evaluatordo.EvaluatorTypeCode, result.EvaluatorType)
				assert.NotNil(t, result.CodeEvaluatorVersion)
				assert.Equal(t, int64(890), result.CodeEvaluatorVersion.ID)
				assert.Equal(t, "print('hello world')", result.CodeEvaluatorVersion.CodeContent)
				assert.Equal(t, evaluatordo.LanguageTypePython, result.CodeEvaluatorVersion.LanguageType)
			},
		},
		{
			name: "CustomRPC评估器带版本信息",
			evaluatorDTO: &evaluatordto.Evaluator{
				EvaluatorID:    gptr.Of(int64(123)),
				WorkspaceID:    gptr.Of(int64(456)),
				Name:           gptr.Of("Test Prompt Evaluator"),
				Description:    gptr.Of("Test description"),
				DraftSubmitted: gptr.Of(true),
				EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_CustomRPC),
				LatestVersion:  gptr.Of("1"),
				CurrentVersion: &evaluatordto.EvaluatorVersion{
					ID:          gptr.Of(int64(789)),
					Version:     gptr.Of("1"),
					Description: gptr.Of("Version description"),
					EvaluatorContent: &evaluatordto.EvaluatorContent{
						ReceiveChatHistory: gptr.Of(true),
						CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
							ProviderEvaluatorCode: gptr.Of("mock provider evaluator code"),
							AccessProtocol:        evaluatordto.EvaluatorAccessProtocolRPC,
							ServiceName:           gptr.Of("mock service name"),
							Cluster:               gptr.Of("mock cluster"),
							Timeout:               gptr.Of(int64(time.Second)),
							RateLimit: &commondto.RateLimit{
								Rate:   gptr.Of(int32(10)),
								Burst:  gptr.Of(int32(10)),
								Period: gptr.Of("1s"),
							},
						},
					},
				},
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{
					Benchmark:     gptr.Of("mock benchmark"),
					Vendor:        gptr.Of("mock vendor"),
					VendorURL:     gptr.Of("https://mock.vendor.url"),
					UserManualURL: gptr.Of("https://mock.user.manual.url"),
				},
			},
			validate: func(t *testing.T, result *evaluatordo.Evaluator, err error) {
				assert.NoError(t, err)
				assert.Equal(t, int64(123), result.ID)
				assert.Equal(t, evaluatordo.EvaluatorTypeCustomRPC, result.EvaluatorType)
				assert.NotNil(t, result.CustomRPCEvaluatorVersion)
				assert.Equal(t, int64(789), result.CustomRPCEvaluatorVersion.ID)
				assert.Equal(t, "mock provider evaluator code", *result.CustomRPCEvaluatorVersion.ProviderEvaluatorCode)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ConvertEvaluatorDTO2DO(tt.evaluatorDTO)

			if tt.validate != nil {
				tt.validate(t, result, err)
			}
		})
	}
}

func TestConvertEvaluatorDO2DTO_WithVersions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluatorDO *evaluatordo.Evaluator
		validate    func(t *testing.T, result *evaluatordto.Evaluator)
	}{
		{
			name: "Prompt评估器带版本信息",
			evaluatorDO: &evaluatordo.Evaluator{
				ID:             123,
				SpaceID:        456,
				Name:           "Test Prompt Evaluator",
				Description:    "Test description",
				DraftSubmitted: true,
				EvaluatorType:  evaluatordo.EvaluatorTypePrompt,
				LatestVersion:  "1",
				PromptEvaluatorVersion: &evaluatordo.PromptEvaluatorVersion{
					ID:                789,
					Version:           "1",
					Description:       "Version description",
					PromptSourceType:  evaluatordo.PromptSourceTypeBuiltinTemplate,
					PromptTemplateKey: "test_template",
				},
			},
			validate: func(t *testing.T, result *evaluatordto.Evaluator) {
				assert.Equal(t, int64(123), result.GetEvaluatorID())
				assert.Equal(t, evaluatordto.EvaluatorType_Prompt, result.GetEvaluatorType())
				assert.NotNil(t, result.CurrentVersion)
				assert.Equal(t, int64(789), result.CurrentVersion.GetID())
			},
		},
		{
			name: "Code评估器带版本信息",
			evaluatorDO: &evaluatordo.Evaluator{
				ID:             124,
				SpaceID:        457,
				Name:           "Test Code Evaluator",
				Description:    "Code test description",
				DraftSubmitted: false,
				EvaluatorType:  evaluatordo.EvaluatorTypeCode,
				LatestVersion:  "2",
				CodeEvaluatorVersion: &evaluatordo.CodeEvaluatorVersion{
					ID:               890,
					Version:          "2",
					Description:      "Code version description",
					CodeTemplateKey:  gptr.Of("test_code_template"),
					CodeTemplateName: gptr.Of("Test Code Template"),
					CodeContent:      "print('hello world')",
					LanguageType:     evaluatordo.LanguageTypePython,
				},
			},
			validate: func(t *testing.T, result *evaluatordto.Evaluator) {
				assert.Equal(t, int64(124), result.GetEvaluatorID())
				assert.Equal(t, evaluatordto.EvaluatorType_Code, result.GetEvaluatorType())
				assert.NotNil(t, result.CurrentVersion)
				assert.Equal(t, int64(890), result.CurrentVersion.GetID())
				assert.NotNil(t, result.CurrentVersion.EvaluatorContent.CodeEvaluator)
				assert.Equal(t, "print('hello world')", result.CurrentVersion.EvaluatorContent.CodeEvaluator.GetCodeContent())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDO2DTO(tt.evaluatorDO)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertCodeEvaluatorVersionDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluatorID int64
		spaceID     int64
		dto         *evaluatordto.EvaluatorVersion
		expected    *evaluatordo.CodeEvaluatorVersion
	}{
		{
			name:        "nil DTO",
			evaluatorID: 123,
			spaceID:     456,
			dto:         nil,
			expected:    nil,
		},
		{
			name:        "nil EvaluatorContent",
			evaluatorID: 123,
			spaceID:     456,
			dto: &evaluatordto.EvaluatorVersion{
				ID:               gptr.Of(int64(789)),
				Version:          gptr.Of("1.0"),
				Description:      gptr.Of("Test version"),
				EvaluatorContent: nil,
			},
			expected: nil,
		},
		{
			name:        "nil CodeEvaluator",
			evaluatorID: 123,
			spaceID:     456,
			dto: &evaluatordto.EvaluatorVersion{
				ID:          gptr.Of(int64(789)),
				Version:     gptr.Of("1.0"),
				Description: gptr.Of("Test version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: nil,
				},
			},
			expected: nil,
		},
		{
			name:        "valid CodeEvaluator",
			evaluatorID: 123,
			spaceID:     456,
			dto: &evaluatordto.EvaluatorVersion{
				ID:          gptr.Of(int64(789)),
				Version:     gptr.Of("1.0"),
				Description: gptr.Of("Test version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeTemplateKey:  gptr.Of("test_template"),
						CodeTemplateName: gptr.Of("Test Template"),
						CodeContent:      gptr.Of("print('test')"),
						LanguageType:     gptr.Of(evaluatordto.LanguageType("Python")),
					},
				},
			},
			expected: &evaluatordo.CodeEvaluatorVersion{
				ID:               789,
				SpaceID:          456,
				EvaluatorType:    evaluatordo.EvaluatorTypeCode,
				EvaluatorID:      123,
				Description:      "Test version",
				Version:          "1.0",
				CodeTemplateKey:  gptr.Of("test_template"),
				CodeTemplateName: gptr.Of("Test Template"),
				CodeContent:      "print('test')",
				LanguageType:     evaluatordo.LanguageTypePython,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertCodeEvaluatorVersionDTO2DO(tt.evaluatorID, tt.spaceID, tt.dto)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.EvaluatorID, result.EvaluatorID)
				assert.Equal(t, tt.expected.LanguageType, result.LanguageType)
			}
		})
	}
}

func TestConvertCodeEvaluatorVersionDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		do       *evaluatordo.CodeEvaluatorVersion
		expected *evaluatordto.EvaluatorVersion
	}{
		{
			name:     "nil DO",
			do:       nil,
			expected: nil,
		},
		{
			name: "valid DO",
			do: &evaluatordo.CodeEvaluatorVersion{
				ID:               789,
				Version:          "1.0",
				Description:      "Test version",
				CodeTemplateKey:  gptr.Of("test_template"),
				CodeTemplateName: gptr.Of("Test Template"),
				CodeContent:      "print('test')",
				LanguageType:     evaluatordo.LanguageTypePython,
			},
			expected: &evaluatordto.EvaluatorVersion{
				ID:          gptr.Of(int64(789)),
				Version:     gptr.Of("1.0"),
				Description: gptr.Of("Test version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CodeEvaluator: &evaluatordto.CodeEvaluator{
						CodeTemplateKey:  gptr.Of("test_template"),
						CodeTemplateName: gptr.Of("Test Template"),
						CodeContent:      gptr.Of("print('test')"),
						LanguageType:     gptr.Of(evaluatordto.LanguageType("Python")),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertCodeEvaluatorVersionDO2DTO(tt.do)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.GetID(), result.GetID())
				assert.Equal(t, tt.expected.GetVersion(), result.GetVersion())
				assert.NotNil(t, result.EvaluatorContent.CodeEvaluator)
			}
		})
	}
}

// 删除重复的表驱动复杂用例，保留上方的简化覆盖

// Test additional functions to improve coverage
func TestConvertEvaluatorDTO2DO_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		evaluatorDTO *evaluatordto.Evaluator
		validate     func(t *testing.T, result *evaluatordo.Evaluator, err error)
	}{
		{
			name: "evaluator without current version",
			evaluatorDTO: &evaluatordto.Evaluator{
				EvaluatorID:    gptr.Of(int64(123)),
				WorkspaceID:    gptr.Of(int64(456)),
				Name:           gptr.Of("Test Evaluator"),
				EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				CurrentVersion: nil,
			},
			validate: func(t *testing.T, result *evaluatordo.Evaluator, err error) {
				assert.NoError(t, err)
				assert.Equal(t, int64(123), result.ID)
				assert.Nil(t, result.PromptEvaluatorVersion)
				assert.Nil(t, result.CodeEvaluatorVersion)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ConvertEvaluatorDTO2DO(tt.evaluatorDTO)

			if tt.validate != nil {
				tt.validate(t, result, err)
			}
		})
	}
}

func TestConvertEvaluatorDO2DTO_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluatorDO *evaluatordo.Evaluator
		validate    func(t *testing.T, result *evaluatordto.Evaluator)
	}{
		{
			name: "evaluator with unknown type",
			evaluatorDO: &evaluatordo.Evaluator{
				ID:            123,
				SpaceID:       456,
				Name:          "Test Evaluator",
				EvaluatorType: evaluatordo.EvaluatorType(999), // Unknown type
			},
			validate: func(t *testing.T, result *evaluatordto.Evaluator) {
				assert.Equal(t, int64(123), result.GetEvaluatorID())
				assert.Nil(t, result.CurrentVersion)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDO2DTO(tt.evaluatorDO)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertPromptEvaluatorVersionDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluatorID int64
		spaceID     int64
		dto         *evaluatordto.EvaluatorVersion
		validate    func(t *testing.T, result *evaluatordo.PromptEvaluatorVersion)
	}{
		{
			name:        "basic conversion",
			evaluatorID: 123,
			spaceID:     456,
			dto: &evaluatordto.EvaluatorVersion{
				ID:          gptr.Of(int64(789)),
				Version:     gptr.Of("1.0"),
				Description: gptr.Of("Test version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					ReceiveChatHistory: gptr.Of(true),
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType_BuiltinTemplate),
						PromptTemplateKey: gptr.Of("test_template"),
					},
				},
			},
			validate: func(t *testing.T, result *evaluatordo.PromptEvaluatorVersion) {
				assert.Equal(t, int64(789), result.ID)
				assert.Equal(t, int64(123), result.EvaluatorID)
				assert.Equal(t, int64(456), result.SpaceID)
				assert.Equal(t, "test_template", result.PromptTemplateKey)
				assert.Equal(t, gptr.Of(true), result.ReceiveChatHistory)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertPromptEvaluatorVersionDTO2DO(tt.evaluatorID, tt.spaceID, tt.dto)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertPromptEvaluatorVersionDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		do       *evaluatordo.PromptEvaluatorVersion
		expected *evaluatordto.EvaluatorVersion
	}{
		{
			name:     "nil DO",
			do:       nil,
			expected: nil,
		},
		{
			name: "valid DO",
			do: &evaluatordo.PromptEvaluatorVersion{
				ID:                 789,
				Version:            "1.0",
				Description:        "Test version",
				PromptSourceType:   evaluatordo.PromptSourceTypeBuiltinTemplate,
				PromptTemplateKey:  "test_template",
				ReceiveChatHistory: gptr.Of(true),
			},
			expected: &evaluatordto.EvaluatorVersion{
				ID:          gptr.Of(int64(789)),
				Version:     gptr.Of("1.0"),
				Description: gptr.Of("Test version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					ReceiveChatHistory: gptr.Of(true),
					PromptEvaluator: &evaluatordto.PromptEvaluator{
						PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType_BuiltinTemplate),
						PromptTemplateKey: gptr.Of("test_template"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertPromptEvaluatorVersionDO2DTO(tt.do)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.GetID(), result.GetID())
				assert.Equal(t, tt.expected.GetVersion(), result.GetVersion())
			}
		})
	}
}

func TestConvertEvaluatorTagsDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		dtoTags  map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string
		expected map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string
	}{
		{
			name: "正常转换",
			dtoTags: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{
				evaluatordto.EvaluatorTagLangTypeEn: {
					evaluatordto.EvaluatorTagKeyCategory:         {"LLM", "Code"},
					evaluatordto.EvaluatorTagKeyObjective:        {"Quality"},
					evaluatordto.EvaluatorTagKeyBusinessScenario: {"AI Coding"},
				},
			},
			expected: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{
				evaluatordo.EvaluatorTagLangType_En: {
					evaluatordo.EvaluatorTagKey_Category:         {"LLM", "Code"},
					evaluatordo.EvaluatorTagKey_Objective:        {"Quality"},
					evaluatordo.EvaluatorTagKey_BusinessScenario: {"AI Coding"},
				},
			},
		},
		{
			name:     "空Tags",
			dtoTags:  nil,
			expected: nil,
		},
		{
			name:     "空map",
			dtoTags:  map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{},
			expected: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertEvaluatorLangTagsDTO2DO(tt.dtoTags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorTagsDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		doTags   map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string
		expected map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string
	}{
		{
			name: "正常转换",
			doTags: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{
				evaluatordo.EvaluatorTagLangType_En: {
					evaluatordo.EvaluatorTagKey_Category:         {"LLM", "Code"},
					evaluatordo.EvaluatorTagKey_Objective:        {"Quality"},
					evaluatordo.EvaluatorTagKey_BusinessScenario: {"AI Coding"},
				},
			},
			expected: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{
				evaluatordto.EvaluatorTagLangTypeEn: {
					evaluatordto.EvaluatorTagKeyCategory:         {"LLM", "Code"},
					evaluatordto.EvaluatorTagKeyObjective:        {"Quality"},
					evaluatordto.EvaluatorTagKeyBusinessScenario: {"AI Coding"},
				},
			},
		},
		{
			name:     "空Tags",
			doTags:   nil,
			expected: nil,
		},
		{
			name:     "空map",
			doTags:   map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{},
			expected: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertEvaluatorLangTagsDO2DTO(tt.doTags)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorTagKeyDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		doKey    evaluatordo.EvaluatorTagKey
		expected evaluatordto.EvaluatorTagKey
	}{
		{
			name:     "Category",
			doKey:    evaluatordo.EvaluatorTagKey_Category,
			expected: evaluatordto.EvaluatorTagKeyCategory,
		},
		{
			name:     "TargetType",
			doKey:    evaluatordo.EvaluatorTagKey_TargetType,
			expected: evaluatordto.EvaluatorTagKeyTargetType,
		},
		{
			name:     "Objective",
			doKey:    evaluatordo.EvaluatorTagKey_Objective,
			expected: evaluatordto.EvaluatorTagKeyObjective,
		},
		{
			name:     "BusinessScenario",
			doKey:    evaluatordo.EvaluatorTagKey_BusinessScenario,
			expected: evaluatordto.EvaluatorTagKeyBusinessScenario,
		},
		{
			name:     "BoxType",
			doKey:    evaluatordo.EvaluatorTagKey_BoxType,
			expected: "BoxType",
		},
		{
			name:     "Name",
			doKey:    evaluatordo.EvaluatorTagKey_Name,
			expected: evaluatordto.EvaluatorTagKeyName,
		},
		{
			name:     "未知类型",
			doKey:    evaluatordo.EvaluatorTagKey("Unknown"),
			expected: evaluatordto.EvaluatorTagKey("Unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertEvaluatorTagKeyDO2DTO(tt.doKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// 新增：覆盖 CodeEvaluatorContent 的 lang_2_code_content 转换路径（DTO -> DO）
func TestConvertCodeEvaluatorContentDTO2DO_OldFieldsToMap(t *testing.T) {
	t.Parallel()

	dto := &evaluatordto.CodeEvaluator{
		LanguageType: gptr.Of(evaluatordto.LanguageType("Python")),
		CodeContent:  gptr.Of("print('old')"),
	}
	// 使用旧字段，期望转换为单元素 Lang2CodeContent

	do := ConvertCodeEvaluatorContentDTO2DO(dto)
	// 期望将 map 完整落入 DO
	assert.NotNil(t, do)
	assert.NotNil(t, do.Lang2CodeContent)
	assert.Equal(t, "print('old')", do.Lang2CodeContent[evaluatordo.LanguageType("Python")])
}

// 新增：覆盖 CodeEvaluatorContent 的 lang_2_code_content 转换路径（DO -> DTO）
func TestConvertCodeEvaluatorContentDO2DTO_Lang2(t *testing.T) {
	t.Parallel()

	do := &evaluatordo.CodeEvaluatorContent{
		Lang2CodeContent: map[evaluatordo.LanguageType]string{
			evaluatordo.LanguageType("Python"): "print('py')",
		},
	}

	dto := ConvertCodeEvaluatorContentDO2DTO(do)
	assert.NotNil(t, dto)
	// 兼容旧字段：从 map 回填一个 language_type/code_content
	assert.Equal(t, "Python", dto.GetLanguageType())
	assert.Equal(t, "print('py')", dto.GetCodeContent())
	// 不校验新字段（兼容老字段即可）
}

// 新增：覆盖 CodeEvaluatorVersion 的 DTO -> DO（优先根据 language_type 命中 lang_2_code_content）
func TestConvertCodeEvaluatorVersionDTO2DO_Lang2_PickByLanguageType(t *testing.T) {
	t.Parallel()

	ev := &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(int64(100)),
		Version:     gptr.Of("1.0.0"),
		Description: gptr.Of("desc"),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			CodeEvaluator: &evaluatordto.CodeEvaluator{
				LanguageType:     gptr.Of(evaluatordto.LanguageType("JS")),
				CodeTemplateKey:  gptr.Of("tpl-1"),
				CodeTemplateName: gptr.Of("TPL1"),
			},
		},
	}
	// 不使用新字段，使用旧字段验证兼容路径
	ev.EvaluatorContent.CodeEvaluator.CodeContent = gptr.Of("console.log('js')")

	do := ConvertCodeEvaluatorVersionDTO2DO(1, 2, ev)
	assert.NotNil(t, do)
	assert.Equal(t, int64(1), do.EvaluatorID)
	assert.Equal(t, int64(2), do.SpaceID)
	// 根据 language_type=JS 命中 map
	assert.Equal(t, "console.log('js')", do.CodeContent)
	assert.Equal(t, evaluatordo.LanguageType("JS"), do.LanguageType)
}

// 新增：覆盖 CodeEvaluatorVersion 的 DTO -> DO（未给 language_type 时取第一个）
func TestConvertCodeEvaluatorVersionDTO2DO_Lang2_PickFirst(t *testing.T) {
	t.Parallel()

	ev := &evaluatordto.EvaluatorVersion{
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			CodeEvaluator: &evaluatordto.CodeEvaluator{},
		},
	}
	// 不使用新字段，使用旧字段验证兼容路径
	ev.EvaluatorContent.CodeEvaluator.LanguageType = gptr.Of(evaluatordto.LanguageType("Python"))
	ev.EvaluatorContent.CodeEvaluator.CodeContent = gptr.Of("print('py')")

	do := ConvertCodeEvaluatorVersionDTO2DO(1, 2, ev)
	assert.NotNil(t, do)
	assert.Equal(t, "print('py')", do.CodeContent)
	assert.Equal(t, evaluatordo.LanguageType("Python"), do.LanguageType)
}

// 新增：覆盖 ConvertEvaluatorContent2DO 的 Code 分支（优先 lang_2_code_content）
func TestConvertEvaluatorContent2DO_Code_Lang2(t *testing.T) {
	t.Parallel()
	content := &evaluatordto.EvaluatorContent{
		CodeEvaluator: &evaluatordto.CodeEvaluator{
			LanguageType: gptr.Of(evaluatordto.LanguageType("Python")),
		},
	}
	// 不使用新字段，使用旧字段验证兼容路径
	content.CodeEvaluator.CodeContent = gptr.Of("print('py')")

	do, err := ConvertEvaluatorContent2DO(content, evaluatordto.EvaluatorType_Code)
	assert.NoError(t, err)
	assert.NotNil(t, do)
	if do.CodeEvaluatorVersion == nil {
		t.Fatalf("expected CodeEvaluatorVersion not nil")
	}
	assert.Equal(t, "print('py')", do.CodeEvaluatorVersion.CodeContent)
	assert.Equal(t, evaluatordo.LanguageType("Python"), do.CodeEvaluatorVersion.LanguageType)
}

// TestConvertCustomRPCEvaluatorVersionDTO2DO 测试将 CustomRPC EvaluatorVersion DTO 转换为 DO
func TestConvertCustomRPCEvaluatorVersionDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		evaluatorID int64
		spaceID     int64
		dto         *evaluatordto.EvaluatorVersion
		validate    func(t *testing.T, result *evaluatordo.CustomRPCEvaluatorVersion, err error)
		description string
	}{
		{
			name:        "nil输入",
			evaluatorID: 123,
			spaceID:     456,
			dto:         nil,
			validate: func(t *testing.T, result *evaluatordo.CustomRPCEvaluatorVersion, err error) {
				assert.Nil(t, result)
				assert.NoError(t, err)
			},
			description: "nil输入应该返回nil",
		},
		{
			name:        "成功 - 基本转换",
			evaluatorID: 123,
			spaceID:     456,
			dto: &evaluatordto.EvaluatorVersion{
				ID:          gptr.Of(int64(789)),
				Version:     gptr.Of("1.0.0"),
				Description: gptr.Of("Test CustomRPC version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{
					CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
						ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
						AccessProtocol:        evaluatordto.EvaluatorAccessProtocol("rpc"),
						ServiceName:           gptr.Of("test_service"),
						Cluster:               gptr.Of("test_cluster"),
						Timeout:               gptr.Of(int64(5000)),
					},
					InputSchemas: []*commondto.ArgsSchema{
						{
							Key:                 gptr.Of("input1"),
							SupportContentTypes: []commondto.ContentType{"Text"},
							JSONSchema:          gptr.Of(`{"type": "string"}`),
						},
					},
					OutputSchemas: []*commondto.ArgsSchema{
						{
							Key:                 gptr.Of("output1"),
							SupportContentTypes: []commondto.ContentType{"Text"},
							JSONSchema:          gptr.Of(`{"type": "string"}`),
						},
					},
				},
			},
			validate: func(t *testing.T, result *evaluatordo.CustomRPCEvaluatorVersion, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, int64(789), result.ID)
				assert.Equal(t, int64(123), result.EvaluatorID)
				assert.Equal(t, int64(456), result.SpaceID)
				assert.Equal(t, "1.0.0", result.Version)
				assert.Equal(t, "Test CustomRPC version", result.Description)
				assert.Equal(t, evaluatordo.EvaluatorTypeCustomRPC, result.EvaluatorType)
				assert.NotNil(t, result.ProviderEvaluatorCode)
				assert.Equal(t, "PROVIDER_001", *result.ProviderEvaluatorCode)
				assert.Equal(t, evaluatordo.EvaluatorAccessProtocol("rpc"), result.AccessProtocol)
				assert.NotNil(t, result.ServiceName)
				assert.Equal(t, "test_service", *result.ServiceName)
				assert.NotNil(t, result.Cluster)
				assert.Equal(t, "test_cluster", *result.Cluster)
				assert.NotNil(t, result.Timeout)
				assert.Equal(t, int64(5000), *result.Timeout)
				assert.NotNil(t, result.InputSchemas)
				assert.Len(t, result.InputSchemas, 1)
				assert.NotNil(t, result.OutputSchemas)
				assert.Len(t, result.OutputSchemas, 1)
			},
			description: "成功转换CustomRPC评估器版本",
		},
		{
			name:        "成功 - 空EvaluatorContent",
			evaluatorID: 123,
			spaceID:     456,
			dto: &evaluatordto.EvaluatorVersion{
				ID:               gptr.Of(int64(789)),
				Version:          gptr.Of("1.0.0"),
				Description:      gptr.Of("Test version"),
				EvaluatorContent: &evaluatordto.EvaluatorContent{},
			},
			validate: func(t *testing.T, result *evaluatordo.CustomRPCEvaluatorVersion, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, int64(789), result.ID)
				assert.Nil(t, result.InputSchemas)
				assert.Nil(t, result.OutputSchemas)
			},
			description: "成功转换空EvaluatorContent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ConvertCustomRPCEvaluatorVersionDTO2DO(tt.evaluatorID, tt.spaceID, tt.dto)

			if tt.validate != nil {
				tt.validate(t, result, err)
			}
		})
	}
}

// TestConvertCustomRPCEvaluatorVersionDO2DTO 测试将 CustomRPC EvaluatorVersion DO 转换为 DTO
func TestConvertCustomRPCEvaluatorVersionDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		do          *evaluatordo.CustomRPCEvaluatorVersion
		validate    func(t *testing.T, result *evaluatordto.EvaluatorVersion)
		description string
	}{
		{
			name: "nil输入",
			do:   nil,
			validate: func(t *testing.T, result *evaluatordto.EvaluatorVersion) {
				assert.Nil(t, result)
			},
			description: "nil输入应该返回nil",
		},
		{
			name: "成功 - 完整转换",
			do: &evaluatordo.CustomRPCEvaluatorVersion{
				ID:                    789,
				EvaluatorID:           123,
				SpaceID:               456,
				Version:               "1.0.0",
				Description:           "Test CustomRPC version",
				EvaluatorType:         evaluatordo.EvaluatorTypeCustomRPC,
				ProviderEvaluatorCode: gptr.Of("PROVIDER_001"),
				AccessProtocol:        evaluatordo.EvaluatorAccessProtocol("rpc"),
				ServiceName:           gptr.Of("test_service"),
				Cluster:               gptr.Of("test_cluster"),
				Timeout:               gptr.Of(int64(5000)),
				InputSchemas: []*evaluatordo.ArgsSchema{
					{
						Key:                 gptr.Of("input1"),
						SupportContentTypes: []evaluatordo.ContentType{evaluatordo.ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
				OutputSchemas: []*evaluatordo.ArgsSchema{
					{
						Key:                 gptr.Of("output1"),
						SupportContentTypes: []evaluatordo.ContentType{evaluatordo.ContentTypeText},
						JsonSchema:          gptr.Of(`{"type": "string"}`),
					},
				},
			},
			validate: func(t *testing.T, result *evaluatordto.EvaluatorVersion) {
				assert.NotNil(t, result)
				assert.Equal(t, int64(789), result.GetID())
				assert.Equal(t, "1.0.0", result.GetVersion())
				assert.Equal(t, "Test CustomRPC version", result.GetDescription())
				assert.NotNil(t, result.EvaluatorContent)
				assert.NotNil(t, result.EvaluatorContent.CustomRPCEvaluator)
				assert.Equal(t, "PROVIDER_001", *result.EvaluatorContent.CustomRPCEvaluator.ProviderEvaluatorCode)
				assert.Equal(t, evaluatordto.EvaluatorAccessProtocol("rpc"), result.EvaluatorContent.CustomRPCEvaluator.AccessProtocol)
				assert.Equal(t, "test_service", *result.EvaluatorContent.CustomRPCEvaluator.ServiceName)
				assert.Equal(t, "test_cluster", *result.EvaluatorContent.CustomRPCEvaluator.Cluster)
				assert.Equal(t, int64(5000), *result.EvaluatorContent.CustomRPCEvaluator.Timeout)
				assert.NotNil(t, result.EvaluatorContent.InputSchemas)
				assert.Len(t, result.EvaluatorContent.InputSchemas, 1)
				assert.NotNil(t, result.EvaluatorContent.OutputSchemas)
				assert.Len(t, result.EvaluatorContent.OutputSchemas, 1)
			},
			description: "成功转换CustomRPC评估器版本DO为DTO",
		},
		{
			name: "成功 - 空字段",
			do: &evaluatordo.CustomRPCEvaluatorVersion{
				ID:            789,
				EvaluatorID:   123,
				SpaceID:       456,
				Version:       "1.0.0",
				Description:   "",
				EvaluatorType: evaluatordo.EvaluatorTypeCustomRPC,
			},
			validate: func(t *testing.T, result *evaluatordto.EvaluatorVersion) {
				assert.NotNil(t, result)
				assert.Equal(t, "", result.GetDescription())
				assert.Nil(t, result.EvaluatorContent.InputSchemas)
				assert.Nil(t, result.EvaluatorContent.OutputSchemas)
			},
			description: "成功转换空字段的CustomRPC版本",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertCustomRPCEvaluatorVersionDO2DTO(tt.do)

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestConvertEvaluatorHTTPInfoDTO2DO(t *testing.T) {
	t.Parallel()

	// nil 输入
	assert.Nil(t, ConvertEvaluatorHTTPInfoDTO2DO(nil))

	// 正常输入
	method := evaluatordto.EvaluatorHTTPMethod("post")
	path := gptr.Of("/v1/eval")
	dto := &evaluatordto.EvaluatorHTTPInfo{Method: &method, Path: path}

	do := ConvertEvaluatorHTTPInfoDTO2DO(dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, method, gptr.Indirect(do.Method))
		assert.Equal(t, "/v1/eval", gptr.Indirect(do.Path))
	}
}

func TestConvertEvaluatorHTTPInfoDO2DTO(t *testing.T) {
	t.Parallel()

	// nil 输入
	assert.Nil(t, ConvertEvaluatorHTTPInfoDO2DTO(nil))

	// 正常输入
	method := evaluatordo.EvaluatorHTTPMethod("get")
	path := gptr.Of("/ping")
	do := &evaluatordo.EvaluatorHTTPInfo{Method: &method, Path: path}

	dto := ConvertEvaluatorHTTPInfoDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, method, gptr.Indirect(dto.Method))
		assert.Equal(t, "/ping", gptr.Indirect(dto.Path))
	}
}

func TestConvertEvaluatorRunConfDTO2DO(t *testing.T) {
	t.Parallel()

	// nil 输入
	assert.Nil(t, ConvertEvaluatorRunConfDTO2DO(nil))

	// 正常输入：包含 Env 与 RuntimeParam.JSONValue
	env := gptr.Of("prod")
	rp := &commondto.RuntimeParam{JSONValue: gptr.Of(`{"model_config":{"model_id":"m-1"}}`)}
	dto := &evaluatordto.EvaluatorRunConfig{Env: env, EvaluatorRuntimeParam: rp}

	do := ConvertEvaluatorRunConfDTO2DO(dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, "prod", gptr.Indirect(do.Env))
		if assert.NotNil(t, do.EvaluatorRuntimeParam) {
			assert.Equal(t, `{"model_config":{"model_id":"m-1"}}`, gptr.Indirect(do.EvaluatorRuntimeParam.JSONValue))
		}
	}
}

func TestConvertEvaluatorRunConfDO2DTO(t *testing.T) {
	t.Parallel()

	// nil 输入
	assert.Nil(t, ConvertEvaluatorRunConfDO2DTO(nil))

	// 正常输入：包含 Env 与 RuntimeParam.JSONValue
	env := gptr.Of("staging")
	rp := &evaluatordo.RuntimeParam{JSONValue: gptr.Of(`{"model_config":{"temperature":0.5}}`)}
	do := &evaluatordo.EvaluatorRunConfig{Env: env, EvaluatorRuntimeParam: rp}

	dto := ConvertEvaluatorRunConfDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, "staging", gptr.Indirect(dto.Env))
		if assert.NotNil(t, dto.EvaluatorRuntimeParam) {
			assert.Equal(t, `{"model_config":{"temperature":0.5}}`, gptr.Indirect(dto.EvaluatorRuntimeParam.JSONValue))
		}
	}
}
