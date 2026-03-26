// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

// CommonToolDO2DTO 将 Domain Object 转换为 IDL DTO
func CommonToolDO2DTO(do *entity.CommonTool) *tool.Tool {
	if do == nil {
		return nil
	}
	return &tool.Tool{
		ID:          ptr.Of(do.ID),
		WorkspaceID: ptr.Of(do.SpaceID),
		ToolBasic:   ToolBasicDO2DTO(do.ToolBasic),
		ToolCommit:  ToolCommitDO2DTO(do.ToolCommit),
	}
}

// BatchCommonToolDO2DTO 批量转换
func BatchCommonToolDO2DTO(dos []*entity.CommonTool) []*tool.Tool {
	if len(dos) == 0 {
		return nil
	}
	tools := make([]*tool.Tool, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		tools = append(tools, CommonToolDO2DTO(do))
	}
	return tools
}

// ToolBasicDO2DTO 转换 ToolBasic
func ToolBasicDO2DTO(do *entity.CommonToolBasic) *tool.ToolBasic {
	if do == nil {
		return nil
	}
	return &tool.ToolBasic{
		Name:                   ptr.Of(do.Name),
		Description:            ptr.Of(do.Description),
		LatestCommittedVersion: ptr.Of(do.LatestCommittedVersion),
		CreatedBy:              ptr.Of(do.CreatedBy),
		UpdatedBy:              ptr.Of(do.UpdatedBy),
		CreatedAt:              ptr.Of(do.CreatedAt.UnixMilli()),
		UpdatedAt:              ptr.Of(do.UpdatedAt.UnixMilli()),
	}
}

// ToolCommitDO2DTO 转换 ToolCommit
func ToolCommitDO2DTO(do *entity.CommonToolCommit) *tool.ToolCommit {
	if do == nil {
		return nil
	}
	return &tool.ToolCommit{
		Detail:     ToolDetailDO2DTO(do.ToolDetail),
		CommitInfo: CommitInfoDO2ToolDTO(do.CommitInfo),
	}
}

// ToolDetailDO2DTO 转换 ToolDetail
func ToolDetailDO2DTO(do *entity.CommonToolDetail) *tool.ToolDetail {
	if do == nil {
		return nil
	}
	return &tool.ToolDetail{
		Content: ptr.Of(do.Content),
	}
}

// ToolDetailDTO2DO 转换 ToolDetail DTO 到 DO
func ToolDetailDTO2DO(dto *tool.ToolDetail) *entity.CommonToolDetail {
	if dto == nil {
		return nil
	}
	return &entity.CommonToolDetail{
		Content: dto.GetContent(),
	}
}

// CommitInfoDO2ToolDTO 转换 CommitInfo
func CommitInfoDO2ToolDTO(do *entity.CommonToolCommitInfo) *tool.CommitInfo {
	if do == nil {
		return nil
	}
	return &tool.CommitInfo{
		Version:     ptr.Of(do.Version),
		BaseVersion: ptr.Of(do.BaseVersion),
		Description: ptr.Of(do.Description),
		CommittedBy: ptr.Of(do.CommittedBy),
		CommittedAt: ptr.Of(do.CommittedAt.UnixMilli()),
	}
}

// BatchCommitInfoDO2ToolDTO 批量转换 CommitInfo
func BatchCommitInfoDO2ToolDTO(dos []*entity.CommonToolCommitInfo) []*tool.CommitInfo {
	if len(dos) == 0 {
		return nil
	}
	infos := make([]*tool.CommitInfo, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		infos = append(infos, CommitInfoDO2ToolDTO(do))
	}
	return infos
}

// ToolDetailDOMap2DTOMap 转换 detail mapping
func ToolDetailDOMap2DTOMap(doMap map[string]*entity.CommonToolDetail) map[string]*tool.ToolDetail {
	if len(doMap) == 0 {
		return nil
	}
	result := make(map[string]*tool.ToolDetail, len(doMap))
	for k, v := range doMap {
		result[k] = ToolDetailDO2DTO(v)
	}
	return result
}

