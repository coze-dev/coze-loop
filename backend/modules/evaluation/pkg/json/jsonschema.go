// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package json

import (
	"fmt"
	"strings"

	jsonschemav5 "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

var SchemaCompiler *jsonschemav5.Compiler

// isStringTypeSchema 判断 schema 是否语义等价于 {"type":"string"}：
// 只要顶层是对象、type=="string"（或 type 为包含 "string" 的数组），就视为字符串型 schema，
// 允许附带 description/title/pattern/minLength/enum 等其它字段。
// 解析失败或结构不符时返回 false，走回原有的通用校验路径。
func isStringTypeSchema(schemaStr string) bool {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(schemaStr)), &m); err != nil {
		return false
	}
	t, ok := m["type"]
	if !ok {
		return false
	}
	switch v := t.(type) {
	case string:
		return v == "string"
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok && s == "string" {
				return true
			}
		}
	}
	return false
}

// ValidateJSONSchema 验证JSON字符串是否符合schema
func ValidateJSONSchema(schemaStr, dataStr string) (bool, error) {
	// 获取 JSON Schema 编译器实例
	compiler := jsonschemav5.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaStr)); err != nil {
		return false, err
	}
	schema, err := compiler.Compile("schema.json") // 使用正确的Compile方法
	if err != nil {
		return false, err
	}
	// 语义等价于 {"type":"string"} 的 schema（允许附带 description/title 等注解、或包含 minLength/pattern/enum 等对字符串收紧的约束）
	// 都应把入参当作纯字符串处理：把 dataStr 包一层引号，避免下面的 json.Unmarshal 把「字符串化 JSON」解成 map/array 后再拿去校验 type:"string" 而失败。
	if isStringTypeSchema(schemaStr) {
		dataStr = fmt.Sprintf("\"%s\"", dataStr)
	}
	var data interface{}
	err = json.Unmarshal([]byte(dataStr), &data)
	if err != nil {
		// 当解析失败或类型不是基础string类型时，使用原始字符串
		data = dataStr
	}
	err = schema.Validate(data) // 改为验证处理后的data
	if err != nil {
		return false, err
	}
	return true, nil
}

// ExtractFieldValue 用 JSON Schema 验证 JSON 数据并提取指定字段的值
func ExtractFieldValue(schemaStr, dataStr, fieldName string) (interface{}, error) {
	// 获取 JSON Schema 编译器实例
	compiler := jsonschemav5.NewCompiler()

	// 添加 JSON Schema 资源
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaStr)); err != nil {
		return nil, err
	}

	// 编译 JSON Schema
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, err
	}

	// 解析 JSON 数据
	var data interface{}
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return nil, err
	}

	// 验证 JSON 数据是否符合 Schema
	if err := schema.Validate(data); err != nil {
		return nil, err
	}

	// 将数据转换为 map[string]interface{} 以便提取字段值
	if dataMap, ok := data.(map[string]interface{}); ok {
		if value, exists := dataMap[fieldName]; exists {
			return value, nil
		}
		return nil, fmt.Errorf("field %s not found in JSON data", fieldName)
	}

	return nil, fmt.Errorf("JSON data is not an object")
}
