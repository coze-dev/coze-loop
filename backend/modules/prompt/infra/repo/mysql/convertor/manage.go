// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func BatchBasicAndDraftPO2PromptDO(basicPOs []*model.PromptBasic, draftPOMap map[mysql.PromptIDUserIDPair]*model.PromptUserDraft, UserID string) []*entity.Prompt {
	if len(basicPOs) <= 0 {
		return nil
	}
	promptDOs := make([]*entity.Prompt, 0, len(basicPOs))
	for _, basicPO := range basicPOs {
		promptDO := PromptPO2DO(basicPO, nil, draftPOMap[mysql.PromptIDUserIDPair{
			PromptID: basicPO.ID,
			UserID:   UserID,
		}])
		if promptDO == nil {
			continue
		}
		promptDOs = append(promptDOs, promptDO)
	}
	if len(promptDOs) <= 0 {
		return nil
	}
	return promptDOs
}

func BatchBasicPO2PromptDO(basicPOs []*model.PromptBasic) []*entity.Prompt {
	if len(basicPOs) <= 0 {
		return nil
	}
	promptDOs := make([]*entity.Prompt, 0, len(basicPOs))
	for _, basicPO := range basicPOs {
		promptDO := PromptPO2DO(basicPO, nil, nil)
		if promptDO == nil {
			continue
		}
		promptDOs = append(promptDOs, promptDO)
	}
	if len(promptDOs) <= 0 {
		return nil
	}
	return promptDOs
}

func PromptPO2DO(basicPO *model.PromptBasic, commitPO *model.PromptCommit, draftPO *model.PromptUserDraft) *entity.Prompt {
	if basicPO == nil {
		return nil
	}
	return &entity.Prompt{
		ID:           basicPO.ID,
		SpaceID:      basicPO.SpaceID,
		PromptKey:    basicPO.PromptKey,
		PromptBasic:  BasicPO2DO(basicPO),
		PromptCommit: CommitPO2DO(commitPO),
		PromptDraft:  DraftPO2DO(draftPO),
	}
}

func BasicPO2DO(promptPO *model.PromptBasic) *entity.PromptBasic {
	if promptPO == nil {
		return nil
	}
	return &entity.PromptBasic{
		DisplayName:       promptPO.Name,
		Description:       promptPO.Description,
		LatestVersion:     promptPO.LatestVersion,
		CreatedBy:         promptPO.CreatedBy,
		UpdatedBy:         promptPO.UpdatedBy,
		CreatedAt:         promptPO.CreatedAt,
		UpdatedAt:         promptPO.UpdatedAt,
		LatestCommittedAt: promptPO.LatestCommitTime,
	}
}

func BatchGetCommitInfoDOFromCommitDO(commitDOs []*entity.PromptCommit) []*entity.CommitInfo {
	if len(commitDOs) <= 0 {
		return nil
	}
	commitInfoDOs := make([]*entity.CommitInfo, 0, len(commitDOs))
	for _, commitDO := range commitDOs {
		if commitDO == nil || commitDO.CommitInfo == nil {
			continue
		}
		commitInfoDOs = append(commitInfoDOs, commitDO.CommitInfo)
	}
	if len(commitInfoDOs) <= 0 {
		return nil
	}
	return commitInfoDOs
}

func BatchCommitPO2DO(commitPOs []*model.PromptCommit) []*entity.PromptCommit {
	if commitPOs == nil {
		return nil
	}
	commitDOs := make([]*entity.PromptCommit, 0, len(commitPOs))
	for _, commitPO := range commitPOs {
		commitDOs = append(commitDOs, CommitPO2DO(commitPO))
	}
	if len(commitDOs) <= 0 {
		return nil
	}
	return commitDOs
}

func CommitPO2DO(commitPO *model.PromptCommit) *entity.PromptCommit {
	if commitPO == nil {
		return nil
	}
	return &entity.PromptCommit{
		CommitInfo: &entity.CommitInfo{
			Version:     commitPO.Version,
			BaseVersion: commitPO.BaseVersion,
			Description: ptr.From(commitPO.Description),
			CommittedBy: commitPO.CommittedBy,
			CommittedAt: commitPO.CreatedAt,
		},
		PromptDetail: PromptCommitPO2PromptDetailDO(commitPO),
	}
}

// =====================================================================

func PromptDO2BasicPO(do *entity.Prompt) *model.PromptBasic {
	if do == nil || do.PromptBasic == nil { // todo 子结构体不专门判并拦截，为空相应字段就不set了
		return nil
	}

	return &model.PromptBasic{
		ID:            do.ID,
		SpaceID:       do.SpaceID,
		PromptKey:     do.PromptKey,
		Name:          do.PromptBasic.DisplayName,
		Description:   do.PromptBasic.Description,
		CreatedBy:     do.PromptBasic.CreatedBy,
		UpdatedBy:     do.PromptBasic.UpdatedBy,
		LatestVersion: do.PromptBasic.LatestVersion,
		CreatedAt:     do.PromptBasic.CreatedAt,
		UpdatedAt:     do.PromptBasic.UpdatedAt,
	}
}

func PromptDO2CommitPO(do *entity.Prompt) *model.PromptCommit {
	if do == nil {
		return nil
	}

	po := &model.PromptCommit{
		SpaceID:   do.SpaceID,
		PromptID:  do.ID,
		PromptKey: do.PromptKey,
	}

	if do.PromptCommit != nil {
		if do.PromptCommit.CommitInfo != nil {
			po.Version = do.PromptCommit.CommitInfo.Version
			po.BaseVersion = do.PromptCommit.CommitInfo.BaseVersion
			po.CommittedBy = do.PromptCommit.CommitInfo.CommittedBy
			po.CreatedAt = do.PromptCommit.CommitInfo.CommittedAt
			po.UpdatedAt = do.PromptCommit.CommitInfo.CommittedAt
			po.Description = ptr.Of(do.PromptCommit.CommitInfo.Description)
		}
		if do.PromptCommit.PromptDetail != nil {
			if do.PromptCommit.PromptDetail.ModelConfig != nil {
				po.ModelConfig = ptr.Of(json.Jsonify(do.PromptCommit.PromptDetail.ModelConfig))
			}
			if do.PromptCommit.PromptDetail.Tools != nil {
				po.Tools = ptr.Of(json.Jsonify(do.PromptCommit.PromptDetail.Tools))
			}
			if do.PromptCommit.PromptDetail.ToolCallConfig != nil {
				po.ToolCallConfig = ptr.Of(json.Jsonify(do.PromptCommit.PromptDetail.ToolCallConfig))
			}
			if do.PromptCommit.PromptDetail.PromptTemplate != nil {
				po.TemplateType = ptr.Of(string(do.PromptCommit.PromptDetail.PromptTemplate.TemplateType))
				if do.PromptCommit.PromptDetail.PromptTemplate.Messages != nil {
					po.Messages = ptr.Of(json.Jsonify(do.PromptCommit.PromptDetail.PromptTemplate.Messages))
				}
				if do.PromptCommit.PromptDetail.PromptTemplate.VariableDefs != nil {
					po.VariableDefs = ptr.Of(json.Jsonify(do.PromptCommit.PromptDetail.PromptTemplate.VariableDefs))
				}
			}
		}
	}

	return po
}

func PromptDO2DraftPO(promptDO *entity.Prompt) *model.PromptUserDraft {
	if promptDO == nil {
		return nil
	}

	po := &model.PromptUserDraft{
		SpaceID:  promptDO.SpaceID,
		PromptID: promptDO.ID,
	}
	if promptDO.PromptDraft != nil {
		detailDO := promptDO.PromptDraft.PromptDetail
		if detailDO != nil {
			if detailDO.PromptTemplate != nil {
				po.TemplateType = ptr.Of(string(detailDO.PromptTemplate.TemplateType))
				if detailDO.PromptTemplate.Messages != nil {
					po.Messages = ptr.Of(json.Jsonify(detailDO.PromptTemplate.Messages))
				}
				if detailDO.PromptTemplate.VariableDefs != nil {
					po.VariableDefs = ptr.Of(json.Jsonify(detailDO.PromptTemplate.VariableDefs))
				}
			}
			if detailDO.ModelConfig != nil {
				po.ModelConfig = ptr.Of(json.Jsonify(detailDO.ModelConfig))
			}
			if detailDO.Tools != nil {
				po.Tools = ptr.Of(json.Jsonify(detailDO.Tools))
			}
			if detailDO.ToolCallConfig != nil {
				po.ToolCallConfig = ptr.Of(json.Jsonify(detailDO.ToolCallConfig))
			}
		}
		infoDO := promptDO.PromptDraft.DraftInfo
		if infoDO != nil {
			po.UserID = infoDO.UserID
			po.BaseVersion = infoDO.BaseVersion
			po.IsDraftEdited = MarshalBool(infoDO.IsModified)
		}
	}
	return po
}

func DraftPO2DO(draftPO *model.PromptUserDraft) *entity.PromptDraft {
	if draftPO == nil {
		return nil
	}
	return &entity.PromptDraft{
		DraftInfo: &entity.DraftInfo{
			UserID:      draftPO.UserID,
			BaseVersion: draftPO.BaseVersion,
			IsModified:  UnmarshalBool(draftPO.IsDraftEdited),
			CreatedAt:   draftPO.CreatedAt,
			UpdatedAt:   draftPO.UpdatedAt,
		},
		PromptDetail: PromptUserDraftPO2PromptDetailDO(draftPO),
	}
}

func PromptUserDraftPO2PromptDetailDO(draftPO *model.PromptUserDraft) *entity.PromptDetail {
	if draftPO == nil {
		return nil
	}
	return &entity.PromptDetail{
		PromptTemplate: &entity.PromptTemplate{
			Messages:     UnmarshalMessageDOs(draftPO.Messages),
			VariableDefs: UnmarshalVariableDefDOs(draftPO.VariableDefs),
			TemplateType: UnmarshalTemplateType(draftPO.TemplateType),
		},
		Tools:          UnmarshalToolDOs(draftPO.Tools),
		ToolCallConfig: UnmarshalToolCallConfig(draftPO.ToolCallConfig),
		ModelConfig:    UnmarshalModelConfig(draftPO.ModelConfig),
	}
}

func PromptCommitPO2PromptDetailDO(commitPO *model.PromptCommit) *entity.PromptDetail {
	if commitPO == nil {
		return nil
	}
	return &entity.PromptDetail{
		PromptTemplate: &entity.PromptTemplate{
			Messages:     UnmarshalMessageDOs(commitPO.Messages),
			VariableDefs: UnmarshalVariableDefDOs(commitPO.VariableDefs),
			TemplateType: UnmarshalTemplateType(commitPO.TemplateType),
		},
		Tools:          UnmarshalToolDOs(commitPO.Tools),
		ToolCallConfig: UnmarshalToolCallConfig(commitPO.ToolCallConfig),
		ModelConfig:    UnmarshalModelConfig(commitPO.ModelConfig),
	}
}

func UnmarshalMessageDOs(text *string) []*entity.Message {
	if text == nil {
		return nil
	}
	messages := make([]*entity.Message, 0)
	_ = json.Unmarshal([]byte(*text), &messages)
	return messages
}

func UnmarshalVariableDefDOs(text *string) []*entity.VariableDef {
	if text == nil {
		return nil
	}
	variableDefs := make([]*entity.VariableDef, 0)
	_ = json.Unmarshal([]byte(*text), &variableDefs)
	return variableDefs
}

func UnmarshalTemplateType(text *string) entity.TemplateType {
	if text == nil {
		return entity.TemplateType("")
	}
	return entity.TemplateType(*text)
}

func UnmarshalToolCallConfig(text *string) *entity.ToolCallConfig {
	if text == nil {
		return nil
	}
	toolCallConfig := &entity.ToolCallConfig{}
	_ = json.Unmarshal([]byte(*text), &toolCallConfig)
	return toolCallConfig
}

func UnmarshalModelConfig(text *string) *entity.ModelConfig {
	if text == nil {
		return nil
	}
	modelConfig := &entity.ModelConfig{}
	_ = json.Unmarshal([]byte(*text), &modelConfig)
	return modelConfig
}

func UnmarshalToolDOs(text *string) []*entity.Tool {
	if text == nil {
		return nil
	}
	tools := make([]*entity.Tool, 0)
	_ = json.Unmarshal([]byte(*text), &tools)
	return tools
}

func UnmarshalBool(val int32) bool {
	return val != 0
}

func MarshalBool(val bool) int32 {
	return int32(lo.Ternary(val, 1, 0))
}
