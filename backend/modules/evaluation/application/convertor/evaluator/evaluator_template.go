// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"github.com/bytedance/gg/gptr"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	commonconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ConvertEvaluatorTemplateDTO2DO 将 EvaluatorTemplate DTO 转换为 DO
func ConvertEvaluatorTemplateDTO2DO(dto *evaluatordto.EvaluatorTemplate) *evaluatordo.EvaluatorTemplate {
	if dto == nil {
		return nil
	}

	do := &evaluatordo.EvaluatorTemplate{
		ID:            dto.GetID(),
		SpaceID:       dto.GetWorkspaceID(),
		Name:          dto.GetName(),
		Description:   dto.GetDescription(),
		EvaluatorType: evaluatordo.EvaluatorType(dto.GetEvaluatorType()),
		Popularity:    dto.GetPopularity(),
		Benchmark:     dto.GetBenchmark(),
		Vendor:        dto.GetVendor(),
		InputSchemas:  make([]*evaluatordo.ArgsSchema, 0),
		OutputSchemas: make([]*evaluatordo.ArgsSchema, 0),
		Tags:          make(map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string),
	}

	// 处理标签
	if len(dto.GetTags()) > 0 {
		for lang, kv := range dto.GetTags() {
			inner := make(map[evaluatordo.EvaluatorTagKey][]string, len(kv))
			for key, values := range kv {
				inner[evaluatordo.EvaluatorTagKey(key)] = values
			}
			do.Tags[evaluatordo.EvaluatorTagLangType(lang)] = inner
		}
	}

	// 处理基础信息
	if dto.GetBaseInfo() != nil {
		do.BaseInfo = commonconvertor.ConvertBaseInfoDTO2DO(dto.GetBaseInfo())
	}

	// 处理评估器内容
	if dto.GetEvaluatorContent() != nil {
		// 处理输入模式
		if len(dto.GetEvaluatorContent().GetInputSchemas()) > 0 {
			for _, schema := range dto.GetEvaluatorContent().GetInputSchemas() {
				do.InputSchemas = append(do.InputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		// 处理输出模式
		if len(dto.GetEvaluatorContent().GetOutputSchemas()) > 0 {
			for _, schema := range dto.GetEvaluatorContent().GetOutputSchemas() {
				do.OutputSchemas = append(do.OutputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		// 处理接收聊天历史
		do.ReceiveChatHistory = dto.GetEvaluatorContent().ReceiveChatHistory

		// 根据评估器类型处理具体内容
		switch do.EvaluatorType {
		case evaluatordo.EvaluatorTypePrompt:
			if dto.GetEvaluatorContent().PromptEvaluator != nil {
				do.PromptEvaluatorContent = ConvertPromptEvaluatorContentDTO2DO(dto.GetEvaluatorContent().PromptEvaluator)
			}
		case evaluatordo.EvaluatorTypeCode:
			if dto.GetEvaluatorContent().CodeEvaluator != nil {
				do.CodeEvaluatorContent = ConvertCodeEvaluatorContentDTO2DO(dto.GetEvaluatorContent().CodeEvaluator)
			}
		}
	}

	return do
}

// ConvertEvaluatorTemplateDO2DTO 将 EvaluatorTemplate DO 转换为 DTO
func ConvertEvaluatorTemplateDO2DTO(do *evaluatordo.EvaluatorTemplate) *evaluatordto.EvaluatorTemplate {
	if do == nil {
		return nil
	}

	dto := &evaluatordto.EvaluatorTemplate{
		ID:            gptr.Of(do.ID),
		WorkspaceID:   gptr.Of(do.SpaceID),
		Name:          gptr.Of(do.Name),
		Description:   gptr.Of(do.Description),
		EvaluatorType: evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType(do.EvaluatorType)),
		Popularity:    gptr.Of(do.Popularity),
		Benchmark:     gptr.Of(do.Benchmark),
		Vendor:        gptr.Of(do.Vendor),
		Tags:          make(map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string),
	}

	// 处理标签
	if len(do.Tags) > 0 {
		for lang, kv := range do.Tags {
			inner := make(map[evaluatordto.EvaluatorTagKey][]string, len(kv))
			for key, values := range kv {
				inner[evaluatordto.EvaluatorTagKey(key)] = values
			}
			dto.Tags[evaluatordto.EvaluatorTagLangType(lang)] = inner
		}
	}

	// 处理基础信息
	if do.BaseInfo != nil {
		dto.BaseInfo = commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo)
	}

	// 构建评估器内容
	dto.EvaluatorContent = &evaluatordto.EvaluatorContent{
		InputSchemas:       make([]*commondto.ArgsSchema, 0),
		OutputSchemas:      make([]*commondto.ArgsSchema, 0),
		ReceiveChatHistory: do.ReceiveChatHistory,
	}

	// 处理输入模式
	if len(do.InputSchemas) > 0 {
		for _, schema := range do.InputSchemas {
			dto.EvaluatorContent.InputSchemas = append(dto.EvaluatorContent.InputSchemas, commonconvertor.ConvertArgsSchemaDO2DTO(schema))
		}
	}

	// 处理输出模式
	if len(do.OutputSchemas) > 0 {
		for _, schema := range do.OutputSchemas {
			dto.EvaluatorContent.OutputSchemas = append(dto.EvaluatorContent.OutputSchemas, commonconvertor.ConvertArgsSchemaDO2DTO(schema))
		}
	}

	// 根据评估器类型处理具体内容
	switch do.EvaluatorType {
	case evaluatordo.EvaluatorTypePrompt:
		if do.PromptEvaluatorContent != nil {
			dto.EvaluatorContent.PromptEvaluator = ConvertPromptEvaluatorContentDO2DTO(do.PromptEvaluatorContent)
		}
	case evaluatordo.EvaluatorTypeCode:
		if do.CodeEvaluatorContent != nil {
			dto.EvaluatorContent.CodeEvaluator = ConvertCodeEvaluatorContentDO2DTO(do.CodeEvaluatorContent)
		}
	}

	return dto
}

// ConvertEvaluatorTemplateDOList2DTO 将 EvaluatorTemplate DO 列表转换为 DTO 列表
func ConvertEvaluatorTemplateDOList2DTO(doList []*evaluatordo.EvaluatorTemplate) []*evaluatordto.EvaluatorTemplate {
	if doList == nil {
		return nil
	}

	dtoList := make([]*evaluatordto.EvaluatorTemplate, 0, len(doList))
	for _, do := range doList {
		dtoList = append(dtoList, ConvertEvaluatorTemplateDO2DTO(do))
	}
	return dtoList
}

// ConvertPromptEvaluatorContentDTO2DO 将 PromptEvaluator DTO 转换为 DO
func ConvertPromptEvaluatorContentDTO2DO(dto *evaluatordto.PromptEvaluator) *evaluatordo.PromptEvaluatorContent {
	if dto == nil {
		return nil
	}

	do := &evaluatordo.PromptEvaluatorContent{
		// ParseType和PromptSuffix在IDL中没有对应字段，使用默认值
		ParseType:    evaluatordo.ParseTypeContent,
		PromptSuffix: "",
	}

	// 转换消息列表
	if len(dto.GetMessageList()) > 0 {
		do.MessageList = make([]*evaluatordo.Message, 0, len(dto.GetMessageList()))
		for _, msg := range dto.GetMessageList() {
			do.MessageList = append(do.MessageList, commonconvertor.ConvertMessageDTO2DO(msg))
		}
	}

	// 转换模型配置
	do.ModelConfig = commonconvertor.ConvertModelConfigDTO2DO(dto.GetModelConfig())

	// 转换工具列表
	if len(dto.GetTools()) > 0 {
		do.Tools = make([]*evaluatordo.Tool, 0, len(dto.GetTools()))
		for _, tool := range dto.GetTools() {
			do.Tools = append(do.Tools, ConvertToolDTO2DO(tool))
		}
	}

	return do
}

// ConvertPromptEvaluatorContentDO2DTO 将 PromptEvaluatorContent DO 转换为 DTO
func ConvertPromptEvaluatorContentDO2DTO(do *evaluatordo.PromptEvaluatorContent) *evaluatordto.PromptEvaluator {
	if do == nil {
		return nil
	}

	dto := &evaluatordto.PromptEvaluator{
		// ParseType和PromptSuffix在IDL中没有对应字段，不设置
	}

	// 转换消息列表
	if len(do.MessageList) > 0 {
		dto.MessageList = make([]*commondto.Message, 0, len(do.MessageList))
		for _, msg := range do.MessageList {
			dto.MessageList = append(dto.MessageList, commonconvertor.ConvertMessageDO2DTO(msg))
		}
	}

	// 转换模型配置
	dto.ModelConfig = commonconvertor.ConvertModelConfigDO2DTO(do.ModelConfig)

	// 转换工具列表
	if len(do.Tools) > 0 {
		dto.Tools = make([]*evaluatordto.Tool, 0, len(do.Tools))
		for _, tool := range do.Tools {
			dto.Tools = append(dto.Tools, ConvertToolDO2DTO(tool))
		}
	}

	return dto
}

// ConvertCodeEvaluatorContentDTO2DO 将 CodeEvaluator DTO 转换为 DO
func ConvertCodeEvaluatorContentDTO2DO(dto *evaluatordto.CodeEvaluator) *evaluatordo.CodeEvaluatorContent {
	if dto == nil {
		return nil
	}

	return &evaluatordo.CodeEvaluatorContent{
		CodeContent:  dto.GetCodeContent(),
		LanguageType: evaluatordo.LanguageType(dto.GetLanguageType()),
	}
}

// ConvertCodeEvaluatorContentDO2DTO 将 CodeEvaluatorContent DO 转换为 DTO
func ConvertCodeEvaluatorContentDO2DTO(do *evaluatordo.CodeEvaluatorContent) *evaluatordto.CodeEvaluator {
	if do == nil {
		return nil
	}

	return &evaluatordto.CodeEvaluator{
		CodeContent:  gptr.Of(do.CodeContent),
		LanguageType: gptr.Of(evaluatordto.LanguageType(do.LanguageType)),
	}
}
