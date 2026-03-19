// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	domainopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func OpenAPIPromptDetailDO2DTO(do *entity.PromptDetail) *domainopenapi.PromptDetail {
	if do == nil {
		return nil
	}
	return &domainopenapi.PromptDetail{
		PromptTemplate: OpenAPIPromptTemplateDO2DTO(do.PromptTemplate),
		Tools:          OpenAPIBatchToolDO2DTO(do.Tools),
		ToolCallConfig: OpenAPIToolCallConfigDO2DTO(do.ToolCallConfig),
		ModelConfig:    OpenAPIModelConfigForDetailDO2DTO(do.ModelConfig),
		ExtInfos:       do.ExtInfos,
	}
}

func OpenAPIModelConfigForDetailDO2DTO(do *entity.ModelConfig) *domainopenapi.ModelConfig {
	if do == nil {
		return nil
	}
	return &domainopenapi.ModelConfig{
		ModelID:           ptr.Of(do.ModelID),
		MaxTokens:         do.MaxTokens,
		Temperature:       do.Temperature,
		TopK:              do.TopK,
		TopP:              do.TopP,
		PresencePenalty:   do.PresencePenalty,
		FrequencyPenalty:  do.FrequencyPenalty,
		JSONMode:          do.JSONMode,
		Extra:             do.Extra,
		Thinking:          OpenAPIThinkingConfigDO2DTO(do.Thinking),
		ParamConfigValues: OpenAPIBatchParamConfigValueDO2DTO(do.ParamConfigValues),
	}
}

func OpenAPIBatchParamConfigValueDO2DTO(dos []*entity.ParamConfigValue) []*domainopenapi.ParamConfigValue {
	if len(dos) == 0 {
		return nil
	}
	result := make([]*domainopenapi.ParamConfigValue, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		result = append(result, OpenAPIParamConfigValueDO2DTO(do))
	}
	return result
}

func OpenAPIParamConfigValueDO2DTO(do *entity.ParamConfigValue) *domainopenapi.ParamConfigValue {
	if do == nil {
		return nil
	}
	return &domainopenapi.ParamConfigValue{
		Name:  ptr.Of(do.Name),
		Label: ptr.Of(do.Label),
		Value: OpenAPIParamOptionDO2DTO(do.Value),
	}
}

func OpenAPIParamOptionDO2DTO(do *entity.ParamOption) *domainopenapi.ParamOption {
	if do == nil {
		return nil
	}
	return &domainopenapi.ParamOption{
		Value: ptr.Of(do.Value),
		Label: ptr.Of(do.Label),
	}
}

func OpenAPIPromptTemplateDTO2DO(dto *domainopenapi.PromptTemplate) *entity.PromptTemplate {
	if dto == nil {
		return nil
	}
	return &entity.PromptTemplate{
		TemplateType: entity.TemplateType(dto.GetTemplateType()),
		Messages:     OpenAPIBatchMessageDTO2DO(dto.Messages),
		VariableDefs: OpenAPIBatchVariableDefDTO2DO(dto.VariableDefs),
		Metadata:     dto.Metadata,
	}
}

func OpenAPIBatchVariableDefDTO2DO(dtos []*domainopenapi.VariableDef) []*entity.VariableDef {
	if dtos == nil {
		return nil
	}
	defs := make([]*entity.VariableDef, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		defs = append(defs, OpenAPIVariableDefDTO2DO(dto))
	}
	return defs
}

func OpenAPIVariableDefDTO2DO(dto *domainopenapi.VariableDef) *entity.VariableDef {
	if dto == nil {
		return nil
	}
	return &entity.VariableDef{
		Key:  dto.GetKey(),
		Desc: dto.GetDesc(),
		Type: entity.VariableType(dto.GetType()),
	}
}

func OpenAPIPromptDetailDTO2DO(dto *domainopenapi.PromptDetail) *entity.PromptDetail {
	if dto == nil {
		return nil
	}
	return &entity.PromptDetail{
		PromptTemplate: OpenAPIPromptTemplateDTO2DO(dto.PromptTemplate),
		Tools:          OpenAPIBatchToolDTO2DO(dto.Tools),
		ToolCallConfig: OpenAPIToolCallConfigDTO2DO(dto.ToolCallConfig),
		ModelConfig:    OpenAPIModelConfigDTO2DO(dto.ModelConfig),
		ExtInfos:       dto.ExtInfos,
	}
}

func OpenAPIDraftInfoDO2DTO(do *entity.DraftInfo) *domainopenapi.DraftInfo {
	if do == nil {
		return nil
	}
	return &domainopenapi.DraftInfo{
		UserID:      ptr.Of(do.UserID),
		BaseVersion: ptr.Of(do.BaseVersion),
		IsModified:  ptr.Of(do.IsModified),
		CreatedAt:   ptr.Of(do.CreatedAt.UnixMilli()),
		UpdatedAt:   ptr.Of(do.UpdatedAt.UnixMilli()),
	}
}

func OpenAPICommitInfoDO2DTO(do *entity.CommitInfo) *domainopenapi.CommitInfo {
	if do == nil {
		return nil
	}
	return &domainopenapi.CommitInfo{
		Version:     ptr.Of(do.Version),
		BaseVersion: ptr.Of(do.BaseVersion),
		Description: ptr.Of(do.Description),
		CommittedBy: ptr.Of(do.CommittedBy),
		CommittedAt: ptr.Of(do.CommittedAt.UnixMilli()),
	}
}

func OpenAPIBatchCommitInfoDO2DTO(dos []*entity.CommitInfo) []*domainopenapi.CommitInfo {
	if len(dos) == 0 {
		return nil
	}
	infos := make([]*domainopenapi.CommitInfo, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		infos = append(infos, OpenAPICommitInfoDO2DTO(do))
	}
	return infos
}

func OpenAPIPromptDraftDO2DTO(do *entity.PromptDraft) *domainopenapi.PromptDraft {
	if do == nil {
		return nil
	}
	return &domainopenapi.PromptDraft{
		Detail:    OpenAPIPromptDetailDO2DTO(do.PromptDetail),
		DraftInfo: OpenAPIDraftInfoDO2DTO(do.DraftInfo),
	}
}

func OpenAPIPromptCommitDO2DTO(do *entity.PromptCommit) *domainopenapi.PromptCommit {
	if do == nil {
		return nil
	}
	return &domainopenapi.PromptCommit{
		Detail:     OpenAPIPromptDetailDO2DTO(do.PromptDetail),
		CommitInfo: OpenAPICommitInfoDO2DTO(do.CommitInfo),
	}
}

func OpenAPIPromptManageDO2DTO(do *entity.Prompt) *domainopenapi.PromptManage {
	if do == nil {
		return nil
	}
	return &domainopenapi.PromptManage{
		ID:          ptr.Of(do.ID),
		WorkspaceID: ptr.Of(do.SpaceID),
		PromptKey:   ptr.Of(do.PromptKey),
		PromptBasic: OpenAPIPromptBasicDO2DTO(do),
		PromptDraft: OpenAPIPromptDraftDO2DTO(do.PromptDraft),
		PromptCommit: func() *domainopenapi.PromptCommit {
			if do.PromptCommit == nil {
				return nil
			}
			return OpenAPIPromptCommitDO2DTO(do.PromptCommit)
		}(),
	}
}

func OpenAPIDraftInfoDTO2DO(dto *domainopenapi.DraftInfo) *entity.DraftInfo {
	if dto == nil {
		return nil
	}
	return &entity.DraftInfo{
		UserID:      dto.GetUserID(),
		BaseVersion: dto.GetBaseVersion(),
		IsModified:  dto.GetIsModified(),
		CreatedAt:   time.UnixMilli(dto.GetCreatedAt()),
		UpdatedAt:   time.UnixMilli(dto.GetUpdatedAt()),
	}
}
