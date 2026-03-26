// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

//go:generate mockgen -destination=mocks/tool_repo.go -package=mocks . IToolRepo

type IToolRepo interface {
	CreateTool(ctx context.Context, toolDO *entity.CommonTool) (toolID int64, err error)
	GetTool(ctx context.Context, param GetToolParam) (*entity.CommonTool, error)
	MGetTool(ctx context.Context, queries []MGetToolQuery) (map[MGetToolQuery]*entity.CommonTool, error)
	ListTool(ctx context.Context, param ListToolParam) (*ListToolResult, error)
	SaveDraft(ctx context.Context, toolDO *entity.CommonTool) error
	CommitDraft(ctx context.Context, param CommitToolDraftParam) error
	ListToolCommitInfo(ctx context.Context, param ListToolCommitParam) (*ListToolCommitResult, error)
}

type GetToolParam struct {
	ToolID        int64
	SpaceID       int64
	WithCommit    bool
	CommitVersion string
	WithDraft     bool
}

type MGetToolQuery struct {
	ToolID  int64
	Version string
}

type ListToolParam struct {
	SpaceID       int64
	KeyWord       string
	CreatedBys    []string
	CommittedOnly bool
	PageNum       int
	PageSize      int
	OrderBy       ListToolOrderBy
	Asc           bool
}

type ListToolOrderBy int

const (
	ListToolOrderByCreatedAt   ListToolOrderBy = 0
	ListToolOrderByCommittedAt ListToolOrderBy = 1
)

type ListToolResult struct {
	Total   int64
	ToolDOs []*entity.CommonTool
}

type CommitToolDraftParam struct {
	ToolID            int64
	SpaceID           int64
	CommitVersion     string
	CommitDescription string
	BaseVersion       string
	CommittedBy       string
}

type ListToolCommitParam struct {
	ToolID           int64
	WithCommitDetail bool
	PageSize         int
	PageToken        *int64
	Asc              bool
}

type ListToolCommitResult struct {
	CommitInfoDOs          []*entity.CommonToolCommitInfo
	CommitDetailMapping    map[string]*entity.CommonToolDetail
	NextPageToken          int64
	HasMore                bool
}
