// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAuthActionConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "AuthActionTraceRead",
			constant: AuthActionTraceRead,
			expected: "readLoopTrace",
		},
		{
			name:     "AuthActionTraceIngest",
			constant: AuthActionTraceIngest,
			expected: "ingestLoopTrace",
		},
		{
			name:     "AuthActionTraceViewCreate",
			constant: AuthActionTraceViewCreate,
			expected: "createLoopTraceView",
		},
		{
			name:     "AuthActionTraceViewList",
			constant: AuthActionTraceViewList,
			expected: "listLoopTraceView",
		},
		{
			name:     "AuthActionTraceViewEdit",
			constant: AuthActionTraceViewEdit,
			expected: "edit",
		},
		{
			name:     "AuthActionAnnotationCreate",
			constant: AuthActionAnnotationCreate,
			expected: "createLoopTraceAnnotation",
		},
		{
			name:     "AuthActionTraceExport",
			constant: AuthActionTraceExport,
			expected: "exportLoopTrace",
		},
		{
			name:     "AuthActionTracePreviewExport",
			constant: AuthActionTracePreviewExport,
			expected: "previewExportLoopTrace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.constant)
		})
	}
}

func TestAuthActionConstants_Uniqueness(t *testing.T) {
	// Test that all auth action constants are unique
	actions := []string{
		AuthActionTraceRead,
		AuthActionTraceIngest,
		AuthActionTraceViewCreate,
		AuthActionTraceViewList,
		AuthActionTraceViewEdit,
		AuthActionAnnotationCreate,
		AuthActionTraceExport,
		AuthActionTracePreviewExport,
	}

	seen := make(map[string]bool)
	for _, action := range actions {
		assert.False(t, seen[action], "Duplicate auth action constant found: %s", action)
		seen[action] = true
	}
}

func TestAuthActionConstants_NotEmpty(t *testing.T) {
	// Test that all auth action constants are not empty
	actions := []string{
		AuthActionTraceRead,
		AuthActionTraceIngest,
		AuthActionTraceViewCreate,
		AuthActionTraceViewList,
		AuthActionTraceViewEdit,
		AuthActionAnnotationCreate,
		AuthActionTraceExport,
		AuthActionTracePreviewExport,
	}

	for _, action := range actions {
		assert.NotEmpty(t, action, "Auth action constant should not be empty")
	}
}
