// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"

	"github.com/stretchr/testify/assert"
)

func TestEvalTargetType_String(t *testing.T) {
	assert.Equal(t, "CozeBot", EvalTargetTypeCozeBot.String())
	assert.Equal(t, "LoopPrompt", EvalTargetTypeLoopPrompt.String())
	assert.Equal(t, "LoopTrace", EvalTargetTypeLoopTrace.String())
	assert.Equal(t, "CozeWorkflow", EvalTargetTypeCozeWorkflow.String())
	var unknown EvalTargetType = 99
	assert.Equal(t, "<UNSET>", unknown.String())
}

func TestEvalTargetTypePtr_Value_Scan(t *testing.T) {
	v := EvalTargetTypeCozeBot
	ptr := EvalTargetTypePtr(v)
	assert.Equal(t, EvalTargetTypeCozeBot, *ptr)

	var typ EvalTargetType
	// Scan from int64
	assert.NoError(t, typ.Scan(int64(2)))
	assert.Equal(t, EvalTargetTypeLoopPrompt, typ)
	// Value
	val, err := typ.Value()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), val)
	// nil receiver
	var nilPtr *EvalTargetType
	val, err = nilPtr.Value()
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestEvalTargetInputData_ValidateInputSchema(t *testing.T) {
	// 空输入
	input := &EvalTargetInputData{InputFields: map[string]*Content{
		"input": {
			ContentType: gptr.Of(ContentTypeText),
			Text:        gptr.Of("hi"),
		},
	}}
	assert.NoError(t, input.ValidateInputSchema([]*ArgsSchema{
		{
			Key:                 gptr.Of("input"),
			SupportContentTypes: []ContentType{ContentTypeText},
			JsonSchema:          gptr.Of("{ \"type\": \"string\" }"),
		},
	}))
}

func TestCozeBotInfoTypeConsts(t *testing.T) {
	assert.Equal(t, int64(1), int64(CozeBotInfoTypeDraftBot))
	assert.Equal(t, int64(2), int64(CozeBotInfoTypeProductBot))
}

func TestLoopPromptConsts(t *testing.T) {
	assert.Equal(t, int64(0), int64(SubmitStatus_Undefined))
	assert.Equal(t, int64(1), int64(SubmitStatus_UnSubmit))
	assert.Equal(t, int64(2), int64(SubmitStatus_Submitted))
}

func TestEvalTargetVersion_RuntimeParamDemo(t *testing.T) {
	tests := []struct {
		name     string
		version  *EvalTargetVersion
		demo     *string
		expected *string
	}{
		{
			name:     "nil runtime param demo",
			version:  &EvalTargetVersion{RuntimeParamDemo: nil},
			demo:     nil,
			expected: nil,
		},
		{
			name:     "empty runtime param demo",
			version:  &EvalTargetVersion{},
			demo:     &[]string{""}[0],
			expected: &[]string{""}[0],
		},
		{
			name:     "normal runtime param demo",
			version:  &EvalTargetVersion{},
			demo:     &[]string{`{"model_config": {"model_id": "123"}}`}[0],
			expected: &[]string{`{"model_config": {"model_id": "123"}}`}[0],
		},
		{
			name:     "complex runtime param demo",
			version:  &EvalTargetVersion{},
			demo:     &[]string{`{"model_config": {"model_id": "123", "temperature": 0.7, "max_tokens": 100}}`}[0],
			expected: &[]string{`{"model_config": {"model_id": "123", "temperature": 0.7, "max_tokens": 100}}`}[0],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.version.RuntimeParamDemo = tt.demo
			assert.Equal(t, tt.expected, tt.version.RuntimeParamDemo)
		})
	}
}

func TestEvalTargetVersion_RuntimeParamDemo_Integration(t *testing.T) {
	// Test RuntimeParamDemo field integration with other EvalTargetVersion fields
	version := &EvalTargetVersion{
		ID:                  1,
		SpaceID:             100,
		TargetID:            200,
		SourceTargetVersion: "v1.0",
		EvalTargetType:      EvalTargetTypeLoopPrompt,
		RuntimeParamDemo:    &[]string{`{"model_config": {"model_id": "test_model", "temperature": 0.8}}`}[0],
		InputSchema: []*ArgsSchema{
			{
				Key:                 &[]string{"input_field"}[0],
				SupportContentTypes: []ContentType{ContentTypeText},
				JsonSchema:          &[]string{`{"type": "string"}`}[0],
			},
		},
		OutputSchema: []*ArgsSchema{
			{
				Key:                 &[]string{"output_field"}[0],
				SupportContentTypes: []ContentType{ContentTypeText},
				JsonSchema:          &[]string{`{"type": "string"}`}[0],
			},
		},
	}

	assert.Equal(t, int64(1), version.ID)
	assert.Equal(t, int64(100), version.SpaceID)
	assert.Equal(t, int64(200), version.TargetID)
	assert.Equal(t, "v1.0", version.SourceTargetVersion)
	assert.Equal(t, EvalTargetTypeLoopPrompt, version.EvalTargetType)
	assert.Equal(t, &[]string{`{"model_config": {"model_id": "test_model", "temperature": 0.8}}`}[0], version.RuntimeParamDemo)
	assert.Len(t, version.InputSchema, 1)
	assert.Len(t, version.OutputSchema, 1)
}
