// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseEvaluatorScoreFieldKey_PureNumber(t *testing.T) {
	verID, alias, err := ParseEvaluatorScoreFieldKey("12345")
	assert.NoError(t, err)
	assert.Equal(t, int64(12345), verID)
	assert.Equal(t, "", alias)
}

func TestParseEvaluatorScoreFieldKey_VerIDAlias(t *testing.T) {
	verID, alias, err := ParseEvaluatorScoreFieldKey("12345:judge_A")
	assert.NoError(t, err)
	assert.Equal(t, int64(12345), verID)
	assert.Equal(t, "judge_A", alias)
}

func TestParseEvaluatorScoreFieldKey_EmptyAlias(t *testing.T) {
	// "12345:" 是合法的: alias 为空串
	verID, alias, err := ParseEvaluatorScoreFieldKey("12345:")
	assert.NoError(t, err)
	assert.Equal(t, int64(12345), verID)
	assert.Equal(t, "", alias)
}

func TestParseEvaluatorScoreFieldKey_AliasWithColon(t *testing.T) {
	// alias 含 ":" (SplitN by IndexByte 仅切第一个冒号)
	verID, alias, err := ParseEvaluatorScoreFieldKey("99:a:b:c")
	assert.NoError(t, err)
	assert.Equal(t, int64(99), verID)
	assert.Equal(t, "a:b:c", alias)
}

func TestParseEvaluatorScoreFieldKey_InvalidNonNumeric(t *testing.T) {
	_, _, err := ParseEvaluatorScoreFieldKey("not_a_number")
	assert.Error(t, err)
}

func TestParseEvaluatorScoreFieldKey_InvalidAliasFormPrefix(t *testing.T) {
	_, _, err := ParseEvaluatorScoreFieldKey("abc:judge_A")
	assert.Error(t, err)
}

func TestParseEvaluatorScoreFieldKey_Empty(t *testing.T) {
	_, _, err := ParseEvaluatorScoreFieldKey("")
	assert.Error(t, err)
}
