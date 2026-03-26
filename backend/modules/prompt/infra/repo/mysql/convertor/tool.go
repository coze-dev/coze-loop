// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ToolPO2DO 从 BasicPO 和可选的 CommitPO 组装 Domain Object
func ToolPO2DO(basicPO *model.ToolBasic, commitPO *model.ToolCommit) *entity.CommonTool {
	if basicPO == nil {
		return nil
	}

	toolDO := &entity.CommonTool{
		ID:      basicPO.ID,
		SpaceID: basicPO.SpaceID,
		ToolBasic: &entity.CommonToolBasic{
			Name:                   basicPO.Name,
			Description:            basicPO.Description,
			LatestCommittedVersion: derefStr(basicPO.LatestCommittedVersion),
			CreatedAt:              basicPO.CreatedAt,
			CreatedBy:              basicPO.CreatedBy,
			UpdatedAt:              basicPO.UpdatedAt,
			UpdatedBy:              basicPO.UpdatedBy,
		},
	}

	if commitPO != nil {
		toolDO.ToolCommit = CommitPO2ToolCommitDO(commitPO)
	}

	return toolDO
}

// CommitPO2ToolCommitDO 将 CommitPO 转换为 ToolCommit Domain Object
func CommitPO2ToolCommitDO(commitPO *model.ToolCommit) *entity.CommonToolCommit {
	if commitPO == nil {
		return nil
	}
	return &entity.CommonToolCommit{
		ToolDetail: &entity.CommonToolDetail{
			Content: derefStr(commitPO.Content),
		},
		CommitInfo: &entity.CommonToolCommitInfo{
			Version:     commitPO.Version,
			BaseVersion: commitPO.BaseVersion,
			Description: derefStr(commitPO.Description),
			CommittedBy: commitPO.CommittedBy,
			CommittedAt: commitPO.CreatedAt,
		},
	}
}

// ToolDO2BasicPO 将 Domain Object 转换为 BasicPO
func ToolDO2BasicPO(toolDO *entity.CommonTool) *model.ToolBasic {
	if toolDO == nil || toolDO.ToolBasic == nil {
		return nil
	}
	return &model.ToolBasic{
		ID:                     toolDO.ID,
		SpaceID:                toolDO.SpaceID,
		Name:                   toolDO.ToolBasic.Name,
		Description:            toolDO.ToolBasic.Description,
		LatestCommittedVersion: ptr.Of(toolDO.ToolBasic.LatestCommittedVersion),
		CreatedBy:              toolDO.ToolBasic.CreatedBy,
		UpdatedBy:              toolDO.ToolBasic.UpdatedBy,
	}
}

// ToolDO2CommitPO 将 Domain Object 转换为 CommitPO
func ToolDO2CommitPO(toolDO *entity.CommonTool, toolID int64, spaceID int64) *model.ToolCommit {
	if toolDO == nil || toolDO.ToolCommit == nil {
		return nil
	}

	commitPO := &model.ToolCommit{
		SpaceID: spaceID,
		ToolID:  toolID,
	}

	if toolDO.ToolCommit.ToolDetail != nil {
		commitPO.Content = ptr.Of(toolDO.ToolCommit.ToolDetail.Content)
	}

	if toolDO.ToolCommit.CommitInfo != nil {
		commitPO.Version = toolDO.ToolCommit.CommitInfo.Version
		commitPO.BaseVersion = toolDO.ToolCommit.CommitInfo.BaseVersion
		commitPO.Description = ptr.Of(toolDO.ToolCommit.CommitInfo.Description)
		commitPO.CommittedBy = toolDO.ToolCommit.CommitInfo.CommittedBy
	}

	return commitPO
}

// BatchBasicPO2ToolDO 批量转换 BasicPO 到 Domain Object
func BatchBasicPO2ToolDO(basicPOs []*model.ToolBasic) []*entity.CommonTool {
	if len(basicPOs) == 0 {
		return nil
	}
	tools := make([]*entity.CommonTool, 0, len(basicPOs))
	for _, po := range basicPOs {
		if po == nil {
			continue
		}
		tools = append(tools, ToolPO2DO(po, nil))
	}
	return tools
}

// CommitInfoDOFromCommitPO 从 CommitPO 提取 CommitInfo
func CommitInfoDOFromCommitPO(commitPO *model.ToolCommit) *entity.CommonToolCommitInfo {
	if commitPO == nil {
		return nil
	}
	return &entity.CommonToolCommitInfo{
		Version:     commitPO.Version,
		BaseVersion: commitPO.BaseVersion,
		Description: derefStr(commitPO.Description),
		CommittedBy: commitPO.CommittedBy,
		CommittedAt: commitPO.CreatedAt,
	}
}

// BatchCommitInfoDOFromCommitPO 批量从 CommitPO 提取 CommitInfo
func BatchCommitInfoDOFromCommitPO(commitPOs []*model.ToolCommit) []*entity.CommonToolCommitInfo {
	if len(commitPOs) == 0 {
		return nil
	}
	infos := make([]*entity.CommonToolCommitInfo, 0, len(commitPOs))
	for _, po := range commitPOs {
		if po == nil {
			continue
		}
		infos = append(infos, CommitInfoDOFromCommitPO(po))
	}
	return infos
}

// ToolDetailDOFromCommitPO 从 CommitPO 提取 ToolDetail
func ToolDetailDOFromCommitPO(commitPO *model.ToolCommit) *entity.CommonToolDetail {
	if commitPO == nil {
		return nil
	}
	return &entity.CommonToolDetail{
		Content: derefStr(commitPO.Content),
	}
}
