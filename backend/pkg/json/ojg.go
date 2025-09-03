// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package json

import (
	"fmt"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// GetByJSONPath 通过 JSONPath 从 JSON 字符串中获取数据
// 如果 jsonpath 为空，则返回整个文档
// 如果找到多个匹配项，则返回数组
// 如果找到单个匹配项，则返回该项
// 如果未找到匹配项，则返回 nil
// recursive 是否对data进行递归解析，递归解析会将data中的所有字符串都解析为json对象
func GetByJSONPath(data, jsonpath string, recursive bool) (interface{}, error) {
	if jsonpath == "" {
		return data, nil
	}

	obj, err := oj.ParseString(data)
	if err != nil {
		return nil, err
	}

	if recursive {
		obj = recursiveUnmarshal(obj)
	}

	parser, err := jp.ParseString(jsonpath)
	if err != nil {
		return nil, err
	}
	result := parser.Get(obj)
	if len(result) == 0 {
		return nil, nil
	} else if len(result) == 1 {
		return result[0], nil
	}
	return result, nil
}

// GetStringByJSONPath 通过 JSONPath 从 JSON 字符串中获取数据，并将结果转换为字符串
func GetStringByJSONPath(data, jsonpath string) (string, error) {
	result, err := GetByJSONPath(data, jsonpath, false)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return ConvertToString(result)
}

// GetStringByJSONPath 通过 JSONPath 从 JSON 字符串中获取数据，并将结果转换为字符串。会递归解析data中的所有字符串
func GetStringByJSONPathRecursively(data, jsonpath string) (string, error) {
	result, err := GetByJSONPath(data, jsonpath, true)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", nil
	}
	return ConvertToString(result)
}

// ConvertToString 将任意类型转换为字符串
// - 字符串类型直接返回
// - 数字和布尔类型使用 fmt.Sprint
// - 其他类型（如数组、对象）使用 marshalJSON 转换为 JSON 字符串
func ConvertToString(result interface{}) (string, error) {
	switch v := result.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return fmt.Sprint(v), nil
	default:
		bytes, err := Marshal(v)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	}
}

// GetFirstJSONPathField 提取 JSONPath 的第一级字段名
func GetFirstJSONPathField(jsonpath string) (string, error) {
	path := jsonpath
	if len(path) == 0 {
		return "", fmt.Errorf("jsonpath 为空")
	}
	if path[0] == '$' {
		path = path[1:]
	}
	// 跳过前导的点或中括号
	for len(path) > 0 {
		if path[0] == '.' {
			// 检查是否为连续点（..）
			if len(path) > 1 && path[1] == '.' {
				return "", fmt.Errorf("不支持 .. 语法或未找到字段名")
			}
			path = path[1:]
			continue
		}
		if path[0] == '[' {
			// 处理 ['field'] 或 [0] 这种情况
			if len(path) > 1 && path[1] == '\'' {
				// ['field']
				end := 2
				for end < len(path) && path[end] != '\'' {
					end++
				}
				if end < len(path) && end+1 < len(path) && path[end+1] == ']' {
					return path[2:end], nil
				}
				return "", fmt.Errorf("jsonpath 格式错误")
			} else {
				// [0] 或其他下标，跳过到下一个 ']' 后继续
				end := 1
				for end < len(path) && path[end] != ']' {
					end++
				}
				if end < len(path) {
					path = path[end+1:]
					continue
				}
				return "", fmt.Errorf("jsonpath 格式错误")
			}
		}
		break
	}
	// 处理 field 或 field[0] 或 field.bar
	end := 0
	for end < len(path) && path[end] != '.' && path[end] != '[' {
		end++
	}
	if end == 0 {
		return "", fmt.Errorf("未找到字段名")
	}
	return path[:end], nil
}

// GetJSONPathLevel 计算 JSONPath 的层级数量，兼容 $ 开头和非 $ 开头
func GetJSONPathLevel(jsonpath string) (int, error) {
	path := jsonpath
	if len(path) == 0 {
		return 0, fmt.Errorf("jsonpath 为空")
	}
	if path[0] == '$' {
		path = path[1:]
	}
	level := 0
	for len(path) > 0 {
		// 跳过前导的点或中括号
		for len(path) > 0 && (path[0] == '.' || path[0] == '[') {
			// 检查是否为连续点（..）
			if path[0] == '.' && len(path) > 1 && path[1] == '.' {
				return 0, fmt.Errorf("不支持 .. 语法或未找到字段名")
			}
			path = path[1:]
		}
		if len(path) == 0 {
			break
		}
		// 处理 ['field'] 这种情况
		if path[0] == '\'' {
			end := 1
			for end < len(path) && path[end] != '\'' {
				end++
			}
			if end < len(path) {
				level++
				// 跳过 ']'
				if end+1 < len(path) && path[end+1] == ']' {
					path = path[end+2:]
					// 跳过后续的点或中括号
					for len(path) > 0 && (path[0] == '.' || path[0] == '[') {
						// 检查是否为连续点（..）
						if path[0] == '.' && len(path) > 1 && path[1] == '.' {
							return 0, fmt.Errorf("不支持 .. 语法或未找到字段名")
						}
						path = path[1:]
					}
					continue
				}
				return level, fmt.Errorf("jsonpath 格式错误")
			}
			return level, fmt.Errorf("jsonpath 格式错误")
		}
		// 处理普通字段
		end := 0
		for end < len(path) && path[end] != '.' && path[end] != '[' {
			end++
		}
		if end > 0 {
			level++
			path = path[end:]
		} else {
			break
		}
	}
	return level, nil
}

// RemoveFirstJSONPathLevel 删除 JSONPath 的第一级字段，保留其余层级
// 例如：$.foo.bar => bar，foo.bar => bar，$['foo'].bar => bar，$.foo[0].bar => [0].bar
func RemoveFirstJSONPathLevel(jsonpath string) (string, error) {
	path := jsonpath
	if len(path) == 0 {
		return "", fmt.Errorf("jsonpath 为空")
	}
	if path[0] == '$' {
		path = path[1:]
	}
	// 跳过前导的点或中括号
	for len(path) > 0 && (path[0] == '.' || path[0] == '[') {
		path = path[1:]
	}
	if len(path) == 0 {
		return "", nil
	}
	// 处理 ['field'] 这种情况
	if path[0] == '\'' {
		end := 1
		for end < len(path) && path[end] != '\'' {
			end++
		}
		if end < len(path) && end+1 < len(path) && path[end+1] == ']' {
			// 跳过 ']'
			path = path[end+2:]
		} else {
			return "", fmt.Errorf("jsonpath 格式错误")
		}
	} else {
		// 普通字段
		end := 0
		for end < len(path) && path[end] != '.' && path[end] != '[' {
			end++
		}
		path = path[end:]
	}
	// 移除返回路径前面的 .
	for len(path) > 0 && path[0] == '.' {
		path = path[1:]
	}
	return path, nil
}

func recursiveUnmarshal(v any) any {
	switch val := v.(type) {
	case map[string]any:
		for k, v2 := range val {
			val[k] = recursiveUnmarshal(v2)
		}
		return val
	case []any:
		for i, v2 := range val {
			val[i] = recursiveUnmarshal(v2)
		}
		return val
	case string:
		parsed, err := oj.ParseString(val)
		if err == nil {
			return recursiveUnmarshal(parsed)
		}
		return val
	default:
		return val
	}
}
