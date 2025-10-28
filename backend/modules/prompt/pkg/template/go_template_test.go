// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/stretchr/testify/assert"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestInterpolateGoTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		templateStr string
		variables   map[string]any
		want        string
		wantErr     bool
		errCode     int32
	}{
		{
			name:        "simple string interpolation",
			templateStr: "Hello, {{.name}}!",
			variables:   map[string]any{"name": "World"},
			want:        "Hello, World!",
			wantErr:     false,
		},
		{
			name:        "multiple variables",
			templateStr: "{{.greeting}}, {{.name}}! You are {{.age}} years old.",
			variables: map[string]any{
				"greeting": "Hi",
				"name":     "Alice",
				"age":      30,
			},
			want:    "Hi, Alice! You are 30 years old.",
			wantErr: false,
		},
		{
			name:        "integer variable",
			templateStr: "The answer is {{.number}}",
			variables:   map[string]any{"number": 42},
			want:        "The answer is 42",
			wantErr:     false,
		},
		{
			name:        "float variable",
			templateStr: "Pi is approximately {{.pi}}",
			variables:   map[string]any{"pi": 3.14159},
			want:        "Pi is approximately 3.14159",
			wantErr:     false,
		},
		{
			name:        "boolean variable",
			templateStr: "Is active: {{.active}}",
			variables:   map[string]any{"active": true},
			want:        "Is active: true",
			wantErr:     false,
		},
		{
			name:        "empty template",
			templateStr: "",
			variables:   map[string]any{},
			want:        "",
			wantErr:     false,
		},
		{
			name:        "template with no variables",
			templateStr: "Static text without variables",
			variables:   map[string]any{},
			want:        "Static text without variables",
			wantErr:     false,
		},
		{
			name:        "empty variables map",
			templateStr: "Hello, World!",
			variables:   map[string]any{},
			want:        "Hello, World!",
			wantErr:     false,
		},
		{
			name:        "nil variables map",
			templateStr: "Hello, World!",
			variables:   nil,
			want:        "Hello, World!",
			wantErr:     false,
		},
		{
			name:        "conditional in template",
			templateStr: "{{if .show}}Visible{{else}}Hidden{{end}}",
			variables:   map[string]any{"show": true},
			want:        "Visible",
			wantErr:     false,
		},
		{
			name:        "range over slice",
			templateStr: "{{range .items}}{{.}},{{end}}",
			variables:   map[string]any{"items": []string{"a", "b", "c"}},
			want:        "a,b,c,",
			wantErr:     false,
		},
		{
			name:        "nested object access",
			templateStr: "{{.user.name}} is {{.user.age}} years old",
			variables: map[string]any{
				"user": map[string]any{
					"name": "Bob",
					"age":  25,
				},
			},
			want:    "Bob is 25 years old",
			wantErr: false,
		},
		{
			name:        "template with newlines",
			templateStr: "Line 1: {{.line1}}\nLine 2: {{.line2}}",
			variables: map[string]any{
				"line1": "First",
				"line2": "Second",
			},
			want:    "Line 1: First\nLine 2: Second",
			wantErr: false,
		},
		{
			name:        "template parse error - unclosed action",
			templateStr: "Hello, {{.name!",
			variables:   map[string]any{"name": "World"},
			want:        "",
			wantErr:     true,
			errCode:     prompterr.TemplateParseErrorCode,
		},
		{
			name:        "template parse error - invalid syntax",
			templateStr: "Hello, {{..name}}",
			variables:   map[string]any{"name": "World"},
			want:        "",
			wantErr:     true,
			errCode:     prompterr.TemplateParseErrorCode,
		},
		{
			name:        "missing variable returns no value",
			templateStr: "Hello, {{.name}}!",
			variables:   map[string]any{},
			want:        "Hello, <no value>!",
			wantErr:     false,
		},
		{
			name:        "accessing field on non-existent map key returns no value",
			templateStr: "{{.user.name}}",
			variables:   map[string]any{},
			want:        "<no value>",
			wantErr:     false,
		},
		{
			name:        "special characters in string",
			templateStr: "Special: {{.text}}",
			variables:   map[string]any{"text": "<>&\"'"},
			want:        "Special: <>&\"'",
			wantErr:     false,
		},
		{
			name:        "unicode characters",
			templateStr: "你好，{{.name}}！",
			variables:   map[string]any{"name": "世界"},
			want:        "你好，世界！",
			wantErr:     false,
		},
		{
			name:        "with function - printf",
			templateStr: "{{printf \"Hello, %s!\" .name}}",
			variables:   map[string]any{"name": "World"},
			want:        "Hello, World!",
			wantErr:     false,
		},
		{
			name:        "template execution error - index out of range",
			templateStr: "{{index .items 10}}",
			variables:   map[string]any{"items": []int{1, 2, 3}},
			want:        "",
			wantErr:     true,
			errCode:     prompterr.TemplateRenderErrorCode,
		},
		{
			name:        "template execution error - invalid index operation on non-indexable type",
			templateStr: "{{index .value 0}}",
			variables:   map[string]any{"value": 42},
			want:        "",
			wantErr:     true,
			errCode:     prompterr.TemplateRenderErrorCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := InterpolateGoTemplate(tt.templateStr, tt.variables)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCode != 0 {
					statusErr, ok := errorx.FromStatusError(err)
					if assert.True(t, ok, "Error should be a StatusError") {
						assert.Equal(t, tt.errCode, statusErr.Code())
					}
				}
				assert.Empty(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestInterpolateGoTemplate_ComplexScenarios tests more complex template scenarios
func TestInterpolateGoTemplate_ComplexScenarios(t *testing.T) {
	t.Parallel()

	t.Run("template with multiple operations", func(t *testing.T) {
		t.Parallel()
		templateStr := `
{{- if .showGreeting -}}
Hello, {{.user.name}}!
{{- end -}}
{{- if .showDetails -}}
Age: {{.user.age}}
City: {{.user.city}}
{{- end -}}
`
		variables := map[string]any{
			"showGreeting": true,
			"showDetails":  true,
			"user": map[string]any{
				"name": "Alice",
				"age":  30,
				"city": "NYC",
			},
		}

		got, err := InterpolateGoTemplate(templateStr, variables)
		assert.NoError(t, err)
		assert.Contains(t, got, "Hello, Alice!")
		assert.Contains(t, got, "Age: 30")
		assert.Contains(t, got, "City: NYC")
	})

	t.Run("template with array of structs", func(t *testing.T) {
		t.Parallel()
		templateStr := `Users:
{{range .users}}
- {{.name}} ({{.role}})
{{end}}`
		variables := map[string]any{
			"users": []map[string]any{
				{"name": "Alice", "role": "Admin"},
				{"name": "Bob", "role": "User"},
			},
		}

		got, err := InterpolateGoTemplate(templateStr, variables)
		assert.NoError(t, err)
		assert.Contains(t, got, "Alice (Admin)")
		assert.Contains(t, got, "Bob (User)")
	})
}

// TestInterpolateGoTemplate_EdgeCases tests edge cases
func TestInterpolateGoTemplate_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("very long template", func(t *testing.T) {
		t.Parallel()
		var templateStr string
		for i := 0; i < 100; i++ {
			templateStr += "{{.value}}"
		}
		variables := map[string]any{"value": "x"}

		got, err := InterpolateGoTemplate(templateStr, variables)
		assert.NoError(t, err)
		assert.Len(t, got, 100)
	})

	t.Run("deeply nested structure", func(t *testing.T) {
		t.Parallel()
		templateStr := "{{.a.b.c.d.e}}"
		variables := map[string]any{
			"a": map[string]any{
				"b": map[string]any{
					"c": map[string]any{
						"d": map[string]any{
							"e": "deep",
						},
					},
				},
			},
		}

		got, err := InterpolateGoTemplate(templateStr, variables)
		assert.NoError(t, err)
		assert.Equal(t, "deep", got)
	})

	t.Run("nil value in map", func(t *testing.T) {
		t.Parallel()
		templateStr := "Value: {{.value}}"
		variables := map[string]any{"value": nil}

		got, err := InterpolateGoTemplate(templateStr, variables)
		assert.NoError(t, err)
		assert.Equal(t, "Value: <no value>", got)
	})
}
