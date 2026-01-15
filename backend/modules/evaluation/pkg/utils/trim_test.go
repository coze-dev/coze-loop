// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCountJsonArrayElements(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "only brackets no elements",
			input:    "[]",
			expected: 0,
		},
		{
			name:     "whitespace array no elements",
			input:    "[   ]",
			expected: 1,
		},
		{
			name:     "simple numbers",
			input:    "[1,2,3]",
			expected: 3,
		},
		{
			name:     "simple strings",
			input:    `["a","b","c"]`,
			expected: 3,
		},
		{
			name:     "strings with comma",
			input:    `["a,b","c"]`,
			expected: 2,
		},
		{
			name:     "escaped quotes and comma in string",
			input:    `["a, \"b\"", "c"]`,
			expected: 2,
		},
		{
			name:     "nested array and object",
			input:    `[1, [2,3], {"a":4}]`,
			expected: 3,
		},
		{
			name:     "invalid json no closing bracket",
			input:    `[1,2,3`,
			expected: 0,
		},
		{
			name:     "unicode escape sequence",
			input:    `["\u4f60\u597d","world"]`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := countJsonArrayElements([]byte(tt.input))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGenerateJsonObjectPreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "non object input - empty string",
			input:    `""`,
			expected: "",
		},
		{
			name:     "non object input - array",
			input:    `[]`,
			expected: "",
		},
		{
			name:     "simple object with primitive values",
			input:    `{"a":123,"b":"hello world","c":true}`,
			expected: `{"a": 123, "b": "...", "c": true}`,
		},
		{
			name:     "object with nested structures",
			input:    `{"a":{"x":1},"b":[1,2,3]}`,
			expected: `{"a": "{...}", "b": "[...]"}`,
		},
		{
			name:     "object with empty value",
			input:    `{"a": ""}`,
			expected: `{"a": ""}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := GenerateJsonObjectPreview([]byte(tt.input))
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSummarizeValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		expected string
	}{
		{
			name:     "empty value",
			value:    "",
			expected: "...",
		},
		{
			name:     "short string with quotes",
			value:    `"abc"`,
			expected: `"abc"`,
		},
		{
			name:     "long string with quotes",
			value:    `"abcdef"`,
			expected: `"..."` ,
		},
		{
			name:     "object value",
			value:    `{"a":1}`,
			expected: `"{...}"`,
		},
		{
			name:     "array value",
			value:    `[1,2,3]`,
			expected: `"[...]"`,
		},
		{
			name:     "short number",
			value:    "123",
			expected: "123",
		},
		{
			name:     "long number",
			value:    "123456789",
			expected: "12345...",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := summarizeValue(tt.value)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGenerateTextPreview(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short ascii content",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "exact 100 runes",
			input:    string(make([]rune, 100)),
			expected: string(make([]rune, 100)),
		},
		{
			name:  "long ascii content should be trimmed",
			input: string(make([]byte, 120)),
		},
		{
			name:     "utf8 content shorter than limit should not be trimmed",
			input:    "你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界",
			expected: "你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界你好世界",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := generateTextPreview([]byte(tt.input))

			switch tt.name {
			case "long ascii content should be trimmed":
				assert.Len(t, []rune(got), 103) // 100 chars + "..."
				assert.Equal(t, "...", got[len(got)-3:])
			default:
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

