package toolmgmt

import "time"

const PublicDraftVersion = "$PublicDraft"

type Tool struct {
	ID         int64       `json:"id"`
	SpaceID    int64       `json:"space_id"`
	ToolBasic  *ToolBasic  `json:"tool_basic,omitempty"`
	ToolCommit *ToolCommit `json:"tool_commit,omitempty"`
}

type ToolBasic struct {
	Name                   string    `json:"name"`
	Description            string    `json:"description"`
	LatestCommittedVersion string    `json:"latest_committed_version"`
	CreatedAt              time.Time `json:"created_at"`
	CreatedBy              string    `json:"created_by"`
	UpdatedAt              time.Time `json:"updated_at"`
	UpdatedBy              string    `json:"updated_by"`
}

type ToolCommit struct {
	ToolDetail *ToolDetail `json:"tool_detail,omitempty"`
	CommitInfo *CommitInfo `json:"commit_info,omitempty"`
}

type CommitInfo struct {
	Version     string    `json:"version"`
	BaseVersion string    `json:"base_version"`
	Description string    `json:"description"`
	CommittedBy string    `json:"committed_by"`
	CommittedAt time.Time `json:"committed_at"`
}

func (v CommitInfo) IsPublicDraft() bool {
	return v.Version == PublicDraftVersion
}

type ToolDetail struct {
	Content string `json:"content"`
}
