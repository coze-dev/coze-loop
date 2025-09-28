// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
)

func TestEvaluator_GetSetEvaluatorVersion(t *testing.T) {
	// Prompt类型
	promptVer := &PromptEvaluatorVersion{Version: "v1", ID: 123}
	promptEval := &Evaluator{
		EvaluatorType:          EvaluatorTypePrompt,
		PromptEvaluatorVersion: promptVer,
	}
	assert.Equal(t, "v1", promptEval.GetVersion())
	assert.Equal(t, int64(123), promptEval.GetEvaluatorVersionID())

	// 非Prompt类型
	codeEval := &Evaluator{EvaluatorType: EvaluatorTypeCode}
	assert.Equal(t, "", codeEval.GetVersion())
	assert.Equal(t, int64(0), codeEval.GetEvaluatorVersionID())

	// SetEvaluatorVersion
	newPromptVer := &Evaluator{PromptEvaluatorVersion: &PromptEvaluatorVersion{Version: "v2"}, EvaluatorType: EvaluatorTypePrompt}
	promptEval.SetEvaluatorVersion(newPromptVer)
	assert.Equal(t, "v2", promptEval.PromptEvaluatorVersion.Version)

	// SetEvaluatorVersion 非Prompt类型
	codeEval.SetEvaluatorVersion(newPromptVer)
	assert.Nil(t, codeEval.PromptEvaluatorVersion)
}

func TestEvaluatorRecord_GetSetBaseInfo(t *testing.T) {
	rec := &EvaluatorRecord{}
	assert.Nil(t, rec.GetBaseInfo())
	base := &BaseInfo{CreatedBy: &UserInfo{UserID: gptr.Of("u1")}}
	rec.SetBaseInfo(base)
	assert.Equal(t, base, rec.GetBaseInfo())
}

func TestEvaluator_DescriptionMethods(t *testing.T) {
	// 测试 Evaluator 本身的描述
	evaluator := &Evaluator{
		Description:   "evaluator desc",
		EvaluatorType: EvaluatorTypePrompt,
		PromptEvaluatorVersion: &PromptEvaluatorVersion{
			Description: "version desc",
		},
	}

	// 测试获取评估器描述
	assert.Equal(t, "evaluator desc", evaluator.GetEvaluatorDescription())

	// 测试设置评估器描述
	evaluator.SetEvaluatorDescription("new evaluator desc")
	assert.Equal(t, "new evaluator desc", evaluator.GetEvaluatorDescription())
	assert.Equal(t, "new evaluator desc", evaluator.Description)

	// 测试获取评估器版本描述
	assert.Equal(t, "version desc", evaluator.GetEvaluatorVersionDescription())

	// 测试设置评估器版本描述
	evaluator.SetEvaluatorVersionDescription("new version desc")
	assert.Equal(t, "new version desc", evaluator.GetEvaluatorVersionDescription())
	assert.Equal(t, "new version desc", evaluator.PromptEvaluatorVersion.Description)
}

func TestEvaluator_GetEvaluatorID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		expected  int64
	}{
		{
			name: "prompt evaluator with valid ID",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					EvaluatorID: 123,
				},
			},
			expected: 123,
		},
		{
			name: "code evaluator with valid ID",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					EvaluatorID: 456,
				},
			},
			expected: 456,
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expected: 0,
		},
		{
			name: "code evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			expected: 0,
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.evaluator.GetEvaluatorID()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_GetSpaceID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		expected  int64
	}{
		{
			name: "prompt evaluator with valid space ID",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					SpaceID: 789,
				},
			},
			expected: 789,
		},
		{
			name: "code evaluator with valid space ID",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					SpaceID: 101112,
				},
			},
			expected: 101112,
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expected: 0,
		},
		{
			name: "code evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			expected: 0,
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.evaluator.GetSpaceID()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_GetBaseInfo(t *testing.T) {
	t.Parallel()
	baseInfo := &BaseInfo{
		CreatedBy: &UserInfo{UserID: gptr.Of("user123")},
	}

	tests := []struct {
		name      string
		evaluator *Evaluator
		expected  *BaseInfo
	}{
		{
			name: "prompt evaluator with base info",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					BaseInfo: baseInfo,
				},
			},
			expected: baseInfo,
		},
		{
			name: "code evaluator with base info",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					BaseInfo: baseInfo,
				},
			},
			expected: baseInfo,
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "code evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.evaluator.GetBaseInfo()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_GetPromptTemplateKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		expected  string
	}{
		{
			name: "prompt evaluator with template key",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					PromptTemplateKey: "test_template_key",
				},
			},
			expected: "test_template_key",
		},
		{
			name: "code evaluator should return empty",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					ID: 123,
				},
			},
			expected: "",
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expected: "",
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.evaluator.GetPromptTemplateKey()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_GetModelConfig(t *testing.T) {
	t.Parallel()
	modelConfig := &ModelConfig{
		ModelID:   123,
		ModelName: "test_model",
	}

	tests := []struct {
		name      string
		evaluator *Evaluator
		expected  *ModelConfig
	}{
		{
			name: "prompt evaluator with model config",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					ModelConfig: modelConfig,
				},
			},
			expected: modelConfig,
		},
		{
			name: "code evaluator should return nil",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					ID: 123,
				},
			},
			expected: nil,
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.evaluator.GetModelConfig()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_ValidateInput(t *testing.T) {
	t.Parallel()
	inputData := &EvaluatorInputData{
		InputFields: map[string]*Content{
			"test": {ContentType: gptr.Of(ContentTypeText), Text: gptr.Of("test")},
		},
	}

	tests := []struct {
		name      string
		evaluator *Evaluator
		input     *EvaluatorInputData
		expectErr bool
	}{
		{
			name: "prompt evaluator with valid input",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					InputSchemas: []*ArgsSchema{
						{Key: gptr.Of("test"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of("{}")},
					},
				},
			},
			input:     inputData,
			expectErr: false,
		},
		{
			name: "code evaluator with valid input",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					ID: 123,
				},
			},
			input:     inputData,
			expectErr: false,
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			input:     inputData,
			expectErr: false,
		},
		{
			name: "code evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			input:     inputData,
			expectErr: false,
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			input:     inputData,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.evaluator.ValidateInput(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluator_ValidateBaseInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		expectErr bool
	}{
		{
			name: "prompt evaluator with valid base info",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					MessageList: []*Message{{Role: RoleUser}},
					ModelConfig: &ModelConfig{ModelID: 123},
				},
			},
			expectErr: false,
		},
		{
			name: "code evaluator with valid base info",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					CodeContent:  "print('hello')",
					LanguageType: LanguageTypePython,
				},
			},
			expectErr: false,
		},
		{
			name: "prompt evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expectErr: false,
		},
		{
			name: "code evaluator with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			expectErr: false,
		},
		{
			name: "unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.evaluator.ValidateBaseInfo()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluator_SetEvaluatorVersionID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		id        int64
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator version ID",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			id: 123,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, int64(123), e.PromptEvaluatorVersion.ID)
			},
		},
		{
			name: "set code evaluator version ID",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			id: 456,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, int64(456), e.CodeEvaluatorVersion.ID)
			},
		},
		{
			name: "set prompt evaluator version ID with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			id: 123,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set code evaluator version ID with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			id: 456,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.CodeEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			id: 789,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetEvaluatorVersionID(tt.id)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		version   string
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			version: "v1.0.0",
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, "v1.0.0", e.PromptEvaluatorVersion.Version)
			},
		},
		{
			name: "set code evaluator version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			version: "v2.0.0",
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, "v2.0.0", e.CodeEvaluatorVersion.Version)
			},
		},
		{
			name: "set prompt evaluator version with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			version: "v1.0.0",
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set code evaluator version with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			version: "v2.0.0",
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.CodeEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			version: "v3.0.0",
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetVersion(tt.version)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetBaseInfo(t *testing.T) {
	t.Parallel()
	baseInfo := &BaseInfo{
		CreatedBy: &UserInfo{UserID: gptr.Of("user123")},
	}

	tests := []struct {
		name      string
		evaluator *Evaluator
		baseInfo  *BaseInfo
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator base info",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			baseInfo: baseInfo,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, baseInfo, e.PromptEvaluatorVersion.BaseInfo)
			},
		},
		{
			name: "set code evaluator base info",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			baseInfo: baseInfo,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, baseInfo, e.CodeEvaluatorVersion.BaseInfo)
			},
		},
		{
			name: "set prompt evaluator base info with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			baseInfo: baseInfo,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set code evaluator base info with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			baseInfo: baseInfo,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.CodeEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			baseInfo: baseInfo,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetBaseInfo(tt.baseInfo)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetTools(t *testing.T) {
	t.Parallel()
	tools := []*Tool{
		{Type: ToolTypeFunction, Function: &Function{Name: "test_func", Description: "test", Parameters: "{}"}},
	}

	tests := []struct {
		name      string
		evaluator *Evaluator
		tools     []*Tool
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator tools",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			tools: tools,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, tools, e.PromptEvaluatorVersion.Tools)
			},
		},
		{
			name: "set code evaluator tools should do nothing",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			tools: tools,
			verify: func(t *testing.T, e *Evaluator) {
				// Code evaluator doesn't support tools, should do nothing
			},
		},
		{
			name: "set prompt evaluator tools with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			tools: tools,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			tools: tools,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetTools(tt.tools)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetPromptSuffix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		suffix    string
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator suffix",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			suffix: "test_suffix",
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, "test_suffix", e.PromptEvaluatorVersion.PromptSuffix)
			},
		},
		{
			name: "set code evaluator suffix should do nothing",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			suffix: "test_suffix",
			verify: func(t *testing.T, e *Evaluator) {
				// Code evaluator doesn't support prompt suffix, should do nothing
			},
		},
		{
			name: "set prompt evaluator suffix with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			suffix: "test_suffix",
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			suffix: "test_suffix",
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetPromptSuffix(tt.suffix)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetParseType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		parseType ParseType
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator parse type",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			parseType: ParseTypeFunctionCall,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, ParseTypeFunctionCall, e.PromptEvaluatorVersion.ParseType)
			},
		},
		{
			name: "set code evaluator parse type should do nothing",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			parseType: ParseTypeFunctionCall,
			verify: func(t *testing.T, e *Evaluator) {
				// Code evaluator doesn't support parse type, should do nothing
			},
		},
		{
			name: "set prompt evaluator parse type with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			parseType: ParseTypeFunctionCall,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			parseType: ParseTypeFunctionCall,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetParseType(tt.parseType)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetEvaluatorID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		evaluator   *Evaluator
		evaluatorID int64
		verify      func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator ID",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			evaluatorID: 123,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, int64(123), e.PromptEvaluatorVersion.EvaluatorID)
			},
		},
		{
			name: "set code evaluator ID",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			evaluatorID: 456,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, int64(456), e.CodeEvaluatorVersion.EvaluatorID)
			},
		},
		{
			name: "set prompt evaluator ID with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			evaluatorID: 123,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set code evaluator ID with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			evaluatorID: 456,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.CodeEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			evaluatorID: 789,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetEvaluatorID(tt.evaluatorID)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetSpaceID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		spaceID   int64
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator space ID",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			spaceID: 789,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, int64(789), e.PromptEvaluatorVersion.SpaceID)
			},
		},
		{
			name: "set code evaluator space ID",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			spaceID: 101112,
			verify: func(t *testing.T, e *Evaluator) {
				assert.Equal(t, int64(101112), e.CodeEvaluatorVersion.SpaceID)
			},
		},
		{
			name: "set prompt evaluator space ID with nil version",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			spaceID: 789,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.PromptEvaluatorVersion)
			},
		},
		{
			name: "set code evaluator space ID with nil version",
			evaluator: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: nil,
			},
			spaceID: 101112,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
				assert.Nil(t, e.CodeEvaluatorVersion)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			spaceID: 131415,
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetSpaceID(tt.spaceID)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluator_SetEvaluatorVersion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		evaluator *Evaluator
		version   *Evaluator
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set prompt evaluator version",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
			},
			version: &Evaluator{
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					Version: "v1.0.0",
					ID:      123,
				},
			},
			verify: func(t *testing.T, e *Evaluator) {
				assert.NotNil(t, e.PromptEvaluatorVersion)
				assert.Equal(t, "v1.0.0", e.PromptEvaluatorVersion.Version)
				assert.Equal(t, int64(123), e.PromptEvaluatorVersion.ID)
			},
		},
		{
			name: "set code evaluator version",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCode,
			},
			version: &Evaluator{
				CodeEvaluatorVersion: &CodeEvaluatorVersion{
					Version: "v2.0.0",
					ID:      456,
				},
			},
			verify: func(t *testing.T, e *Evaluator) {
				assert.NotNil(t, e.CodeEvaluatorVersion)
				assert.Equal(t, "v2.0.0", e.CodeEvaluatorVersion.Version)
				assert.Equal(t, int64(456), e.CodeEvaluatorVersion.ID)
			},
		},
		{
			name: "set unknown evaluator type",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			version: &Evaluator{
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					Version: "v1.0.0",
				},
			},
			verify: func(t *testing.T, e *Evaluator) {
				// Should not panic, just do nothing
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.evaluator.SetEvaluatorVersion(tt.version)
			tt.verify(t, tt.evaluator)
		})
	}
}

func TestEvaluatorRecord_GetScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		record   *EvaluatorRecord
		expected *float64
	}{
		{
			name: "nil evaluator output data",
			record: &EvaluatorRecord{
				EvaluatorOutputData: nil,
			},
			expected: nil,
		},
		{
			name: "nil evaluator result",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: nil,
				},
			},
			expected: nil,
		},
		{
			name: "score from correction",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: &EvaluatorResult{
						Score: gptr.Of(0.8),
						Correction: &Correction{
							Score: gptr.Of(0.9),
						},
					},
				},
			},
			expected: gptr.Of(0.9),
		},
		{
			name: "score from result when no correction",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: &EvaluatorResult{
						Score:      gptr.Of(0.8),
						Correction: nil,
					},
				},
			},
			expected: gptr.Of(0.8),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.record.GetScore()
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

func TestEvaluatorRecord_GetReasoning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		record   *EvaluatorRecord
		expected string
	}{
		{
			name: "nil evaluator output data",
			record: &EvaluatorRecord{
				EvaluatorOutputData: nil,
			},
			expected: "",
		},
		{
			name: "nil evaluator result",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: nil,
				},
			},
			expected: "",
		},
		{
			name: "reasoning from correction",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: &EvaluatorResult{
						Reasoning: "original reasoning",
						Correction: &Correction{
							Explain: "corrected reasoning",
						},
					},
				},
			},
			expected: "corrected reasoning",
		},
		{
			name: "reasoning from result when no correction",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: &EvaluatorResult{
						Reasoning:  "original reasoning",
						Correction: nil,
					},
				},
			},
			expected: "original reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.record.GetReasoning()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluatorRecord_GetCorrected(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		record   *EvaluatorRecord
		expected bool
	}{
		{
			name: "nil evaluator output data",
			record: &EvaluatorRecord{
				EvaluatorOutputData: nil,
			},
			expected: false,
		},
		{
			name: "nil evaluator result",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: nil,
				},
			},
			expected: false,
		},
		{
			name: "has correction",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: &EvaluatorResult{
						Correction: &Correction{
							Score: gptr.Of(0.9),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "no correction",
			record: &EvaluatorRecord{
				EvaluatorOutputData: &EvaluatorOutputData{
					EvaluatorResult: &EvaluatorResult{
						Correction: nil,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.record.GetCorrected()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCodeEvaluatorVersion_GetSetMethods(t *testing.T) {
	t.Parallel()

	t.Run("all getter and setter methods", func(t *testing.T) {
		t.Parallel()
		ver := &CodeEvaluatorVersion{}

		// Test ID
		ver.SetID(123)
		assert.Equal(t, int64(123), ver.GetID())

		// Test EvaluatorID
		ver.SetEvaluatorID(456)
		assert.Equal(t, int64(456), ver.GetEvaluatorID())

		// Test SpaceID
		ver.SetSpaceID(789)
		assert.Equal(t, int64(789), ver.GetSpaceID())

		// Test Version
		ver.SetVersion("v1.0.0")
		assert.Equal(t, "v1.0.0", ver.GetVersion())

		// Test Description
		ver.SetDescription("test description")
		assert.Equal(t, "test description", ver.GetDescription())

		// Test BaseInfo
		baseInfo := &BaseInfo{CreatedBy: &UserInfo{UserID: gptr.Of("user123")}}
		ver.SetBaseInfo(baseInfo)
		assert.Equal(t, baseInfo, ver.GetBaseInfo())

		// Test CodeTemplateKey
		key := "test_key"
		ver.SetCodeTemplateKey(&key)
		assert.Equal(t, &key, ver.GetCodeTemplateKey())

		// Test CodeTemplateName
		name := "test_name"
		ver.SetCodeTemplateName(&name)
		assert.Equal(t, &name, ver.GetCodeTemplateName())

		// Test CodeContent
		content := "print('hello')"
		ver.SetCodeContent(content)
		assert.Equal(t, content, ver.GetCodeContent())

		// Test LanguageType
		ver.SetLanguageType(LanguageTypePython)
		assert.Equal(t, LanguageTypePython, ver.GetLanguageType())
	})
}

func TestCodeEvaluatorVersion_ValidateInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		version   *CodeEvaluatorVersion
		input     *EvaluatorInputData
		expectErr bool
	}{
		{
			name: "valid input",
			version: &CodeEvaluatorVersion{
				CodeContent:  "print('hello')",
				LanguageType: LanguageTypePython,
			},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"test": {ContentType: gptr.Of(ContentTypeText), Text: gptr.Of("test")},
				},
			},
			expectErr: false,
		},
		{
			name:      "nil version",
			version:   nil,
			input:     &EvaluatorInputData{},
			expectErr: false,
		},
		{
			name: "empty code evaluator",
			version: &CodeEvaluatorVersion{
				CodeContent:  "",
				LanguageType: LanguageTypePython,
			},
			input:     &EvaluatorInputData{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.version.ValidateInput(tt.input)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCodeEvaluatorVersion_ValidateBaseInfo(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		version   *CodeEvaluatorVersion
		expectErr bool
	}{
		{
			name:      "nil version",
			version:   nil,
			expectErr: true,
		},
		{
			name: "empty code content",
			version: &CodeEvaluatorVersion{
				CodeContent:  "",
				LanguageType: LanguageTypePython,
			},
			expectErr: true,
		},
		{
			name: "invalid language type",
			version: &CodeEvaluatorVersion{
				CodeContent:  "print('hello')",
				LanguageType: "InvalidLang",
			},
			expectErr: true,
		},
		{
			name: "valid version",
			version: &CodeEvaluatorVersion{
				CodeContent:  "print('hello')",
				LanguageType: LanguageTypePython,
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.version.ValidateBaseInfo()
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPromptEvaluatorVersion_GetSetMethods(t *testing.T) {
	t.Parallel()
	ver := &PromptEvaluatorVersion{}
	ver.SetID(11)
	assert.Equal(t, int64(11), ver.GetID())
	ver.SetEvaluatorID(22)
	assert.Equal(t, int64(22), ver.GetEvaluatorID())
	ver.SetSpaceID(33)
	assert.Equal(t, int64(33), ver.GetSpaceID())
	ver.SetVersion("v1")
	assert.Equal(t, "v1", ver.GetVersion())
	ver.SetDescription("desc")
	assert.Equal(t, "desc", ver.GetDescription())
	base := &BaseInfo{CreatedBy: &UserInfo{UserID: gptr.Of("u2")}}
	ver.SetBaseInfo(base)
	assert.Equal(t, base, ver.GetBaseInfo())
	tools := []*Tool{{Type: ToolTypeFunction, Function: &Function{Name: "f1", Description: "d1", Parameters: "p1"}}}
	ver.SetTools(tools)
	assert.Equal(t, tools, ver.Tools)
	ver.SetPromptSuffix("suf")
	assert.Equal(t, "suf", ver.PromptSuffix)
	ver.SetParseType(ParseTypeFunctionCall)
	assert.Equal(t, ParseTypeFunctionCall, ver.ParseType)
}

func TestPromptEvaluatorVersion_GetPromptTemplateKey(t *testing.T) {
	ver := &PromptEvaluatorVersion{PromptTemplateKey: "key1"}
	assert.Equal(t, "key1", ver.GetPromptTemplateKey())
}

func TestPromptEvaluatorVersion_GetModelConfig(t *testing.T) {
	mc := &ModelConfig{ModelID: 123}
	ver := &PromptEvaluatorVersion{ModelConfig: mc}
	assert.Equal(t, mc, ver.GetModelConfig())
}

func TestPromptEvaluatorVersion_ValidateInput(t *testing.T) {
	ver := &PromptEvaluatorVersion{
		InputSchemas: []*ArgsSchema{
			{Key: gptr.Of("field1"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of("{}")},
		},
	}
	input := &EvaluatorInputData{
		InputFields: map[string]*Content{
			"field1": {ContentType: gptr.Of(ContentTypeText), Text: gptr.Of("abc")},
		},
	}
	// schema校验通过
	assert.NoError(t, ver.ValidateInput(input))

	// 不支持的ContentType
	ver.InputSchemas[0].SupportContentTypes = []ContentType{ContentTypeImage}
	err := ver.ValidateInput(input)
	assert.Error(t, err)

	// ContentType为Text但json校验不通过
	ver.InputSchemas[0].SupportContentTypes = []ContentType{ContentTypeText}
	ver.InputSchemas[0].JsonSchema = gptr.Of("{invalid json}")
	err = ver.ValidateInput(input)
	assert.Error(t, err)
}

func TestPromptEvaluatorVersion_ValidateBaseInfo(t *testing.T) {
	// nil
	var ver *PromptEvaluatorVersion
	assert.Error(t, ver.ValidateBaseInfo())

	// message list 为空
	ver = &PromptEvaluatorVersion{ModelConfig: &ModelConfig{ModelID: 1}}
	assert.Error(t, ver.ValidateBaseInfo())

	// model config 为空
	ver = &PromptEvaluatorVersion{MessageList: []*Message{{Role: RoleUser}}}
	assert.Error(t, ver.ValidateBaseInfo())

	// model id 为空
	ver = &PromptEvaluatorVersion{MessageList: []*Message{{Role: RoleUser}}, ModelConfig: &ModelConfig{}}
	assert.Error(t, ver.ValidateBaseInfo())

	// 正常
	ver = &PromptEvaluatorVersion{MessageList: []*Message{{Role: RoleUser}}, ModelConfig: &ModelConfig{ModelID: 1}}
	assert.NoError(t, ver.ValidateBaseInfo())
}
