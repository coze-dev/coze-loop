package jsonmock

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xeipuuv/gojsonschema"
)

// TestGenerateMockData 测试核心函数
func TestGenerateMockData(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		wantErr  bool
		validate func(t *testing.T, result string)
	}{
		{
			name: "简单对象",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"age": {"type": "integer"}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(result), &data)
				assert.NoError(t, err)
				assert.Contains(t, data, "name")
				assert.Contains(t, data, "age")
			},
		},
		{
			name: "带约束条件",
			schema: `{
				"type": "object",
				"properties": {
					"id": {"type": "integer", "minimum": 1, "maximum": 100},
					"email": {"type": "string", "format": "email"},
					"status": {"type": "string", "enum": ["active", "inactive"]}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(result), &data)
				assert.NoError(t, err)

				if id, ok := data["id"].(float64); ok {
					assert.GreaterOrEqual(t, id, 1.0)
					assert.LessOrEqual(t, id, 100.0)
				}

				if email, ok := data["email"].(string); ok {
					assert.Contains(t, email, "@")
				}

				if status, ok := data["status"].(string); ok {
					assert.Contains(t, []string{"active", "inactive"}, status)
				}
			},
		},
		{
			name: "嵌套对象",
			schema: `{
				"type": "object",
				"properties": {
					"user": {
						"type": "object",
						"properties": {
							"name": {"type": "string"},
							"profile": {
								"type": "object",
								"properties": {
									"bio": {"type": "string"}
								}
							}
						}
					}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(result), &data)
				assert.NoError(t, err)
				assert.Contains(t, data, "user")

				if user, ok := data["user"].(map[string]interface{}); ok {
					assert.Contains(t, user, "name")
					assert.Contains(t, user, "profile")

					if profile, ok := user["profile"].(map[string]interface{}); ok {
						assert.Contains(t, profile, "bio")
					}
				}
			},
		},
		{
			name: "数组类型",
			schema: `{
				"type": "object",
				"properties": {
					"tags": {
						"type": "array",
						"items": {"type": "string"},
						"minItems": 2,
						"maxItems": 4
					}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(result), &data)
				assert.NoError(t, err)
				assert.Contains(t, data, "tags")

				if tags, ok := data["tags"].([]interface{}); ok {
					assert.GreaterOrEqual(t, len(tags), 2)
					assert.LessOrEqual(t, len(tags), 4)
					for _, tag := range tags {
						assert.IsType(t, "", tag)
					}
				}
			},
		},
		{
			name: "必需字段",
			schema: `{
				"type": "object",
				"properties": {
					"id": {"type": "integer"},
					"name": {"type": "string"},
					"email": {"type": "string"}
				},
				"required": ["id", "name", "email"]
			}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(result), &data)
				assert.NoError(t, err)
				assert.Contains(t, data, "id")
				assert.Contains(t, data, "name")
				assert.Contains(t, data, "email")
			},
		},
		{
			name: "默认值",
			schema: `{
				"type": "object",
				"properties": {
					"status": {"type": "string", "default": "pending"},
					"count": {"type": "integer", "default": 0}
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, result string) {
				var data map[string]interface{}
				err := json.Unmarshal([]byte(result), &data)
				assert.NoError(t, err)
				assert.Equal(t, "pending", data["status"])
				assert.Equal(t, float64(0), data["count"])
			},
		},
		{
			name: "无效的schema",
			schema: `{
				"type": "invalid_type"
			}`,
			wantErr: true,
		},
		{
			name: "格式错误的JSON",
			schema: `{
				"type": "object",
				"properties": {
					"name": {"type": "string"
				}
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateMockData(tt.schema)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, result)

			// 验证生成的数据是有效的 JSON
			var jsonData interface{}
			err = json.Unmarshal([]byte(result), &jsonData)
			assert.NoError(t, err)

			// 验证生成的数据符合 schema
			schemaLoader := gojsonschema.NewStringLoader(tt.schema)
			documentLoader := gojsonschema.NewStringLoader(result)

			schema, err := gojsonschema.NewSchema(schemaLoader)
			assert.NoError(t, err)

			validation, err := schema.Validate(documentLoader)
			assert.NoError(t, err)
			if !validation.Valid() {
				t.Logf("Validation errors: %v", validation.Errors())
				t.Logf("Generated data: %s", result)
			}
			assert.True(t, validation.Valid(), "Generated data should be valid against schema")

			// 执行自定义验证
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestGenerateString 测试字符串生成
func TestGenerateString(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]interface{}
		check  func(string) bool
	}{
		{
			name: "email格式",
			schema: map[string]interface{}{
				"type":   "string",
				"format": "email",
			},
			check: func(s string) bool {
				return strings.Contains(s, "@") && len(s) > 0
			},
		},
		{
			name: "长度约束",
			schema: map[string]interface{}{
				"type":      "string",
				"minLength": float64(5),
				"maxLength": float64(10),
			},
			check: func(s string) bool {
				return len(s) >= 5 && len(s) <= 10
			},
		},
		{
			name: "枚举值",
			schema: map[string]interface{}{
				"type": "string",
				"enum": []interface{}{"option1", "option2", "option3"},
			},
			check: func(s string) bool {
				return s == "option1" || s == "option2" || s == "option3"
			},
		},
		{
			name: "UUID格式",
			schema: map[string]interface{}{
				"type":   "string",
				"format": "uuid",
			},
			check: func(s string) bool {
				return len(s) == 36 && strings.Count(s, "-") == 4
			},
		},
		{
			name: "日期格式",
			schema: map[string]interface{}{
				"type":   "string",
				"format": "date",
			},
			check: func(s string) bool {
				return len(s) == 10 && strings.Count(s, "-") == 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateString(tt.schema)
			assert.True(t, tt.check(result), "Generated string should meet requirements: %s", result)
		})
	}
}

// TestGenerateInteger 测试整数生成
func TestGenerateInteger(t *testing.T) {
	schema := map[string]interface{}{
		"type":    "integer",
		"minimum": float64(10),
		"maximum": float64(20),
	}

	for i := 0; i < 100; i++ {
		result := generateInteger(schema)
		assert.GreaterOrEqual(t, result, 10)
		assert.LessOrEqual(t, result, 20)
	}
}

// TestGenerateNumber 测试数字生成
func TestGenerateNumber(t *testing.T) {
	schema := map[string]interface{}{
		"type":    "number",
		"minimum": float64(1.5),
		"maximum": float64(10.5),
	}

	for i := 0; i < 100; i++ {
		result := generateNumber(schema)
		assert.GreaterOrEqual(t, result, 1.5)
		assert.LessOrEqual(t, result, 10.5)
	}
}

// TestGenerateArray 测试数组生成
func TestGenerateArray(t *testing.T) {
	schema := map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type": "string",
		},
		"minItems": float64(2),
		"maxItems": float64(4),
	}

	result := generateArray(schema)
	assert.GreaterOrEqual(t, len(result), 2)
	assert.LessOrEqual(t, len(result), 4)

	for _, item := range result {
		assert.IsType(t, "", item)
	}
}

// TestGenerateObject 测试对象生成
func TestGenerateObject(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"age": map[string]interface{}{
				"type": "integer",
			},
		},
		"required": []interface{}{"name"},
	}

	result := generateObject(schema)
	assert.Contains(t, result, "name")
	assert.Contains(t, result, "age")
	assert.IsType(t, "", result["name"])
	assert.IsType(t, 0, result["age"])
}

// TestComplexNestedStructure 测试复杂嵌套结构
func TestComplexNestedStructure(t *testing.T) {
	schema := `{
		"type": "object",
		"properties": {
			"user": {
				"type": "object",
				"properties": {
					"id": {"type": "integer", "minimum": 1, "maximum": 1000},
					"name": {"type": "string", "minLength": 2, "maxLength": 50},
					"email": {"type": "string", "format": "email"},
					"age": {"type": "integer", "minimum": 18, "maximum": 100},
					"status": {"type": "string", "enum": ["active", "inactive", "pending"]},
					"tags": {
						"type": "array",
						"items": {"type": "string"},
						"minItems": 1,
						"maxItems": 5
					},
					"profile": {
						"type": "object",
						"properties": {
							"bio": {"type": "string", "maxLength": 200},
							"website": {"type": "string", "format": "uri"},
							"preferences": {
								"type": "object",
								"properties": {
									"theme": {"type": "string", "enum": ["light", "dark"]},
									"notifications": {"type": "boolean"}
								}
							}
						}
					}
				},
				"required": ["id", "name", "email"]
			}
		}
	}`

	result, err := GenerateMockData(schema)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	// 验证生成的数据符合 schema
	schemaLoader := gojsonschema.NewStringLoader(schema)
	documentLoader := gojsonschema.NewStringLoader(result)

	schemaObj, err := gojsonschema.NewSchema(schemaLoader)
	assert.NoError(t, err)

	validation, err := schemaObj.Validate(documentLoader)
	assert.NoError(t, err)
	if !validation.Valid() {
		t.Logf("Validation errors: %v", validation.Errors())
		t.Logf("Generated data: %s", result)
	}
	assert.True(t, validation.Valid())
}

// TestEdgeCases 测试边界情况
func TestEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		schema string
	}{
		{
			name: "空对象",
			schema: `{
				"type": "object",
				"properties": {}
			}`,
		},
		{
			name: "空数组",
			schema: `{
				"type": "array",
				"items": {"type": "string"},
				"minItems": 0,
				"maxItems": 0
			}`,
		},
		{
			name: "只有默认值",
			schema: `{
				"type": "object",
				"properties": {
					"value": {"default": "test"}
				}
			}`,
		},
		{
			name: "只有枚举值",
			schema: `{
				"type": "object",
				"properties": {
					"choice": {"enum": ["a", "b", "c"]}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateMockData(tt.schema)
			assert.NoError(t, err)
			assert.NotEmpty(t, result)

			// 验证是有效的JSON
			var data interface{}
			err = json.Unmarshal([]byte(result), &data)
			assert.NoError(t, err)
		})
	}
}

// BenchmarkGenerateMockData 性能测试
func BenchmarkGenerateMockData(b *testing.B) {
	schema := `{
		"type": "object",
		"properties": {
			"id": {"type": "integer"},
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"},
			"tags": {
				"type": "array",
				"items": {"type": "string"},
				"maxItems": 3
			}
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateMockData(schema)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComplexSchema 复杂schema性能测试
func BenchmarkComplexSchema(b *testing.B) {
	schema := `{
		"type": "object",
		"properties": {
			"users": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"id": {"type": "integer"},
						"name": {"type": "string"},
						"email": {"type": "string", "format": "email"},
						"profile": {
							"type": "object",
							"properties": {
								"bio": {"type": "string"},
								"tags": {
									"type": "array",
									"items": {"type": "string"}
								}
							}
						}
					}
				},
				"maxItems": 10
			}
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateMockData(schema)
		if err != nil {
			b.Fatal(err)
		}
	}
}
