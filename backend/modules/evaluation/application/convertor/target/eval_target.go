// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"github.com/bytedance/gg/gptr"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	dto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	commonconvertor "github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/common"
	do "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func EvalTargetDTO2DO(targetDTO *dto.EvalTarget) (targetDO *do.EvalTarget) {
	if targetDTO == nil {
		return nil
	}
	targetDO = &do.EvalTarget{}
	targetDO.ID = targetDTO.GetID()
	targetDO.SpaceID = targetDTO.GetWorkspaceID()
	targetDO.SourceTargetID = targetDTO.GetSourceTargetID()
	targetDO.EvalTargetType = do.EvalTargetType(targetDTO.GetEvalTargetType())
	targetDO.BaseInfo = commonconvertor.ConvertBaseInfoDTO2DO(targetDTO.GetBaseInfo())
	targetDO.EvalTargetVersion = &do.EvalTargetVersion{}

	targetDO.EvalTargetVersion = EvalTargetVersionDTO2DO(targetDTO.GetEvalTargetVersion())

	return targetDO
}

func EvalTargetVersionDTO2DO(targetVersionDTO *dto.EvalTargetVersion) (targetVersionDO *do.EvalTargetVersion) {
	if targetVersionDTO == nil {
		return nil
	}

	targetVersionDO = &do.EvalTargetVersion{}

	targetVersionDO.ID = targetVersionDTO.GetID()
	targetVersionDO.SpaceID = targetVersionDTO.GetWorkspaceID()
	targetVersionDO.TargetID = targetVersionDTO.GetTargetID()
	targetVersionDO.SourceTargetVersion = targetVersionDTO.GetSourceTargetVersion()
	if targetVersionDTO.GetEvalTargetContent() != nil {
		targetVersionDO.InputSchema = make([]*do.ArgsSchema, 0)
		for _, schema := range targetVersionDTO.GetEvalTargetContent().GetInputSchemas() {
			targetVersionDO.InputSchema = append(targetVersionDO.InputSchema, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
		}
		targetVersionDO.OutputSchema = make([]*do.ArgsSchema, 0)
		for _, schema := range targetVersionDTO.GetEvalTargetContent().GetOutputSchemas() {
			targetVersionDO.OutputSchema = append(targetVersionDO.OutputSchema, commonconvertor.ConvertArgsSchemaDTO2DO(schema))
		}
		if targetVersionDTO.GetEvalTargetContent().GetCozeBot() != nil {
			targetVersionDO.CozeBot = &do.CozeBot{
				BotID:       targetVersionDTO.GetEvalTargetContent().GetCozeBot().GetBotID(),
				BotVersion:  targetVersionDTO.GetEvalTargetContent().GetCozeBot().GetBotVersion(),
				BotInfoType: do.CozeBotInfoType(gptr.Indirect(targetVersionDTO.GetEvalTargetContent().GetCozeBot().BotInfoType)),
				BotName:     targetVersionDTO.GetEvalTargetContent().GetCozeBot().GetBotName(),
				AvatarURL:   targetVersionDTO.GetEvalTargetContent().GetCozeBot().GetAvatarURL(),
				Description: targetVersionDTO.GetEvalTargetContent().GetCozeBot().GetDescription(),
				BaseInfo:    commonconvertor.ConvertBaseInfoDTO2DO(targetVersionDTO.GetEvalTargetContent().GetCozeBot().GetBaseInfo()),
			}
		}
		if targetVersionDTO.GetEvalTargetContent().GetPrompt() != nil {
			targetVersionDO.Prompt = &do.LoopPrompt{
				PromptID:     targetVersionDTO.GetEvalTargetContent().GetPrompt().GetPromptID(),
				Version:      targetVersionDTO.GetEvalTargetContent().GetPrompt().GetVersion(),
				PromptKey:    targetVersionDTO.GetEvalTargetContent().GetPrompt().GetPromptKey(),
				Name:         targetVersionDTO.GetEvalTargetContent().GetPrompt().GetName(),
				SubmitStatus: do.SubmitStatus(targetVersionDTO.GetEvalTargetContent().GetPrompt().GetSubmitStatus()),
				Description:  targetVersionDTO.GetEvalTargetContent().GetPrompt().GetDescription(),
			}
		}
		targetVersionDO.RuntimeParamDemo = gptr.Of(targetVersionDTO.GetEvalTargetContent().GetRuntimeParamJSONDemo())
	}

	return targetVersionDO
}

func EvalTargetListDO2DTO(targetDOList []*do.EvalTarget) (targetDTOList []*dto.EvalTarget) {
	res := make([]*dto.EvalTarget, 0)
	for _, evalTarget := range targetDOList {
		res = append(res, EvalTargetDO2DTO(evalTarget))
	}
	return res
}

func EvalTargetDO2DTO(targetDO *do.EvalTarget) (targetDTO *dto.EvalTarget) {
	if targetDO == nil {
		return nil
	}

	targetDTO = &dto.EvalTarget{
		ID:             &targetDO.ID,
		WorkspaceID:    &targetDO.SpaceID,
		SourceTargetID: &targetDO.SourceTargetID,
		EvalTargetType: gptr.Of(dto.EvalTargetType(targetDO.EvalTargetType)),
	}
	if targetDO.EvalTargetVersion != nil {
		// 填充version上的类型
		if targetDO.EvalTargetVersion.EvalTargetType == 0 {
			targetDO.EvalTargetVersion.EvalTargetType = targetDO.EvalTargetType
		}
		targetDTO.EvalTargetVersion = EvalTargetVersionDO2DTO(targetDO.EvalTargetVersion)
	}
	// 处理BaseInfo
	targetDTO.BaseInfo = commonconvertor.ConvertBaseInfoDO2DTO(targetDO.BaseInfo)
	return targetDTO
}

func EvalTargetVersionDO2DTO(targetVersionDO *do.EvalTargetVersion) (targetVersionDTO *dto.EvalTargetVersion) {
	if targetVersionDO == nil {
		return nil
	}

	targetVersionDTO = &dto.EvalTargetVersion{
		ID:                  &targetVersionDO.ID,
		WorkspaceID:         &targetVersionDO.SpaceID,
		TargetID:            &targetVersionDO.TargetID,
		SourceTargetVersion: &targetVersionDO.SourceTargetVersion,
	}
	switch targetVersionDO.EvalTargetType {
	case do.EvalTargetTypeCozeBot:
		targetVersionDTO.EvalTargetContent = &dto.EvalTargetContent{
			InputSchemas:  make([]*commondto.ArgsSchema, 0),
			OutputSchemas: make([]*commondto.ArgsSchema, 0),
		}
		if targetVersionDO.CozeBot != nil {
			targetVersionDTO.EvalTargetContent.CozeBot = &dto.CozeBot{
				BotID:       &targetVersionDO.CozeBot.BotID,
				BotVersion:  &targetVersionDO.CozeBot.BotVersion,
				BotInfoType: gptr.Of(dto.CozeBotInfoType(targetVersionDO.CozeBot.BotInfoType)),
				BotName:     &targetVersionDO.CozeBot.BotName,
				AvatarURL:   &targetVersionDO.CozeBot.AvatarURL,
				Description: &targetVersionDO.CozeBot.Description,
				BaseInfo:    commonconvertor.ConvertBaseInfoDO2DTO(targetVersionDO.CozeBot.BaseInfo),
			}
		}
	case do.EvalTargetTypeLoopPrompt:
		targetVersionDTO.EvalTargetContent = &dto.EvalTargetContent{
			InputSchemas:  make([]*commondto.ArgsSchema, 0),
			OutputSchemas: make([]*commondto.ArgsSchema, 0),
		}
		if targetVersionDO.Prompt != nil {
			targetVersionDTO.EvalTargetContent.Prompt = &dto.EvalPrompt{
				PromptID:     &targetVersionDO.Prompt.PromptID,
				Version:      &targetVersionDO.Prompt.Version,
				PromptKey:    &targetVersionDO.Prompt.PromptKey,
				Name:         &targetVersionDO.Prompt.Name,
				SubmitStatus: gptr.Of(dto.SubmitStatus(targetVersionDO.Prompt.SubmitStatus)),
				Description:  &targetVersionDO.Prompt.Description,
			}
		}
	case do.EvalTargetTypeCozeWorkflow:
		targetVersionDTO.EvalTargetContent = &dto.EvalTargetContent{
			InputSchemas:  make([]*commondto.ArgsSchema, 0),
			OutputSchemas: make([]*commondto.ArgsSchema, 0),
		}
		if targetVersionDO.CozeWorkflow != nil {
			targetVersionDTO.EvalTargetContent.CozeWorkflow = &dto.CozeWorkflow{
				ID:          &targetVersionDO.CozeWorkflow.ID,
				Version:     &targetVersionDO.CozeWorkflow.Version,
				Name:        &targetVersionDO.CozeWorkflow.Name,
				AvatarURL:   &targetVersionDO.CozeWorkflow.AvatarURL,
				Description: &targetVersionDO.CozeWorkflow.Description,
				BaseInfo:    commonconvertor.ConvertBaseInfoDO2DTO(targetVersionDO.CozeWorkflow.BaseInfo),
			}
		}
	default:
		targetVersionDTO.EvalTargetContent = &dto.EvalTargetContent{
			InputSchemas:  make([]*commondto.ArgsSchema, 0),
			OutputSchemas: make([]*commondto.ArgsSchema, 0),
		}
	}
	for _, v := range targetVersionDO.InputSchema {
		targetVersionDTO.EvalTargetContent.InputSchemas = append(targetVersionDTO.EvalTargetContent.InputSchemas, commonconvertor.ConvertArgsSchemaDO2DTO(v))
	}
	for _, v := range targetVersionDO.OutputSchema {
		targetVersionDTO.EvalTargetContent.OutputSchemas = append(targetVersionDTO.EvalTargetContent.OutputSchemas, commonconvertor.ConvertArgsSchemaDO2DTO(v))
	}
	targetVersionDTO.BaseInfo = commonconvertor.ConvertBaseInfoDO2DTO(targetVersionDO.BaseInfo)

	return targetVersionDTO
}
