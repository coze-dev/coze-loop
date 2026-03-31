// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ToolPO2DO(basicPO *model.ToolBasic, commitPO *model.ToolCommit) *toolmgmt.Tool {
	if basicPO == nil {
		return nil
	}
	do := &toolmgmt.Tool{
		ID:      basicPO.ID,
		SpaceID: basicPO.SpaceID,
		ToolBasic: &toolmgmt.ToolBasic{
			Name:                   basicPO.Name,
			Description:            basicPO.Description,
			LatestCommittedVersion: basicPO.LatestCommittedVersion,
			CreatedAt:              basicPO.CreatedAt,
			CreatedBy:              basicPO.CreatedBy,
			UpdatedAt:              basicPO.UpdatedAt,
			UpdatedBy:              basicPO.UpdatedBy,
		},
	}
	if commitPO != nil {
		do.ToolCommit = ToolCommitPO2DO(commitPO)
	}
	return do
}

func ToolCommitPO2DO(commitPO *model.ToolCommit) *toolmgmt.ToolCommit {
	if commitPO == nil {
		return nil
	}
	return &toolmgmt.ToolCommit{
		CommitInfo: &toolmgmt.CommitInfo{
			Version:     commitPO.Version,
			BaseVersion: commitPO.BaseVersion,
			Description: ptr.From(commitPO.Description),
			CommittedBy: commitPO.CommittedBy,
			CommittedAt: commitPO.CreatedAt,
		},
		ToolDetail: &toolmgmt.ToolDetail{
			Content: ptr.From(commitPO.Content),
		},
	}
}

func ToolDO2BasicPO(do *toolmgmt.Tool) *model.ToolBasic {
	if do == nil || do.ToolBasic == nil {
		return nil
	}
	return &model.ToolBasic{
		ID:                     do.ID,
		SpaceID:                do.SpaceID,
		Name:                   do.ToolBasic.Name,
		Description:            do.ToolBasic.Description,
		LatestCommittedVersion: do.ToolBasic.LatestCommittedVersion,
		CreatedBy:              do.ToolBasic.CreatedBy,
		UpdatedBy:              do.ToolBasic.UpdatedBy,
		CreatedAt:              do.ToolBasic.CreatedAt,
		UpdatedAt:              do.ToolBasic.UpdatedAt,
	}
}
