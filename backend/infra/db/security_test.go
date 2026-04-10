// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package db

import "testing"

func TestQuoteSQLData(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "''"},
		{name: "simple", in: "abc", want: "'abc'"},
		{name: "single_quote", in: "O'Reilly", want: "'O\\'Reilly'"},
		{name: "double_quote", in: "a\"b", want: "'a\\\"b'"},
		{name: "backslash", in: `a\b`, want: "'a\\\\b'"},
		{name: "newline", in: "line1\nline2", want: "'line1\\nline2'"},
		{name: "tab", in: "a\tb", want: "'a\\tb'"},
		{name: "carriage_return", in: "a\rb", want: "'a\\rb'"},
		{name: "null_byte", in: string([]byte{0}), want: "'\\0'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuoteSQLData(tt.in)
			if got != tt.want {
				t.Fatalf("QuoteSQLData(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestEscapeSQLData(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "simple", in: "abc", want: "abc"},
		{name: "single_quote", in: "O'Reilly", want: "O\\'Reilly"},
		{name: "double_quote", in: "a\"b", want: "a\\\"b"},
		{name: "backslash", in: `a\b`, want: "a\\\\b"},
		{name: "newline", in: "line1\nline2", want: "line1\\nline2"},
		{name: "tab", in: "a\tb", want: "a\\tb"},
		{name: "carriage_return", in: "a\rb", want: "a\\rb"},
		{name: "null_byte", in: string([]byte{0}), want: "\\0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeSQLData(tt.in)
			if got != tt.want {
				t.Fatalf("EscapeSQLData(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
