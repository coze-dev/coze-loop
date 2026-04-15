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
	assert.Equal(t, "VolcengineAgent", EvalTargetTypeVolcengineAgent.String())
	assert.Equal(t, "WebAgent", EvalTargetTypeWebAgent.String())
	assert.Equal(t, "CustomRPCServer", EvalTargetTypeCustomRPCServer.String())
	assert.Equal(t, "VolcengineAgentKit", EvalTargetTypeVolcengineAgentAgentkit.String())
	assert.Equal(t, "CozeBotOnline", EvalTargetTypeCozeBotOnline.String())
	assert.Equal(t, "CozeLoopPromptOnline", EvalTargetTypeCozeLoopPromptOnline.String())
	assert.Equal(t, "CozeWorkflowOnline", EvalTargetTypeCozeWorkflowOnline.String())
	assert.Equal(t, "VolcengineAgentOnline", EvalTargetTypeVolcengineAgentOnline.String())
	assert.Equal(t, "CustomRPCServerOnline", EvalTargetTypeCustomRPCServerOnline.String())
	assert.Equal(t, "VolcengineAgentAgentkitOnline", EvalTargetTypeVolcengineAgentAgentkitOnline.String())
	var unknown EvalTargetType = 99
	assert.Equal(t, "<UNSET>", unknown.String())
}

func TestEvalTargetType_SupptTrajectory(t *testing.T) {
	tests := []struct {
		name       string
		targetType EvalTargetType
		expected   bool
	}{
		{
			name:       "VolcengineAgent supports trajectory",
			targetType: EvalTargetTypeVolcengineAgent,
			expected:   true,
		},
		{
			name:       "CustomRPCServer supports trajectory",
			targetType: EvalTargetTypeCustomRPCServer,
			expected:   true,
		},
		{
			name:       "LoopPrompt supports trajectory",
			targetType: EvalTargetTypeLoopPrompt,
			expected:   true,
		},
		{
			name:       "CozeBot does not support trajectory",
			targetType: EvalTargetTypeCozeBot,
			expected:   false,
		},
		{
			name:       "LoopTrace does not support trajectory",
			targetType: EvalTargetTypeLoopTrace,
			expected:   false,
		},
		{
			name:       "CozeWorkflow does not support trajectory",
			targetType: EvalTargetTypeCozeWorkflow,
			expected:   false,
		},
		{
			name:       "VolcengineAgentAgentkit does not support trajectory",
			targetType: EvalTargetTypeVolcengineAgentAgentkit,
			expected:   false,
		},
		{
			name:       "WebAgent does not support trajectory",
			targetType: EvalTargetTypeWebAgent,
			expected:   false,
		},
		{
			name:       "Unknown type does not support trajectory",
			targetType: EvalTargetType(99),
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.targetType.SupptTrajectory())
		})
	}
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

func TestEvalTargetType_NeedExecuteTarget(t *testing.T) {
	tests := []struct {
		name       string
		targetType EvalTargetType
		expected   bool
	}{
		{
			name:       "CozeBot needs execute target",
			targetType: EvalTargetTypeCozeBot,
			expected:   true,
		},
		{
			name:       "LoopPrompt needs execute target",
			targetType: EvalTargetTypeLoopPrompt,
			expected:   true,
		},
		{
			name:       "LoopTrace needs execute target",
			targetType: EvalTargetTypeLoopTrace,
			expected:   true,
		},
		{
			name:       "CozeWorkflow needs execute target",
			targetType: EvalTargetTypeCozeWorkflow,
			expected:   true,
		},
		{
			name:       "VolcengineAgent needs execute target",
			targetType: EvalTargetTypeVolcengineAgent,
			expected:   true,
		},
		{
			name:       "CustomRPCServer needs execute target",
			targetType: EvalTargetTypeCustomRPCServer,
			expected:   true,
		},
		{
			name:       "VolcengineAgentAgentkit needs execute target",
			targetType: EvalTargetTypeVolcengineAgentAgentkit,
			expected:   true,
		},
		{
			name:       "CozeBotOnline does not need execute target",
			targetType: EvalTargetTypeCozeBotOnline,
			expected:   false,
		},
		{
			name:       "CozeLoopPromptOnline does not need execute target",
			targetType: EvalTargetTypeCozeLoopPromptOnline,
			expected:   false,
		},
		{
			name:       "CozeWorkflowOnline does not need execute target",
			targetType: EvalTargetTypeCozeWorkflowOnline,
			expected:   false,
		},
		{
			name:       "VolcengineAgentOnline does not need execute target",
			targetType: EvalTargetTypeVolcengineAgentOnline,
			expected:   false,
		},
		{
			name:       "CustomRPCServerOnline does not need execute target",
			targetType: EvalTargetTypeCustomRPCServerOnline,
			expected:   false,
		},
		{
			name:       "VolcengineAgentAgentkitOnline does not need execute target",
			targetType: EvalTargetTypeVolcengineAgentAgentkitOnline,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.targetType.NeedExecuteTarget())
		})
	}
}

func TestEvalTargetType_RecordOnlyTypeToBaseType(t *testing.T) {
	tests := []struct {
		name         string
		targetType   EvalTargetType
		expectedType EvalTargetType
		expectedOk   bool
	}{
		{
			name:         "CozeBotOnline to CozeBot",
			targetType:   EvalTargetTypeCozeBotOnline,
			expectedType: EvalTargetTypeCozeBot,
			expectedOk:   true,
		},
		{
			name:         "CozeLoopPromptOnline to LoopPrompt",
			targetType:   EvalTargetTypeCozeLoopPromptOnline,
			expectedType: EvalTargetTypeLoopPrompt,
			expectedOk:   true,
		},
		{
			name:         "CozeWorkflowOnline to CozeWorkflow",
			targetType:   EvalTargetTypeCozeWorkflowOnline,
			expectedType: EvalTargetTypeCozeWorkflow,
			expectedOk:   true,
		},
		{
			name:         "VolcengineAgentOnline to VolcengineAgent",
			targetType:   EvalTargetTypeVolcengineAgentOnline,
			expectedType: EvalTargetTypeVolcengineAgent,
			expectedOk:   true,
		},
		{
			name:         "CustomRPCServerOnline to CustomRPCServer",
			targetType:   EvalTargetTypeCustomRPCServerOnline,
			expectedType: EvalTargetTypeCustomRPCServer,
			expectedOk:   true,
		},
		{
			name:         "VolcengineAgentAgentkitOnline to VolcengineAgentAgentkit",
			targetType:   EvalTargetTypeVolcengineAgentAgentkitOnline,
			expectedType: EvalTargetTypeVolcengineAgentAgentkit,
			expectedOk:   true,
		},
		{
			name:         "CozeBot is not record only type",
			targetType:   EvalTargetTypeCozeBot,
			expectedType: 0,
			expectedOk:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseType, ok := tt.targetType.RecordOnlyTypeToBaseType()
			assert.Equal(t, tt.expectedType, baseType)
			assert.Equal(t, tt.expectedOk, ok)
		})
	}
}

func TestEvalTargetType_ToOperatorBaseType(t *testing.T) {
	assert.Equal(t, EvalTargetTypeLoopPrompt, EvalTargetTypeCozeLoopPromptOnline.ToOperatorBaseType())
	assert.Equal(t, EvalTargetTypeCozeBot, EvalTargetTypeCozeBotOnline.ToOperatorBaseType())
	assert.Equal(t, EvalTargetTypeCozeBot, EvalTargetTypeCozeBot.ToOperatorBaseType())
}

func TestEvalTargetType_BaseTypeToRecordOnlyType(t *testing.T) {
	tests := []struct {
		name         string
		targetType   EvalTargetType
		expectedType EvalTargetType
		expectedOk   bool
	}{
		{name: "CozeBot to CozeBotOnline", targetType: EvalTargetTypeCozeBot, expectedType: EvalTargetTypeCozeBotOnline, expectedOk: true},
		{name: "LoopPrompt to CozeLoopPromptOnline", targetType: EvalTargetTypeLoopPrompt, expectedType: EvalTargetTypeCozeLoopPromptOnline, expectedOk: true},
		{name: "CozeWorkflow to CozeWorkflowOnline", targetType: EvalTargetTypeCozeWorkflow, expectedType: EvalTargetTypeCozeWorkflowOnline, expectedOk: true},
		{name: "VolcengineAgent to VolcengineAgentOnline", targetType: EvalTargetTypeVolcengineAgent, expectedType: EvalTargetTypeVolcengineAgentOnline, expectedOk: true},
		{name: "CustomRPCServer to CustomRPCServerOnline", targetType: EvalTargetTypeCustomRPCServer, expectedType: EvalTargetTypeCustomRPCServerOnline, expectedOk: true},
		{name: "VolcengineAgentAgentkit to Online", targetType: EvalTargetTypeVolcengineAgentAgentkit, expectedType: EvalTargetTypeVolcengineAgentAgentkitOnline, expectedOk: true},
		{name: "LoopTrace no mapping", targetType: EvalTargetTypeLoopTrace, expectedType: 0, expectedOk: false},
		{name: "CozeBotOnline not base", targetType: EvalTargetTypeCozeBotOnline, expectedType: 0, expectedOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			onlineT, ok := tt.targetType.BaseTypeToRecordOnlyType()
			assert.Equal(t, tt.expectedType, onlineT)
			assert.Equal(t, tt.expectedOk, ok)
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
