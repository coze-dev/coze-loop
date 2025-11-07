// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consts

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
)

func TestInputFieldKeyPromptUserQuery(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "verify InputFieldKeyPromptUserQuery constant value",
			expected: "builtin_prompt_user_query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, InputFieldKeyPromptUserQuery)
		})
	}
}

func TestInputFieldKeyPromptUserQueryConsistency(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{
			name:     "verify consistency with expt.PromptUserQueryFieldKey",
			expected: expt.PromptUserQueryFieldKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, InputFieldKeyPromptUserQuery)
			assert.Equal(t, InputFieldKeyPromptUserQuery, expt.PromptUserQueryFieldKey)
		})
	}
}