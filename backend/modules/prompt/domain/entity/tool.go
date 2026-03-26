// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// CommonTool 公共函数（Tool）实体，与 Prompt 中的 Tool（tool_call 配置）不同
type CommonTool struct {
	ID         int64            `json:"id"`
	SpaceID    int64            `json:"space_id"`
	ToolBasic  *CommonToolBasic `json:"tool_basic,omitempty"`
	ToolCommit *CommonToolCommit `json:"tool_commit,omitempty"`
}

type CommonToolBasic struct {
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	LatestCommittedVersion string    `json:"latest_committed_version"`
	CreatedAt              time.Time `json:"created_at"`
	CreatedBy              string    `json:"created_by"`
	UpdatedAt              time.Time `json:"updated_at"`
	UpdatedBy              string    `json:"updated_by"`
}

type CommonToolCommit struct {
	ToolDetail *CommonToolDetail     `json:"tool_detail,omitempty"`
	CommitInfo *CommonToolCommitInfo `json:"commit_info,omitempty"`
}

type CommonToolCommitInfo struct {
	Version     string    `json:"version"`
	BaseVersion string    `json:"base_version"`
	Description string    `json:"description"`
	CommittedBy string    `json:"committed_by"`
	CommittedAt time.Time `json:"committed_at"`
}

const (
	ToolPublicDraftVersion = "$PublicDraft"
)

func (v CommonToolCommitInfo) IsPublicDraft() bool {
	return v.Version == ToolPublicDraftVersion
}

type CommonToolDetail struct {
	Content string `json:"content"`
}
