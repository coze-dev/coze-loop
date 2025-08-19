// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

//go:generate mockgen -destination=mocks/manage_repo.go -package=mocks . IManageRepo
type IManageRepo interface {
	CreatePrompt(ctx context.Context, promptDO *entity.Prompt) (promptID int64, err error)

	DeletePrompt(ctx context.Context, promptID int64) (err error)

	GetPrompt(ctx context.Context, param GetPromptParam) (promptDO *entity.Prompt, err error)
	MGetPrompt(ctx context.Context, queries []GetPromptParam, opts ...GetPromptOptionFunc) (promptDOMap map[GetPromptParam]*entity.Prompt, err error)
	MGetPromptBasicByPromptKey(ctx context.Context, spaceID int64, promptKeys []string, opts ...GetPromptBasicOptionFunc) (promptDOs []*entity.Prompt, err error)
	ListPrompt(ctx context.Context, param ListPromptParam) (result *ListPromptResult, err error)

	UpdatePrompt(ctx context.Context, param UpdatePromptParam) (err error)

	SaveDraft(ctx context.Context, promptDO *entity.Prompt) (draftInfo *entity.DraftInfo, err error)

	CommitDraft(ctx context.Context, param CommitDraftParam) (err error)
	ListCommitInfo(ctx context.Context, param ListCommitInfoParam) (result *ListCommitResult, err error)
}

type GetPromptParam struct {
	PromptID int64

	WithCommit    bool
	CommitVersion string

	WithDraft bool
	UserID    string
}

type ListPromptParam struct {
	SpaceID int64

	KeyWord       string
	CreatedBys    []string
	UserID        string
	CommittedOnly bool

	PageNum  int
	PageSize int
	OrderBy  int
	Asc      bool
}

type ListPromptResult struct {
	Total     int64
	PromptDOs []*entity.Prompt
}

type UpdatePromptParam struct {
	PromptID  int64
	UpdatedBy string

	PromptName        string
	PromptDescription string
}

type CommitDraftParam struct {
	PromptID int64

	UserID string

	CommitVersion     string
	CommitDescription string
}

type ListCommitInfoParam struct {
	PromptID int64

	PageSize  int
	PageToken *int64
	Asc       bool
}

type ListCommitResult struct {
	CommitInfoDOs []*entity.CommitInfo
	NextPageToken int64
}

type CacheOption struct {
	CacheEnable bool
}

type GetPromptBasicOption struct {
	CacheOption
}

type GetPromptBasicOptionFunc func(option *GetPromptBasicOption)

func WithPromptBasicCacheEnable() GetPromptBasicOptionFunc {
	return func(option *GetPromptBasicOption) {
		option.CacheEnable = true
	}
}

type GetPromptOption struct {
	CacheOption
}

type GetPromptOptionFunc func(option *GetPromptOption)

func WithPromptCacheEnable() GetPromptOptionFunc {
	return func(option *GetPromptOption) {
		option.CacheEnable = true
	}
}
