// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ToolPO2DO(basicPO *model.ToolBasic, commitPO *model.ToolCommit) *entity.CommonTool {
	if basicPO == nil {
		return nil
	}
	return &entity.CommonTool{
		ID:         basicPO.ID,
		SpaceID:    basicPO.SpaceID,
		ToolBasic:  ToolBasicPO2DO(basicPO),
		ToolCommit: ToolCommitPO2DO(commitPO),
	}
}

func ToolBasicPO2DO(basicPO *model.ToolBasic) *entity.ToolBasic {
	if basicPO == nil {
		return nil
	}
	latestVersion := ""
	if basicPO.LatestCommittedVersion != nil {
		latestVersion = *basicPO.LatestCommittedVersion
	}
	return &entity.ToolBasic{
		Name:                   basicPO.Name,
		Description:            basicPO.Description,
		LatestCommittedVersion: latestVersion,
		CreatedBy:              basicPO.CreatedBy,
		UpdatedBy:              basicPO.UpdatedBy,
		CreatedAt:              basicPO.CreatedAt,
		UpdatedAt:              basicPO.UpdatedAt,
	}
}

func ToolCommitPO2DO(commitPO *model.ToolCommit) *entity.ToolCommit {
	if commitPO == nil {
		return nil
	}
	return &entity.ToolCommit{
		ToolDetail: &entity.ToolDetail{
			Content: ptr.From(commitPO.Content),
		},
		CommitInfo: &entity.ToolCommitInfo{
			Version:     commitPO.Version,
			BaseVersion: commitPO.BaseVersion,
			Description: ptr.From(commitPO.Description),
			CommittedBy: commitPO.CommittedBy,
			CommittedAt: commitPO.CreatedAt,
		},
	}
}

func BatchToolBasicPO2DO(basicPOs []*model.ToolBasic) []*entity.CommonTool {
	if len(basicPOs) == 0 {
		return nil
	}
	toolDOs := make([]*entity.CommonTool, 0, len(basicPOs))
	for _, basicPO := range basicPOs {
		toolDO := ToolPO2DO(basicPO, nil)
		if toolDO == nil {
			continue
		}
		toolDOs = append(toolDOs, toolDO)
	}
	return toolDOs
}

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

func ToolDO2CommitPO(toolDO *entity.CommonTool, toolID int64) *model.ToolCommit {
	if toolDO == nil || toolDO.ToolCommit == nil {
		return nil
	}
	po := &model.ToolCommit{
		SpaceID: toolDO.SpaceID,
		ToolID:  toolID,
	}
	if toolDO.ToolCommit.ToolDetail != nil {
		po.Content = ptr.Of(toolDO.ToolCommit.ToolDetail.Content)
	}
	if toolDO.ToolCommit.CommitInfo != nil {
		po.Version = toolDO.ToolCommit.CommitInfo.Version
		po.BaseVersion = toolDO.ToolCommit.CommitInfo.BaseVersion
		po.Description = ptr.Of(toolDO.ToolCommit.CommitInfo.Description)
		po.CommittedBy = toolDO.ToolCommit.CommitInfo.CommittedBy
	}
	return po
}
