// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func CommonToolDO2DTO(toolDO *entity.CommonTool) *tool.Tool {
	if toolDO == nil {
		return nil
	}
	return &tool.Tool{
		ID:          ptr.Of(toolDO.ID),
		WorkspaceID: ptr.Of(toolDO.SpaceID),
		ToolBasic:   CommonToolBasicDO2DTO(toolDO.ToolBasic),
		ToolCommit:  CommonToolCommitDO2DTO(toolDO.ToolCommit),
	}
}

func BatchCommonToolDO2DTO(toolDOs []*entity.CommonTool) []*tool.Tool {
	if len(toolDOs) == 0 {
		return nil
	}
	result := make([]*tool.Tool, 0, len(toolDOs))
	for _, toolDO := range toolDOs {
		dto := CommonToolDO2DTO(toolDO)
		if dto != nil {
			result = append(result, dto)
		}
	}
	return result
}

func CommonToolBasicDO2DTO(basicDO *entity.ToolBasic) *tool.ToolBasic {
	if basicDO == nil {
		return nil
	}
	return &tool.ToolBasic{
		Name:                   ptr.Of(basicDO.Name),
		Description:            ptr.Of(basicDO.Description),
		LatestCommittedVersion: ptr.Of(basicDO.LatestCommittedVersion),
		CreatedBy:              ptr.Of(basicDO.CreatedBy),
		UpdatedBy:              ptr.Of(basicDO.UpdatedBy),
		CreatedAt:              ptr.Of(basicDO.CreatedAt.UnixMilli()),
		UpdatedAt:              ptr.Of(basicDO.UpdatedAt.UnixMilli()),
	}
}

func CommonToolCommitDO2DTO(commitDO *entity.ToolCommit) *tool.ToolCommit {
	if commitDO == nil {
		return nil
	}
	return &tool.ToolCommit{
		Detail:     CommonToolDetailDO2DTO(commitDO.ToolDetail),
		CommitInfo: ToolCommitInfoDO2DTO(commitDO.CommitInfo),
	}
}

func ToolCommitInfoDO2DTO(info *entity.ToolCommitInfo) *tool.CommitInfo {
	if info == nil {
		return nil
	}
	return &tool.CommitInfo{
		Version:     ptr.Of(info.Version),
		BaseVersion: ptr.Of(info.BaseVersion),
		Description: ptr.Of(info.Description),
		CommittedBy: ptr.Of(info.CommittedBy),
		CommittedAt: ptr.Of(info.CommittedAt.UnixMilli()),
	}
}

func BatchToolCommitInfoDO2DTO(infos []*entity.ToolCommitInfo) []*tool.CommitInfo {
	if len(infos) == 0 {
		return nil
	}
	result := make([]*tool.CommitInfo, 0, len(infos))
	for _, info := range infos {
		dto := ToolCommitInfoDO2DTO(info)
		if dto != nil {
			result = append(result, dto)
		}
	}
	return result
}

func CommonToolDetailDO2DTO(detail *entity.ToolDetail) *tool.ToolDetail {
	if detail == nil {
		return nil
	}
	return &tool.ToolDetail{
		Content: ptr.Of(detail.Content),
	}
}

func CommonToolDetailDTO2DO(detail *tool.ToolDetail) *entity.ToolDetail {
	if detail == nil {
		return nil
	}
	return &entity.ToolDetail{
		Content: detail.GetContent(),
	}
}

func CommonToolDTO2DO(dto *tool.Tool) *entity.CommonTool {
	if dto == nil {
		return nil
	}
	result := &entity.CommonTool{
		ID:      dto.GetID(),
		SpaceID: dto.GetWorkspaceID(),
	}
	if dto.ToolBasic != nil {
		result.ToolBasic = &entity.ToolBasic{
			Name:                   dto.ToolBasic.GetName(),
			Description:            dto.ToolBasic.GetDescription(),
			LatestCommittedVersion: dto.ToolBasic.GetLatestCommittedVersion(),
			CreatedBy:              dto.ToolBasic.GetCreatedBy(),
			UpdatedBy:              dto.ToolBasic.GetUpdatedBy(),
			CreatedAt:              time.UnixMilli(dto.ToolBasic.GetCreatedAt()),
			UpdatedAt:              time.UnixMilli(dto.ToolBasic.GetUpdatedAt()),
		}
	}
	if dto.ToolCommit != nil {
		result.ToolCommit = &entity.ToolCommit{
			ToolDetail: CommonToolDetailDTO2DO(dto.ToolCommit.Detail),
		}
		if dto.ToolCommit.CommitInfo != nil {
			result.ToolCommit.CommitInfo = &entity.ToolCommitInfo{
				Version:     dto.ToolCommit.CommitInfo.GetVersion(),
				BaseVersion: dto.ToolCommit.CommitInfo.GetBaseVersion(),
				Description: dto.ToolCommit.CommitInfo.GetDescription(),
				CommittedBy: dto.ToolCommit.CommitInfo.GetCommittedBy(),
				CommittedAt: time.UnixMilli(dto.ToolCommit.CommitInfo.GetCommittedAt()),
			}
		}
	}
	return result
}
