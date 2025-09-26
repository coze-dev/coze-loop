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
				if tt.wantType == "PythonCodeBuilder" {
					_, ok := builder.(*PythonCodeBuilder)
					assert.True(t, ok, "应该返回PythonCodeBuilder类型")
				} else if tt.wantType == "JavaScriptCodeBuilder" {
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