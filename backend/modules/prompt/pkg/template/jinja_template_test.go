// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestInterpolateJinja2_BasicRendering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		templateStr string
		variables   map[string]any
		want        string
		wantErr     bool
	}{
		{
			name:        "simple variable",
			templateStr: "Hello, {{ name }}!",
			variables:   map[string]any{"name": "World"},
			want:        "Hello, World!",
		},
		{
			name:        "if condition",
			templateStr: "{% if show %}Visible{% else %}Hidden{% endif %}",
			variables:   map[string]any{"show": true},
			want:        "Visible",
		},
		{
			name:        "for loop with small range",
			templateStr: "{% for i in range(5) %}{{ i }},{% endfor %}",
			variables:   nil,
			want:        "0,1,2,3,4,",
		},
		{
			name:        "for loop with start and stop",
			templateStr: "{% for i in range(2, 6) %}{{ i }},{% endfor %}",
			variables:   nil,
			want:        "2,3,4,5,",
		},
		{
			name:        "for loop with step",
			templateStr: "{% for i in range(0, 10, 2) %}{{ i }},{% endfor %}",
			variables:   nil,
			want:        "0,2,4,6,8,",
		},
		{
			name:        "empty template",
			templateStr: "",
			variables:   nil,
			want:        "",
		},
		{
			name:        "static text",
			templateStr: "No variables here",
			variables:   nil,
			want:        "No variables here",
		},
		{
			name:        "for loop over list",
			templateStr: "{% for item in items %}{{ item }},{% endfor %}",
			variables:   map[string]any{"items": []string{"a", "b", "c"}},
			want:        "a,b,c,",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := InterpolateJinja2(tt.templateStr, tt.variables)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestInterpolateJinja2_SafeRange_BlocksLargeRange(t *testing.T) {
	t.Parallel()

	templateStr := "{% for i in range(100000) %}X{% endfor %}"
	_, err := InterpolateJinja2(templateStr, nil)
	assert.Error(t, err)
	statusErr, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, int32(prompterr.TemplateRenderErrorCode), statusErr.Code())
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestInterpolateJinja2_SafeRange_BlocksNestedDoS(t *testing.T) {
	t.Parallel()

	templateStr := "{% for i in range(10000000) %}{% for j in range(100000) %}DoS{% endfor %}{% endfor %}"
	_, err := InterpolateJinja2(templateStr, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestInterpolateJinja2_SafeRange_AllowsMaxBoundary(t *testing.T) {
	t.Parallel()

	templateStr := "{% for i in range(10000) %}X{% endfor %}"
	got, err := InterpolateJinja2(templateStr, nil)
	assert.NoError(t, err)
	assert.Equal(t, 10000, len(got))
}

func TestInterpolateJinja2_SafeRange_StepZero(t *testing.T) {
	t.Parallel()

	templateStr := "{% for i in range(0, 10, 0) %}X{% endfor %}"
	_, err := InterpolateJinja2(templateStr, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "step must not be zero")
}

func TestInterpolateJinja2_SafeRange_NegativeStep(t *testing.T) {
	t.Parallel()

	templateStr := "{% for i in range(10, 0, -1) %}{{ i }},{% endfor %}"
	got, err := InterpolateJinja2(templateStr, nil)
	assert.NoError(t, err)
	assert.Equal(t, "10,9,8,7,6,5,4,3,2,1,", got)
}

func TestInterpolateJinja2_OutputSizeLimit(t *testing.T) {
	t.Parallel()

	longValue := strings.Repeat("X", 512*1024)
	templateStr := "{{ a }}{{ a }}{{ a }}"
	_, err := InterpolateJinja2(templateStr, map[string]any{"a": longValue})
	assert.Error(t, err)
	statusErr, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, int32(prompterr.TemplateRenderErrorCode), statusErr.Code())
}

func TestInterpolateJinja2_DisabledControlStructures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		templateStr string
	}{
		{
			name:        "include is disabled",
			templateStr: `{% include "other.html" %}`,
		},
		{
			name:        "extends is disabled",
			templateStr: `{% extends "base.html" %}`,
		},
		{
			name:        "import is disabled",
			templateStr: `{% import "macros.html" as macros %}`,
		},
		{
			name:        "from is disabled",
			templateStr: `{% from "macros.html" import hello %}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := InterpolateJinja2(tt.templateStr, nil)
			assert.Error(t, err)
		})
	}
}

func TestInterpolateJinja2_ParseError(t *testing.T) {
	t.Parallel()

	_, err := InterpolateJinja2("{% if %}", nil)
	assert.Error(t, err)
	statusErr, ok := errorx.FromStatusError(err)
	assert.True(t, ok)
	assert.Equal(t, int32(prompterr.TemplateParseErrorCode), statusErr.Code())
}
