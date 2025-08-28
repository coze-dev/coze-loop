// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestPromptTemplate_formatMessages(t *testing.T) {
	tests := []struct {
		name          string
		template      *PromptTemplate
		messages      []*Message
		variableVals  []*VariableVal
		expectedMsgs  []*Message
		expectedError error
	}{
		{
			name:          "nil template",
			template:      nil,
			messages:      []*Message{},
			variableVals:  []*VariableVal{},
			expectedMsgs:  nil,
			expectedError: nil,
		},
		{
			name: "empty messages",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are a helpful assistant."),
					},
				},
			},
			messages:     []*Message{},
			variableVals: []*VariableVal{},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
			},
			expectedError: nil,
		},
		{
			name: "nil variable values",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are a {{role}}."),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "role",
						Desc: "role",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: nil,
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a ."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "nil variable defs",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are a {{role}}."),
					},
				},
				VariableDefs: nil,
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{
				{
					Key:   "role",
					Value: ptr.Of("helpful assistant"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a {{role}}."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "placeholder role with valid messages",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RolePlaceholder,
						Content: ptr.Of("greeting"),
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key: "greeting",
					PlaceholderMessages: []*Message{
						{
							Role:    RoleSystem,
							Content: ptr.Of("Hello!"),
						},
					},
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("Hello!"),
				},
			},
			expectedError: nil,
		},
		{
			name: "placeholder role with invalid message role",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RolePlaceholder,
						Content: ptr.Of("greeting"),
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key: "greeting",
					PlaceholderMessages: []*Message{
						{
							Role:    RolePlaceholder,
							Content: ptr.Of("Hello!"),
						},
					},
				},
			},
			expectedMsgs:  nil,
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "normal message with variable replacement",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are a {{role}}."),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "role",
						Desc: "role",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{
				{
					Key:   "role",
					Value: ptr.Of("helpful assistant"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "message with parts",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are a {{role}}."),
						Parts: []*ContentPart{
							{
								Type: ContentTypeText,
								Text: ptr.Of("Additional info: {{info}}"),
							},
						},
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "role",
						Desc: "role",
						Type: VariableTypeString,
					},
					{
						Key:  "info",
						Desc: "info",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{
				{
					Key:   "role",
					Value: ptr.Of("helpful assistant"),
				},
				{
					Key:   "info",
					Value: ptr.Of("some info"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
					Parts: []*ContentPart{
						{
							Type: ContentTypeText,
							Text: ptr.Of("Additional info: some info"),
						},
					},
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "message with empty content",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of(""),
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of(""),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "message with nil content",
			template: &PromptTemplate{
				TemplateType: TemplateTypeNormal,
				Messages: []*Message{
					{
						Role: RoleSystem,
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{},
			expectedMsgs: []*Message{
				{
					Role: RoleSystem,
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formattedMsgs, err := tt.template.formatMessages(tt.messages, tt.variableVals)
			unittest.AssertErrorEqual(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedMsgs, formattedMsgs)
		})
	}
}

func TestCmpEqual(t *testing.T) {
	var pd1 *PromptDetail
	var pd2 *PromptDetail
	fmt.Printf("nil cmp nil = %t\n", cmp.Equal(pd1, pd2))              // true
	fmt.Printf("nil cmp !nil = %t\n", cmp.Equal(pd1, &PromptDetail{})) // false
	fmt.Printf("!nil cmp nil = %t\n", cmp.Equal(&PromptDetail{}, pd2)) // false
}

func TestConvertVariablesToMap(t *testing.T) {
	tests := []struct {
		name          string
		defMap        map[string]*VariableDef
		valMap        map[string]*VariableVal
		expected      map[string]any
		expectedError error
	}{
		{
			name:          "nil maps",
			defMap:        nil,
			valMap:        nil,
			expected:      nil,
			expectedError: nil,
		},
		{
			name:          "empty maps",
			defMap:        make(map[string]*VariableDef),
			valMap:        make(map[string]*VariableVal),
			expected:      nil,
			expectedError: nil,
		},
		{
			name: "string variable",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
			},
			expected: map[string]any{
				"name": "John",
			},
			expectedError: nil,
		},
		{
			name: "boolean variable true",
			defMap: map[string]*VariableDef{
				"enabled": {Key: "enabled", Type: VariableTypeBoolean},
			},
			valMap: map[string]*VariableVal{
				"enabled": {Key: "enabled", Value: ptr.Of("true")},
			},
			expected: map[string]any{
				"enabled": true,
			},
			expectedError: nil,
		},
		{
			name: "boolean variable false",
			defMap: map[string]*VariableDef{
				"enabled": {Key: "enabled", Type: VariableTypeBoolean},
			},
			valMap: map[string]*VariableVal{
				"enabled": {Key: "enabled", Value: ptr.Of("false")},
			},
			expected: map[string]any{
				"enabled": false,
			},
			expectedError: nil,
		},
		{
			name: "integer variable",
			defMap: map[string]*VariableDef{
				"count": {Key: "count", Type: VariableTypeInteger},
			},
			valMap: map[string]*VariableVal{
				"count": {Key: "count", Value: ptr.Of("42")},
			},
			expected: map[string]any{
				"count": int64(42),
			},
			expectedError: nil,
		},
		{
			name: "integer variable negative",
			defMap: map[string]*VariableDef{
				"count": {Key: "count", Type: VariableTypeInteger},
			},
			valMap: map[string]*VariableVal{
				"count": {Key: "count", Value: ptr.Of("-10")},
			},
			expected: map[string]any{
				"count": int64(-10),
			},
			expectedError: nil,
		},
		{
			name: "integer variable invalid",
			defMap: map[string]*VariableDef{
				"count": {Key: "count", Type: VariableTypeInteger},
			},
			valMap: map[string]*VariableVal{
				"count": {Key: "count", Value: ptr.Of("not_a_number")},
			},
			expected:      nil,
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "float variable",
			defMap: map[string]*VariableDef{
				"price": {Key: "price", Type: VariableTypeFloat},
			},
			valMap: map[string]*VariableVal{
				"price": {Key: "price", Value: ptr.Of("3.14")},
			},
			expected: map[string]any{
				"price": float64(3.14),
			},
			expectedError: nil,
		},
		{
			name: "float variable invalid",
			defMap: map[string]*VariableDef{
				"price": {Key: "price", Type: VariableTypeFloat},
			},
			valMap: map[string]*VariableVal{
				"price": {Key: "price", Value: ptr.Of("not_a_float")},
			},
			expected:      nil,
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "array string variable",
			defMap: map[string]*VariableDef{
				"items": {Key: "items", Type: VariableTypeArrayString},
			},
			valMap: map[string]*VariableVal{
				"items": {Key: "items", Value: ptr.Of(`["apple", "banana", "cherry"]`)},
			},
			expected: map[string]any{
				"items": []string{"apple", "banana", "cherry"},
			},
			expectedError: nil,
		},
		{
			name: "array string variable invalid json",
			defMap: map[string]*VariableDef{
				"items": {Key: "items", Type: VariableTypeArrayString},
			},
			valMap: map[string]*VariableVal{
				"items": {Key: "items", Value: ptr.Of(`["apple", "banana"`)},
			},
			expected:      nil,
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "array boolean variable",
			defMap: map[string]*VariableDef{
				"flags": {Key: "flags", Type: VariableTypeArrayBoolean},
			},
			valMap: map[string]*VariableVal{
				"flags": {Key: "flags", Value: ptr.Of(`[true, false, true]`)},
			},
			expected: map[string]any{
				"flags": []bool{true, false, true},
			},
			expectedError: nil,
		},
		{
			name: "array integer variable",
			defMap: map[string]*VariableDef{
				"numbers": {Key: "numbers", Type: VariableTypeArrayInteger},
			},
			valMap: map[string]*VariableVal{
				"numbers": {Key: "numbers", Value: ptr.Of(`[1, 2, 3, -5]`)},
			},
			expected: map[string]any{
				"numbers": []int64{1, 2, 3, -5},
			},
			expectedError: nil,
		},
		{
			name: "array float variable",
			defMap: map[string]*VariableDef{
				"prices": {Key: "prices", Type: VariableTypeArrayFloat},
			},
			valMap: map[string]*VariableVal{
				"prices": {Key: "prices", Value: ptr.Of(`[1.1, 2.2, 3.3]`)},
			},
			expected: map[string]any{
				"prices": []float64{1.1, 2.2, 3.3},
			},
			expectedError: nil,
		},
		{
			name: "object variable",
			defMap: map[string]*VariableDef{
				"user": {Key: "user", Type: VariableTypeObject},
			},
			valMap: map[string]*VariableVal{
				"user": {Key: "user", Value: ptr.Of(`{"name": "John", "age": 30}`)},
			},
			expected: map[string]any{
				"user": map[string]interface{}{"name": "John", "age": float64(30)},
			},
			expectedError: nil,
		},
		{
			name: "array object variable",
			defMap: map[string]*VariableDef{
				"users": {Key: "users", Type: VariableTypeArrayObject},
			},
			valMap: map[string]*VariableVal{
				"users": {Key: "users", Value: ptr.Of(`[{"name": "John"}, {"name": "Jane"}]`)},
			},
			expected: map[string]any{
				"users": []interface{}{
					map[string]interface{}{"name": "John"},
					map[string]interface{}{"name": "Jane"},
				},
			},
			expectedError: nil,
		},
		{
			name: "object variable invalid json",
			defMap: map[string]*VariableDef{
				"user": {Key: "user", Type: VariableTypeObject},
			},
			valMap: map[string]*VariableVal{
				"user": {Key: "user", Value: ptr.Of(`{"name": "John"`)},
			},
			expected:      nil,
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "nil variable value",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: nil},
			},
			expected:      map[string]any{},
			expectedError: nil,
		},
		{
			name: "empty variable value",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("")},
			},
			expected:      map[string]any{},
			expectedError: nil,
		},
		{
			name: "variable not in definition",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"age": {Key: "age", Value: ptr.Of("30")},
			},
			expected:      map[string]any{},
			expectedError: nil,
		},
		{
			name: "mixed variable types",
			defMap: map[string]*VariableDef{
				"name":    {Key: "name", Type: VariableTypeString},
				"age":     {Key: "age", Type: VariableTypeInteger},
				"enabled": {Key: "enabled", Type: VariableTypeBoolean},
				"score":   {Key: "score", Type: VariableTypeFloat},
			},
			valMap: map[string]*VariableVal{
				"name":    {Key: "name", Value: ptr.Of("John")},
				"age":     {Key: "age", Value: ptr.Of("30")},
				"enabled": {Key: "enabled", Value: ptr.Of("true")},
				"score":   {Key: "score", Value: ptr.Of("95.5")},
			},
			expected: map[string]any{
				"name":    "John",
				"age":     int64(30),
				"enabled": true,
				"score":   float64(95.5),
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertVariablesToMap(tt.defMap, tt.valMap)
			unittest.AssertErrorEqual(t, tt.expectedError, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderJinja2Template(t *testing.T) {
	tests := []struct {
		name          string
		templateStr   string
		defMap        map[string]*VariableDef
		valMap        map[string]*VariableVal
		expected      string
		expectedError error
	}{
		{
			name:        "empty template",
			templateStr: "",
			defMap:      nil,
			valMap:      nil,
			expected:    "",
		},
		{
			name:        "template without variables",
			templateStr: "Hello World",
			defMap:      nil,
			valMap:      nil,
			expected:    "Hello World",
		},
		{
			name:        "simple variable substitution",
			templateStr: "Hello {{ name }}!",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
			},
			expected: "Hello John!",
		},
		{
			name:        "multiple variables",
			templateStr: "Hello {{ name }}, you are {{ age }} years old.",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
				"age":  {Key: "age", Type: VariableTypeInteger},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
				"age":  {Key: "age", Value: ptr.Of("30")},
			},
			expected: "Hello John, you are 30 years old.",
		},
		{
			name:        "boolean variable in condition",
			templateStr: "{% if enabled %}Feature is enabled{% else %}Feature is disabled{% endif %}",
			defMap: map[string]*VariableDef{
				"enabled": {Key: "enabled", Type: VariableTypeBoolean},
			},
			valMap: map[string]*VariableVal{
				"enabled": {Key: "enabled", Value: ptr.Of("true")},
			},
			expected: "Feature is enabled",
		},
		{
			name:        "boolean variable false in condition",
			templateStr: "{% if enabled %}Feature is enabled{% else %}Feature is disabled{% endif %}",
			defMap: map[string]*VariableDef{
				"enabled": {Key: "enabled", Type: VariableTypeBoolean},
			},
			valMap: map[string]*VariableVal{
				"enabled": {Key: "enabled", Value: ptr.Of("false")},
			},
			expected: "Feature is disabled",
		},
		{
			name:        "array iteration",
			templateStr: "Items: {% for item in items %}{{ item }}{% if not loop.last %}, {% endif %}{% endfor %}",
			defMap: map[string]*VariableDef{
				"items": {Key: "items", Type: VariableTypeArrayString},
			},
			valMap: map[string]*VariableVal{
				"items": {Key: "items", Value: ptr.Of(`["apple", "banana", "cherry"]`)},
			},
			expected: "Items: apple, banana, cherry",
		},
		{
			name:        "object property access",
			templateStr: "User: {{ user.name }} ({{ user.age }})",
			defMap: map[string]*VariableDef{
				"user": {Key: "user", Type: VariableTypeObject},
			},
			valMap: map[string]*VariableVal{
				"user": {Key: "user", Value: ptr.Of(`{"name": "John", "age": 30}`)},
			},
			expected: "User: John (30.0)",
		},
		{
			name:        "float variable with filter",
			templateStr: "Price: ${{ price | round(2) }}",
			defMap: map[string]*VariableDef{
				"price": {Key: "price", Type: VariableTypeFloat},
			},
			valMap: map[string]*VariableVal{
				"price": {Key: "price", Value: ptr.Of("3.14159")},
			},
			expected: "Price: $3.15",
		},
		{
			name:        "array of objects",
			templateStr: "Users: {% for user in users %}{{ user.name }}{% if not loop.last %}, {% endif %}{% endfor %}",
			defMap: map[string]*VariableDef{
				"users": {Key: "users", Type: VariableTypeArrayObject},
			},
			valMap: map[string]*VariableVal{
				"users": {Key: "users", Value: ptr.Of(`[{"name": "John"}, {"name": "Jane"}]`)},
			},
			expected: "Users: John, Jane",
		},
		{
			name:        "invalid template syntax",
			templateStr: "Hello {% invalid_tag %}",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
			},
			expectedError: errorx.NewByCode(prompterr.TemplateParseErrorCode),
		},
		{
			name:        "variable conversion error",
			templateStr: "Count: {{ count }}",
			defMap: map[string]*VariableDef{
				"count": {Key: "count", Type: VariableTypeInteger},
			},
			valMap: map[string]*VariableVal{
				"count": {Key: "count", Value: ptr.Of("not_a_number")},
			},
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name:        "template with undefined variable",
			templateStr: "Hello {{ undefined_var }}",
			defMap:      map[string]*VariableDef{},
			valMap:      map[string]*VariableVal{},
			expected:    "Hello ",
		},
		{
			name:        "empty variable maps",
			templateStr: "Hello World",
			defMap:      map[string]*VariableDef{},
			valMap:      map[string]*VariableVal{},
			expected:    "Hello World",
		},
		{
			name:        "complex nested template",
			templateStr: "{% if user %}Hello {{ user.name }}!{% if user.items %} You have {{ user.items|length }} items.{% endif %}{% endif %}",
			defMap: map[string]*VariableDef{
				"user": {Key: "user", Type: VariableTypeObject},
			},
			valMap: map[string]*VariableVal{
				"user": {Key: "user", Value: ptr.Of(`{"name": "John", "items": ["a", "b", "c"]}`)},
			},
			expected: "Hello John! You have 3 items.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderJinja2Template(tt.templateStr, tt.defMap, tt.valMap)
			unittest.AssertErrorEqual(t, tt.expectedError, err)
			if tt.expectedError == nil {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatText_Jinja2(t *testing.T) {
	tests := []struct {
		name          string
		templateType  TemplateType
		templateStr   string
		defMap        map[string]*VariableDef
		valMap        map[string]*VariableVal
		expected      string
		expectedError error
	}{
		{
			name:         "jinja2 template type",
			templateType: TemplateTypeJinja2,
			templateStr:  "Hello {{ name }}!",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
			},
			expected: "Hello John!",
		},
		{
			name:         "normal template type",
			templateType: TemplateTypeNormal,
			templateStr:  "Hello {{name}}!",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
			},
			expected: "Hello John!",
		},
		{
			name:          "unsupported template type",
			templateType:  TemplateType("unknown"),
			templateStr:   "Hello World",
			defMap:        nil,
			valMap:        nil,
			expectedError: errorx.NewByCode(prompterr.UnsupportedTemplateTypeCode),
		},
		{
			name:         "jinja2 with boolean condition",
			templateType: TemplateTypeJinja2,
			templateStr:  "{% if enabled %}Active{% else %}Inactive{% endif %}",
			defMap: map[string]*VariableDef{
				"enabled": {Key: "enabled", Type: VariableTypeBoolean},
			},
			valMap: map[string]*VariableVal{
				"enabled": {Key: "enabled", Value: ptr.Of("true")},
			},
			expected: "Active",
		},
		{
			name:         "jinja2 template parse error",
			templateType: TemplateTypeJinja2,
			templateStr:  "Hello {% invalid_tag %}",
			defMap: map[string]*VariableDef{
				"name": {Key: "name", Type: VariableTypeString},
			},
			valMap: map[string]*VariableVal{
				"name": {Key: "name", Value: ptr.Of("John")},
			},
			expectedError: errorx.NewByCode(prompterr.TemplateParseErrorCode),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := formatText(tt.templateType, tt.templateStr, tt.defMap, tt.valMap)
			unittest.AssertErrorEqual(t, tt.expectedError, err)
			if tt.expectedError == nil {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestPromptTemplate_formatMessages_Jinja2(t *testing.T) {
	tests := []struct {
		name          string
		template      *PromptTemplate
		messages      []*Message
		variableVals  []*VariableVal
		expectedMsgs  []*Message
		expectedError error
	}{
		{
			name: "jinja2 template with simple variable",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are a {{ role }}."),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "role",
						Desc: "role",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			variableVals: []*VariableVal{
				{
					Key:   "role",
					Value: ptr.Of("helpful assistant"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a helpful assistant."),
				},
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello"),
				},
			},
			expectedError: nil,
		},
		{
			name: "jinja2 template with boolean condition",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("{% if verbose %}You are a detailed assistant.{% else %}You are a brief assistant.{% endif %}"),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "verbose",
						Desc: "verbose mode",
						Type: VariableTypeBoolean,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "verbose",
					Value: ptr.Of("true"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are a detailed assistant."),
				},
			},
			expectedError: nil,
		},
		{
			name: "jinja2 template with array iteration",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("Available tools: {% for tool in tools %}{{ tool }}{% if not loop.last %}, {% endif %}{% endfor %}"),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "tools",
						Desc: "tools list",
						Type: VariableTypeArrayString,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "tools",
					Value: ptr.Of(`["calculator", "search", "translator"]`),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("Available tools: calculator, search, translator"),
				},
			},
			expectedError: nil,
		},
		{
			name: "jinja2 template with object access",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("Hello {{ user.name }}, your level is {{ user.level }}."),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "user",
						Desc: "user info",
						Type: VariableTypeObject,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "user",
					Value: ptr.Of(`{"name": "John", "level": 5}`),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("Hello John, your level is 5.0."),
				},
			},
			expectedError: nil,
		},
		{
			name: "jinja2 template with parts",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("Main: {{ main_content }}"),
						Parts: []*ContentPart{
							{
								Type: ContentTypeText,
								Text: ptr.Of("Additional: {% if show_extra %}{{ extra_info }}{% endif %}"),
							},
						},
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "main_content",
						Desc: "main content",
						Type: VariableTypeString,
					},
					{
						Key:  "show_extra",
						Desc: "show extra info",
						Type: VariableTypeBoolean,
					},
					{
						Key:  "extra_info",
						Desc: "extra info",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "main_content",
					Value: ptr.Of("Hello World"),
				},
				{
					Key:   "show_extra",
					Value: ptr.Of("true"),
				},
				{
					Key:   "extra_info",
					Value: ptr.Of("Extra details"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("Main: Hello World"),
					Parts: []*ContentPart{
						{
							Type: ContentTypeText,
							Text: ptr.Of("Additional: Extra details"),
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "jinja2 template with mixed variable types",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("User {{ name }} has {{ count }} items with score {{ score }}."),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "name",
						Desc: "user name",
						Type: VariableTypeString,
					},
					{
						Key:  "count",
						Desc: "item count",
						Type: VariableTypeInteger,
					},
					{
						Key:  "score",
						Desc: "user score",
						Type: VariableTypeFloat,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "name",
					Value: ptr.Of("John"),
				},
				{
					Key:   "count",
					Value: ptr.Of("5"),
				},
				{
					Key:   "score",
					Value: ptr.Of("98.5"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleSystem,
					Content: ptr.Of("User John has 5 items with score 98.5."),
				},
			},
			expectedError: nil,
		},
		{
			name: "jinja2 template parse error",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("Hello {% invalid_tag %}"),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "name",
						Desc: "name",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "name",
					Value: ptr.Of("John"),
				},
			},
			expectedMsgs:  nil,
			expectedError: errorx.NewByCode(prompterr.TemplateParseErrorCode),
		},
		{
			name: "jinja2 template variable conversion error",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RoleSystem,
						Content: ptr.Of("Count: {{ count }}"),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "count",
						Desc: "count",
						Type: VariableTypeInteger,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key:   "count",
					Value: ptr.Of("not_a_number"),
				},
			},
			expectedMsgs:  nil,
			expectedError: errorx.NewByCode(prompterr.CommonInvalidParamCode),
		},
		{
			name: "jinja2 template with placeholder role",
			template: &PromptTemplate{
				TemplateType: TemplateTypeJinja2,
				Messages: []*Message{
					{
						Role:    RolePlaceholder,
						Content: ptr.Of("greeting"),
					},
					{
						Role:    RoleSystem,
						Content: ptr.Of("You are {{ role }}."),
					},
				},
				VariableDefs: []*VariableDef{
					{
						Key:  "role",
						Desc: "role",
						Type: VariableTypeString,
					},
				},
			},
			messages: []*Message{},
			variableVals: []*VariableVal{
				{
					Key: "greeting",
					PlaceholderMessages: []*Message{
						{
							Role:    RoleUser,
							Content: ptr.Of("Hello!"),
						},
					},
				},
				{
					Key:   "role",
					Value: ptr.Of("assistant"),
				},
			},
			expectedMsgs: []*Message{
				{
					Role:    RoleUser,
					Content: ptr.Of("Hello!"),
				},
				{
					Role:    RoleSystem,
					Content: ptr.Of("You are assistant."),
				},
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formattedMsgs, err := tt.template.formatMessages(tt.messages, tt.variableVals)
			unittest.AssertErrorEqual(t, tt.expectedError, err)
			assert.Equal(t, tt.expectedMsgs, formattedMsgs)
		})
	}
}
