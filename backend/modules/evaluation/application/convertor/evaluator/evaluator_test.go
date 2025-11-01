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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestConvertEvaluatorDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		evaluatorDTO *evaluatordto.Evaluator
		expected     *evaluatordo.Evaluator
	}{
		{
			name: "Prompt评估器转换",
			evaluatorDTO: &evaluatordto.Evaluator{
				EvaluatorID:    gptr.Of(int64(123)),
				WorkspaceID:    gptr.Of(int64(456)),
				Name:           gptr.Of("Test Prompt Evaluator"),
				Description:    gptr.Of("Test description"),
				DraftSubmitted: gptr.Of(true),
				EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Prompt),
				LatestVersion:  gptr.Of("1"),
				Tags: map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string{
					evaluatordto.EvaluatorTagLangTypeEn: {
						evaluatordto.EvaluatorTagKeyCategory:  {"LLM", "Code"},
						evaluatordto.EvaluatorTagKeyObjective: {"Quality"},
					},
				},
			},
			expected: &evaluatordo.Evaluator{
				ID:             123,
				SpaceID:        456,
				Name:           "Test Prompt Evaluator",
				Description:    "Test description",
				DraftSubmitted: true,
				EvaluatorType:  evaluatordo.EvaluatorTypePrompt,
				LatestVersion:  "1",
				Tags: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{
					evaluatordo.EvaluatorTagLangType_En: {
						evaluatordo.EvaluatorTagKey_Category:  {"LLM", "Code"},
						evaluatordo.EvaluatorTagKey_Objective: {"Quality"},
					},
				},
			},
		},
		{
			name: "Code评估器转换",
			evaluatorDTO: &evaluatordto.Evaluator{
				EvaluatorID:    gptr.Of(int64(124)),
				WorkspaceID:    gptr.Of(int64(457)),
				Name:           gptr.Of("Test Code Evaluator"),
				Description:    gptr.Of("Code test description"),
				DraftSubmitted: gptr.Of(false),
				EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType_Code),
				LatestVersion:  gptr.Of("2"),
			},
			expected: &evaluatordo.Evaluator{
				ID:             124,
				SpaceID:        457,
				Name:           "Test Code Evaluator",
				Description:    "Code test description",
				DraftSubmitted: false,
				EvaluatorType:  evaluatordo.EvaluatorTypeCode,
				LatestVersion:  "2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDTO2DO(tt.evaluatorDTO)

			assert.Equal(t, tt.expected.ID, result.ID)
			assert.Equal(t, tt.expected.SpaceID, result.SpaceID)
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Description, result.Description)
			assert.Equal(t, tt.expected.DraftSubmitted, result.DraftSubmitted)
			assert.Equal(t, tt.expected.EvaluatorType, result.EvaluatorType)
			assert.Equal(t, tt.expected.LatestVersion, result.LatestVersion)
		})
	}
}

func TestConvertEvaluatorDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		evaluatorDO  *evaluatordo.Evaluator
		expectedType evaluatordto.EvaluatorType
	}{
		{
			name: "Prompt评估器转换",
			evaluatorDO: &evaluatordo.Evaluator{
				ID:             123,
				SpaceID:        456,
				Name:           "Test Prompt Evaluator",
				Description:    "Test description",
				DraftSubmitted: true,
				EvaluatorType:  evaluatordo.EvaluatorTypePrompt,
				LatestVersion:  "1",
				Tags: map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string{
					evaluatordo.EvaluatorTagLangType_En: {
						evaluatordo.EvaluatorTagKey_Category:  {"LLM", "Code"},
						evaluatordo.EvaluatorTagKey_Objective: {"Quality"},
					},
				},
			},
			expectedType: evaluatordto.EvaluatorType_Prompt,
		},
		{
			name: "Code评估器转换",
			evaluatorDO: &evaluatordo.Evaluator{
				ID:             124,
				SpaceID:        457,
				Name:           "Test Code Evaluator",
				Description:    "Code test description",
				DraftSubmitted: false,
				EvaluatorType:  evaluatordo.EvaluatorTypeCode,
				LatestVersion:  "2",
			},
			expectedType: evaluatordto.EvaluatorType_Code,
		},
		{
			name:        "nil输入",
			evaluatorDO: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDO2DTO(tt.evaluatorDO)

			if tt.evaluatorDO == nil {
				assert.Nil(t, result)
				return
			}

			assert.Equal(t, tt.evaluatorDO.ID, result.GetEvaluatorID())
			assert.Equal(t, tt.evaluatorDO.SpaceID, result.GetWorkspaceID())
			assert.Equal(t, tt.evaluatorDO.Name, result.GetName())
			assert.Equal(t, tt.evaluatorDO.Description, result.GetDescription())
			assert.Equal(t, tt.evaluatorDO.DraftSubmitted, result.GetDraftSubmitted())
			assert.Equal(t, tt.expectedType, result.GetEvaluatorType())
			assert.Equal(t, tt.evaluatorDO.LatestVersion, result.GetLatestVersion())
		})
	}
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

func TestNormalizeLanguageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		langType evaluatordo.LanguageType
		expected evaluatordo.LanguageType
	}{
		{
			name:     "python小写",
			langType: "python",
			expected: evaluatordo.LanguageTypePython,
		},
		{
			name:     "Python首字母大写",
			langType: "Python",
			expected: evaluatordo.LanguageTypePython,
		},
		{
			name:     "js小写",
			langType: "js",
			expected: evaluatordo.LanguageTypeJS,
		},
		{
			name:     "javascript",
			langType: "javascript",
			expected: evaluatordo.LanguageTypeJS,
		},
		{
			name:     "未知类型",
			langType: "golang",
			expected: "Golang",
		},
		{
			name:     "空字符串",
			langType: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := normalizeLanguageType(tt.langType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

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
		validate     func(t *testing.T, result *evaluatordo.Evaluator)
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
			validate: func(t *testing.T, result *evaluatordo.Evaluator) {
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
			validate: func(t *testing.T, result *evaluatordo.Evaluator) {
				assert.Equal(t, int64(124), result.ID)
				assert.Equal(t, evaluatordo.EvaluatorTypeCode, result.EvaluatorType)
				assert.NotNil(t, result.CodeEvaluatorVersion)
				assert.Equal(t, int64(890), result.CodeEvaluatorVersion.ID)
				assert.Equal(t, "print('hello world')", result.CodeEvaluatorVersion.CodeContent)
				assert.Equal(t, evaluatordo.LanguageTypePython, result.CodeEvaluatorVersion.LanguageType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDTO2DO(tt.evaluatorDTO)

			if tt.validate != nil {
				tt.validate(t, result)
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

func TestConvertEvaluatorContent2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		content         *evaluatordto.EvaluatorContent
		evaluatorType   evaluatordto.EvaluatorType
		expectedErr     bool
		expectedErrCode int32
		validate        func(t *testing.T, result *evaluatordo.Evaluator)
	}{
		{
			name:            "nil content",
			content:         nil,
			evaluatorType:   evaluatordto.EvaluatorType_Prompt,
			expectedErr:     true,
			expectedErrCode: errno.InvalidInputDataCode,
		},
		{
			name: "Prompt evaluator with nil PromptEvaluator",
			content: &evaluatordto.EvaluatorContent{
				PromptEvaluator: nil,
			},
			evaluatorType:   evaluatordto.EvaluatorType_Prompt,
			expectedErr:     true,
			expectedErrCode: errno.InvalidInputDataCode,
		},
		{
			name: "Code evaluator with nil CodeEvaluator",
			content: &evaluatordto.EvaluatorContent{
				CodeEvaluator: nil,
			},
			evaluatorType:   evaluatordto.EvaluatorType_Code,
			expectedErr:     true,
			expectedErrCode: errno.InvalidInputDataCode,
		},
		{
			name: "unsupported evaluator type",
			content: &evaluatordto.EvaluatorContent{
				PromptEvaluator: &evaluatordto.PromptEvaluator{},
			},
			evaluatorType:   evaluatordto.EvaluatorType(999), // Invalid type
			expectedErr:     true,
			expectedErrCode: errno.InvalidEvaluatorTypeCode,
		},
		{
			name: "valid Prompt evaluator",
			content: &evaluatordto.EvaluatorContent{
				ReceiveChatHistory: gptr.Of(true),
				PromptEvaluator: &evaluatordto.PromptEvaluator{
					PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType_BuiltinTemplate),
					PromptTemplateKey: gptr.Of("test_template"),
					MessageList: []*commondto.Message{
						{
							Role: gptr.Of(commondto.Role(1)),
							Content: &commondto.Content{
								ContentType: gptr.Of("text"),
								Text:        gptr.Of("Hello"),
							},
						},
					},
				},
				InputSchemas: []*commondto.ArgsSchema{
					{
						Key:        gptr.Of("input1"),
						JSONSchema: gptr.Of("{}"),
					},
				},
			},
			evaluatorType: evaluatordto.EvaluatorType_Prompt,
			expectedErr:   false,
			validate: func(t *testing.T, result *evaluatordo.Evaluator) {
				assert.Equal(t, evaluatordo.EvaluatorTypePrompt, result.EvaluatorType)
				assert.NotNil(t, result.PromptEvaluatorVersion)
				assert.Equal(t, "test_template", result.PromptEvaluatorVersion.PromptTemplateKey)
				assert.Len(t, result.PromptEvaluatorVersion.MessageList, 1)
				assert.Len(t, result.PromptEvaluatorVersion.InputSchemas, 1)
			},
		},
		{
			name: "valid Code evaluator",
			content: &evaluatordto.EvaluatorContent{
				CodeEvaluator: &evaluatordto.CodeEvaluator{
					CodeTemplateKey:  gptr.Of("test_code_template"),
					CodeTemplateName: gptr.Of("Test Code Template"),
					CodeContent:      gptr.Of("print('hello')"),
					LanguageType:     gptr.Of(evaluatordto.LanguageType("js")),
				},
			},
			evaluatorType: evaluatordto.EvaluatorType_Code,
			expectedErr:   false,
			validate: func(t *testing.T, result *evaluatordo.Evaluator) {
				assert.Equal(t, evaluatordo.EvaluatorTypeCode, result.EvaluatorType)
				assert.NotNil(t, result.CodeEvaluatorVersion)
				assert.Equal(t, "print('hello')", result.CodeEvaluatorVersion.CodeContent)
				assert.Equal(t, evaluatordo.LanguageTypeJS, result.CodeEvaluatorVersion.LanguageType)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ConvertEvaluatorContent2DO(tt.content, tt.evaluatorType)

			if tt.expectedErr {
				assert.Error(t, err)
				if tt.expectedErrCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if ok {
						assert.Equal(t, tt.expectedErrCode, statusErr.Code())
					}
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

// Test additional functions to improve coverage
func TestConvertEvaluatorDTO2DO_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		evaluatorDTO *evaluatordto.Evaluator
		validate     func(t *testing.T, result *evaluatordo.Evaluator)
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
			validate: func(t *testing.T, result *evaluatordo.Evaluator) {
				assert.Equal(t, int64(123), result.ID)
				assert.Nil(t, result.PromptEvaluatorVersion)
				assert.Nil(t, result.CodeEvaluatorVersion)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := ConvertEvaluatorDTO2DO(tt.evaluatorDTO)

			if tt.validate != nil {
				tt.validate(t, result)
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
