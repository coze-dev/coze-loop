// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makePromptEvaluatorVersion() *PromptEvaluatorVersion {
	return &PromptEvaluatorVersion{
		ID:                 11,
		SpaceID:            22,
		EvaluatorID:        33,
		Version:            "v1",
		Description:        "pd",
		PromptTemplateKey:  "ptk",
		BaseInfo:           &BaseInfo{},
		ModelConfig:        &ModelConfig{},
		ReceiveChatHistory: gptr.Of(true),
	}
}

func makeCodeEvaluatorVersion() *CodeEvaluatorVersion {
	return &CodeEvaluatorVersion{
		ID:           44,
		SpaceID:      55,
		EvaluatorID:  66,
		Version:      "v2",
		Description:  "cd",
		LanguageType: LanguageTypePython,
	}
}

func makeCustomRPCEvaluatorVersion() *CustomRPCEvaluatorVersion {
	return &CustomRPCEvaluatorVersion{
		ID:          77,
		SpaceID:     88,
		EvaluatorID: 99,
		Version:     "v3",
		Description: "rd",
	}
}

func TestEvaluator_Getters_Setters_Prompt(t *testing.T) {
	t.Parallel()
	e := &Evaluator{EvaluatorType: EvaluatorTypePrompt, PromptEvaluatorVersion: makePromptEvaluatorVersion(), Description: "desc"}
	assert.Equal(t, int64(11), e.GetEvaluatorVersionID())
	assert.Equal(t, "v1", e.GetVersion())
	assert.Equal(t, int64(33), e.GetEvaluatorID())
	assert.Equal(t, int64(22), e.GetSpaceID())
	assert.Equal(t, "desc", e.GetEvaluatorDescription())
	assert.Equal(t, "pd", e.GetEvaluatorVersionDescription())
	assert.NotNil(t, e.GetBaseInfo())
	assert.Equal(t, "ptk", e.GetPromptTemplateKey())
	assert.NotNil(t, e.GetModelConfig())

	e.SetEvaluatorVersionID(101)
	e.SetVersion("v101")
	e.SetEvaluatorDescription("d2")
	e.SetEvaluatorVersionDescription("pd2")
	e.SetBaseInfo(&BaseInfo{})
	e.SetTools([]*Tool{})
	e.SetPromptSuffix("suf")
	e.SetParseType(ParseTypeContent)
	e.SetEvaluatorID(202)
	e.SetSpaceID(303)

	assert.Equal(t, int64(101), e.GetEvaluatorVersionID())
	assert.Equal(t, "v101", e.GetVersion())
	assert.Equal(t, "d2", e.GetEvaluatorDescription())
	assert.Equal(t, "pd2", e.GetEvaluatorVersionDescription())
	assert.Equal(t, int64(202), e.GetEvaluatorID())
	assert.Equal(t, int64(303), e.GetSpaceID())
}

func TestEvaluator_Getters_Setters_Code(t *testing.T) {
	t.Parallel()
	e := &Evaluator{EvaluatorType: EvaluatorTypeCode, CodeEvaluatorVersion: makeCodeEvaluatorVersion(), Description: "desc"}
	assert.Equal(t, int64(44), e.GetEvaluatorVersionID())
	assert.Equal(t, "v2", e.GetVersion())
	assert.Equal(t, int64(66), e.GetEvaluatorID())
	assert.Equal(t, int64(55), e.GetSpaceID())
	assert.Equal(t, "desc", e.GetEvaluatorDescription())
	assert.Equal(t, "cd", e.GetEvaluatorVersionDescription())
	assert.Nil(t, e.GetModelConfig())

	e.SetEvaluatorVersionID(404)
	e.SetVersion("v404")
	e.SetEvaluatorVersionDescription("cd2")
	e.SetBaseInfo(&BaseInfo{})
	e.SetEvaluatorID(505)
	e.SetSpaceID(606)
	assert.Equal(t, int64(404), e.GetEvaluatorVersionID())
	assert.Equal(t, "v404", e.GetVersion())
	assert.Equal(t, "cd2", e.GetEvaluatorVersionDescription())
	assert.Equal(t, int64(505), e.GetEvaluatorID())
	assert.Equal(t, int64(606), e.GetSpaceID())
}

func TestEvaluator_Getters_Setters_CustomRPC(t *testing.T) {
	t.Parallel()
	e := &Evaluator{EvaluatorType: EvaluatorTypeCustomRPC, CustomRPCEvaluatorVersion: makeCustomRPCEvaluatorVersion(), Description: "desc"}
	assert.Equal(t, int64(77), e.GetEvaluatorVersionID())
	assert.Equal(t, "v3", e.GetVersion())
	assert.Equal(t, int64(99), e.GetEvaluatorID())
	assert.Equal(t, int64(88), e.GetSpaceID())
	assert.Equal(t, "desc", e.GetEvaluatorDescription())
	assert.Equal(t, "rd", e.GetEvaluatorVersionDescription())

	e.SetEvaluatorVersionID(707)
	e.SetVersion("v707")
	e.SetEvaluatorVersionDescription("rd2")
	e.SetBaseInfo(&BaseInfo{})
	e.SetEvaluatorID(808)
	e.SetSpaceID(909)
	assert.Equal(t, int64(707), e.GetEvaluatorVersionID())
	assert.Equal(t, "v707", e.GetVersion())
	assert.Equal(t, "rd2", e.GetEvaluatorVersionDescription())
	assert.Equal(t, int64(808), e.GetEvaluatorID())
	assert.Equal(t, int64(909), e.GetSpaceID())
}

func TestEvaluator_DefaultBranches(t *testing.T) {
	t.Parallel()
	// 无版本对象
	e := &Evaluator{EvaluatorType: EvaluatorTypePrompt}
	assert.Equal(t, int64(0), e.GetEvaluatorVersionID())
	assert.Equal(t, "", e.GetVersion())
	assert.Equal(t, int64(0), e.GetEvaluatorID())
	assert.Equal(t, int64(0), e.GetSpaceID())
	assert.Equal(t, "", e.GetEvaluatorVersionDescription())
	assert.Nil(t, e.GetBaseInfo())
	assert.Equal(t, "", e.GetPromptTemplateKey())
	assert.Nil(t, e.GetModelConfig())

	// 未知类型
	e = &Evaluator{EvaluatorType: 999}
	assert.Equal(t, int64(0), e.GetEvaluatorVersionID())
	assert.Equal(t, "", e.GetVersion())
	assert.Equal(t, int64(0), e.GetEvaluatorID())
	assert.Equal(t, int64(0), e.GetSpaceID())
	assert.Equal(t, "", e.GetEvaluatorVersionDescription())
	assert.Nil(t, e.GetBaseInfo())
}

// 下面追加的测试沿用同一文件与包，不重复声明

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
		ModelID:   gptr.Of(int64(123)),
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
					ModelConfig: &ModelConfig{ModelID: gptr.Of(int64(123))},
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
		{
			name: "nil version no panic",
			evaluator: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{Version: "keep"},
			},
			version: nil,
			verify: func(t *testing.T, e *Evaluator) {
				require.NotNil(t, e.PromptEvaluatorVersion)
				assert.Equal(t, "keep", e.PromptEvaluatorVersion.Version)
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
	mc := &ModelConfig{ModelID: gptr.Of(int64(123))}
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

	err = ver.ValidateInput(nil)
	assert.Error(t, err)
}

func TestPromptEvaluatorVersion_ValidateBaseInfo(t *testing.T) {
	// nil
	var ver *PromptEvaluatorVersion
	assert.Error(t, ver.ValidateBaseInfo())

	// message list 为空
	ver = &PromptEvaluatorVersion{ModelConfig: &ModelConfig{ModelID: gptr.Of(int64(1))}}
	assert.Error(t, ver.ValidateBaseInfo())

	// model config 为空
	ver = &PromptEvaluatorVersion{MessageList: []*Message{{Role: RoleUser}}}
	assert.Error(t, ver.ValidateBaseInfo())

	// model id 为空
	ver = &PromptEvaluatorVersion{MessageList: []*Message{{Role: RoleUser}}, ModelConfig: &ModelConfig{}}
	assert.Error(t, ver.ValidateBaseInfo())

	// 正常
	ver = &PromptEvaluatorVersion{MessageList: []*Message{{Role: RoleUser}}, ModelConfig: &ModelConfig{ModelID: gptr.Of(int64(1))}}
	assert.NoError(t, ver.ValidateBaseInfo())
}

func TestEvaluator_VersionDelegation_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		e    *Evaluator
		want struct {
			versionID int64
			version   string
			evalID    int64
			spaceID   int64
			verDesc   string
			hasBase   bool
			createdAt int64
			ptk       string
			hasModel  bool
		}
	}{
		{
			name: "prompt delegates",
			e: func() *Evaluator {
				base := &BaseInfo{CreatedAt: gptr.Of(int64(1))}
				return &Evaluator{
					Description:   "meta-desc",
					EvaluatorType: EvaluatorTypePrompt,
					PromptEvaluatorVersion: &PromptEvaluatorVersion{
						ID:                11,
						Version:           "v1",
						EvaluatorID:       101,
						SpaceID:           201,
						Description:       "ver-desc",
						BaseInfo:          base,
						PromptTemplateKey: "ptk",
						ModelConfig:       &ModelConfig{},
					},
				}
			}(),
			want: struct {
				versionID int64
				version   string
				evalID    int64
				spaceID   int64
				verDesc   string
				hasBase   bool
				createdAt int64
				ptk       string
				hasModel  bool
			}{versionID: 11, version: "v1", evalID: 101, spaceID: 201, verDesc: "ver-desc", hasBase: true, createdAt: 1, ptk: "ptk", hasModel: true},
		},
		{
			name: "code delegates",
			e: func() *Evaluator {
				base := &BaseInfo{CreatedAt: gptr.Of(int64(2))}
				return &Evaluator{
					EvaluatorType: EvaluatorTypeCode,
					CodeEvaluatorVersion: &CodeEvaluatorVersion{
						ID:          12,
						Version:     "v2",
						EvaluatorID: 102,
						SpaceID:     202,
						Description: "d2",
						BaseInfo:    base,
					},
				}
			}(),
			want: struct {
				versionID int64
				version   string
				evalID    int64
				spaceID   int64
				verDesc   string
				hasBase   bool
				createdAt int64
				ptk       string
				hasModel  bool
			}{versionID: 12, version: "v2", evalID: 102, spaceID: 202, verDesc: "d2", hasBase: true, createdAt: 2},
		},
		{
			name: "custom rpc delegates",
			e: func() *Evaluator {
				base := &BaseInfo{CreatedAt: gptr.Of(int64(3))}
				return &Evaluator{
					EvaluatorType: EvaluatorTypeCustomRPC,
					CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
						ID:          13,
						Version:     "v3",
						EvaluatorID: 103,
						SpaceID:     203,
						Description: "d3",
						BaseInfo:    base,
					},
				}
			}(),
			want: struct {
				versionID int64
				version   string
				evalID    int64
				spaceID   int64
				verDesc   string
				hasBase   bool
				createdAt int64
				ptk       string
				hasModel  bool
			}{versionID: 13, version: "v3", evalID: 103, spaceID: 203, verDesc: "d3", hasBase: true, createdAt: 3},
		},
		{
			name: "agent delegates",
			e: func() *Evaluator {
				base := &BaseInfo{CreatedAt: gptr.Of(int64(4))}
				return &Evaluator{
					EvaluatorType: EvaluatorTypeAgent,
					AgentEvaluatorVersion: &AgentEvaluatorVersion{
						ID:          14,
						Version:     "v4",
						EvaluatorID: 104,
						SpaceID:     204,
						Description: "d4",
						BaseInfo:    base,
					},
				}
			}(),
			want: struct {
				versionID int64
				version   string
				evalID    int64
				spaceID   int64
				verDesc   string
				hasBase   bool
				createdAt int64
				ptk       string
				hasModel  bool
			}{versionID: 14, version: "v4", evalID: 104, spaceID: 204, verDesc: "d4", hasBase: true, createdAt: 4},
		},
		{
			name: "unknown type returns zero values",
			e: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
		},
		{
			name: "nil version returns zero values",
			e: &Evaluator{
				EvaluatorType: EvaluatorTypeAgent,
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want.versionID, tc.e.GetEvaluatorVersionID())
			assert.Equal(t, tc.want.version, tc.e.GetVersion())
			assert.Equal(t, tc.want.evalID, tc.e.GetEvaluatorID())
			assert.Equal(t, tc.want.spaceID, tc.e.GetSpaceID())
			assert.Equal(t, tc.want.verDesc, tc.e.GetEvaluatorVersionDescription())
			if tc.want.hasBase {
				if assert.NotNil(t, tc.e.GetBaseInfo()) {
					assert.Equal(t, tc.want.createdAt, gptr.Indirect(tc.e.GetBaseInfo().CreatedAt))
				}
			} else {
				assert.Nil(t, tc.e.GetBaseInfo())
			}
			assert.Equal(t, tc.want.ptk, tc.e.GetPromptTemplateKey())
			if tc.want.hasModel {
				assert.NotNil(t, tc.e.GetModelConfig())
			} else if tc.e.EvaluatorType != EvaluatorTypePrompt {
				assert.Nil(t, tc.e.GetModelConfig())
			}
		})
	}
}

func TestEvaluator_SettersDelegate_Extra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		e    *Evaluator
	}{
		{name: "prompt", e: &Evaluator{EvaluatorType: EvaluatorTypePrompt, PromptEvaluatorVersion: &PromptEvaluatorVersion{}}},
		{name: "code", e: &Evaluator{EvaluatorType: EvaluatorTypeCode, CodeEvaluatorVersion: &CodeEvaluatorVersion{}}},
		{name: "custom rpc", e: &Evaluator{EvaluatorType: EvaluatorTypeCustomRPC, CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{}}},
		{name: "agent", e: &Evaluator{EvaluatorType: EvaluatorTypeAgent, AgentEvaluatorVersion: &AgentEvaluatorVersion{}}},
		{name: "unknown", e: &Evaluator{EvaluatorType: EvaluatorType(999)}},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			base := &BaseInfo{CreatedAt: gptr.Of(int64(1))}
			tc.e.SetEvaluatorVersionID(999)
			tc.e.SetVersion("v")
			tc.e.SetEvaluatorID(101)
			tc.e.SetSpaceID(202)
			tc.e.SetEvaluatorDescription("meta-desc")
			tc.e.SetEvaluatorVersionDescription("ver-desc")
			tc.e.SetBaseInfo(base)

			switch tc.e.EvaluatorType {
			case EvaluatorTypePrompt:
				assert.Equal(t, int64(999), tc.e.PromptEvaluatorVersion.ID)
				assert.Equal(t, "v", tc.e.PromptEvaluatorVersion.Version)
				assert.Equal(t, int64(101), tc.e.PromptEvaluatorVersion.EvaluatorID)
				assert.Equal(t, int64(202), tc.e.PromptEvaluatorVersion.SpaceID)
				assert.Equal(t, "ver-desc", tc.e.PromptEvaluatorVersion.Description)
				assert.Equal(t, base, tc.e.PromptEvaluatorVersion.BaseInfo)
			case EvaluatorTypeCode:
				assert.Equal(t, int64(999), tc.e.CodeEvaluatorVersion.ID)
				assert.Equal(t, "v", tc.e.CodeEvaluatorVersion.Version)
				assert.Equal(t, int64(101), tc.e.CodeEvaluatorVersion.EvaluatorID)
				assert.Equal(t, int64(202), tc.e.CodeEvaluatorVersion.SpaceID)
				assert.Equal(t, "ver-desc", tc.e.CodeEvaluatorVersion.Description)
				assert.Equal(t, base, tc.e.CodeEvaluatorVersion.BaseInfo)
			case EvaluatorTypeCustomRPC:
				assert.Equal(t, int64(999), tc.e.CustomRPCEvaluatorVersion.ID)
				assert.Equal(t, "v", tc.e.CustomRPCEvaluatorVersion.Version)
				assert.Equal(t, int64(101), tc.e.CustomRPCEvaluatorVersion.EvaluatorID)
				assert.Equal(t, int64(202), tc.e.CustomRPCEvaluatorVersion.SpaceID)
				assert.Equal(t, "ver-desc", tc.e.CustomRPCEvaluatorVersion.Description)
				assert.Equal(t, base, tc.e.CustomRPCEvaluatorVersion.BaseInfo)
			case EvaluatorTypeAgent:
				assert.Equal(t, int64(999), tc.e.AgentEvaluatorVersion.ID)
				assert.Equal(t, "v", tc.e.AgentEvaluatorVersion.Version)
				assert.Equal(t, int64(101), tc.e.AgentEvaluatorVersion.EvaluatorID)
				assert.Equal(t, int64(202), tc.e.AgentEvaluatorVersion.SpaceID)
				assert.Equal(t, "ver-desc", tc.e.AgentEvaluatorVersion.Description)
				assert.Equal(t, base, tc.e.AgentEvaluatorVersion.BaseInfo)
			default:
				assert.Equal(t, int64(0), tc.e.GetEvaluatorVersionID())
			}
		})
	}
}

func TestEvaluator_GetRateLimit(t *testing.T) {
	t.Parallel()

	rl := &RateLimit{Rate: gptr.Of(int32(10))}

	tests := []struct {
		name     string
		e        *Evaluator
		expected *RateLimit
	}{
		{
			name: "custom rpc with rate limit",
			e: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					RateLimit: rl,
				},
			},
			expected: rl,
		},
		{
			name: "custom rpc with nil rate limit",
			e: &Evaluator{
				EvaluatorType:             EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{},
			},
			expected: nil,
		},
		{
			name: "custom rpc with nil version",
			e: &Evaluator{
				EvaluatorType:             EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "prompt type returns nil",
			e: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{},
			},
			expected: nil,
		},
		{
			name: "code type returns nil",
			e: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			expected: nil,
		},
		{
			name: "agent type returns nil",
			e: &Evaluator{
				EvaluatorType:         EvaluatorTypeAgent,
				AgentEvaluatorVersion: &AgentEvaluatorVersion{},
			},
			expected: nil,
		},
		{
			name: "unknown type returns nil",
			e: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.e.GetRateLimit()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_GetInputSchemas(t *testing.T) {
	t.Parallel()

	schemas := []*ArgsSchema{
		{Key: gptr.Of("field1"), SupportContentTypes: []ContentType{ContentTypeText}},
	}

	tests := []struct {
		name     string
		e        *Evaluator
		expected []*ArgsSchema
	}{
		{
			name: "prompt with schemas",
			e: &Evaluator{
				EvaluatorType: EvaluatorTypePrompt,
				PromptEvaluatorVersion: &PromptEvaluatorVersion{
					InputSchemas: schemas,
				},
			},
			expected: schemas,
		},
		{
			name: "prompt with nil version",
			e: &Evaluator{
				EvaluatorType:          EvaluatorTypePrompt,
				PromptEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "code type returns nil",
			e: &Evaluator{
				EvaluatorType:        EvaluatorTypeCode,
				CodeEvaluatorVersion: &CodeEvaluatorVersion{},
			},
			expected: nil,
		},
		{
			name: "custom rpc with schemas",
			e: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					InputSchemas: schemas,
				},
			},
			expected: schemas,
		},
		{
			name: "custom rpc with nil version",
			e: &Evaluator{
				EvaluatorType:             EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "agent with schemas",
			e: &Evaluator{
				EvaluatorType: EvaluatorTypeAgent,
				AgentEvaluatorVersion: &AgentEvaluatorVersion{
					InputSchemas: schemas,
				},
			},
			expected: schemas,
		},
		{
			name: "agent with nil version",
			e: &Evaluator{
				EvaluatorType:         EvaluatorTypeAgent,
				AgentEvaluatorVersion: nil,
			},
			expected: nil,
		},
		{
			name: "unknown type returns nil",
			e: &Evaluator{
				EvaluatorType: EvaluatorType(999),
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.e.GetInputSchemas()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_ValidateInput_CustomRPCAndAgent(t *testing.T) {
	t.Parallel()

	validInput := &EvaluatorInputData{
		InputFields: map[string]*Content{
			"f1": {ContentType: gptr.Of(ContentTypeText), Text: gptr.Of(`"hello"`)},
		},
	}

	tests := []struct {
		name      string
		evaluator *Evaluator
		input     *EvaluatorInputData
		expectErr bool
	}{
		{
			name: "custom rpc with valid version delegates",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					InputSchemas: []*ArgsSchema{
						{Key: gptr.Of("f1"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of(`{"type": "string"}`)},
					},
				},
			},
			input:     validInput,
			expectErr: false,
		},
		{
			name: "custom rpc with nil input returns error",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					InputSchemas: []*ArgsSchema{},
				},
			},
			input:     nil,
			expectErr: true,
		},
		{
			name: "agent with valid version delegates",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeAgent,
				AgentEvaluatorVersion: &AgentEvaluatorVersion{
					InputSchemas: []*ArgsSchema{
						{Key: gptr.Of("f1"), SupportContentTypes: []ContentType{ContentTypeText}, JsonSchema: gptr.Of(`{"type": "string"}`)},
					},
				},
			},
			input:     validInput,
			expectErr: false,
		},
		{
			name: "agent with nil input returns error",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeAgent,
				AgentEvaluatorVersion: &AgentEvaluatorVersion{
					InputSchemas: []*ArgsSchema{},
				},
			},
			input:     nil,
			expectErr: true,
		},
		{
			name: "agent with nil version returns nil",
			evaluator: &Evaluator{
				EvaluatorType:         EvaluatorTypeAgent,
				AgentEvaluatorVersion: nil,
			},
			input:     validInput,
			expectErr: false,
		},
		{
			name: "custom rpc with nil version returns nil",
			evaluator: &Evaluator{
				EvaluatorType:             EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: nil,
			},
			input:     validInput,
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

func TestEvaluator_ValidateBaseInfo_CustomRPCAndAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		evaluator *Evaluator
		expectErr bool
	}{
		{
			name: "custom rpc with valid version delegates",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					AccessProtocol: EvaluatorAccessProtocolRPC,
					ServiceName:    gptr.Of("svc"),
				},
			},
			expectErr: false,
		},
		{
			name: "custom rpc with invalid version returns error",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					AccessProtocol: "",
				},
			},
			expectErr: true,
		},
		{
			name: "custom rpc with nil version returns nil",
			evaluator: &Evaluator{
				EvaluatorType:             EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: nil,
			},
			expectErr: false,
		},
		{
			name: "agent with valid version delegates",
			evaluator: &Evaluator{
				EvaluatorType:         EvaluatorTypeAgent,
				AgentEvaluatorVersion: &AgentEvaluatorVersion{},
			},
			expectErr: false,
		},
		{
			name: "agent with nil version returns nil",
			evaluator: &Evaluator{
				EvaluatorType:         EvaluatorTypeAgent,
				AgentEvaluatorVersion: nil,
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

func TestEvaluator_SetEvaluatorVersion_CustomRPCAndAgent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		evaluator *Evaluator
		version   *Evaluator
		verify    func(*testing.T, *Evaluator)
	}{
		{
			name: "set custom rpc evaluator version",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeCustomRPC,
			},
			version: &Evaluator{
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{
					Version: "v3.0.0",
					ID:      789,
				},
			},
			verify: func(t *testing.T, e *Evaluator) {
				assert.NotNil(t, e.CustomRPCEvaluatorVersion)
				assert.Equal(t, "v3.0.0", e.CustomRPCEvaluatorVersion.Version)
				assert.Equal(t, int64(789), e.CustomRPCEvaluatorVersion.ID)
			},
		},
		{
			name: "set agent evaluator version",
			evaluator: &Evaluator{
				EvaluatorType: EvaluatorTypeAgent,
			},
			version: &Evaluator{
				AgentEvaluatorVersion: &AgentEvaluatorVersion{
					Version: "v4.0.0",
					ID:      101,
				},
			},
			verify: func(t *testing.T, e *Evaluator) {
				assert.NotNil(t, e.AgentEvaluatorVersion)
				assert.Equal(t, "v4.0.0", e.AgentEvaluatorVersion.Version)
				assert.Equal(t, int64(101), e.AgentEvaluatorVersion.ID)
			},
		},
		{
			name: "nil version for custom rpc no panic",
			evaluator: &Evaluator{
				EvaluatorType:             EvaluatorTypeCustomRPC,
				CustomRPCEvaluatorVersion: &CustomRPCEvaluatorVersion{Version: "keep"},
			},
			version: nil,
			verify: func(t *testing.T, e *Evaluator) {
				require.NotNil(t, e.CustomRPCEvaluatorVersion)
				assert.Equal(t, "keep", e.CustomRPCEvaluatorVersion.Version)
			},
		},
		{
			name: "nil version for agent no panic",
			evaluator: &Evaluator{
				EvaluatorType:         EvaluatorTypeAgent,
				AgentEvaluatorVersion: &AgentEvaluatorVersion{Version: "keep"},
			},
			version: nil,
			verify: func(t *testing.T, e *Evaluator) {
				require.NotNil(t, e.AgentEvaluatorVersion)
				assert.Equal(t, "keep", e.AgentEvaluatorVersion.Version)
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
