// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package json

import (
	"fmt"
	"strings"

	jsonschemav5 "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

var SchemaCompiler *jsonschemav5.Compiler

// ValidateJSONSchema 验证JSON字符串是否符合schema
func ValidateJSONSchema(schemaStr string, dataStr string) (bool, error) {
	// 获取 JSON Schema 编译器实例
	compiler := jsonschemav5.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaStr)); err != nil {
		return false, err
	}
	schema, err := compiler.Compile("schema.json") // 使用正确的Compile方法
	if err != nil {
		return false, err
	}
	if strings.ReplaceAll(schemaStr, " ", "") == strings.ReplaceAll(consts.StringJsonSchema, " ", "") {
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
func ExtractFieldValue(schemaStr string, dataStr string, fieldName string) (interface{}, error) {
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
