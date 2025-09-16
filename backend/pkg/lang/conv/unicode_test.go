// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conv

import (
	"testing"
)

func TestUnescapeUnicode(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single unicode escape",
			input:    "Hello\u0026World",
			expected: "Hello&World",
		},
		{
			name:     "multiple unicode escapes",
			input:    "Test\u0026String\u003DValue",
			expected: "Test&String=Value",
		},
		{
			name:     "mixed content",
			input:    "URL: https://example.com?param\u003Dvalue\u0026other\u003Ddata",
			expected: "URL: https://example.com?param=value&other=data",
		},
		{
			name:     "no unicode escapes",
			input:    "Normal string without escapes",
			expected: "Normal string without escapes",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only unicode escapes",
			input:    "\u0026\u003D\u003C\u003E",
			expected: "&=<>",
		},
		{
			name:     "chinese characters",
			input:    "Hello\u4E16\u754C",
			expected: "Hello世界",
		},
		{
			name:     "invalid unicode escape",
			input:    "Test\\uZZZZInvalid",
			expected: "Test\\uZZZZInvalid", // should remain unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnescapeUnicode(tt.input)
			if result != tt.expected {
				t.Errorf("UnescapeUnicode(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUnescapeUnicodeBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "unicode escape in bytes",
			input:    []byte("Hello\u0026World"),
			expected: []byte("Hello&World"),
		},
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "mixed content bytes",
			input:    []byte("Data\u003Dvalue\u0026key"),
			expected: []byte("Data=value&key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UnescapeUnicodeBytes(tt.input)
			if string(result) != string(tt.expected) {
				t.Errorf("UnescapeUnicodeBytes(%q) = %q, want %q", string(tt.input), string(result), string(tt.expected))
			}
		})
	}
}
