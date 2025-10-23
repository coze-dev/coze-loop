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
		targetVersionDO.CustomRPCServer = CustomRPCServerDTO2DO(targetVersionDTO.GetEvalTargetContent().GetCustomRPCServer())
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
	case do.EvalTargetTypeVolcengineAgent:
		targetVersionDTO.EvalTargetContent = &dto.EvalTargetContent{
			InputSchemas:  make([]*commondto.ArgsSchema, 0),
			OutputSchemas: make([]*commondto.ArgsSchema, 0),
		}
		if targetVersionDO.VolcengineAgent != nil {
			endpoints := make([]*dto.VolcengineAgentEndpoint, 0)
			for _, e := range targetVersionDO.VolcengineAgent.VolcengineAgentEndpoints {
				endpoints = append(endpoints, &dto.VolcengineAgentEndpoint{
					EndpointID: &e.EndpointID,
					APIKey:     &e.APIKey,
				})
			}
			targetVersionDTO.EvalTargetContent.VolcengineAgent = &dto.VolcengineAgent{
				ID:                       &targetVersionDO.VolcengineAgent.ID,
				Name:                     &targetVersionDO.VolcengineAgent.Name,
				Description:              &targetVersionDO.VolcengineAgent.Description,
				VolcengineAgentEndpoints: endpoints,
				Protocol:                 gptr.Of(dto.VolcengineAgentProtocol(gptr.Indirect(targetVersionDO.VolcengineAgent.Protocol))),
				BaseInfo:                 commonconvertor.ConvertBaseInfoDO2DTO(targetVersionDO.VolcengineAgent.BaseInfo),
			}
		}
	case do.EvalTargetTypeCustomRPCServer:
		targetVersionDTO.EvalTargetContent = &dto.EvalTargetContent{
			InputSchemas:  make([]*commondto.ArgsSchema, 0),
			OutputSchemas: make([]*commondto.ArgsSchema, 0),
		}
		if targetVersionDO.CustomRPCServer != nil {
			targetVersionDTO.EvalTargetContent.CustomRPCServer = CustomRPCServerDO2DTO(targetVersionDO.CustomRPCServer)
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

func CustomRPCServerDO2DTO(do *do.CustomRPCServer) (dtoRes *dto.CustomRPCServer) {
	return &dto.CustomRPCServer{
		ID:                  &do.ID,
		Name:                &do.Name,
		Description:         &do.Description,
		ServerName:          &do.ServerName,
		AccessProtocol:      &do.AccessProtocol,
		Regions:             do.Regions,
		Cluster:             &do.Cluster,
		InvokeHTTPInfo:      HttpInfoDO2DTO(do.InvokeHTTPInfo),
		AsyncInvokeHTTPInfo: HttpInfoDO2DTO(do.AsyncInvokeHTTPInfo),
		NeedSearchTarget:    do.NeedSearchTarget,
		SearchHTTPInfo:      HttpInfoDO2DTO(do.SearchHTTPInfo),
		CustomEvalTarget:    CustomEvalTargetDO2DTO(do.CustomEvalTarget),
		IsAsync:             do.IsAsync,
		ExecRegion:          gptr.Of(do.ExecRegion),
		ExecEnv:             do.ExecEnv,
		Timeout:             do.Timeout,
		AsyncTimeout:        do.AsyncTimeout,
		Ext:                 do.Ext,
	}
}

func CustomRPCServerDTO2DO(dto *dto.CustomRPCServer) (doRes *do.CustomRPCServer) {
	if dto == nil {
		return nil
	}
	return &do.CustomRPCServer{
		ID:                  gptr.Indirect(dto.ID),
		Name:                gptr.Indirect(dto.Name),
		Description:         gptr.Indirect(dto.Description),
		ServerName:          gptr.Indirect(dto.ServerName),
		AccessProtocol:      gptr.Indirect(dto.AccessProtocol),
		Regions:             dto.Regions,
		Cluster:             gptr.Indirect(dto.Cluster),
		NeedSearchTarget:    dto.NeedSearchTarget,
		IsAsync:             dto.IsAsync,
		InvokeHTTPInfo:      HttpInfoDTO2DO(dto.InvokeHTTPInfo),
		AsyncInvokeHTTPInfo: HttpInfoDTO2DO(dto.AsyncInvokeHTTPInfo),
		SearchHTTPInfo:      HttpInfoDTO2DO(dto.SearchHTTPInfo),
		CustomEvalTarget:    CustomEvalTargetDTO2DO(dto.CustomEvalTarget),
		ExecRegion:          gptr.Indirect(dto.ExecRegion),
		ExecEnv:             dto.ExecEnv,
		Timeout:             dto.Timeout,
		AsyncTimeout:        dto.AsyncTimeout,
		Ext:                 dto.Ext,
	}
}

func HttpInfoDTO2DO(httpInfoDTO *dto.HTTPInfo) (httpInfoDO *do.HTTPInfo) {
	if httpInfoDTO == nil {
		return nil
	}
	return &do.HTTPInfo{
		Method: gptr.Indirect(httpInfoDTO.Method),
		Path:   gptr.Indirect(httpInfoDTO.Path),
	}
}

func HttpInfoDO2DTO(httpInfoDO *do.HTTPInfo) (httpInfoDTO *dto.HTTPInfo) {
	if httpInfoDO == nil {
		return nil
	}
	return &dto.HTTPInfo{
		Method: gptr.Of(httpInfoDO.Method),
		Path:   gptr.Of(httpInfoDO.Path),
	}
}

func CustomEvalTargetDTO2DO(customEvalTargetDTO *dto.CustomEvalTarget) (customEvalTargetDO *do.CustomEvalTarget) {
	if customEvalTargetDTO == nil {
		return nil
	}
	return &do.CustomEvalTarget{
		ID:        customEvalTargetDTO.ID,
		Name:      customEvalTargetDTO.Name,
		AvatarURL: customEvalTargetDTO.AvatarURL,
		Ext:       customEvalTargetDTO.Ext,
	}
}

func CustomEvalTargetDO2DTO(customEvalTargetDO *do.CustomEvalTarget) (customEvalTargetDTO *dto.CustomEvalTarget) {
	if customEvalTargetDO == nil {
		return nil
	}
	return &dto.CustomEvalTarget{
		ID:        customEvalTargetDO.ID,
		Name:      customEvalTargetDO.Name,
		AvatarURL: customEvalTargetDO.AvatarURL,
		Ext:       customEvalTargetDO.Ext,
	}
}

func CustomEvalTargetDO2DTOs(customEvalTargetDOs []*do.CustomEvalTarget) (customEvalTargetDTOs []*dto.CustomEvalTarget) {
	if customEvalTargetDOs == nil {
		return nil
	}
	customEvalTargetDTOs = make([]*dto.CustomEvalTarget, 0)
	for _, customEvalTargetDO := range customEvalTargetDOs {
		if customEvalTargetDO == nil {
			continue
		}
		customEvalTargetDTOs = append(customEvalTargetDTOs, CustomEvalTargetDO2DTO(customEvalTargetDO))
	}
	return customEvalTargetDTOs
}
