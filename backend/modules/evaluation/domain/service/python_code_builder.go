// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/templates"
)

// PythonCodeBuilder Python代码构建器
type PythonCodeBuilder struct{}

// NewPythonCodeBuilder 创建Python代码构建器实例
func NewPythonCodeBuilder() *PythonCodeBuilder {
	return &PythonCodeBuilder{}
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
	// 替换turn变量占位符
	pythonCode = strings.Replace(pythonCode, "{{TURN_DATA}}", turnDataStr, 1)

	// 替换exec_evaluation函数定义占位符
	// 用户的code_content应该包含完整的函数定义，不需要额外缩进
	pythonCode = strings.Replace(pythonCode, "{{EXEC_EVALUATION_FUNCTION}}", codeVersion.CodeContent, 1)

	return pythonCode, nil
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
func (b *PythonCodeBuilder) validateInputData(inputData map[string]interface{}) error {
	// 验证turn结构的完整性
	if turn, exists := inputData["turn"]; exists {
		turnMap, ok := turn.(map[string]interface{})
		if !ok {
			return fmt.Errorf("turn field must be a map")
		}

		// 验证eval_set和eval_target的存在性
		if _, hasEvalSet := turnMap["eval_set"]; !hasEvalSet {
			if _, hasEvalTarget := turnMap["eval_target"]; !hasEvalTarget {
				return fmt.Errorf("turn must contain either eval_set or eval_target")
			}
		}
	}

	return nil
}

// BuildSyntaxCheckCode 构建Python语法检查代码
func (b *PythonCodeBuilder) BuildSyntaxCheckCode(userCode string) string {
	// 使用模板构建语法检查代码
	syntaxCheckTemplate := templates.PythonSyntaxCheckTemplate
	
	// 转义用户代码中的特殊字符，确保能正确嵌入到三引号字符串中
	escapedCode := strings.ReplaceAll(userCode, "\\", "\\\\")
	escapedCode = strings.ReplaceAll(escapedCode, `"""`, `\"\"\"`)
	
	// 替换模板中的用户代码占位符
	syntaxCheckCode := strings.Replace(syntaxCheckTemplate, "{{USER_CODE}}", escapedCode, 1)
	
	return syntaxCheckCode
}

// buildInputData 构建代码执行的输入数据
func (b *PythonCodeBuilder) buildInputData(input *entity.EvaluatorInputData) (map[string]interface{}, error) {
	inputData := make(map[string]interface{})

	// 构建turn结构
	turn := make(map[string]interface{})

	// 处理FromEvalSetFields - 映射到turn.eval_set
	if len(input.FromEvalSetFields) > 0 {
		evalSet := make(map[string]interface{})
		for key, content := range input.FromEvalSetFields {
			if content != nil {
				if mockFormat := b.convertContentToMockFormat(content); mockFormat != nil {
					evalSet[key] = mockFormat
				}
			}
		}
		if len(evalSet) > 0 {
			turn["eval_set"] = evalSet
		}
	}

	// 处理FromEvalTargetFields - 映射到turn.eval_target
	if len(input.FromEvalTargetFields) > 0 {
		evalTarget := make(map[string]interface{})
		for key, content := range input.FromEvalTargetFields {
			if content != nil {
				if mockFormat := b.convertContentToMockFormat(content); mockFormat != nil {
					evalTarget[key] = mockFormat
				}
			}
		}
		if len(evalTarget) > 0 {
			turn["eval_target"] = evalTarget
		}
	}

	// 只有当turn不为空时才添加
	if len(turn) > 0 {
		inputData["turn"] = turn
	}

	// 处理Ext字段 - 直接映射到根级别的ext
	if len(input.Ext) > 0 {
		inputData["ext"] = input.Ext
	}

	// 保持向后兼容性：处理其他字段
	// 添加历史消息（如果需要）
	if len(input.HistoryMessages) > 0 {
		messages := make([]map[string]interface{}, 0, len(input.HistoryMessages))
		for _, msg := range input.HistoryMessages {
			message := map[string]interface{}{
				"role": msg.Role,
			}
			if msg.Content != nil {
				message["content"] = msg.Content.GetText()
			}
			messages = append(messages, message)
		}
		inputData["history_messages"] = messages
	}

	// 添加输入字段（如果需要）
	if len(input.InputFields) > 0 {
		inputFields := make(map[string]interface{})
		for key, content := range input.InputFields {
			if content != nil {
				inputFields[key] = content.GetText()
			}
		}
		inputData["input_fields"] = inputFields
	}

	// 验证生成的数据格式
	if err := b.validateInputData(inputData); err != nil {
		return nil, fmt.Errorf("invalid input data format: %v", err)
	}

	return inputData, nil
}