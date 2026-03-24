package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
)

type IToolRepo interface {
	CreateTool(ctx context.Context, tool *toolmgmt.Tool) (toolID int64, err error)
	DeleteTool(ctx context.Context, toolID int64) (err error)
	GetTool(ctx context.Context, param GetToolParam) (tool *toolmgmt.Tool, err error)
	ListTool(ctx context.Context, param ListToolParam) (result *ListToolResult, err error)
	SaveToolDetail(ctx context.Context, param SaveToolDetailParam) (err error)
	CommitToolDraft(ctx context.Context, param CommitToolDraftParam) (err error)
	ListToolCommit(ctx context.Context, param ListToolCommitParam) (result *ListToolCommitResult, err error)
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
	Total int64
	Tools []*toolmgmt.Tool
}

type SaveToolDetailParam struct {
	ToolID int64

	BaseVersion string
	Content     string
	UpdatedBy   string
}

type CommitToolDraftParam struct {
	ToolID int64

	CommitVersion     string
	CommitDescription string
	BaseVersion       string
	CommittedBy       string
}

type ListToolCommitParam struct {
	ToolID int64

	PageSize  int
	PageToken *int64
	Asc       bool

	WithCommitDetail bool
}

type ListToolCommitResult struct {
	CommitInfos   []*toolmgmt.CommitInfo
	CommitDetails map[string]*toolmgmt.ToolDetail
	NextPageToken int64
}
