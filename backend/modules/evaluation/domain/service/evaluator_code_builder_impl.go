// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf/templates"
)

// CodeBuilderFactoryImpl 代码构建器工厂实现
type CodeBuilderFactoryImpl struct {
	runtimeManager component.IRuntimeManager // 运行时管理器，用于获取runtime实例
}

// NewCodeBuilderFactory 创建代码构建器工厂实例
func NewCodeBuilderFactory() CodeBuilderFactory {
	return &CodeBuilderFactoryImpl{}
}

// SetRuntimeManager 设置运行时管理器
func (f *CodeBuilderFactoryImpl) SetRuntimeManager(manager component.IRuntimeManager) {
	f.runtimeManager = manager
}

// CreateBuilder 根据语言类型创建代码构建器
func (f *CodeBuilderFactoryImpl) CreateBuilder(languageType entity.LanguageType) (UserCodeBuilder, error) {
	var builder UserCodeBuilder
	var err error

	switch languageType {
	case entity.LanguageTypePython:
		builder = NewPythonCodeBuilder()
	case entity.LanguageTypeJS:
		builder = NewJavaScriptCodeBuilder()
	default:
		return nil, fmt.Errorf("unsupported language type: %s", languageType)
	}

	// 如果有runtimeManager，尝试获取对应的runtime并设置到builder中
	if f.runtimeManager != nil {
		if runtime, runtimeErr := f.runtimeManager.GetRuntime(languageType); runtimeErr == nil {
			builder.SetRuntime(runtime)
		}
		// 如果获取runtime失败，不影响builder的创建，只是无法使用runtime的return_val函数
	}

	return builder, err
}

// GetSupportedLanguages 获取支持的语言类型列表
func (f *CodeBuilderFactoryImpl) GetSupportedLanguages() []entity.LanguageType {
	return []entity.LanguageType{
		entity.LanguageTypePython,
		entity.LanguageTypeJS,
	}
}

// JavaScriptCodeBuilder JavaScript代码构建器
type JavaScriptCodeBuilder struct {
	runtime component.IRuntime // 运行时实例，用于获取return_val函数
}

// NewJavaScriptCodeBuilder 创建JavaScript代码构建器实例
func NewJavaScriptCodeBuilder() *JavaScriptCodeBuilder {
	return &JavaScriptCodeBuilder{}
}

// SetRuntime 设置运行时实例
func (b *JavaScriptCodeBuilder) SetRuntime(runtime component.IRuntime) {
	b.runtime = runtime
}

// GetLanguageType 获取支持的语言类型
func (b *JavaScriptCodeBuilder) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypeJS
}

// BuildCode 构建可执行的JavaScript代码
func (b *JavaScriptCodeBuilder) BuildCode(input *entity.EvaluatorInputData, codeVersion *entity.CodeEvaluatorVersion) (string, error) {
	// 构建输入数据
	inputData, err := b.buildInputData(input)
	if err != nil {
		return "", fmt.Errorf("failed to build input data: %v", err)
	}

	// 将inputData转换为JavaScript对象格式
	turnDataBytes, err := json.MarshalIndent(inputData, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal turn data: %v", err)
	}
	turnDataStr := string(turnDataBytes)

	// 从模板开始构建代码
	jsCode := templates.JavaScriptTemplate

	// 使用strings.Replace替换占位符
	// 替换return_val函数占位符 - 现在从runtime获取
	jsCode = strings.Replace(jsCode, "{{RETURN_VAL_FUNCTION}}", b.getReturnValFunctionFromRuntime(), 1)

	// 替换turn变量占位符
	jsCode = strings.Replace(jsCode, "{{TURN_DATA}}", turnDataStr, 1)

	// 替换exec_evaluation函数定义占位符
	// 用户的code_content应该包含完整的函数定义，不需要额外缩进
	jsCode = strings.Replace(jsCode, "{{EXEC_EVALUATION_FUNCTION}}", codeVersion.CodeContent, 1)

	return jsCode, nil
}

// BuildSyntaxCheckCode 构建JavaScript语法检查代码
func (b *JavaScriptCodeBuilder) BuildSyntaxCheckCode(userCode string) string {
	// 使用模板构建语法检查代码
	syntaxCheckTemplate := templates.JavaScriptSyntaxCheckTemplate

	// 转义用户代码中的特殊字符，确保能正确嵌入到模板字符串中
	escapedCode := b.escapeCodeForTemplate(userCode)

	// 替换return_val函数占位符 - 现在从runtime获取
	syntaxCheckCode := strings.Replace(syntaxCheckTemplate, "{{RETURN_VAL_FUNCTION}}", b.getReturnValFunctionFromRuntime(), 1)

	// 替换模板中的用户代码占位符
	syntaxCheckCode = strings.Replace(syntaxCheckCode, "{{USER_CODE}}", escapedCode, 1)

	return syntaxCheckCode
}

// escapeCodeForTemplate 转义代码用于模板嵌入
func (b *JavaScriptCodeBuilder) escapeCodeForTemplate(userCode string) string {
	// 转义反斜杠
	escaped := strings.ReplaceAll(userCode, "\\", "\\\\")
	// 转义反引号
	escaped = strings.ReplaceAll(escaped, "`", "\\`")
	// 转义模板字符串中的 ${}
	escaped = strings.ReplaceAll(escaped, "$", "\\$")
	return escaped
}

// getReturnValFunctionFromRuntime 从runtime获取JavaScript return_val函数实现
func (b *JavaScriptCodeBuilder) getReturnValFunctionFromRuntime() string {
	// 如果有runtime实例，优先使用runtime提供的实现
	if b.runtime != nil {
		return b.runtime.GetReturnValFunction()
	}

	// 如果没有runtime实例，使用默认实现保持向后兼容
	return `
// return_val函数实现
function return_val(value) {
    /**
     * 标准return_val函数实现 - 输出返回值供FaaS服务捕获
     * @param {string} value - 要返回的值，通常是JSON字符串
     */
    console.log(value);
}
`
}

// convertContentToMockFormat 将Content转换为mockInput格式
func (b *JavaScriptCodeBuilder) convertContentToMockFormat(content *entity.Content) map[string]interface{} {
	if content == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 设置content_type
	if content.ContentType != nil {
		result["content_type"] = string(*content.ContentType)
	} else {
		result["content_type"] = string(entity.ContentTypeText) // 默认为Text
	}

	// 设置具体内容
	if content.Text != nil {
		result["text"] = *content.Text
	} else if content.Image != nil {
		result["image"] = content.Image
	} else if content.Audio != nil {
		result["audio"] = content.Audio
	} else if len(content.MultiPart) > 0 {
		// 对于MultiPart内容，递归转换每个部分
		multiPartData := make([]map[string]interface{}, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			if partData := b.convertContentToMockFormat(part); partData != nil {
				multiPartData = append(multiPartData, partData)
			}
		}
		result["multi_part"] = multiPartData
	}

	return result
}

// validateInputData 验证mockInput数据格式
func (b *JavaScriptCodeBuilder) validateInputData(inputData map[string]interface{}) error {
	// 验证新数据结构的完整性
	hasEvalSetFields := false
	hasEvalTargetFields := false

	if _, exists := inputData["evaluate_dataset_fields"]; exists {
		hasEvalSetFields = true
	}

	if _, exists := inputData["evaluate_target_output_fields"]; exists {
		hasEvalTargetFields = true
	}

	// 至少需要有一个字段存在
	if !hasEvalSetFields && !hasEvalTargetFields {
		return fmt.Errorf("input data must contain either evaluate_dataset_fields or evaluate_target_output_fields")
	}

	return nil
}

// buildInputData 构建代码执行的输入数据
func (b *JavaScriptCodeBuilder) buildInputData(input *entity.EvaluatorInputData) (map[string]interface{}, error) {
	inputData := make(map[string]interface{})

	// 处理EvaluateDatasetFields - 直接映射到evaluate_dataset_fields
	if len(input.EvaluateDatasetFields) > 0 {
		fromEvalSetFields := make(map[string]interface{})
		for key, content := range input.EvaluateDatasetFields {
			if content != nil {
				if mockFormat := b.convertContentToMockFormat(content); mockFormat != nil {
					fromEvalSetFields[key] = mockFormat
				}
			}
		}
		if len(fromEvalSetFields) > 0 {
			inputData["evaluate_dataset_fields"] = fromEvalSetFields
		}
	}

	// 处理EvaluateTargetOutputFields - 直接映射到evaluate_target_output_fields
	if len(input.EvaluateTargetOutputFields) > 0 {
		fromEvalTargetFields := make(map[string]interface{})
		for key, content := range input.EvaluateTargetOutputFields {
			if content != nil {
				if mockFormat := b.convertContentToMockFormat(content); mockFormat != nil {
					fromEvalTargetFields[key] = mockFormat
				}
			}
		}
		if len(fromEvalTargetFields) > 0 {
			inputData["evaluate_target_output_fields"] = fromEvalTargetFields
		}
	}

	// 处理Ext字段 - 直接映射到根级别的ext
	if len(input.Ext) > 0 {
		inputData["ext"] = input.Ext
	}

	// 验证生成的数据格式
	if err := b.validateInputData(inputData); err != nil {
		return nil, fmt.Errorf("invalid input data format: %v", err)
	}

	return inputData, nil
}

// PythonCodeBuilder Python代码构建器
type PythonCodeBuilder struct {
	runtime component.IRuntime // 运行时实例，用于获取return_val函数
}

// NewPythonCodeBuilder 创建Python代码构建器实例
func NewPythonCodeBuilder() *PythonCodeBuilder {
	return &PythonCodeBuilder{}
}

// SetRuntime 设置运行时实例
func (b *PythonCodeBuilder) SetRuntime(runtime component.IRuntime) {
	b.runtime = runtime
}

// GetLanguageType 获取支持的语言类型
func (b *PythonCodeBuilder) GetLanguageType() entity.LanguageType {
	return entity.LanguageTypePython
}

// BuildCode 构建可执行的Python代码
func (b *PythonCodeBuilder) BuildCode(input *entity.EvaluatorInputData, codeVersion *entity.CodeEvaluatorVersion) (string, error) {
	// 构建输入数据
	inputData, err := b.buildInputData(input)
	if err != nil {
		return "", fmt.Errorf("failed to build input data: %v", err)
	}

	// 将inputData转换为Python字典格式
	turnDataBytes, err := json.MarshalIndent(inputData, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal turn data: %v", err)
	}
	turnDataStr := string(turnDataBytes)

	// 从模板开始构建代码
	pythonCode := templates.PythonTemplate

	// 使用strings.Replace替换占位符
	// 替换return_val函数占位符 - 现在从runtime获取
	pythonCode = strings.Replace(pythonCode, "{{RETURN_VAL_FUNCTION}}", b.getReturnValFunctionFromRuntime(), 1)

	// 替换turn变量占位符
	pythonCode = strings.Replace(pythonCode, "{{TURN_DATA}}", turnDataStr, 1)

	// 替换exec_evaluation函数定义占位符
	// 用户的code_content应该包含完整的函数定义，不需要额外缩进
	pythonCode = strings.Replace(pythonCode, "{{EXEC_EVALUATION_FUNCTION}}", codeVersion.CodeContent, 1)

	return pythonCode, nil
}

// BuildSyntaxCheckCode 构建Python语法检查代码
func (b *PythonCodeBuilder) BuildSyntaxCheckCode(userCode string) string {
	// 使用模板构建语法检查代码
	syntaxCheckTemplate := templates.PythonSyntaxCheckTemplate

	// 转义用户代码中的特殊字符，确保能正确嵌入到三引号字符串中
	escapedCode := strings.ReplaceAll(userCode, "\\", "\\\\")
	escapedCode = strings.ReplaceAll(escapedCode, `"""`, `\"\"\"`)

	// 替换return_val函数占位符 - 现在从runtime获取
	syntaxCheckCode := strings.Replace(syntaxCheckTemplate, "{{RETURN_VAL_FUNCTION}}", b.getReturnValFunctionFromRuntime(), 1)

	// 替换模板中的用户代码占位符
	syntaxCheckCode = strings.Replace(syntaxCheckCode, "{{USER_CODE}}", escapedCode, 1)

	return syntaxCheckCode
}

// getReturnValFunctionFromRuntime 从runtime获取Python return_val函数实现
func (b *PythonCodeBuilder) getReturnValFunctionFromRuntime() string {
	// 如果有runtime实例，优先使用runtime提供的实现
	if b.runtime != nil {
		return b.runtime.GetReturnValFunction()
	}

	// 如果没有runtime实例，使用默认实现保持向后兼容
	return `
# return_val函数实现
def return_val(value):
    """
    标准return_val函数实现 - 设置返回值到ret_val字段
    Args:
        value: 要返回的值，通常是JSON字符串
    """
    # 这里不使用print，而是设置一个全局变量
    # 该变量会被FaaS服务器捕获到ret_val字段
    global _return_val_output
    _return_val_output = value
`
}

// convertContentToMockFormat 将Content转换为mockInput格式
func (b *PythonCodeBuilder) convertContentToMockFormat(content *entity.Content) map[string]interface{} {
	if content == nil {
		return nil
	}

	result := make(map[string]interface{})

	// 设置content_type
	if content.ContentType != nil {
		result["content_type"] = string(*content.ContentType)
	} else {
		result["content_type"] = string(entity.ContentTypeText) // 默认为Text
	}

	switch gptr.Indirect(content.ContentType) {
	case entity.ContentTypeText:
		result["text"] = gptr.Indirect(content.Text)
	case entity.ContentTypeImage:
		result["image"] = content.Image
	case entity.ContentTypeAudio:
		result["audio"] = content.Audio
	case entity.ContentTypeVideo:
		result["video"] = content.Video
	case entity.ContentTypeMultipart:
		multiPartData := make([]map[string]interface{}, 0, len(content.MultiPart))
		for _, part := range content.MultiPart {
			if partData := b.convertContentToMockFormat(part); partData != nil {
				multiPartData = append(multiPartData, partData)
			}
		}
		result["multi_part"] = multiPartData
	default:
		result["text"] = gptr.Indirect(content.Text)
	}

	return result
}

// buildInputData 构建代码执行的输入数据
func (b *PythonCodeBuilder) buildInputData(input *entity.EvaluatorInputData) (map[string]interface{}, error) {
	inputData := make(map[string]interface{})

	// 处理EvaluateDatasetFields - 直接映射到evaluate_dataset_fields
	if len(input.EvaluateDatasetFields) > 0 {
		fromEvalSetFields := make(map[string]interface{})
		for key, content := range input.EvaluateDatasetFields {
			if content != nil {
				if mockFormat := b.convertContentToMockFormat(content); mockFormat != nil {
					fromEvalSetFields[key] = mockFormat
				}
			}
		}
		if len(fromEvalSetFields) > 0 {
			inputData["evaluate_dataset_fields"] = fromEvalSetFields
		}
	}

	// 处理EvaluateTargetOutputFields - 直接映射到evaluate_target_output_fields
	if len(input.EvaluateTargetOutputFields) > 0 {
		fromEvalTargetFields := make(map[string]interface{})
		for key, content := range input.EvaluateTargetOutputFields {
			if content != nil {
				if mockFormat := b.convertContentToMockFormat(content); mockFormat != nil {
					fromEvalTargetFields[key] = mockFormat
				}
			}
		}
		if len(fromEvalTargetFields) > 0 {
			inputData["evaluate_target_output_fields"] = fromEvalTargetFields
		}
	}

	// 处理Ext字段 - 直接映射到根级别的ext
	if len(input.Ext) > 0 {
		inputData["ext"] = input.Ext
	}

	return inputData, nil
}
