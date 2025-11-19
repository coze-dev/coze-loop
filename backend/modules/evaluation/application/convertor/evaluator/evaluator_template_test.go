// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertEvaluatorTemplateDTO2DO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorTemplateDTO2DO(nil))
}

func TestConvertEvaluatorTemplateDTO2DO_BasicAndTagsAndInfo(t *testing.T) {
	t.Parallel()
	dto := &evaluatordto.EvaluatorTemplate{
		ID:            gptr.Of(int64(123)),
		WorkspaceID:   gptr.Of(int64(456)),
		Name:          gptr.Of("name"),
		Description:   gptr.Of("desc"),
		EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
		Popularity:    gptr.Of(int64(9)),
		BaseInfo:      &commondto.BaseInfo{CreatedBy: &commondto.UserInfo{UserID: gptr.Of("u1")}},
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			ReceiveChatHistory: gptr.Of(true),
			InputSchemas:       []*commondto.ArgsSchema{{Key: gptr.Of("in")}},
			OutputSchemas:      []*commondto.ArgsSchema{{Key: gptr.Of("out")}},
			PromptEvaluator:    &evaluatordto.PromptEvaluator{MessageList: []*commondto.Message{{Content: &commondto.Content{Text: gptr.Of("t")}}}},
		},
		Tags: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{
			evaluatordto.EvaluatorTagLangType("zh"): {
				evaluatordto.EvaluatorTagKeyName: {"n1", "n2"},
			},
		},
	}
	do := ConvertEvaluatorTemplateDTO2DO(dto)
	assert.NotNil(t, do)
	assert.Equal(t, int64(123), do.ID)
	assert.Equal(t, int64(456), do.SpaceID)
	assert.Equal(t, "name", do.Name)
	assert.Equal(t, "desc", do.Description)
	assert.Equal(t, evaluatordo.EvaluatorTypePrompt, do.EvaluatorType)
	assert.Equal(t, int64(9), do.Popularity)
	// EvaluatorInfo 字段在模板DTO可能不存在，忽略该字段校验
	assert.NotNil(t, do.BaseInfo)
	assert.True(t, gptr.Indirect(do.ReceiveChatHistory))
	assert.Len(t, do.InputSchemas, 1)
	assert.Len(t, do.OutputSchemas, 1)
	if assert.NotNil(t, do.PromptEvaluatorContent) {
		assert.Len(t, do.PromptEvaluatorContent.MessageList, 1)
	}
	if assert.NotNil(t, do.Tags) {
		assert.Equal(t, []string{"n1", "n2"}, do.Tags[evaluatordo.EvaluatorTagLangType("zh")][evaluatordo.EvaluatorTagKey("Name")])
	}
}

func TestConvertEvaluatorTemplateDTO2DO_CodeEval_NewAndCompat(t *testing.T) {
	t.Parallel()
	// 新字段：lang_2_code_content
	dtoNew := &evaluatordto.EvaluatorTemplate{
		EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			CodeEvaluator: &evaluatordto.CodeEvaluator{},
		},
	}
	dtoNew.EvaluatorContent.CodeEvaluator.SetLang2CodeContent(map[evaluatordto.LanguageType]string{
		evaluatordto.LanguageTypePython: "print('hi')",
	})
	doNew := ConvertEvaluatorTemplateDTO2DO(dtoNew)
	if assert.NotNil(t, doNew) && assert.NotNil(t, doNew.CodeEvaluatorContent) {
		assert.Equal(t, "print('hi')", doNew.CodeEvaluatorContent.Lang2CodeContent[evaluatordo.LanguageTypePython])
	}

	// 兼容旧字段：language_type + code_content
	dtoOld := &evaluatordto.EvaluatorTemplate{
		EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			CodeEvaluator: &evaluatordto.CodeEvaluator{LanguageType: gptr.Of(evaluatordto.LanguageTypePython), CodeContent: gptr.Of("print('ok')")},
		},
	}
	doOld := ConvertEvaluatorTemplateDTO2DO(dtoOld)
	if assert.NotNil(t, doOld) && assert.NotNil(t, doOld.CodeEvaluatorContent) {
		assert.Equal(t, "print('ok')", doOld.CodeEvaluatorContent.Lang2CodeContent[evaluatordo.LanguageTypePython])
	}
}

func TestConvertEvaluatorTemplateDO2DTO_Nil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, ConvertEvaluatorTemplateDO2DTO(nil))
}

func TestConvertEvaluatorTemplateDO2DTO_Full(t *testing.T) {
	t.Parallel()
	do := &evaluatordo.EvaluatorTemplate{
		ID:                     1,
		SpaceID:                2,
		Name:                   "n",
		Description:            "d",
		EvaluatorType:          evaluatordo.EvaluatorTypePrompt,
		Popularity:             3,
		BaseInfo:               &evaluatordo.BaseInfo{},
		InputSchemas:           []*evaluatordo.ArgsSchema{{Key: gptr.Of("in")}},
		OutputSchemas:          []*evaluatordo.ArgsSchema{{Key: gptr.Of("out")}},
		ReceiveChatHistory:     gptr.Of(true),
		PromptEvaluatorContent: &evaluatordo.PromptEvaluatorContent{MessageList: []*evaluatordo.Message{{Content: &evaluatordo.Content{Text: gptr.Of("t")}}}},
		Tags: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{
			evaluatordo.EvaluatorTagLangType("en"): {evaluatordo.EvaluatorTagKey("Name"): {"x"}},
		},
	}
	dto := ConvertEvaluatorTemplateDO2DTO(do)
	if assert.NotNil(t, dto) {
		assert.Equal(t, int64(1), dto.GetID())
		assert.Equal(t, int64(2), dto.GetWorkspaceID())
		assert.Equal(t, "n", dto.GetName())
		assert.Equal(t, "d", dto.GetDescription())
		assert.Equal(t, evaluatordto.EvaluatorType_Prompt, dto.GetEvaluatorType())
		assert.Equal(t, int64(3), dto.GetPopularity())
		// EvaluatorInfo 字段在模板DTO可能不存在，忽略该字段校验
		if assert.NotNil(t, dto.EvaluatorContent) {
			assert.True(t, gptr.Indirect(dto.EvaluatorContent.ReceiveChatHistory))
			assert.Len(t, dto.EvaluatorContent.InputSchemas, 1)
			assert.Len(t, dto.EvaluatorContent.OutputSchemas, 1)
			if assert.NotNil(t, dto.EvaluatorContent.PromptEvaluator) {
				assert.Len(t, dto.EvaluatorContent.PromptEvaluator.MessageList, 1)
			}
		}
		assert.Equal(t, []string{"x"}, dto.Tags[evaluatordto.EvaluatorTagLangType("en")][evaluatordto.EvaluatorTagKey("Name")])
	}
}

func TestConvertEvaluatorTemplateDOList2DTO(t *testing.T) {
	t.Parallel()
	doList := []*evaluatordo.EvaluatorTemplate{{Name: "a"}, {Name: "b"}}
	dtoList := ConvertEvaluatorTemplateDOList2DTO(doList)
	assert.Len(t, dtoList, 2)
	assert.Equal(t, "a", dtoList[0].GetName())
	assert.Equal(t, "b", dtoList[1].GetName())
}

func TestCodeEvaluatorContentDTOAndDO(t *testing.T) {
	t.Parallel()
	// DTO2DO 新字段
	dto := &evaluatordto.CodeEvaluator{}
	dto.SetLang2CodeContent(map[evaluatordto.LanguageType]string{
		evaluatordto.LanguageTypePython: "print(1)",
	})
	do := ConvertCodeEvaluatorContentDTO2DO(dto)
	if assert.NotNil(t, do) {
		assert.Equal(t, "print(1)", do.Lang2CodeContent[evaluatordo.LanguageTypePython])
	}

	// DTO2DO 旧字段
	dto2 := &evaluatordto.CodeEvaluator{LanguageType: gptr.Of(evaluatordto.LanguageTypeJS), CodeContent: gptr.Of("console.log(1)")}
	do2 := ConvertCodeEvaluatorContentDTO2DO(dto2)
	if assert.NotNil(t, do2) {
		assert.Equal(t, "console.log(1)", do2.Lang2CodeContent[evaluatordo.LanguageTypeJS])
	}

	// DO2DTO
	back := ConvertCodeEvaluatorContentDO2DTO(&evaluatordo.CodeEvaluatorContent{Lang2CodeContent: map[evaluatordo.LanguageType]string{
		evaluatordo.LanguageTypePython: "print(2)",
	}})
	if assert.NotNil(t, back) {
		m := back.GetLang2CodeContent()
		assert.Equal(t, "print(2)", m[evaluatordto.LanguageTypePython])
		// 回填旧字段
		assert.NotNil(t, back.LanguageType)
		assert.NotNil(t, back.CodeContent)
	}
}

// TestConvertEvaluatorTemplateDTO2DO_EvaluatorInfo 测试 ConvertEvaluatorTemplateDTO2DO 的 EvaluatorInfo 处理逻辑
func TestConvertEvaluatorTemplateDTO2DO_EvaluatorInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dto          *evaluatordto.EvaluatorTemplate
		expectedInfo *evaluatordo.EvaluatorInfo
		description  string
	}{
		{
			name: "成功 - EvaluatorInfo为nil",
			dto: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(0)),
			},
			expectedInfo: nil,
			description:  "当DTO的EvaluatorInfo为nil时，DO的EvaluatorInfo应该为nil",
		},
		{
			name: "成功 - EvaluatorInfo存在但所有字段为nil",
			dto: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(0)),
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{},
			},
			expectedInfo: &evaluatordo.EvaluatorInfo{
				Benchmark:     nil,
				Vendor:        nil,
				VendorURL:     nil,
				UserManualURL: nil,
			},
			description: "当DTO的EvaluatorInfo存在但所有字段为nil时，DO的EvaluatorInfo应该创建但所有字段为空字符串",
		},
		{
			name: "成功 - EvaluatorInfo存在且所有字段都有值",
			dto: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(0)),
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{
					Benchmark:     gptr.Of("GLUE"),
					Vendor:        gptr.Of("OpenAI"),
					VendorURL:     gptr.Of("https://openai.com"),
					UserManualURL: gptr.Of("https://docs.openai.com"),
				},
			},
			expectedInfo: &evaluatordo.EvaluatorInfo{
				Benchmark:     gptr.Of("GLUE"),
				Vendor:        gptr.Of("OpenAI"),
				VendorURL:     gptr.Of("https://openai.com"),
				UserManualURL: gptr.Of("https://docs.openai.com"),
			},
			description: "当DTO的EvaluatorInfo存在且所有字段都有值时，DO的EvaluatorInfo应该正确转换所有字段",
		},
		{
			name: "成功 - EvaluatorInfo存在但部分字段有值",
			dto: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(0)),
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{
					Benchmark: gptr.Of("GLUE"),
					Vendor:    gptr.Of("OpenAI"),
					// VendorURL 和 UserManualURL 为 nil
				},
			},
			expectedInfo: &evaluatordo.EvaluatorInfo{
				Benchmark:     gptr.Of("GLUE"),
				Vendor:        gptr.Of("OpenAI"),
				VendorURL:     nil,
				UserManualURL: nil,
			},
			description: "当DTO的EvaluatorInfo存在但部分字段有值时，DO的EvaluatorInfo应该正确转换，nil字段转为空字符串",
		},
		{
			name: "成功 - EvaluatorInfo字段为空字符串",
			dto: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(0)),
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{
					Benchmark:     gptr.Of(""),
					Vendor:        gptr.Of(""),
					VendorURL:     gptr.Of(""),
					UserManualURL: gptr.Of(""),
				},
			},
			expectedInfo: &evaluatordo.EvaluatorInfo{
				Benchmark:     gptr.Of(""),
				Vendor:        gptr.Of(""),
				VendorURL:     gptr.Of(""),
				UserManualURL: gptr.Of(""),
			},
			description: "当DTO的EvaluatorInfo字段为空字符串时，DO的EvaluatorInfo应该保持为空字符串",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			do := ConvertEvaluatorTemplateDTO2DO(tt.dto)

			assert.NotNil(t, do, tt.description)
			if tt.expectedInfo == nil {
				assert.Nil(t, do.EvaluatorInfo, tt.description)
			} else {
				assert.NotNil(t, do.EvaluatorInfo, tt.description)
				assert.Equal(t, tt.expectedInfo.Benchmark, do.EvaluatorInfo.Benchmark, tt.description+" - Benchmark应该相等")
				assert.Equal(t, tt.expectedInfo.Vendor, do.EvaluatorInfo.Vendor, tt.description+" - Vendor应该相等")
				assert.Equal(t, tt.expectedInfo.VendorURL, do.EvaluatorInfo.VendorURL, tt.description+" - VendorURL应该相等")
				assert.Equal(t, tt.expectedInfo.UserManualURL, do.EvaluatorInfo.UserManualURL, tt.description+" - UserManualURL应该相等")
			}
		})
	}
}

// TestConvertEvaluatorTemplateDO2DTO_EvaluatorInfo 测试 ConvertEvaluatorTemplateDO2DTO 的 EvaluatorInfo 处理逻辑
func TestConvertEvaluatorTemplateDO2DTO_EvaluatorInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		do           *evaluatordo.EvaluatorTemplate
		expectedInfo *evaluatordto.EvaluatorInfo
		description  string
	}{
		{
			name: "成功 - EvaluatorInfo为nil",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            1,
				SpaceID:       2,
				Name:          "Test Template",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				Popularity:    0,
			},
			expectedInfo: nil,
			description:  "当DO的EvaluatorInfo为nil时，DTO的EvaluatorInfo应该为nil",
		},
		{
			name: "成功 - EvaluatorInfo存在但所有字段为空字符串",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            1,
				SpaceID:       2,
				Name:          "Test Template",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				Popularity:    0,
				EvaluatorInfo: &evaluatordo.EvaluatorInfo{
					Benchmark:     gptr.Of(""),
					Vendor:        gptr.Of(""),
					VendorURL:     gptr.Of(""),
					UserManualURL: gptr.Of(""),
				},
			},
			expectedInfo: &evaluatordto.EvaluatorInfo{
				Benchmark:     gptr.Of(""),
				Vendor:        gptr.Of(""),
				VendorURL:     gptr.Of(""),
				UserManualURL: gptr.Of(""),
			},
			description: "当DO的EvaluatorInfo存在但所有字段为空字符串时，DTO的EvaluatorInfo应该创建且所有字段为空字符串指针",
		},
		{
			name: "成功 - EvaluatorInfo存在且所有字段都有值",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            1,
				SpaceID:       2,
				Name:          "Test Template",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				Popularity:    0,
				EvaluatorInfo: &evaluatordo.EvaluatorInfo{
					Benchmark:     gptr.Of("GLUE"),
					Vendor:        gptr.Of("OpenAI"),
					VendorURL:     gptr.Of("https://openai.com"),
					UserManualURL: gptr.Of("https://docs.openai.com"),
				},
			},
			expectedInfo: &evaluatordto.EvaluatorInfo{
				Benchmark:     gptr.Of("GLUE"),
				Vendor:        gptr.Of("OpenAI"),
				VendorURL:     gptr.Of("https://openai.com"),
				UserManualURL: gptr.Of("https://docs.openai.com"),
			},
			description: "当DO的EvaluatorInfo存在且所有字段都有值时，DTO的EvaluatorInfo应该正确转换所有字段",
		},
		{
			name: "成功 - EvaluatorInfo存在但部分字段有值",
			do: &evaluatordo.EvaluatorTemplate{
				ID:            1,
				SpaceID:       2,
				Name:          "Test Template",
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				Popularity:    0,
				EvaluatorInfo: &evaluatordo.EvaluatorInfo{
					Benchmark: gptr.Of("GLUE"),
					Vendor:    gptr.Of("OpenAI"),
					// VendorURL 和 UserManualURL 为空字符串
					VendorURL:     gptr.Of(""),
					UserManualURL: gptr.Of(""),
				},
			},
			expectedInfo: &evaluatordto.EvaluatorInfo{
				Benchmark:     gptr.Of("GLUE"),
				Vendor:        gptr.Of("OpenAI"),
				VendorURL:     gptr.Of(""),
				UserManualURL: gptr.Of(""),
			},
			description: "当DO的EvaluatorInfo存在但部分字段有值时，DTO的EvaluatorInfo应该正确转换，空字符串字段转为空字符串指针",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dto := ConvertEvaluatorTemplateDO2DTO(tt.do)

			assert.NotNil(t, dto, tt.description)
			if tt.expectedInfo == nil {
				assert.Nil(t, dto.EvaluatorInfo, tt.description)
			} else {
				assert.NotNil(t, dto.EvaluatorInfo, tt.description)
				assert.Equal(t, tt.expectedInfo.GetBenchmark(), dto.EvaluatorInfo.GetBenchmark(), tt.description+" - Benchmark应该相等")
				assert.Equal(t, tt.expectedInfo.GetVendor(), dto.EvaluatorInfo.GetVendor(), tt.description+" - Vendor应该相等")
				assert.Equal(t, tt.expectedInfo.GetVendorURL(), dto.EvaluatorInfo.GetVendorURL(), tt.description+" - VendorURL应该相等")
				assert.Equal(t, tt.expectedInfo.GetUserManualURL(), dto.EvaluatorInfo.GetUserManualURL(), tt.description+" - UserManualURL应该相等")
			}
		})
	}
}

// TestConvertEvaluatorTemplate_EvaluatorInfo_RoundTrip 测试 EvaluatorInfo 的双向转换
func TestConvertEvaluatorTemplate_EvaluatorInfo_RoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		originalDTO *evaluatordto.EvaluatorTemplate
		description string
	}{
		{
			name: "成功 - EvaluatorInfo完整字段往返转换",
			originalDTO: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(100)),
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{
					Benchmark:     gptr.Of("GLUE"),
					Vendor:        gptr.Of("OpenAI"),
					VendorURL:     gptr.Of("https://openai.com"),
					UserManualURL: gptr.Of("https://docs.openai.com"),
				},
			},
			description: "当EvaluatorInfo所有字段都有值时，DTO->DO->DTO往返转换应该保持数据一致性",
		},
		{
			name: "成功 - EvaluatorInfo部分字段往返转换",
			originalDTO: &evaluatordto.EvaluatorTemplate{
				ID:            gptr.Of(int64(1)),
				WorkspaceID:   gptr.Of(int64(2)),
				Name:          gptr.Of("Test Template"),
				EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				Popularity:    gptr.Of(int64(100)),
				EvaluatorInfo: &evaluatordto.EvaluatorInfo{
					Benchmark: gptr.Of("GLUE"),
					Vendor:    gptr.Of("OpenAI"),
					// VendorURL 和 UserManualURL 为 nil
				},
			},
			description: "当EvaluatorInfo部分字段有值时，DTO->DO->DTO往返转换应该保持数据一致性（nil字段转为空字符串）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// DTO -> DO
			do := ConvertEvaluatorTemplateDTO2DO(tt.originalDTO)
			assert.NotNil(t, do, tt.description)

			// DO -> DTO
			resultDTO := ConvertEvaluatorTemplateDO2DTO(do)
			assert.NotNil(t, resultDTO, tt.description)

			// 验证 EvaluatorInfo 的往返转换
			if tt.originalDTO.EvaluatorInfo == nil {
				assert.Nil(t, resultDTO.EvaluatorInfo, tt.description)
			} else {
				assert.NotNil(t, resultDTO.EvaluatorInfo, tt.description)
				originalBenchmark := tt.originalDTO.EvaluatorInfo.GetBenchmark()
				resultBenchmark := resultDTO.EvaluatorInfo.GetBenchmark()
				if originalBenchmark == "" {
					assert.Equal(t, "", resultBenchmark, tt.description+" - Benchmark应该保持一致")
				} else {
					assert.Equal(t, originalBenchmark, resultBenchmark, tt.description+" - Benchmark应该保持一致")
				}

				originalVendor := tt.originalDTO.EvaluatorInfo.GetVendor()
				resultVendor := resultDTO.EvaluatorInfo.GetVendor()
				if originalVendor == "" {
					assert.Equal(t, "", resultVendor, tt.description+" - Vendor应该保持一致")
				} else {
					assert.Equal(t, originalVendor, resultVendor, tt.description+" - Vendor应该保持一致")
				}

				originalVendorURL := tt.originalDTO.EvaluatorInfo.GetVendorURL()
				resultVendorURL := resultDTO.EvaluatorInfo.GetVendorURL()
				if originalVendorURL == "" {
					assert.Equal(t, "", resultVendorURL, tt.description+" - VendorURL应该保持一致")
				} else {
					assert.Equal(t, originalVendorURL, resultVendorURL, tt.description+" - VendorURL应该保持一致")
				}

				originalUserManualURL := tt.originalDTO.EvaluatorInfo.GetUserManualURL()
				resultUserManualURL := resultDTO.EvaluatorInfo.GetUserManualURL()
				if originalUserManualURL == "" {
					assert.Equal(t, "", resultUserManualURL, tt.description+" - UserManualURL应该保持一致")
				} else {
					assert.Equal(t, originalUserManualURL, resultUserManualURL, tt.description+" - UserManualURL应该保持一致")
				}
			}
		})
	}
}
