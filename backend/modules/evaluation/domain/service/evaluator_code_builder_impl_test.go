// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestCodeBuilderFactoryImpl_CreateBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		languageType entity.LanguageType
		setupMocks   func(*gomock.Controller) *mocks.MockIRuntimeManager
		wantErr      bool
		wantType     string
	}{
		{
			name:         "创建Python代码构建器成功",
			languageType: entity.LanguageTypePython,
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockIRuntimeManager {
				mockRM := mocks.NewMockIRuntimeManager(ctrl)
				mockRuntime := mocks.NewMockIRuntime(ctrl)
				mockRM.EXPECT().GetRuntime(entity.LanguageTypePython).Return(mockRuntime, nil)
				return mockRM
			},
			wantErr:  false,
			wantType: "PythonCodeBuilder",
		},
		{
			name:         "创建JavaScript代码构建器成功",
			languageType: entity.LanguageTypeJS,
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockIRuntimeManager {
				mockRM := mocks.NewMockIRuntimeManager(ctrl)
				mockRuntime := mocks.NewMockIRuntime(ctrl)
				mockRM.EXPECT().GetRuntime(entity.LanguageTypeJS).Return(mockRuntime, nil)
				return mockRM
			},
			wantErr:  false,
			wantType: "JavaScriptCodeBuilder",
		},
		{
			name:         "不支持的语言类型",
			languageType: entity.LanguageType("unsupported"),
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockIRuntimeManager {
				return mocks.NewMockIRuntimeManager(ctrl)
			},
			wantErr: true,
		},
		{
			name:         "Runtime获取失败但不影响构建器创建",
			languageType: entity.LanguageTypePython,
			setupMocks: func(ctrl *gomock.Controller) *mocks.MockIRuntimeManager {
				mockRM := mocks.NewMockIRuntimeManager(ctrl)
				mockRM.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime error"))
				return mockRM
			},
			wantErr:  false,
			wantType: "PythonCodeBuilder",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			factory := &CodeBuilderFactoryImpl{}
			if tt.setupMocks != nil {
				mockRM := tt.setupMocks(ctrl)
				factory.SetRuntimeManager(mockRM)
			}

			builder, err := factory.CreateBuilder(tt.languageType)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, builder)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, builder)

				// 验证构建器类型
				switch tt.wantType {
				case "PythonCodeBuilder":
					_, ok := builder.(*PythonCodeBuilder)
					assert.True(t, ok, "应该返回PythonCodeBuilder类型")
				case "JavaScriptCodeBuilder":
					_, ok := builder.(*JavaScriptCodeBuilder)
					assert.True(t, ok, "应该返回JavaScriptCodeBuilder类型")
				}
			}
		})
	}
}

func TestCodeBuilderFactoryImpl_GetSupportedLanguages(t *testing.T) {
	t.Parallel()

	factory := &CodeBuilderFactoryImpl{}
	languages := factory.GetSupportedLanguages()

	assert.Len(t, languages, 2)
	assert.Contains(t, languages, entity.LanguageTypePython)
	assert.Contains(t, languages, entity.LanguageTypeJS)
}

func TestCodeBuilderFactoryImpl_SetRuntimeManager(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	factory := &CodeBuilderFactoryImpl{}
	mockRM := mocks.NewMockIRuntimeManager(ctrl)

	factory.SetRuntimeManager(mockRM)
	assert.Equal(t, mockRM, factory.runtimeManager)
}

func TestPythonCodeBuilder_BuildCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupBuilder func(*gomock.Controller) *PythonCodeBuilder
		input        *entity.EvaluatorInputData
		codeVersion  *entity.CodeEvaluatorVersion
		wantErr      bool
		validateCode func(*testing.T, string)
	}{
		{
			name: "成功构建Python代码",
			setupBuilder: func(ctrl *gomock.Controller) *PythonCodeBuilder {
				mockRuntime := mocks.NewMockIRuntime(ctrl)
				mockRuntime.EXPECT().GetReturnValFunction().Return("def return_val(value): print(value)")

				builder := NewPythonCodeBuilder()
				builder.SetRuntime(mockRuntime)
				return builder
			},
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
				EvaluateTargetOutputFields: map[string]*entity.Content{
					"model_output": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test output"),
					},
				},
			},
			codeVersion: &entity.CodeEvaluatorVersion{
				CodeContent: "def exec_evaluation(turn):\n    return {'score': 1.0}",
			},
			wantErr: false,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "def return_val(value): print(value)")
				assert.Contains(t, code, "def exec_evaluation(turn):")
				assert.Contains(t, code, "evaluate_dataset_fields")
				assert.Contains(t, code, "evaluate_target_output_fields")
			},
		},
		{
			name: "没有Runtime时使用默认实现",
			setupBuilder: func(ctrl *gomock.Controller) *PythonCodeBuilder {
				return NewPythonCodeBuilder()
			},
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			codeVersion: &entity.CodeEvaluatorVersion{
				CodeContent: "def exec_evaluation(turn):\n    return {'score': 1.0}",
			},
			wantErr: false,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "def return_val(value):")
				assert.Contains(t, code, "global _return_val_output")
				assert.Contains(t, code, "def exec_evaluation(turn):")
			},
		},
		{
			name: "处理MultiPart内容",
			setupBuilder: func(ctrl *gomock.Controller) *PythonCodeBuilder {
				return NewPythonCodeBuilder()
			},
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"multipart_input": {
						ContentType: gptr.Of(entity.ContentTypeMultipart),
						MultiPart: []*entity.Content{
							{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("part1"),
							},
							{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("part2"),
							},
						},
					},
				},
			},
			codeVersion: &entity.CodeEvaluatorVersion{
				CodeContent: "def exec_evaluation(turn):\n    return {'score': 1.0}",
			},
			wantErr: false,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "multi_part")
				assert.Contains(t, code, "part1")
				assert.Contains(t, code, "part2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder := tt.setupBuilder(ctrl)

			code, err := builder.BuildCode(tt.input, tt.codeVersion)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, code)
				if tt.validateCode != nil {
					tt.validateCode(t, code)
				}
			}
		})
	}
}

func TestJavaScriptCodeBuilder_BuildCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupBuilder func(*gomock.Controller) *JavaScriptCodeBuilder
		input        *entity.EvaluatorInputData
		codeVersion  *entity.CodeEvaluatorVersion
		wantErr      bool
		validateCode func(*testing.T, string)
	}{
		{
			name: "成功构建JavaScript代码",
			setupBuilder: func(ctrl *gomock.Controller) *JavaScriptCodeBuilder {
				mockRuntime := mocks.NewMockIRuntime(ctrl)
				mockRuntime.EXPECT().GetReturnValFunction().Return("function return_val(value) { console.log(value); }")

				builder := NewJavaScriptCodeBuilder()
				builder.SetRuntime(mockRuntime)
				return builder
			},
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			codeVersion: &entity.CodeEvaluatorVersion{
				CodeContent: "function execEvaluation(turn) {\n    return {score: 1.0};\n}",
			},
			wantErr: false,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "function return_val(value) { console.log(value); }")
				assert.Contains(t, code, "function execEvaluation(turn)")
				assert.Contains(t, code, "evaluate_dataset_fields")
			},
		},
		{
			name: "没有Runtime时使用默认实现",
			setupBuilder: func(ctrl *gomock.Controller) *JavaScriptCodeBuilder {
				return NewJavaScriptCodeBuilder()
			},
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"user_input": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test input"),
					},
				},
			},
			codeVersion: &entity.CodeEvaluatorVersion{
				CodeContent: "function execEvaluation(turn) {\n    return {score: 1.0};\n}",
			},
			wantErr: false,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "function return_val(value)")
				assert.Contains(t, code, "console.log(value)")
			},
		},
		{
			name: "处理Image内容",
			setupBuilder: func(ctrl *gomock.Controller) *JavaScriptCodeBuilder {
				return NewJavaScriptCodeBuilder()
			},
			input: &entity.EvaluatorInputData{
				EvaluateTargetOutputFields: map[string]*entity.Content{
					"image_output": {
						ContentType: gptr.Of(entity.ContentTypeImage),
						Image: &entity.Image{
							URL: gptr.Of("http://example.com/image.jpg"),
						},
					},
				},
			},
			codeVersion: &entity.CodeEvaluatorVersion{
				CodeContent: "function execEvaluation(turn) {\n    return {score: 1.0};\n}",
			},
			wantErr: false,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "image_output")
				assert.Contains(t, code, "http://example.com/image.jpg")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder := tt.setupBuilder(ctrl)

			code, err := builder.BuildCode(tt.input, tt.codeVersion)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, code)
				if tt.validateCode != nil {
					tt.validateCode(t, code)
				}
			}
		})
	}
}

func TestPythonCodeBuilder_BuildSyntaxCheckCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupBuilder func(*gomock.Controller) *PythonCodeBuilder
		userCode     string
		validateCode func(*testing.T, string)
	}{
		{
			name: "构建语法检查代码",
			setupBuilder: func(ctrl *gomock.Controller) *PythonCodeBuilder {
				mockRuntime := mocks.NewMockIRuntime(ctrl)
				mockRuntime.EXPECT().GetReturnValFunction().Return("def return_val(value): pass")

				builder := NewPythonCodeBuilder()
				builder.SetRuntime(mockRuntime)
				return builder
			},
			userCode: `def exec_evaluation(turn):
    return {"score": 1.0}`,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "def return_val(value): pass")
				assert.Contains(t, code, "def exec_evaluation(turn):")
				assert.Contains(t, code, "ast.parse(")
			},
		},
		{
			name: "处理特殊字符转义",
			setupBuilder: func(ctrl *gomock.Controller) *PythonCodeBuilder {
				return NewPythonCodeBuilder()
			},
			userCode: `def test():
    s = """triple quotes"""
    return s`,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, `\"\"\"`)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder := tt.setupBuilder(ctrl)
			code := builder.BuildSyntaxCheckCode(tt.userCode)

			assert.NotEmpty(t, code)
			if tt.validateCode != nil {
				tt.validateCode(t, code)
			}
		})
	}
}

func TestJavaScriptCodeBuilder_BuildSyntaxCheckCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		setupBuilder func(*gomock.Controller) *JavaScriptCodeBuilder
		userCode     string
		validateCode func(*testing.T, string)
	}{
		{
			name: "构建JavaScript语法检查代码",
			setupBuilder: func(ctrl *gomock.Controller) *JavaScriptCodeBuilder {
				mockRuntime := mocks.NewMockIRuntime(ctrl)
				mockRuntime.EXPECT().GetReturnValFunction().Return("function return_val(value) { }")

				builder := NewJavaScriptCodeBuilder()
				builder.SetRuntime(mockRuntime)
				return builder
			},
			userCode: `function execEvaluation(turn) {
    return {score: 1.0};
}`,
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "function return_val(value) { }")
				assert.Contains(t, code, "function execEvaluation(turn)")
			},
		},
		{
			name: "处理模板字符串转义",
			setupBuilder: func(ctrl *gomock.Controller) *JavaScriptCodeBuilder {
				return NewJavaScriptCodeBuilder()
			},
			userCode: "const template = `Hello ${name}`;",
			validateCode: func(t *testing.T, code string) {
				assert.Contains(t, code, "\\$")
				assert.Contains(t, code, "\\`")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			builder := tt.setupBuilder(ctrl)
			code := builder.BuildSyntaxCheckCode(tt.userCode)

			assert.NotEmpty(t, code)
			if tt.validateCode != nil {
				tt.validateCode(t, code)
			}
		})
	}
}

func TestPythonCodeBuilder_GetLanguageType(t *testing.T) {
	t.Parallel()

	builder := NewPythonCodeBuilder()
	assert.Equal(t, entity.LanguageTypePython, builder.GetLanguageType())
}

func TestJavaScriptCodeBuilder_GetLanguageType(t *testing.T) {
	t.Parallel()

	builder := NewJavaScriptCodeBuilder()
	assert.Equal(t, entity.LanguageTypeJS, builder.GetLanguageType())
}

func TestPythonCodeBuilder_SetRuntime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := NewPythonCodeBuilder()
	mockRuntime := mocks.NewMockIRuntime(ctrl)

	builder.SetRuntime(mockRuntime)
	assert.Equal(t, mockRuntime, builder.runtime)
}

func TestJavaScriptCodeBuilder_SetRuntime(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	builder := NewJavaScriptCodeBuilder()
	mockRuntime := mocks.NewMockIRuntime(ctrl)

	builder.SetRuntime(mockRuntime)
	assert.Equal(t, mockRuntime, builder.runtime)
}

func TestNewCodeBuilderFactory(t *testing.T) {
	t.Parallel()

	factory := NewCodeBuilderFactory()
	assert.NotNil(t, factory)

	impl, ok := factory.(*CodeBuilderFactoryImpl)
	assert.True(t, ok)
	assert.NotNil(t, impl)
}

func TestPythonCodeBuilder_buildInputData_EdgeCases(t *testing.T) {
	t.Parallel()

	builder := NewPythonCodeBuilder()

	tests := []struct {
		name         string
		input        *entity.EvaluatorInputData
		wantErr      bool
		validateData func(*testing.T, map[string]interface{})
	}{
		{
			name: "空输入数据",
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields:      map[string]*entity.Content{},
				EvaluateTargetOutputFields: map[string]*entity.Content{},
			},
			wantErr: false,
			validateData: func(t *testing.T, data map[string]interface{}) {
				assert.Empty(t, data)
			},
		},
		{
			name: "包含nil Content的字段",
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"valid_field": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("valid content"),
					},
					"nil_field": nil,
				},
			},
			wantErr: false,
			validateData: func(t *testing.T, data map[string]interface{}) {
				fields, exists := data["evaluate_dataset_fields"]
				assert.True(t, exists)
				fieldsMap := fields.(map[string]interface{})
				assert.Contains(t, fieldsMap, "valid_field")
				assert.NotContains(t, fieldsMap, "nil_field")
			},
		},
		{
			name: "处理Audio内容",
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"audio_field": {
						ContentType: gptr.Of(entity.ContentTypeAudio),
						Audio: &entity.Audio{
							URL: gptr.Of("http://example.com/audio.mp3"),
						},
					},
				},
			},
			wantErr: false,
			validateData: func(t *testing.T, data map[string]interface{}) {
				fields, exists := data["evaluate_dataset_fields"]
				assert.True(t, exists)
				fieldsMap := fields.(map[string]interface{})
				audioField := fieldsMap["audio_field"].(map[string]interface{})
				assert.Equal(t, "Audio", audioField["content_type"])
				assert.NotNil(t, audioField["audio"])
			},
		},
		{
			name: "处理Ext字段",
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"test_field": {
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("test"),
					},
				},
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			wantErr: false,
			validateData: func(t *testing.T, data map[string]interface{}) {
				ext, exists := data["ext"]
				assert.True(t, exists)
				extMap := ext.(map[string]string)
				assert.Equal(t, "value1", extMap["key1"])
				assert.Equal(t, "value2", extMap["key2"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := builder.buildInputData(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateData != nil {
					tt.validateData(t, data)
				}
			}
		})
	}
}

func TestJavaScriptCodeBuilder_buildInputData_EdgeCases(t *testing.T) {
	t.Parallel()

	builder := NewJavaScriptCodeBuilder()

	tests := []struct {
		name         string
		input        *entity.EvaluatorInputData
		wantErr      bool
		validateData func(*testing.T, map[string]interface{})
	}{
		{
			name: "处理复杂MultiPart嵌套",
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"complex_multipart": {
						ContentType: gptr.Of(entity.ContentTypeMultipart),
						MultiPart: []*entity.Content{
							{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("text part"),
							},
							{
								ContentType: gptr.Of(entity.ContentTypeImage),
								Image: &entity.Image{
									URL: gptr.Of("http://example.com/nested.jpg"),
								},
							},
							nil, // nil part should be skipped
						},
					},
				},
			},
			wantErr: false,
			validateData: func(t *testing.T, data map[string]interface{}) {
				fields, exists := data["evaluate_dataset_fields"]
				assert.True(t, exists)
				fieldsMap := fields.(map[string]interface{})
				multipartField := fieldsMap["complex_multipart"].(map[string]interface{})
				multiPartArray := multipartField["multi_part"].([]map[string]interface{})
				assert.Len(t, multiPartArray, 2) // nil part should be skipped
			},
		},
		{
			name: "Content没有任何内容字段",
			input: &entity.EvaluatorInputData{
				EvaluateDatasetFields: map[string]*entity.Content{
					"empty_content": {
						ContentType: gptr.Of(entity.ContentTypeText),
						// 没有Text, Image, Audio或MultiPart字段
					},
				},
			},
			wantErr: false,
			validateData: func(t *testing.T, data map[string]interface{}) {
				fields, exists := data["evaluate_dataset_fields"]
				assert.True(t, exists)
				fieldsMap := fields.(map[string]interface{})
				emptyContent := fieldsMap["empty_content"].(map[string]interface{})
				assert.Equal(t, "Text", emptyContent["content_type"])
				// 应该只有content_type字段
				assert.Len(t, emptyContent, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			data, err := builder.buildInputData(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateData != nil {
					tt.validateData(t, data)
				}
			}
		})
	}
}

func TestJavaScriptCodeBuilder_validateInputData(t *testing.T) {
	t.Parallel()

	builder := NewJavaScriptCodeBuilder()

	tests := []struct {
		name      string
		inputData map[string]interface{}
		wantErr   bool
	}{
		{
			name: "有效数据 - 包含evaluate_dataset_fields",
			inputData: map[string]interface{}{
				"evaluate_dataset_fields": map[string]interface{}{
					"field1": "value1",
				},
			},
			wantErr: false,
		},
		{
			name: "有效数据 - 包含evaluate_target_output_fields",
			inputData: map[string]interface{}{
				"evaluate_target_output_fields": map[string]interface{}{
					"field1": "value1",
				},
			},
			wantErr: false,
		},
		{
			name: "有效数据 - 包含两个字段",
			inputData: map[string]interface{}{
				"evaluate_dataset_fields": map[string]interface{}{
					"field1": "value1",
				},
				"evaluate_target_output_fields": map[string]interface{}{
					"field2": "value2",
				},
			},
			wantErr: false,
		},
		{
			name: "无效数据 - 缺少必需字段",
			inputData: map[string]interface{}{
				"other_field": "value",
			},
			wantErr: true,
		},
		{
			name:      "无效数据 - 空数据",
			inputData: map[string]interface{}{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := builder.validateInputData(tt.inputData)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJavaScriptCodeBuilder_escapeCodeForTemplate(t *testing.T) {
	t.Parallel()

	builder := NewJavaScriptCodeBuilder()

	tests := []struct {
		name     string
		userCode string
		want     string
	}{
		{
			name:     "转义反斜杠",
			userCode: "const path = \"C:\\\\Users\\\\test\";",
			want:     "const path = \"C:\\\\\\\\Users\\\\\\\\test\";",
		},
		{
			name:     "转义反引号",
			userCode: "const template = `Hello World`;",
			want:     "const template = \\`Hello World\\`;",
		},
		{
			name:     "转义模板字符串变量",
			userCode: "const template = `Hello ${name}`;",
			want:     "const template = \\`Hello \\${name}\\`;",
		},
		{
			name:     "复合转义",
			userCode: "const path = `C:\\\\Users\\\\${user}`;",
			want:     "const path = \\`C:\\\\\\\\Users\\\\\\\\\\${user}\\`;",
		},
		{
			name:     "无需转义的普通代码",
			userCode: "function test() { return 'hello'; }",
			want:     "function test() { return 'hello'; }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := builder.escapeCodeForTemplate(tt.userCode)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPythonCodeBuilder_convertContentToMockFormat_EdgeCases(t *testing.T) {
	t.Parallel()

	builder := NewPythonCodeBuilder()

	tests := []struct {
		name     string
		content  *entity.Content
		wantNil  bool
		validate func(*testing.T, map[string]interface{})
	}{
		{
			name:    "nil content",
			content: nil,
			wantNil: true,
		},
		{
			name: "content with nil ContentType",
			content: &entity.Content{
				Text: gptr.Of("test text"),
			},
			wantNil: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "Text", result["content_type"]) // 默认为Text
				assert.Equal(t, "test text", result["text"])
			},
		},
		{
			name: "content with empty MultiPart",
			content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart:   []*entity.Content{},
			},
			wantNil: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "MultiPart", result["content_type"])
				multiPart, ok := result["multi_part"].([]map[string]interface{})
				if !ok {
					multiPart = []map[string]interface{}{}
				}
				assert.Empty(t, multiPart)
			},
		},
		{
			name: "content with MultiPart containing nil parts",
			content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeText),
						Text:        gptr.Of("valid part"),
					},
					nil, // nil part
				},
			},
			wantNil: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				multiPart := result["multi_part"].([]map[string]interface{})
				assert.Len(t, multiPart, 1) // nil part should be skipped
				assert.Equal(t, "valid part", multiPart[0]["text"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := builder.convertContentToMockFormat(tt.content)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				if tt.validate != nil {
					tt.validate(t, got)
				}
			}
		})
	}
}

func TestJavaScriptCodeBuilder_convertContentToMockFormat_EdgeCases(t *testing.T) {
	t.Parallel()

	builder := NewJavaScriptCodeBuilder()

	tests := []struct {
		name     string
		content  *entity.Content
		wantNil  bool
		validate func(*testing.T, map[string]interface{})
	}{
		{
			name:    "nil content",
			content: nil,
			wantNil: true,
		},
		{
			name: "content with all content types nil",
			content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeText),
				// 所有内容字段都为nil
			},
			wantNil: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "Text", result["content_type"])
				// 应该只有content_type字段
				assert.Len(t, result, 1)
			},
		},
		{
			name: "content with nested MultiPart",
			content: &entity.Content{
				ContentType: gptr.Of(entity.ContentTypeMultipart),
				MultiPart: []*entity.Content{
					{
						ContentType: gptr.Of(entity.ContentTypeMultipart),
						MultiPart: []*entity.Content{
							{
								ContentType: gptr.Of(entity.ContentTypeText),
								Text:        gptr.Of("nested text"),
							},
						},
					},
				},
			},
			wantNil: false,
			validate: func(t *testing.T, result map[string]interface{}) {
				multiPart := result["multi_part"].([]map[string]interface{})
				assert.Len(t, multiPart, 1)
				nestedMultiPart := multiPart[0]["multi_part"].([]map[string]interface{})
				assert.Len(t, nestedMultiPart, 1)
				assert.Equal(t, "nested text", nestedMultiPart[0]["text"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := builder.convertContentToMockFormat(tt.content)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				if tt.validate != nil {
					tt.validate(t, got)
				}
			}
		})
	}
}

func TestCodeBuilderFactoryImpl_CreateBuilder_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		languageType    entity.LanguageType
		setupFactory    func(*gomock.Controller) *CodeBuilderFactoryImpl
		wantErr         bool
		validateBuilder func(*testing.T, UserCodeBuilder)
	}{
		{
			name:         "无RuntimeManager时创建Python构建器",
			languageType: entity.LanguageTypePython,
			setupFactory: func(ctrl *gomock.Controller) *CodeBuilderFactoryImpl {
				return &CodeBuilderFactoryImpl{} // 没有设置runtimeManager
			},
			wantErr: false,
			validateBuilder: func(t *testing.T, builder UserCodeBuilder) {
				pythonBuilder, ok := builder.(*PythonCodeBuilder)
				assert.True(t, ok)
				assert.Nil(t, pythonBuilder.runtime) // runtime应该为nil
			},
		},
		{
			name:         "无RuntimeManager时创建JavaScript构建器",
			languageType: entity.LanguageTypeJS,
			setupFactory: func(ctrl *gomock.Controller) *CodeBuilderFactoryImpl {
				return &CodeBuilderFactoryImpl{} // 没有设置runtimeManager
			},
			wantErr: false,
			validateBuilder: func(t *testing.T, builder UserCodeBuilder) {
				jsBuilder, ok := builder.(*JavaScriptCodeBuilder)
				assert.True(t, ok)
				assert.Nil(t, jsBuilder.runtime) // runtime应该为nil
			},
		},
		{
			name:         "RuntimeManager获取Runtime失败",
			languageType: entity.LanguageTypePython,
			setupFactory: func(ctrl *gomock.Controller) *CodeBuilderFactoryImpl {
				mockRM := mocks.NewMockIRuntimeManager(ctrl)
				mockRM.EXPECT().GetRuntime(entity.LanguageTypePython).Return(nil, errors.New("runtime not available"))
				factory := &CodeBuilderFactoryImpl{}
				factory.SetRuntimeManager(mockRM)
				return factory
			},
			wantErr: false,
			validateBuilder: func(t *testing.T, builder UserCodeBuilder) {
				pythonBuilder, ok := builder.(*PythonCodeBuilder)
				assert.True(t, ok)
				assert.Nil(t, pythonBuilder.runtime) // runtime获取失败时应该为nil
			},
		},
		{
			name:         "空字符串语言类型",
			languageType: entity.LanguageType(""),
			setupFactory: func(ctrl *gomock.Controller) *CodeBuilderFactoryImpl {
				return &CodeBuilderFactoryImpl{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			factory := tt.setupFactory(ctrl)
			builder, err := factory.CreateBuilder(tt.languageType)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, builder)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, builder)
				if tt.validateBuilder != nil {
					tt.validateBuilder(t, builder)
				}
			}
		})
	}
}
