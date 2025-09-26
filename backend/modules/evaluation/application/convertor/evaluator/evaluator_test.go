// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
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
			},
			expected: &evaluatordo.Evaluator{
				ID:             123,
				SpaceID:        456,
				Name:           "Test Prompt Evaluator",
				Description:    "Test description",
				DraftSubmitted: true,
				EvaluatorType:  evaluatordo.EvaluatorTypePrompt,
				LatestVersion:  "1",
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
			assert.Equal(t, string(tt.expected), string(result))
		})
	}
}