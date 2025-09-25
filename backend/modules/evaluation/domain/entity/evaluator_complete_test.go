// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

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

func TestEvaluatorRecord_GetScore_EdgeCases(t *testing.T) {
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

func TestEvaluatorRecord_GetReasoning_EdgeCases(t *testing.T) {
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

func TestEvaluatorRecord_GetCorrected_EdgeCases(t *testing.T) {
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
		ver.SetCodeContent("print('hello')")
		assert.Equal(t, "print('hello')", ver.GetCodeContent())
		
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
			name:    "valid input",
			version: &CodeEvaluatorVersion{},
			input: &EvaluatorInputData{
				InputFields: map[string]*Content{
					"test": {ContentType: gptr.Of(ContentTypeText), Text: gptr.Of("test")},
				},
			},
			expectErr: false,
		},
		{
			name:      "nil input",
			version:   &CodeEvaluatorVersion{},
			input:     nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.version.ValidateInput(tt.input)
					if tt.expectErr {
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "input data is nil")
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
		errCode   int32
	}{
		{
			name:      "nil version",
			version:   nil,
			expectErr: true,
			errCode:   errno.EvaluatorNotExistCode,
		},
		{
			name: "empty code content",
			version: &CodeEvaluatorVersion{
				CodeContent:  "",
				LanguageType: LanguageTypePython,
			},
			expectErr: true,
			errCode:   errno.InvalidCodeContentCode,
		},
		{
			name: "invalid language type",
			version: &CodeEvaluatorVersion{
				CodeContent:  "print('hello')",
				LanguageType: "InvalidLang",
			},
			expectErr: true,
			errCode:   errno.InvalidLanguageTypeCode,
		},
		{
			name: "valid code evaluator - Python",
			version: &CodeEvaluatorVersion{
				CodeContent:  "print('hello')",
				LanguageType: LanguageTypePython,
			},
			expectErr: false,
		},
		{
			name: "valid code evaluator - JS",
			version: &CodeEvaluatorVersion{
				CodeContent:  "console.log('hello')",
				LanguageType: LanguageTypeJS,
			},
			expectErr: false,
		},
		{
			name: "valid code evaluator - case insensitive python",
			version: &CodeEvaluatorVersion{
				CodeContent:  "print('hello')",
				LanguageType: "python",
			},
			expectErr: false,
		},
		{
			name: "valid code evaluator - case insensitive javascript",
			version: &CodeEvaluatorVersion{
				CodeContent:  "console.log('hello')",
				LanguageType: "javascript",
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
			// Check specific error messages based on error code
			switch tt.errCode {
			case errno.EvaluatorNotExistCode:
				assert.Contains(t, err.Error(), "evaluator_version is nil")
			case errno.InvalidCodeContentCode:
				assert.Contains(t, err.Error(), "code content is empty")
			case errno.InvalidLanguageTypeCode:
				assert.Contains(t, err.Error(), "invalid language type")
			}
		} else {
			assert.NoError(t, err)
			// Check if language type was normalized
			if tt.version != nil {
				switch tt.version.LanguageType {
				case "python":
					assert.Equal(t, LanguageTypePython, tt.version.LanguageType)
				case "javascript":
					assert.Equal(t, LanguageTypeJS, tt.version.LanguageType)
				}
			}
		}
		})
	}
}

func TestNormalizeLanguageType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    LanguageType
		expected LanguageType
	}{
		{
			name:     "python lowercase",
			input:    "python",
			expected: LanguageTypePython,
		},
		{
			name:     "Python uppercase",
			input:    "Python",
			expected: LanguageTypePython,
		},
		{
			name:     "js lowercase",
			input:    "js",
			expected: LanguageTypeJS,
		},
		{
			name:     "javascript lowercase",
			input:    "javascript",
			expected: LanguageTypeJS,
		},
		{
			name:     "JavaScript mixed case",
			input:    "JavaScript",
			expected: LanguageTypeJS,
		},
		{
			name:     "unknown language",
			input:    "go",
			expected: "Go",
		},
		{
			name:     "empty language",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := normalizeLanguageType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContent_GetSetMethods(t *testing.T) {
	t.Parallel()
	
	t.Run("text operations", func(t *testing.T) {
		t.Parallel()
		content := &Content{}
		
		// Test GetText with nil text
		assert.Equal(t, "", content.GetText())
		
		// Test SetText
		content.SetText("test text")
		assert.Equal(t, "test text", content.GetText())
		assert.NotNil(t, content.Text)
		assert.Equal(t, "test text", *content.Text)
	})
	
	t.Run("content type operations", func(t *testing.T) {
		t.Parallel()
		content := &Content{}
		
		// Test GetContentType with nil content type
		assert.Equal(t, ContentType(""), content.GetContentType())
		
		// Test SetContentType
		content.SetContentType(ContentTypeText)
		assert.Equal(t, ContentTypeText, content.GetContentType())
		assert.NotNil(t, content.ContentType)
		assert.Equal(t, ContentTypeText, *content.ContentType)
	})
	
	t.Run("nil content operations", func(t *testing.T) {
		t.Parallel()
		var content *Content
		
		// Test GetText with nil content
		assert.Equal(t, "", content.GetText())
		
		// Test GetContentType with nil content
		assert.Equal(t, ContentType(""), content.GetContentType())
		
		// Test SetText with nil content (should not panic)
		content.SetText("test")
		assert.Nil(t, content)
		
		// Test SetContentType with nil content (should not panic)
		content.SetContentType(ContentTypeText)
		assert.Nil(t, content)
	})
}

func TestBaseInfo_GetSetMethods(t *testing.T) {
	t.Parallel()
	
	t.Run("created by operations", func(t *testing.T) {
		t.Parallel()
		baseInfo := &BaseInfo{}
		userInfo := &UserInfo{UserID: gptr.Of("user123")}
		
		// Test GetCreatedBy
		assert.Nil(t, baseInfo.GetCreatedBy())
		
		// Test SetCreatedBy
		baseInfo.SetCreatedBy(userInfo)
		assert.Equal(t, userInfo, baseInfo.GetCreatedBy())
		assert.Equal(t, userInfo, baseInfo.CreatedBy)
	})
	
	t.Run("updated by operations", func(t *testing.T) {
		t.Parallel()
		baseInfo := &BaseInfo{}
		userInfo := &UserInfo{UserID: gptr.Of("user456")}
		
		// Test GetUpdatedBy
		assert.Nil(t, baseInfo.GetUpdatedBy())
		
		// Test SetUpdatedBy
		baseInfo.SetUpdatedBy(userInfo)
		assert.Equal(t, userInfo, baseInfo.GetUpdatedBy())
		assert.Equal(t, userInfo, baseInfo.UpdatedBy)
	})
	
	t.Run("updated at operations", func(t *testing.T) {
		t.Parallel()
		baseInfo := &BaseInfo{}
		timestamp := gptr.Of(int64(1234567890))
		
		// Test SetUpdatedAt
		baseInfo.SetUpdatedAt(timestamp)
		assert.Equal(t, timestamp, baseInfo.UpdatedAt)
	})
}