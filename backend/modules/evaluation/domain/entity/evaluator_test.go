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
	promptVer := &PromptEvaluatorVersion{Version: "v1"}
	promptEval := &Evaluator{
		EvaluatorType:          EvaluatorTypePrompt,
		PromptEvaluatorVersion: promptVer,
	}
	ver := promptEval.GetEvaluatorVersion()
	assert.Equal(t, promptVer, ver)

	// 非Prompt类型
	codeEval := &Evaluator{EvaluatorType: EvaluatorTypeCode}
	assert.Nil(t, codeEval.GetEvaluatorVersion())

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

func TestEvaluatorRecord_GetScore(t *testing.T) {
	rec := &EvaluatorRecord{
		EvaluatorOutputData: &EvaluatorOutputData{
			EvaluatorResult: &EvaluatorResult{
				Correction: &Correction{
					Score: gptr.Of(0.9),
				},
			},
		},
	}
	assert.NotNil(t, rec.GetScore())
}

func TestEvaluatorRecord_GetReasoning(t *testing.T) {
	rec := &EvaluatorRecord{
		EvaluatorOutputData: &EvaluatorOutputData{
			EvaluatorResult: &EvaluatorResult{
				Correction: &Correction{
					Explain: "test",
				},
			},
		},
	}
	assert.NotNil(t, rec.GetReasoning())
}

func TestEvaluatorRecord_GetCorrected(t *testing.T) {
	rec := &EvaluatorRecord{
		EvaluatorOutputData: &EvaluatorOutputData{
			EvaluatorResult: &EvaluatorResult{
				Correction: &Correction{
					Explain: "test",
				},
			},
		},
	}
	assert.NotNil(t, rec.GetCorrected())
}

func TestPromptEvaluatorVersion_GetSetMethods(t *testing.T) {
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
