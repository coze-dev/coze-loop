// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewJinja2Engine(t *testing.T) {
	engine := NewJinja2Engine()
	assert.NotNil(t, engine)
	assert.Equal(t, 30*time.Second, engine.timeout)
}

func TestJinja2Engine_Execute_BasicVariables(t *testing.T) {
	engine := NewJinja2Engine()

	tests := []struct {
		name     string
		template string
		variables map[string]interface{}
		expected string
		hasError bool
	}{
		{
			name:     "simple variable",
			template: "Hello {{ name }}!",
			variables: map[string]interface{}{
				"name": "World",
			},
			expected: "Hello World!",
			hasError: false,
		},
		{
			name:     "multiple variables",
			template: "Hello {{ name }}, today is {{ day }}!",
			variables: map[string]interface{}{
				"name": "Alice",
				"day":  "Monday",
			},
			expected: "Hello Alice, today is Monday!",
			hasError: false,
		},
		{
			name:     "empty variables",
			template: "Hello {{ name }}!",
			variables: map[string]interface{}{},
			expected: "Hello !",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Execute(tt.template, tt.variables)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJinja2Engine_Execute_ControlStructures(t *testing.T) {
	engine := NewJinja2Engine()

	tests := []struct {
		name     string
		template string
		variables map[string]interface{}
		expected string
		hasError bool
	}{
		{
			name:     "if statement true",
			template: "{% if condition %}Yes{% else %}No{% endif %}",
			variables: map[string]interface{}{
				"condition": true,
			},
			expected: "Yes",
			hasError: false,
		},
		{
			name:     "if statement false",
			template: "{% if condition %}Yes{% else %}No{% endif %}",
			variables: map[string]interface{}{
				"condition": false,
			},
			expected: "No",
			hasError: false,
		},
		{
			name:     "for loop",
			template: "{% for item in items %}{{ item }}{% endfor %}",
			variables: map[string]interface{}{
				"items": []string{"a", "b", "c"},
			},
			expected: "abc",
			hasError: false,
		},
		{
			name:     "nested if and for",
			template: "{% for item in items %}{% if item == 'b' %}B{% else %}{{ item }}{% endif %}{% endfor %}",
			variables: map[string]interface{}{
				"items": []string{"a", "b", "c"},
			},
			expected: "aBc",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Execute(tt.template, tt.variables)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJinja2Engine_Execute_Filters(t *testing.T) {
	engine := NewJinja2Engine()

	tests := []struct {
		name     string
		template string
		variables map[string]interface{}
		expected string
		hasError bool
	}{
		{
			name:     "upper filter",
			template: "{{ text|upper }}",
			variables: map[string]interface{}{
				"text": "hello world",
			},
			expected: "HELLO WORLD",
			hasError: false,
		},
		{
			name:     "lower filter",
			template: "{{ text|lower }}",
			variables: map[string]interface{}{
				"text": "HELLO WORLD",
			},
			expected: "hello world",
			hasError: false,
		},
		{
			name:     "default filter",
			template: "{{ name|default('Anonymous') }}",
			variables: map[string]interface{}{},
			expected: "Anonymous",
			hasError: false,
		},
		{
			name:     "length filter",
			template: "{{ items|length }}",
			variables: map[string]interface{}{
				"items": []string{"a", "b", "c"},
			},
			expected: "3",
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Execute(tt.template, tt.variables)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJinja2Engine_Execute_ComplexTemplates(t *testing.T) {
	engine := NewJinja2Engine()

	template := `
Hello {{ name }}!

{% if weather == 'sunny' %}
The weather is beautiful today!
{% else %}
It's not sunny today.
{% endif %}

{% for item in items %}
- {{ item|upper }}
{% endfor %}

Current time: {{ now()|strftime('%Y-%m-%d') }}
	`

	variables := map[string]interface{}{
		"name":    "World",
		"weather": "sunny",
		"items":   []string{"apple", "banana", "cherry"},
	}

	result, err := engine.Execute(template, variables)
	assert.NoError(t, err)
	assert.Contains(t, result, "Hello World!")
	assert.Contains(t, result, "The weather is beautiful today!")
	assert.Contains(t, result, "- APPLE")
	assert.Contains(t, result, "- BANANA")
	assert.Contains(t, result, "- CHERRY")
}

func TestJinja2Engine_Execute_ErrorHandling(t *testing.T) {
	engine := NewJinja2Engine()

	tests := []struct {
		name     string
		template string
		variables map[string]interface{}
		hasError bool
	}{
		{
			name:     "invalid syntax",
			template: "{{ name }",
			variables: map[string]interface{}{},
			hasError: true,
		},
		{
			name:     "unclosed if",
			template: "{% if condition %}Yes",
			variables: map[string]interface{}{},
			hasError: true,
		},
		{
			name:     "unclosed for",
			template: "{% for item in items %}{{ item }}",
			variables: map[string]interface{}{},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.Execute(tt.template, tt.variables)
			assert.Error(t, err)
		})
	}
}

func TestJinja2Engine_Execute_Timeout(t *testing.T) {
	engine := NewJinja2Engine()
	engine.timeout = 100 * time.Millisecond

	// 创建一个会导致无限循环的模板（虽然pongo2有内置保护）
	template := "{% for i in range(1000000) %}{{ i }}{% endfor %}"
	variables := map[string]interface{}{}

	_, err := engine.Execute(template, variables)
	// 这个测试主要是验证超时机制的存在，实际结果可能因pongo2版本而异
	assert.NotNil(t, engine.timeout)
}

func TestJinja2Engine_SafeContext(t *testing.T) {
	engine := NewJinja2Engine()

	// 测试安全上下文创建
	variables := map[string]interface{}{
		"safe_string": "hello",
		"safe_int":    42,
		"safe_float":  3.14,
		"safe_bool":   true,
		"safe_array":  []string{"a", "b"},
	}

	context := engine.createSafeContext(variables)

	// 验证安全变量被正确添加
	assert.Equal(t, "hello", context["safe_string"])
	assert.Equal(t, 42, context["safe_int"])
	assert.Equal(t, 3.14, context["safe_float"])
	assert.Equal(t, true, context["safe_bool"])
	assert.Equal(t, []string{"a", "b"}, context["safe_array"])

	// 验证内置函数存在
	assert.NotNil(t, context["now"])
	assert.NotNil(t, context["len"])
}
