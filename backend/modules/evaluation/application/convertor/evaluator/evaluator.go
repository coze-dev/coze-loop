// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"strings"

	"github.com/bytedance/gg/gptr"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	commonconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func ConvertEvaluatorDTO2DO(evaluatorDTO *evaluatordto.Evaluator) *evaluatordo.Evaluator {
	// 从DTO转换为DO
	evaluatorDO := &evaluatordo.Evaluator{
		ID:                     evaluatorDTO.GetEvaluatorID(),
		SpaceID:                evaluatorDTO.GetWorkspaceID(),
		Name:                   evaluatorDTO.GetName(),
		Description:            evaluatorDTO.GetDescription(),
		DraftSubmitted:         evaluatorDTO.GetDraftSubmitted(),
		EvaluatorType:          evaluatordo.EvaluatorType(evaluatorDTO.GetEvaluatorType()),
		LatestVersion:          evaluatorDTO.GetLatestVersion(),
		PromptEvaluatorVersion: nil,
		BaseInfo:               commonconvertor.ConvertBaseInfoDTO2DO(evaluatorDTO.GetBaseInfo()),
	}
	if evaluatorDTO.CurrentVersion != nil {
		switch evaluatorDTO.GetEvaluatorType() {
		case evaluatordto.EvaluatorType_Prompt:
			evaluatorDO.PromptEvaluatorVersion = ConvertPromptEvaluatorVersionDTO2DO(evaluatorDO.ID, evaluatorDO.SpaceID, evaluatorDTO.GetCurrentVersion())
		case evaluatordto.EvaluatorType_Code:
			evaluatorDO.CodeEvaluatorVersion = ConvertCodeEvaluatorVersionDTO2DO(evaluatorDO.ID, evaluatorDO.SpaceID, evaluatorDTO.GetCurrentVersion())
		}
	}
	return evaluatorDO
}

func ConvertEvaluatorDOList2DTO(doList []*evaluatordo.Evaluator) []*evaluatordto.Evaluator {
	dtoList := make([]*evaluatordto.Evaluator, 0, len(doList))
	for _, evaluatorDO := range doList {
		dtoList = append(dtoList, ConvertEvaluatorDO2DTO(evaluatorDO))
	}
	return dtoList
}

// ConvertEvaluatorDO2DTO 将 evaluatordo.Evaluator 转换为 evaluatordto.Evaluator
func ConvertEvaluatorDO2DTO(do *evaluatordo.Evaluator) *evaluatordto.Evaluator {
	if do == nil {
		return nil
	}
	dto := &evaluatordto.Evaluator{
		EvaluatorID:    gptr.Of(do.ID),
		WorkspaceID:    gptr.Of(do.SpaceID),
		Name:           gptr.Of(do.Name),
		Description:    gptr.Of(do.Description),
		DraftSubmitted: gptr.Of(do.DraftSubmitted),
		EvaluatorType:  evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType(do.EvaluatorType)),
		LatestVersion:  gptr.Of(do.LatestVersion),
		BaseInfo:       commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
	}

	switch do.EvaluatorType {
	case evaluatordo.EvaluatorTypePrompt:
		if do.PromptEvaluatorVersion != nil {
			versionDTO := ConvertPromptEvaluatorVersionDO2DTO(do.PromptEvaluatorVersion)
			dto.CurrentVersion = versionDTO
		}
	case evaluatordo.EvaluatorTypeCode:
		if do.CodeEvaluatorVersion != nil {
			versionDTO := ConvertCodeEvaluatorVersionDO2DTO(do.CodeEvaluatorVersion)
			dto.CurrentVersion = versionDTO
		}
	}
	return dto
}

// normalizeLanguageType 标准化语言类型（转换为小写）
func normalizeLanguageType(langType evaluatordo.LanguageType) evaluatordo.LanguageType {
	switch strings.ToLower(string(langType)) {
	case "python":
		return evaluatordo.LanguageTypePython  // "python"
	case "js", "javascript":
		return evaluatordo.LanguageTypeJS      // "js"
	default:
		return evaluatordo.LanguageType(strings.ToLower(string(langType)))
	}
}

// ConvertCodeEvaluatorVersionDTO2DO 将 DTO 转换为 CodeEvaluatorVersion
func ConvertCodeEvaluatorVersionDTO2DO(evaluatorID, spaceID int64, dto *evaluatordto.EvaluatorVersion) *evaluatordo.CodeEvaluatorVersion {
	if dto == nil || dto.EvaluatorContent == nil || dto.EvaluatorContent.CodeEvaluator == nil {
		return nil
	}

	codeEvaluator := dto.EvaluatorContent.CodeEvaluator
	
	// 标准化语言类型
	languageType := evaluatordo.LanguageType(codeEvaluator.GetLanguageType())
	normalizedLangType := normalizeLanguageType(languageType)
	
	return &evaluatordo.CodeEvaluatorVersion{
		ID:               dto.GetID(),
		SpaceID:          spaceID,
		EvaluatorType:    evaluatordo.EvaluatorTypeCode,
		EvaluatorID:      evaluatorID,
		Description:      dto.GetDescription(),
		Version:          dto.GetVersion(),
		BaseInfo:         commonconvertor.ConvertBaseInfoDTO2DO(dto.GetBaseInfo()),
		CodeTemplateKey:  codeEvaluator.CodeTemplateKey,
		CodeTemplateName: codeEvaluator.CodeTemplateName,
		CodeContent:      codeEvaluator.GetCodeContent(),
		LanguageType:     normalizedLangType,
	}
}

// ConvertCodeEvaluatorVersionDO2DTO 将 CodeEvaluatorVersion 转换为 DTO
func ConvertCodeEvaluatorVersionDO2DTO(do *evaluatordo.CodeEvaluatorVersion) *evaluatordto.EvaluatorVersion {
	if do == nil {
		return nil
	}

	return &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(do.ID),
		Version:     gptr.Of(do.Version),
		Description: gptr.Of(do.Description),
		BaseInfo:    commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			CodeEvaluator: &evaluatordto.CodeEvaluator{
				CodeTemplateKey:  do.CodeTemplateKey,
				CodeTemplateName: do.CodeTemplateName,
				CodeContent:      gptr.Of(do.CodeContent),
				LanguageType:     (*evaluatordto.LanguageType)(&do.LanguageType),
			},
		},
	}
}

// ConvertEvaluatorContent2DO 将 EvaluatorContent 转换为 Evaluator DO
func ConvertEvaluatorContent2DO(content *evaluatordto.EvaluatorContent, evaluatorType evaluatordto.EvaluatorType) (*evaluatordo.Evaluator, error) {
	if content == nil {
		return nil, errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("evaluator content is nil"))
	}

	evaluator := &evaluatordo.Evaluator{
		EvaluatorType: evaluatordo.EvaluatorType(evaluatorType),
	}

	switch evaluatorType {
	case evaluatordto.EvaluatorType_Prompt:
		if content.PromptEvaluator == nil {
			return nil, errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("prompt evaluator content is nil"))
		}
		
		promptVersion := &evaluatordo.PromptEvaluatorVersion{
			EvaluatorType:     evaluatordo.EvaluatorTypePrompt,
			PromptSourceType:  evaluatordo.PromptSourceType(content.PromptEvaluator.GetPromptSourceType()),
			PromptTemplateKey: content.PromptEvaluator.GetPromptTemplateKey(),
			ReceiveChatHistory: content.ReceiveChatHistory,
		}

		// 转换消息列表
		if len(content.PromptEvaluator.MessageList) > 0 {
			promptVersion.MessageList = make([]*evaluatordo.Message, 0, len(content.PromptEvaluator.MessageList))
			for _, msg := range content.PromptEvaluator.MessageList {
				promptVersion.MessageList = append(promptVersion.MessageList, commonconvertor.ConvertMessageDTO2DO(msg))
			}
		}

		// 转换模型配置
		promptVersion.ModelConfig = commonconvertor.ConvertModelConfigDTO2DO(content.PromptEvaluator.ModelConfig)

		// 转换工具列表
		if len(content.PromptEvaluator.Tools) > 0 {
			promptVersion.Tools = make([]*evaluatordo.Tool, 0, len(content.PromptEvaluator.Tools))
			for _, tool := range content.PromptEvaluator.Tools {
				promptVersion.Tools = append(promptVersion.Tools, ConvertToolDTO2DO(tool))
			}
		}

		// 转换输入模式
		if len(content.InputSchemas) > 0 {
			promptVersion.InputSchemas = make([]*evaluatordo.ArgsSchema, 0, len(content.InputSchemas))
			for _, schema := range content.InputSchemas {
				promptVersion.InputSchemas = append(promptVersion.InputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		evaluator.PromptEvaluatorVersion = promptVersion

	case evaluatordto.EvaluatorType_Code:
		if content.CodeEvaluator == nil {
			return nil, errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("code evaluator content is nil"))
		}

		// 标准化语言类型
		languageType := evaluatordo.LanguageType(content.CodeEvaluator.GetLanguageType())
		normalizedLangType := normalizeLanguageType(languageType)
		
		codeVersion := &evaluatordo.CodeEvaluatorVersion{
			EvaluatorType:    evaluatordo.EvaluatorTypeCode,
			CodeTemplateKey:  content.CodeEvaluator.CodeTemplateKey,
			CodeTemplateName: content.CodeEvaluator.CodeTemplateName,
			CodeContent:      content.CodeEvaluator.GetCodeContent(),
			LanguageType:     normalizedLangType,
		}

		evaluator.CodeEvaluatorVersion = codeVersion

	default:
		return nil, errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("unsupported evaluator type"))
	}

	return evaluator, nil
}

func ConvertPromptEvaluatorVersionDTO2DO(evaluatorID, spaceID int64, dto *evaluatordto.EvaluatorVersion) *evaluatordo.PromptEvaluatorVersion {
	promptEvaluatorVersion := &evaluatordo.PromptEvaluatorVersion{
		ID:                dto.GetID(),
		SpaceID:           spaceID,
		EvaluatorType:     evaluatordo.EvaluatorTypePrompt,
		EvaluatorID:       evaluatorID,
		Description:       dto.GetDescription(),
		Version:           dto.GetVersion(),
		PromptSourceType:  evaluatordo.PromptSourceType(dto.EvaluatorContent.PromptEvaluator.GetPromptSourceType()),
		PromptTemplateKey: dto.EvaluatorContent.PromptEvaluator.GetPromptTemplateKey(),
		BaseInfo:          commonconvertor.ConvertBaseInfoDTO2DO(dto.GetBaseInfo()),
	}
	if dto.EvaluatorContent != nil {
		promptEvaluatorVersion.ReceiveChatHistory = dto.EvaluatorContent.ReceiveChatHistory
		if len(dto.EvaluatorContent.InputSchemas) > 0 {
			promptEvaluatorVersion.InputSchemas = make([]*evaluatordo.ArgsSchema, 0)
			for _, v := range dto.EvaluatorContent.InputSchemas {
				args := commonconvertor.ConvertArgsSchemaDTO2DO(v)
				promptEvaluatorVersion.InputSchemas = append(promptEvaluatorVersion.InputSchemas, args)
			}
		}
		if dto.EvaluatorContent.PromptEvaluator != nil {
			promptEvaluatorVersion.PromptSourceType = evaluatordo.PromptSourceType(dto.EvaluatorContent.PromptEvaluator.GetPromptSourceType())
			promptEvaluatorVersion.PromptTemplateKey = dto.EvaluatorContent.PromptEvaluator.GetPromptTemplateKey()
			promptEvaluatorVersion.MessageList = make([]*evaluatordo.Message, 0)
			for _, originMessage := range dto.EvaluatorContent.PromptEvaluator.GetMessageList() {
				message := commonconvertor.ConvertMessageDTO2DO(originMessage)
				promptEvaluatorVersion.MessageList = append(promptEvaluatorVersion.MessageList, message)
			}
			promptEvaluatorVersion.ModelConfig = commonconvertor.ConvertModelConfigDTO2DO(dto.EvaluatorContent.PromptEvaluator.ModelConfig)
			promptEvaluatorVersion.Tools = make([]*evaluatordo.Tool, 0)
			for _, doTool := range dto.EvaluatorContent.PromptEvaluator.Tools {
				promptEvaluatorVersion.Tools = append(promptEvaluatorVersion.Tools, ConvertToolDTO2DO(doTool))
			}
		}
	}
	return promptEvaluatorVersion
}

// ConvertPromptEvaluatorVersionDO2DTO 将 prompt.PromptEvaluatorVersion 转换为 evaluatordto.EvaluatorVersion
func ConvertPromptEvaluatorVersionDO2DTO(do *evaluatordo.PromptEvaluatorVersion) *evaluatordto.EvaluatorVersion {
	if do == nil {
		return nil
	}
	dto := &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(do.ID),
		Version:     gptr.Of(do.Version),
		Description: gptr.Of(do.Description),
		BaseInfo:    commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			ReceiveChatHistory: do.ReceiveChatHistory,
			PromptEvaluator: &evaluatordto.PromptEvaluator{
				ModelConfig:       commonconvertor.ConvertModelConfigDO2DTO(do.ModelConfig),
				PromptSourceType:  evaluatordto.PromptSourceTypePtr(evaluatordto.PromptSourceType(do.PromptSourceType)),
				PromptTemplateKey: gptr.Of(do.PromptTemplateKey),
			},
		},
	}
	if len(do.InputSchemas) > 0 {
		dto.EvaluatorContent.InputSchemas = make([]*commondto.ArgsSchema, 0, len(do.InputSchemas))
		for _, v := range do.InputSchemas {
			dto.EvaluatorContent.InputSchemas = append(dto.EvaluatorContent.InputSchemas, commonconvertor.ConvertArgsSchemaDO2DTO(v))
		}
	}
	if len(do.MessageList) > 0 {
		dto.EvaluatorContent.PromptEvaluator.MessageList = make([]*commondto.Message, 0, len(do.MessageList))
		for _, v := range do.MessageList {
			dto.EvaluatorContent.PromptEvaluator.MessageList = append(dto.EvaluatorContent.PromptEvaluator.MessageList, commonconvertor.ConvertMessageDO2DTO(v))
		}
	}
	if len(do.Tools) > 0 {
		dto.EvaluatorContent.PromptEvaluator.Tools = make([]*evaluatordto.Tool, 0, len(do.Tools))
		for _, v := range do.Tools {
			dto.EvaluatorContent.PromptEvaluator.Tools = append(dto.EvaluatorContent.PromptEvaluator.Tools, ConvertToolDO2DTO(v))
		}
	}

	return dto
}