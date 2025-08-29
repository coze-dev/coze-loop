// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

//go:generate mockgen -destination=mocks/label_repo.go -package=mocks . ILabelRepo
type ILabelRepo interface {
	CreateLabel(ctx context.Context, labelDO *entity.PromptLabel) error
	ListLabel(ctx context.Context, param ListLabelParam) ([]*entity.PromptLabel, *int64, error)
	BatchGetLabel(ctx context.Context, spaceID int64, labelKeys []string) (labelDOs []*entity.PromptLabel, err error)

	// Prompt Commit Label operations - now directly on commits
	UpdateCommitLabels(ctx context.Context, param UpdateCommitLabelsParam) error
	GetCommitLabels(ctx context.Context, promptID int64, commitVersions []string) (map[string][]*entity.PromptLabel, error)
	BatchGetPromptVersionByLabel(ctx context.Context, queries []PromptLabelQuery, opts ...GetLabelMappingOptionFunc) (map[PromptLabelQuery]string, error)
}

type ListLabelParam struct {
	SpaceID      int64
	LabelKeyLike string
	PageSize     int
	PageToken    *int64
}

type DeleteCommitLabelMappingParam struct {
	SpaceID   int64
	PromptID  int64
	LabelKeys []string
}

type UpdateCommitLabelsParam struct {
	SpaceID       int64
	PromptID      int64
	PromptKey     string
	LabelKeys     []string
	CommitVersion string
	UpdatedBy     string
}

type GetLabelMappingOption struct {
	CacheEnable bool
}

type GetLabelMappingOptionFunc func(option *GetLabelMappingOption)

func WithLabelMappingCacheEnable() GetLabelMappingOptionFunc {
	return func(option *GetLabelMappingOption) {
		option.CacheEnable = true
	}
}

type PromptLabelQuery struct {
	PromptID int64
	LabelKey string
}
