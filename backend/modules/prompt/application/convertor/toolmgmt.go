package convertor

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ToolMgmtDO2DTO(do *toolmgmt.Tool) *tool.Tool {
	if do == nil {
		return nil
	}
	return &tool.Tool{
		ID:          ptr.Of(do.ID),
		WorkspaceID: ptr.Of(do.SpaceID),
		ToolBasic:   ToolMgmtBasicDO2DTO(do.ToolBasic),
		ToolCommit:  ToolMgmtCommitDO2DTO(do.ToolCommit),
	}
}

func ToolMgmtBasicDO2DTO(do *toolmgmt.ToolBasic) *tool.ToolBasic {
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

func ToolMgmtCommitDO2DTO(do *toolmgmt.ToolCommit) *tool.ToolCommit {
	if do == nil {
		return nil
	}
	return &tool.ToolCommit{
		CommitInfo: ToolMgmtCommitInfoDO2DTO(do.CommitInfo),
		Detail:     ToolMgmtDetailDO2DTO(do.ToolDetail),
	}
}

func ToolMgmtCommitInfoDO2DTO(do *toolmgmt.CommitInfo) *tool.CommitInfo {
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

func ToolMgmtDetailDO2DTO(do *toolmgmt.ToolDetail) *tool.ToolDetail {
	if do == nil {
		return nil
	}
	return &tool.ToolDetail{
		Content: ptr.Of(do.Content),
	}
}

func BatchToolMgmtDO2DTO(dos []*toolmgmt.Tool) []*tool.Tool {
	if len(dos) == 0 {
		return nil
	}
	dtos := make([]*tool.Tool, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		dtos = append(dtos, ToolMgmtDO2DTO(do))
	}
	if len(dtos) == 0 {
		return nil
	}
	return dtos
}
