// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"testing"

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
	input := &EvalTargetInputData{InputFields: map[string]*Content{}}
	assert.NoError(t, input.ValidateInputSchema(nil))
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
