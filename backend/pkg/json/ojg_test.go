// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package json

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sampleJSON 是用于测试的示例 JSON 字符串
const sampleJSON = `{
    "store": {
        "book": [
            {
                "category": "reference",
                "author": "Nigel Rees",
                "title": "Sayings of the Century",
                "price": 8.95,
                "available": true
            },
            {
                "category": "fiction",
                "author": "Evelyn Waugh",
                "title": "Sword of Honour",
                "price": 12.99,
                "available": false
            },
            {
                "category": "fiction",
                "author": "Herman Melville",
                "title": "Moby Dick",
                "isbn": "0-553-21311-3",
                "price": 8.99
            },
            {
                "category": "fiction",
                "author": "J. R. R. Tolkien",
                "title": "The Lord of the Rings",
                "isbn": "0-395-19395-8",
                "price": 22.99
            }
        ],
        "bicycle": {
            "color": "red",
            "price": 19.95,
            "specifications": {
                "height": 26,
                "width": 40
            }
        }
    },
    "expensive": 10,
    "numbers": {
        "integer": 42,
        "float": 3.14,
        "array": [1, 2, 3]
    }
}`

func TestGetByJSONPath(t *testing.T) {
	testCases := []struct {
		name          string
		jsonpath      string
		expectedType  string // "single", "array", "nil", "error"
		expectedValue interface{}
	}{
		{
			name:          "场景：空路径 - 返回整个文档",
			jsonpath:      "",
			expectedType:  "single",
			expectedValue: nil, // 实际值将是整个 JSON 对象，但我们只验证它不为 nil
		},
		{
			name:          "场景：简单字段访问 - 获取自行车颜色",
			jsonpath:      "$.store.bicycle.color",
			expectedType:  "single",
			expectedValue: "red",
		},
		{
			name:          "场景：简单字段访问 - 获取自行车颜色",
			jsonpath:      "store.bicycle.color",
			expectedType:  "single",
			expectedValue: "red",
		},
		{
			name:          "场景：数组索引访问 - 获取第一本书的标题",
			jsonpath:      "$.store.book[0].title",
			expectedType:  "single",
			expectedValue: "Sayings of the Century",
		},
		{
			name:          "场景：通配符 - 获取所有书的作者",
			jsonpath:      "$.store.book[*].author",
			expectedType:  "array",
			expectedValue: []interface{}{"Nigel Rees", "Evelyn Waugh", "Herman Melville", "J. R. R. Tolkien"},
		},
		{
			name:          "场景：深度扫描 - 获取所有价格",
			jsonpath:      "$..price",
			expectedType:  "array",
			expectedValue: []interface{}{float64(8.95), float64(12.99), float64(8.99), float64(22.99), float64(19.95)},
		},
		{
			name:          "场景：过滤器 - 价格大于10的书",
			jsonpath:      "$.store.book[?(@.price > 10)].title",
			expectedType:  "array",
			expectedValue: []interface{}{"Sword of Honour", "The Lord of the Rings"},
		},
		{
			name:          "场景：过滤器 - 具有ISBN的书",
			jsonpath:      "$.store.book[?(@.isbn=='0-553-21311-3' || @.isbn=='0-395-19395-8')].title",
			expectedType:  "array",
			expectedValue: []interface{}{"Moby Dick", "The Lord of the Rings"},
		},
		{
			name:          "场景：布尔值访问",
			jsonpath:      "$.store.book[0].available",
			expectedType:  "single",
			expectedValue: true,
		},
		{
			name:          "场景：嵌套对象访问",
			jsonpath:      "$.store.bicycle.specifications.height",
			expectedType:  "single",
			expectedValue: int64(26), // ojg 将整数解析为 int64
		},
		{
			name:          "场景：不存在的路径",
			jsonpath:      "$.nonexistent",
			expectedType:  "nil",
			expectedValue: nil,
		},
		{
			name:          "场景：无效的JSONPath",
			jsonpath:      "$.[invalid",
			expectedType:  "error",
			expectedValue: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetByJSONPath(sampleJSON, tc.jsonpath, false)

			switch tc.expectedType {
			case "error":
				assert.Error(t, err)
			case "nil":
				assert.NoError(t, err)
				assert.Nil(t, result)
			case "single":
				assert.NoError(t, err)
				if tc.expectedValue == nil {
					assert.NotNil(t, result)
				} else {
					assert.Equal(t, tc.expectedValue, result)
				}
			case "array":
				assert.NoError(t, err)
				resultArray, ok := result.([]interface{})
				require.True(t, ok)
				assert.ElementsMatch(t, tc.expectedValue, resultArray)
			}
		})
	}
}

func TestGetStringByJSONPath(t *testing.T) {
	testCases := []struct {
		name          string
		jsonpath      string
		expectedValue string
		expectedError bool
	}{
		{
			name:          "场景：字符串字段",
			jsonpath:      "$.store.bicycle.color",
			expectedValue: "red",
			expectedError: false,
		},
		{
			name:          "场景：数字字段 - 整数",
			jsonpath:      "$.numbers.integer",
			expectedValue: "42",
			expectedError: false,
		},
		{
			name:          "场景：数字字段 - 浮点数",
			jsonpath:      "$.numbers.float",
			expectedValue: "3.14",
			expectedError: false,
		},
		{
			name:          "场景：布尔字段",
			jsonpath:      "$.store.book[0].available",
			expectedValue: "true",
			expectedError: false,
		},
		{
			name:          "场景：数组字段",
			jsonpath:      "$.numbers.array",
			expectedValue: "[1,2,3]",
			expectedError: false,
		},
		{
			name:          "场景：对象字段",
			jsonpath:      "$.store.bicycle.specifications",
			expectedValue: `{"height":26,"width":40}`,
			expectedError: false,
		},
		{
			name:          "场景：不存在的路径",
			jsonpath:      "$.nonexistent",
			expectedValue: "",
			expectedError: false,
		},
		{
			name:          "场景：无效的JSONPath",
			jsonpath:      "$.[invalid",
			expectedValue: "",
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := GetStringByJSONPath(sampleJSON, tc.jsonpath)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, result)
			}
		})
	}
}

func TestGetStringByJSONPathReal(t *testing.T) {
	testCases := []struct {
		name          string
		jsonpath      string
		expectedValue string
		expectedError bool
	}{
		{
			name:          "场景：字符串字段",
			jsonpath:      "name",
			expectedValue: "dsf",
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonStr := "{\"age\":18,\"msg\":[{\"role\":1,\"query\":\"hi\"}],\"name\":\"dsf\"}"
			result, err := GetStringByJSONPath(jsonStr, tc.jsonpath)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, result)
			}
		})
	}
}

func TestConvertToString(t *testing.T) {
	testCases := []struct {
		name          string
		input         interface{}
		expectedValue string
		expectedError bool
	}{
		{
			name:          "场景：字符串类型",
			input:         "hello",
			expectedValue: "hello",
			expectedError: false,
		},
		{
			name:          "场景：整数类型",
			input:         42,
			expectedValue: "42",
			expectedError: false,
		},
		{
			name:          "场景：浮点数类型",
			input:         3.14,
			expectedValue: "3.14",
			expectedError: false,
		},
		{
			name:          "场景：布尔类型 - true",
			input:         true,
			expectedValue: "true",
			expectedError: false,
		},
		{
			name:          "场景：布尔类型 - false",
			input:         false,
			expectedValue: "false",
			expectedError: false,
		},
		{
			name:          "场景：切片类型",
			input:         []interface{}{1, 2, 3},
			expectedValue: "[1,2,3]",
			expectedError: false,
		},
		{
			name:          "场景：map类型",
			input:         map[string]interface{}{"key": "value"},
			expectedValue: `{"key":"value"}`,
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ConvertToString(tc.input)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, result)
			}
		})
	}
}

func TestGetFirstJSONPathField(t *testing.T) {
	cases := []struct {
		name     string
		jsonpath string
		want     string
		wantErr  bool
	}{
		{"普通点号", "$.foo.bar", "foo", false},
		{"点号无$前缀", "foo.bar", "foo", false},
		{"数组下标后字段", "$[0].foo", "foo", false},
		{"点号加数组下标", "$.foo[0].bar", "foo", false},
		{"单引号字段", "$['foo'].bar", "foo", false},
		{"单引号复杂字段", "$['复杂字段名'].bar", "复杂字段名", false},
		{"无$直接字段", "bar", "bar", false},
		{"只有$", "$", "", true},
		{"只有$.", "$.", "", true},
		{"空字符串", "", "", true},
		{"无效单引号", "$['foo.bar", "", true},
		{"无字段", "$..bar", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := GetFirstJSONPathField(c.jsonpath)
			if c.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
		})
	}
}

func TestGetJSONPathLevel(t *testing.T) {
	cases := []struct {
		name     string
		jsonpath string
		want     int
		wantErr  bool
	}{
		{"空字符串", "", 0, true},
		{"只有$", "$", 0, false},
		{"只有字段", "foo", 1, false},
		{"$加字段", "$.foo", 1, false},
		{"多级点号", "$.foo.bar", 2, false},
		{"多级点号无$", "foo.bar", 2, false},
		{"数组下标", "$.foo[0]", 2, false},
		{"多级数组下标", "$.foo[0][1]", 3, false},
		{"数组下标后字段", "$.foo[0].bar", 3, false},
		{"单引号字段", "$['foo']", 1, false},
		{"单引号多级", "$['foo'].bar", 2, false},
		{"单引号复杂", "$['复杂字段'].bar", 2, false},
		{"连续点号非法", "$..foo", 0, true}, // 这里期望报错
		{"只有点号", ".", 0, false},
		{"只有$点号", "$.", 0, false},
		{"无效单引号", "$['foo.bar", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := GetJSONPathLevel(c.jsonpath)
			if c.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
		})
	}
}

func TestRemoveFirstJSONPathLevel(t *testing.T) {
	cases := []struct {
		name     string
		jsonpath string
		want     string
		wantErr  bool
	}{
		{"空字符串", "", "", true},
		{"只有$", "$", "", false},
		{"只有字段", "foo", "", false},
		{"$加字段", "$.foo", "", false},
		{"多级点号", "$.foo.bar.a", "bar.a", false},
		{"多级点号无$", "foo.bar.a", "bar.a", false},
		{"数组下标", "$.foo[0]", "[0]", false},
		{"多级数组下标", "$.foo[0][1]", "[0][1]", false},
		{"数组下标后字段", "$.foo[0].bar", "[0].bar", false},
		{"单引号字段", "$['foo']", "", false},
		{"单引号多级", "$['foo'].bar", "bar", false},
		{"单引号复杂", "$['复杂字段'].bar", "bar", false},
		{"无效单引号", "$['foo.bar", "", true},
		{"只有点号", ".", "", false},
		{"只有$点号", "$.", "", false},
		{"实测", "parameter.name", "name", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := RemoveFirstJSONPathLevel(c.jsonpath)
			if c.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, c.want, got)
			}
		})
	}
}
