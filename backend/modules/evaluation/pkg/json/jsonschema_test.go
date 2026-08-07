// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package json

import "testing"

func TestValidateJSONSchema_StringTypeWithAnnotations(t *testing.T) {
	// Regression: schema authors (or auto-generated evaluator versions) commonly
	// attach `description` / `title` to a string variable, e.g.
	//   {"type": "string", "description": "Go 代码片段"}
	// The pre-fix fast-path only matched the exact `{"type":"string"}` literal, so
	// annotated schemas fell through to json.Unmarshal — which happily parses a
	// stringified-JSON payload into a map and then trips `type: "string"`.
	tests := []struct {
		name    string
		schema  string
		data    string
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "plain string schema accepts arbitrary text",
			schema: `{"type":"string"}`,
			data:   "hello world",
			wantOK: true,
		},
		{
			name:   "string schema with description accepts plain text",
			schema: `{"type": "string", "description": "Go 代码片段"}`,
			data:   "package main\n\nimport \"fmt\"",
			wantOK: true,
		},
		{
			name:   "string schema with description accepts stringified JSON as string",
			schema: `{"type": "string", "description": "Skill 诊断输出"}`,
			data:   `{"risk_type":"high_frequency_logging","risk_level":"high"}`,
			wantOK: true,
		},
		{
			name:   "string schema declared as type array still triggers fast path",
			schema: `{"type": ["string", "null"], "description": "optional text"}`,
			data:   "hello",
			wantOK: true,
		},
		{
			name:    "object schema rejects plain string",
			schema:  `{"type":"object"}`,
			data:    "hello",
			wantOK:  false,
			wantErr: true,
		},
		{
			name:    "integer schema rejects plain text",
			schema:  `{"type":"integer"}`,
			data:    "not-a-number",
			wantOK:  false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := ValidateJSONSchema(tt.schema, tt.data)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v, want %v (err=%v)", ok, tt.wantOK, err)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestIsStringTypeSchema(t *testing.T) {
	cases := map[string]bool{
		`{"type":"string"}`:                   true,
		`{"type": "string"}`:                  true,
		`{"type":"string","description":"x"}`: true,
		`{"type":["string","null"]}`:          true,
		`{"type":"integer"}`:                  false,
		`{"type":"object"}`:                   false,
		`{"description":"missing type"}`:      false,
		`not json`:                            false,
		`{"type":123}`:                        false,
	}
	for schema, want := range cases {
		if got := isStringTypeSchema(schema); got != want {
			t.Errorf("isStringTypeSchema(%q) = %v, want %v", schema, got, want)
		}
	}
}
