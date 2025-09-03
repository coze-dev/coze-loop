// Package jsonschema_mock 提供基于 JSON Schema 生成 Mock 数据的功能
package jsonmock

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/xeipuuv/gojsonschema"
)

// GenerateMockData 根据 JSON Schema 生成符合规范的 mock 数据
// 参数:
//   - schemaJSON: JSON Schema 字符串
// 返回:
//   - string: 生成的 mock 数据 JSON 字符串
//   - error: 错误信息
func GenerateMockData(schemaJSON string) (string, error) {
	// 验证 schema 格式
	schemaLoader := gojsonschema.NewStringLoader(schemaJSON)
	_, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return "", fmt.Errorf("invalid schema: %v", err)
	}

	// 解析 schema
	var schemaMap map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schemaMap); err != nil {
		return "", fmt.Errorf("parse schema failed: %v", err)
	}

	// 生成 mock 数据
	mockData := generateFromSchema(schemaMap)

	// 序列化为 JSON
	jsonBytes, err := json.MarshalIndent(mockData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal failed: %v", err)
	}

	return string(jsonBytes), nil
}

// generateFromSchema 根据 schema 递归生成数据
func generateFromSchema(schema map[string]interface{}) interface{} {
	// 检查默认值
	if defaultVal, exists := schema["default"]; exists {
		return defaultVal
	}

	// 检查枚举值
	if enumVals, exists := schema["enum"]; exists {
		if enumArray, ok := enumVals.([]interface{}); ok && len(enumArray) > 0 {
			return enumArray[rand.Intn(len(enumArray))]
		}
	}

	// 根据类型生成数据
	schemaType, ok := schema["type"].(string)
	if !ok {
		return nil
	}

	switch schemaType {
	case "string":
		return generateString(schema)
	case "number":
		return generateNumber(schema)
	case "integer":
		return generateInteger(schema)
	case "boolean":
		return gofakeit.Bool()
	case "array":
		return generateArray(schema)
	case "object":
		return generateObject(schema)
	default:
		return nil
	}
}

// generateString 生成字符串类型数据
func generateString(schema map[string]interface{}) string {
	// 先检查枚举值（优先级最高）
	if enumVals, exists := schema["enum"]; exists {
		if enumArray, ok := enumVals.([]interface{}); ok && len(enumArray) > 0 {
			selected := enumArray[rand.Intn(len(enumArray))]
			if str, ok := selected.(string); ok {
				return str
			}
		}
	}

	// 检查格式
	if format, exists := schema["format"]; exists {
		switch format {
		case "email":
			return gofakeit.Email()
		case "date":
			return gofakeit.Date().Format("2006-01-02")
		case "date-time":
			return gofakeit.Date().Format(time.RFC3339)
		case "uri":
			return gofakeit.URL()
		case "uuid":
			return gofakeit.UUID()
		}
	}

	// 检查模式匹配
	if pattern, exists := schema["pattern"]; exists {
		if patternStr, ok := pattern.(string); ok {
			return generateStringByPattern(patternStr)
		}
	}

	// 处理长度约束
	minLength := 1
	maxLength := 20

	if min, exists := schema["minLength"]; exists {
		if minVal, ok := min.(float64); ok {
			minLength = int(minVal)
		}
	}

	if max, exists := schema["maxLength"]; exists {
		if maxVal, ok := max.(float64); ok {
			maxLength = int(maxVal)
		}
	}

	if maxLength < minLength {
		maxLength = minLength
	}

	// 生成指定长度的字符串
	length := minLength + rand.Intn(maxLength-minLength+1)
	return gofakeit.LetterN(uint(length))
}

// generateStringByPattern 根据正则模式生成字符串
func generateStringByPattern(pattern string) string {
	// 简单的模式匹配实现
	switch {
	case strings.Contains(pattern, "@") && strings.Contains(pattern, "\\."):
		return gofakeit.Email()
	case regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(pattern):
		return gofakeit.Date().Format("2006-01-02")
	case strings.Contains(pattern, "\\d"):
		// 生成数字字符串
		return strconv.Itoa(gofakeit.Number(1000, 9999))
	default:
		// 对于复杂模式，返回通用字符串
		return gofakeit.Word()
	}
}

// generateNumber 生成数字类型数据
func generateNumber(schema map[string]interface{}) float64 {
	minimum := 0.0
	maximum := 100.0

	if min, exists := schema["minimum"]; exists {
		if minVal, ok := min.(float64); ok {
			minimum = minVal
		}
	}

	if max, exists := schema["maximum"]; exists {
		if maxVal, ok := max.(float64); ok {
			maximum = maxVal
		}
	}

	if maximum <= minimum {
		return minimum
	}

	return minimum + rand.Float64()*(maximum-minimum)
}

// generateInteger 生成整数类型数据
func generateInteger(schema map[string]interface{}) int {
	minimum := 0
	maximum := 100

	if min, exists := schema["minimum"]; exists {
		if minVal, ok := min.(float64); ok {
			minimum = int(minVal)
		}
	}

	if max, exists := schema["maximum"]; exists {
		if maxVal, ok := max.(float64); ok {
			maximum = int(maxVal)
		}
	}

	if maximum <= minimum {
		return minimum
	}

	return minimum + rand.Intn(maximum-minimum+1)
}

// generateArray 生成数组类型数据
func generateArray(schema map[string]interface{}) []interface{} {
	minItems := 1
	maxItems := 5

	if min, exists := schema["minItems"]; exists {
		if minVal, ok := min.(float64); ok {
			minItems = int(minVal)
		}
	}

	if max, exists := schema["maxItems"]; exists {
		if maxVal, ok := max.(float64); ok {
			maxItems = int(maxVal)
		}
	}

	if maxItems < minItems {
		maxItems = minItems
	}

	itemCount := minItems + rand.Intn(maxItems-minItems+1)
	result := make([]interface{}, itemCount)

	if items, exists := schema["items"]; exists {
		if itemSchema, ok := items.(map[string]interface{}); ok {
			for i := 0; i < itemCount; i++ {
				result[i] = generateFromSchema(itemSchema)
			}
		}
	} else {
		// 如果没有定义 items schema，生成字符串数组
		for i := 0; i < itemCount; i++ {
			result[i] = gofakeit.Word()
		}
	}

	return result
}

// generateObject 生成对象类型数据
func generateObject(schema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	if properties, exists := schema["properties"]; exists {
		if props, ok := properties.(map[string]interface{}); ok {
			for propName, propSchema := range props {
				if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
					result[propName] = generateFromSchema(propSchemaMap)
				}
			}
		}
	}

	// 处理 required 字段，确保必需字段存在
	if required, exists := schema["required"]; exists {
		if requiredArray, ok := required.([]interface{}); ok {
			for _, reqField := range requiredArray {
				if fieldName, ok := reqField.(string); ok {
					if _, exists := result[fieldName]; !exists {
						// 如果必需字段不存在，生成一个默认值
						result[fieldName] = gofakeit.Word()
					}
				}
			}
		}
	}

	return result
}

// init 初始化随机种子
func init() {
	rand.Seed(time.Now().UnixNano())
	gofakeit.Seed(time.Now().UnixNano())
}
