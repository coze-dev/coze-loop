// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

type IToolManageRepo interface {
	CreateTool(ctx context.Context, toolDO *entity.CommonTool) (toolID int64, err error)
	GetTool(ctx context.Context, param GetToolParam) (toolDO *entity.CommonTool, err error)
	MGetTool(ctx context.Context, queries []GetToolParam) (toolDOMap map[GetToolParam]*entity.CommonTool, err error)
	ListTool(ctx context.Context, param ListToolParam) (result *ListToolResult, err error)
	SaveDraft(ctx context.Context, toolDO *entity.CommonTool) error
	CommitDraft(ctx context.Context, param CommitToolDraftParam) error
	ListToolCommitInfo(ctx context.Context, param ListToolCommitParam) (result *ListToolCommitResult, err error)
}

type GetToolParam struct {
	ToolID int64

	WithCommit    bool
	CommitVersion string

	WithDraft bool
}

type ListToolParam struct {
	SpaceID int64

	KeyWord       string
	CreatedBys    []string
	CommittedOnly bool

	PageNum  int
	PageSize int
	OrderBy  int
	Asc      bool
}

type ListToolResult struct {
	Total   int64
	ToolDOs []*entity.CommonTool
}

type CommitToolDraftParam struct {
	ToolID int64

	UserID string

	CommitVersion     string
	CommitDescription string
	BaseVersion       string
}

type ListToolCommitParam struct {
	ToolID int64

	PageSize  int
	PageToken *int64
	Asc       bool
}

type ListToolCommitResult struct {
	CommitInfoDOs []*entity.ToolCommitInfo
	CommitDOs     []*entity.ToolCommit
	NextPageToken int64
}
