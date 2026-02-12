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

func ConvertEvaluatorDTO2DO(evaluatorDTO *evaluatordto.Evaluator) (*evaluatordo.Evaluator, error) {
	// 从DTO转换为DO
	evaluatorDO := &evaluatordo.Evaluator{
		ID:                     evaluatorDTO.GetEvaluatorID(),
		SpaceID:                evaluatorDTO.GetWorkspaceID(),
		Name:                   evaluatorDTO.GetName(),
		Description:            evaluatorDTO.GetDescription(),
		DraftSubmitted:         evaluatorDTO.GetDraftSubmitted(),
		EvaluatorType:          evaluatordo.EvaluatorType(evaluatorDTO.GetEvaluatorType()),
		LatestVersion:          evaluatorDTO.GetLatestVersion(),
		Builtin:                evaluatorDTO.GetBuiltin(),
		BuiltinVisibleVersion:  evaluatorDTO.GetBuiltinVisibleVersion(),
		BoxType:                convertBoxTypeDTO2DO(evaluatorDTO.GetBoxType()),
		PromptEvaluatorVersion: nil,
		BaseInfo:               commonconvertor.ConvertBaseInfoDTO2DO(evaluatorDTO.GetBaseInfo()),
		Tags:                   ConvertEvaluatorLangTagsDTO2DO(evaluatorDTO.GetTags()),
	}
	if evaluatorDTO.GetEvaluatorInfo() != nil {
		evaluatorDO.EvaluatorInfo = &evaluatordo.EvaluatorInfo{
			Benchmark:     evaluatorDTO.GetEvaluatorInfo().Benchmark,
			Vendor:        evaluatorDTO.GetEvaluatorInfo().Vendor,
			VendorURL:     evaluatorDTO.GetEvaluatorInfo().VendorURL,
			UserManualURL: evaluatorDTO.GetEvaluatorInfo().UserManualURL,
		}
	}
	if evaluatorDTO.CurrentVersion != nil {
		switch evaluatorDTO.GetEvaluatorType() {
		case evaluatordto.EvaluatorType_Prompt:
			evaluatorDO.PromptEvaluatorVersion = ConvertPromptEvaluatorVersionDTO2DO(evaluatorDO.ID, evaluatorDO.SpaceID, evaluatorDTO.GetCurrentVersion())
		case evaluatordto.EvaluatorType_Code:
			evaluatorDO.CodeEvaluatorVersion = ConvertCodeEvaluatorVersionDTO2DO(evaluatorDO.ID, evaluatorDO.SpaceID, evaluatorDTO.GetCurrentVersion())
		case evaluatordto.EvaluatorType_CustomRPC:
			customRPCEvaluatorVersion, err := ConvertCustomRPCEvaluatorVersionDTO2DO(evaluatorDO.ID, evaluatorDO.SpaceID, evaluatorDTO.GetCurrentVersion())
			if err != nil {
				return nil, err
			}
			evaluatorDO.CustomRPCEvaluatorVersion = customRPCEvaluatorVersion
		case evaluatordto.EvaluatorType_Agent:
			evaluatorDO.AgentEvaluatorVersion = ConvertAgentEvaluatorVersionDTO2DO(evaluatorDO.ID, evaluatorDO.SpaceID, evaluatorDTO.GetCurrentVersion())
		}
	}
	return evaluatorDO, nil
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
		EvaluatorID:           gptr.Of(do.ID),
		WorkspaceID:           gptr.Of(do.SpaceID),
		Name:                  gptr.Of(do.Name),
		Description:           gptr.Of(do.Description),
		DraftSubmitted:        gptr.Of(do.DraftSubmitted),
		EvaluatorType:         evaluatordto.EvaluatorTypePtr(evaluatordto.EvaluatorType(do.EvaluatorType)),
		LatestVersion:         gptr.Of(do.LatestVersion),
		BuiltinVisibleVersion: gptr.Of(do.BuiltinVisibleVersion),
		Builtin:               gptr.Of(do.Builtin),
		BaseInfo:              commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
		Tags:                  ConvertEvaluatorLangTagsDO2DTO(do.Tags),
	}
	if do.EvaluatorInfo != nil {
		dto.EvaluatorInfo = &evaluatordto.EvaluatorInfo{
			Benchmark:     do.EvaluatorInfo.Benchmark,
			Vendor:        do.EvaluatorInfo.Vendor,
			VendorURL:     do.EvaluatorInfo.VendorURL,
			UserManualURL: do.EvaluatorInfo.UserManualURL,
		}
	}
	// 设置 BoxType
	{
		val := convertBoxTypeDO2DTO(do.BoxType)
		dto.SetBoxType(&val)
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
	case evaluatordo.EvaluatorTypeCustomRPC:
		if do.CustomRPCEvaluatorVersion != nil {
			versionDTO := ConvertCustomRPCEvaluatorVersionDO2DTO(do.CustomRPCEvaluatorVersion)
			dto.CurrentVersion = versionDTO
		}
	case evaluatordo.EvaluatorTypeAgent:
		if do.AgentEvaluatorVersion != nil {
			versionDTO := ConvertAgentEvaluatorVersionDO2DTO(do.AgentEvaluatorVersion)
			dto.CurrentVersion = versionDTO
		}
	}
	return dto
}

// convertBoxTypeDTO2DO 将 DTO 的 BoxType(字符串) 转换为 DO 的 BoxType(数值枚举)
func convertBoxTypeDTO2DO(dtoType string) evaluatordo.EvaluatorBoxType {
	switch dtoType {
	case "White":
		return evaluatordo.EvaluatorBoxTypeWhite
	case "Black":
		return evaluatordo.EvaluatorBoxTypeBlack
	default:
		// 默认回退白盒
		return evaluatordo.EvaluatorBoxTypeWhite
	}
}

// convertBoxTypeDO2DTO 将 DO 的 BoxType(数值枚举) 转换为 DTO 的 BoxType(字符串)
func convertBoxTypeDO2DTO(doType evaluatordo.EvaluatorBoxType) string {
	switch doType {
	case evaluatordo.EvaluatorBoxTypeWhite:
		return "White"
	case evaluatordo.EvaluatorBoxTypeBlack:
		return "Black"
	default:
		return "White"
	}
}

// normalizeLanguageType 标准化语言类型（转换为标准格式）
func normalizeLanguageType(langType evaluatordo.LanguageType) evaluatordo.LanguageType {
	switch strings.ToLower(string(langType)) {
	case "python":
		return evaluatordo.LanguageTypePython // "Python"
	case "js", "javascript":
		return evaluatordo.LanguageTypeJS // "JS"
	default:
		// 对于未知类型，转换为首字母大写格式
		if len(langType) > 0 {
			return evaluatordo.LanguageType(strings.ToUpper(string(langType)[0:1]) + strings.ToLower(string(langType)[1:]))
		}
		return langType
	}
}

// convertLanguageTypeDO2DTO 将DO层的LanguageType转换为DTO层的LanguageType
func convertLanguageTypeDO2DTO(doLangType evaluatordo.LanguageType) evaluatordto.LanguageType {
	// 现在DO和DTO使用相同格式，直接返回
	return evaluatordto.LanguageType(doLangType)
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
				LanguageType:     gptr.Of(convertLanguageTypeDO2DTO(do.LanguageType)),
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
			EvaluatorType:      evaluatordo.EvaluatorTypePrompt,
			PromptSourceType:   evaluatordo.PromptSourceType(content.PromptEvaluator.GetPromptSourceType()),
			PromptTemplateKey:  content.PromptEvaluator.GetPromptTemplateKey(),
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

		// 保持原逻辑：基于旧字段
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

	case evaluatordto.EvaluatorType_CustomRPC:
		if content.CustomRPCEvaluator == nil {
			return nil, errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("custom rpc evaluator content is nil"))
		}

		customRPCVersion := &evaluatordo.CustomRPCEvaluatorVersion{
			ProviderEvaluatorCode: content.CustomRPCEvaluator.ProviderEvaluatorCode,
			AccessProtocol:        content.CustomRPCEvaluator.AccessProtocol,
			ServiceName:           content.CustomRPCEvaluator.ServiceName,
			Cluster:               content.CustomRPCEvaluator.Cluster,
			Timeout:               content.CustomRPCEvaluator.Timeout,
		}
		if content.CustomRPCEvaluator.RateLimit != nil {
			rateLimit, err := commonconvertor.ConvertRateLimitDTO2DO(content.CustomRPCEvaluator.RateLimit)
			if err != nil {
				return nil, err
			}
			customRPCVersion.RateLimit = rateLimit
		}

		// 转换输入模式
		if len(content.InputSchemas) > 0 {
			customRPCVersion.InputSchemas = make([]*evaluatordo.ArgsSchema, 0, len(content.InputSchemas))
			for _, schema := range content.InputSchemas {
				customRPCVersion.InputSchemas = append(customRPCVersion.InputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		// 转换输出模式
		if len(content.OutputSchemas) > 0 {
			customRPCVersion.OutputSchemas = make([]*evaluatordo.ArgsSchema, 0, len(content.OutputSchemas))
			for _, schema := range content.OutputSchemas {
				customRPCVersion.OutputSchemas = append(customRPCVersion.OutputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		evaluator.CustomRPCEvaluatorVersion = customRPCVersion

	case evaluatordto.EvaluatorType_Agent:
		if content.AgentEvaluator == nil {
			return nil, errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("agent evaluator content is nil"))
		}

		agentVersion := &evaluatordo.AgentEvaluatorVersion{
			EvaluatorType: evaluatordo.EvaluatorTypeAgent,
			AgentConfig:   ConvertAgentConfigDTO2DO(content.AgentEvaluator.AgentConfig),
			ModelConfig:   commonconvertor.ConvertModelConfigDTO2DO(content.AgentEvaluator.ModelConfig),
			SkillConfigs:  ConvertSkillConfigsDTO2DO(content.AgentEvaluator.SkillConfigs),
			PromptConfig:  ConvertAgentEvaluatorPromptConfigDTO2DO(content.AgentEvaluator.PromptConfig),
		}

		if len(content.InputSchemas) > 0 {
			agentVersion.InputSchemas = make([]*evaluatordo.ArgsSchema, 0, len(content.InputSchemas))
			for _, schema := range content.InputSchemas {
				agentVersion.InputSchemas = append(agentVersion.InputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		if len(content.OutputSchemas) > 0 {
			agentVersion.OutputSchemas = make([]*evaluatordo.ArgsSchema, 0, len(content.OutputSchemas))
			for _, schema := range content.OutputSchemas {
				agentVersion.OutputSchemas = append(agentVersion.OutputSchemas, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
			}
		}

		evaluator.AgentEvaluatorVersion = agentVersion

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

// ConvertEvaluatorLangTagsDTO2DO 将DTO的多语言Tags转换为DO的多语言Tags
func ConvertEvaluatorLangTagsDTO2DO(dto map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string) map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string {
	if dto == nil {
		return nil
	}
	result := make(map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string, len(dto))
	for lang, tagMap := range dto {
		doLang := evaluatordo.EvaluatorTagLangType(lang)
		if tagMap == nil {
			continue
		}
		inner := make(map[evaluatordo.EvaluatorTagKey][]string, len(tagMap))
		for k, vals := range tagMap {
			inner[ConvertEvaluatorTagKeyDTO2DO(k)] = vals
		}
		result[doLang] = inner
	}
	return result
}

// ConvertEvaluatorLangTagsDO2DTO 将DO的多语言Tags转换为DTO的多语言Tags
func ConvertEvaluatorLangTagsDO2DTO(do map[evaluatordo.EvaluatorTagLangType]map[evaluatordo.EvaluatorTagKey][]string) map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string {
	if do == nil {
		return nil
	}
	result := make(map[evaluatordto.EvaluatorTagLangType]map[evaluatordto.EvaluatorTagKey][]string, len(do))
	for lang, tagMap := range do {
		dtoLang := evaluatordto.EvaluatorTagLangType(lang)
		if tagMap == nil {
			continue
		}
		inner := make(map[evaluatordto.EvaluatorTagKey][]string, len(tagMap))
		for k, vals := range tagMap {
			inner[ConvertEvaluatorTagKeyDO2DTO(k)] = vals
		}
		result[dtoLang] = inner
	}
	return result
}

// ConvertEvaluatorTagKeyDO2DTO 将DO的EvaluatorTagKey转换为DTO的EvaluatorTagKey
func ConvertEvaluatorTagKeyDO2DTO(doKey evaluatordo.EvaluatorTagKey) evaluatordto.EvaluatorTagKey {
	switch doKey {
	case evaluatordo.EvaluatorTagKey_Category:
		return evaluatordto.EvaluatorTagKeyCategory
	case evaluatordo.EvaluatorTagKey_TargetType:
		return evaluatordto.EvaluatorTagKeyTargetType
	case evaluatordo.EvaluatorTagKey_Objective:
		return evaluatordto.EvaluatorTagKeyObjective
	case evaluatordo.EvaluatorTagKey_BusinessScenario:
		return evaluatordto.EvaluatorTagKeyBusinessScenario
	case evaluatordo.EvaluatorTagKey_Name:
		return evaluatordto.EvaluatorTagKeyName
	default:
		return evaluatordto.EvaluatorTagKey(doKey)
	}
}

func ConvertCustomRPCEvaluatorVersionDTO2DO(evaluatorID, spaceID int64, dto *evaluatordto.EvaluatorVersion) (*evaluatordo.CustomRPCEvaluatorVersion, error) {
	if dto == nil {
		return nil, nil
	}
	customRPCEvaluatorVersion := &evaluatordo.CustomRPCEvaluatorVersion{
		ID:            dto.GetID(),
		SpaceID:       spaceID,
		EvaluatorType: evaluatordo.EvaluatorTypeCustomRPC,
		EvaluatorID:   evaluatorID,
		Description:   dto.GetDescription(),
		Version:       dto.GetVersion(),
		BaseInfo:      commonconvertor.ConvertBaseInfoDTO2DO(dto.GetBaseInfo()),
	}
	if dto.EvaluatorContent != nil {
		customRPCEvaluatorVersion.InputSchemas = commonconvertor.ConvertArgsSchemaListDTO2DO(dto.EvaluatorContent.InputSchemas)
		customRPCEvaluatorVersion.OutputSchemas = commonconvertor.ConvertArgsSchemaListDTO2DO(dto.EvaluatorContent.OutputSchemas)
		if dto.EvaluatorContent.CustomRPCEvaluator != nil {
			customRPCEvaluatorVersion.ProviderEvaluatorCode = dto.EvaluatorContent.CustomRPCEvaluator.ProviderEvaluatorCode
			customRPCEvaluatorVersion.AccessProtocol = dto.EvaluatorContent.CustomRPCEvaluator.AccessProtocol
			customRPCEvaluatorVersion.ServiceName = dto.EvaluatorContent.CustomRPCEvaluator.ServiceName
			customRPCEvaluatorVersion.Cluster = dto.EvaluatorContent.CustomRPCEvaluator.Cluster
			customRPCEvaluatorVersion.InvokeHTTPInfo = ConvertEvaluatorHTTPInfoDTO2DO(dto.EvaluatorContent.CustomRPCEvaluator.InvokeHTTPInfo)
			customRPCEvaluatorVersion.Timeout = dto.EvaluatorContent.CustomRPCEvaluator.Timeout
			if dto.EvaluatorContent.CustomRPCEvaluator.RateLimit != nil {
				rateLimit, err := commonconvertor.ConvertRateLimitDTO2DO(dto.EvaluatorContent.CustomRPCEvaluator.RateLimit)
				if err != nil {
					return nil, err
				}
				customRPCEvaluatorVersion.RateLimit = rateLimit
			}
			customRPCEvaluatorVersion.Ext = dto.EvaluatorContent.CustomRPCEvaluator.Ext
		}
	}
	return customRPCEvaluatorVersion, nil
}

func ConvertCustomRPCEvaluatorVersionDO2DTO(do *evaluatordo.CustomRPCEvaluatorVersion) *evaluatordto.EvaluatorVersion {
	if do == nil {
		return nil
	}
	dto := &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(do.ID),
		Version:     gptr.Of(do.Version),
		Description: gptr.Of(do.Description),
		BaseInfo:    commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			ReceiveChatHistory: nil,
			InputSchemas:       commonconvertor.ConvertArgsSchemaListDO2DTO(do.InputSchemas),
			OutputSchemas:      commonconvertor.ConvertArgsSchemaListDO2DTO(do.OutputSchemas),
			CustomRPCEvaluator: &evaluatordto.CustomRPCEvaluator{
				ProviderEvaluatorCode: do.ProviderEvaluatorCode,
				AccessProtocol:        do.AccessProtocol,
				ServiceName:           do.ServiceName,
				Cluster:               do.Cluster,
				Timeout:               do.Timeout,
				InvokeHTTPInfo:        ConvertEvaluatorHTTPInfoDO2DTO(do.InvokeHTTPInfo),
				RateLimit:             commonconvertor.ConvertRateLimitDO2DTO(do.RateLimit),
				Ext:                   do.Ext,
			},
		},
	}
	return dto
}

func ConvertEvaluatorHTTPInfoDTO2DO(dto *evaluatordto.EvaluatorHTTPInfo) *evaluatordo.EvaluatorHTTPInfo {
	if dto == nil {
		return nil
	}
	return &evaluatordo.EvaluatorHTTPInfo{
		Method: dto.Method,
		Path:   dto.Path,
	}
}

func ConvertEvaluatorHTTPInfoDO2DTO(do *evaluatordo.EvaluatorHTTPInfo) *evaluatordto.EvaluatorHTTPInfo {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorHTTPInfo{
		Method: do.Method,
		Path:   do.Path,
	}
}

func ConvertEvaluatorRunConfDTO2DO(dto *evaluatordto.EvaluatorRunConfig) *evaluatordo.EvaluatorRunConfig {
	if dto == nil {
		return nil
	}
	return &evaluatordo.EvaluatorRunConfig{
		Env:                   dto.Env,
		EvaluatorRuntimeParam: commonconvertor.ConvertRuntimeParamDTO2DO(dto.EvaluatorRuntimeParam),
	}
}

func ConvertEvaluatorRunConfDO2DTO(do *evaluatordo.EvaluatorRunConfig) *evaluatordto.EvaluatorRunConfig {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorRunConfig{
		Env:                   do.Env,
		EvaluatorRuntimeParam: commonconvertor.ConvertRuntimeParamDO2DTO(do.EvaluatorRuntimeParam),
	}
}

func ConvertAgentEvaluatorVersionDTO2DO(evaluatorID, spaceID int64, dto *evaluatordto.EvaluatorVersion) *evaluatordo.AgentEvaluatorVersion {
	if dto == nil || dto.EvaluatorContent == nil || dto.EvaluatorContent.AgentEvaluator == nil {
		return nil
	}
	agentEvaluator := dto.EvaluatorContent.AgentEvaluator
	agentEvaluatorVersion := &evaluatordo.AgentEvaluatorVersion{
		ID:            dto.GetID(),
		SpaceID:       spaceID,
		EvaluatorType: evaluatordo.EvaluatorTypeAgent,
		EvaluatorID:   evaluatorID,
		Description:   dto.GetDescription(),
		Version:       dto.GetVersion(),
		BaseInfo:      commonconvertor.ConvertBaseInfoDTO2DO(dto.GetBaseInfo()),
		AgentConfig:   ConvertAgentConfigDTO2DO(agentEvaluator.AgentConfig),
		ModelConfig:   commonconvertor.ConvertModelConfigDTO2DO(agentEvaluator.ModelConfig),
		SkillConfigs:  ConvertSkillConfigsDTO2DO(agentEvaluator.SkillConfigs),
		PromptConfig:  ConvertAgentEvaluatorPromptConfigDTO2DO(agentEvaluator.PromptConfig),
	}
	if dto.EvaluatorContent != nil {
		agentEvaluatorVersion.InputSchemas = commonconvertor.ConvertArgsSchemaListDTO2DO(dto.EvaluatorContent.InputSchemas)
		agentEvaluatorVersion.OutputSchemas = commonconvertor.ConvertArgsSchemaListDTO2DO(dto.EvaluatorContent.OutputSchemas)
	}
	return agentEvaluatorVersion
}

func ConvertAgentEvaluatorVersionDO2DTO(do *evaluatordo.AgentEvaluatorVersion) *evaluatordto.EvaluatorVersion {
	if do == nil {
		return nil
	}
	return &evaluatordto.EvaluatorVersion{
		ID:          gptr.Of(do.ID),
		Version:     gptr.Of(do.Version),
		Description: gptr.Of(do.Description),
		BaseInfo:    commonconvertor.ConvertBaseInfoDO2DTO(do.BaseInfo),
		EvaluatorContent: &evaluatordto.EvaluatorContent{
			InputSchemas:  commonconvertor.ConvertArgsSchemaListDO2DTO(do.InputSchemas),
			OutputSchemas: commonconvertor.ConvertArgsSchemaListDO2DTO(do.OutputSchemas),
			AgentEvaluator: &evaluatordto.AgentEvaluator{
				AgentConfig:  ConvertAgentConfigDO2DTO(do.AgentConfig),
				ModelConfig:  commonconvertor.ConvertModelConfigDO2DTO(do.ModelConfig),
				SkillConfigs: ConvertSkillConfigsDO2DTO(do.SkillConfigs),
				PromptConfig: ConvertAgentEvaluatorPromptConfigDO2DTO(do.PromptConfig),
			},
		},
	}
}

func ConvertAgentConfigDTO2DO(dto *commondto.AgentConfig) *evaluatordo.AgentConfig {
	if dto == nil {
		return nil
	}
	return &evaluatordo.AgentConfig{
		AgentType: evaluatordo.AgentType(dto.GetAgentType()),
	}
}

func ConvertAgentConfigDO2DTO(do *evaluatordo.AgentConfig) *commondto.AgentConfig {
	if do == nil {
		return nil
	}
	return &commondto.AgentConfig{
		AgentType: (*commondto.AgentType)(gptr.Of(string(do.AgentType))),
	}
}

func ConvertSkillConfigsDTO2DO(dtos []*commondto.SkillConfig) []*evaluatordo.SkillConfig {
	if dtos == nil {
		return nil
	}
	dos := make([]*evaluatordo.SkillConfig, 0, len(dtos))
	for _, dto := range dtos {
		if dto != nil {
			dos = append(dos, &evaluatordo.SkillConfig{
				SkillID: dto.SkillID,
				Version: dto.Version,
			})
		}
	}
	return dos
}

func ConvertSkillConfigsDO2DTO(dos []*evaluatordo.SkillConfig) []*commondto.SkillConfig {
	if dos == nil {
		return nil
	}
	dtos := make([]*commondto.SkillConfig, 0, len(dos))
	for _, do := range dos {
		if do != nil {
			dtos = append(dtos, &commondto.SkillConfig{
				SkillID: do.SkillID,
				Version: do.Version,
			})
		}
	}
	return dtos
}

func ConvertAgentEvaluatorPromptConfigDTO2DO(dto *evaluatordto.AgentEvaluatorPromptConfig) *evaluatordo.AgentEvaluatorPromptConfig {
	if dto == nil {
		return nil
	}
	do := &evaluatordo.AgentEvaluatorPromptConfig{}
	if len(dto.MessageList) > 0 {
		do.MessageList = make([]*evaluatordo.Message, 0, len(dto.MessageList))
		for _, msg := range dto.MessageList {
			do.MessageList = append(do.MessageList, commonconvertor.ConvertMessageDTO2DO(msg))
		}
	}
	if dto.OutputRules != nil {
		do.OutputRules = &evaluatordo.AgentEvaluatorPromptConfigOutputRules{
			ScorePrompt:       commonconvertor.ConvertMessageDTO2DO(dto.OutputRules.ScorePrompt),
			ReasoningPrompt:   commonconvertor.ConvertMessageDTO2DO(dto.OutputRules.ReasoningPrompt),
			ExtraOutputPrompt: commonconvertor.ConvertMessageDTO2DO(dto.OutputRules.ExtraOutputPrompt),
		}
	}
	return do
}

func ConvertAgentEvaluatorPromptConfigDO2DTO(do *evaluatordo.AgentEvaluatorPromptConfig) *evaluatordto.AgentEvaluatorPromptConfig {
	if do == nil {
		return nil
	}
	dto := &evaluatordto.AgentEvaluatorPromptConfig{}
	if len(do.MessageList) > 0 {
		dto.MessageList = make([]*commondto.Message, 0, len(do.MessageList))
		for _, msg := range do.MessageList {
			dto.MessageList = append(dto.MessageList, commonconvertor.ConvertMessageDO2DTO(msg))
		}
	}
	if do.OutputRules != nil {
		dto.OutputRules = &evaluatordto.AgentEvaluatorPromptConfigOutputRules{
			ScorePrompt:       commonconvertor.ConvertMessageDO2DTO(do.OutputRules.ScorePrompt),
			ReasoningPrompt:   commonconvertor.ConvertMessageDO2DTO(do.OutputRules.ReasoningPrompt),
			ExtraOutputPrompt: commonconvertor.ConvertMessageDO2DTO(do.OutputRules.ExtraOutputPrompt),
		}
	}
	return dto
}
