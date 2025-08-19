// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	entity2 "github.com/coze-dev/coze-loop/backend/modules/data/domain/tag/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
)

//go:generate mockgen -destination=mocks/tag_mock.go -package=mocks . ITagAPI
type ITagAPI interface {
	ITagKeyRepo
	ITagValueRepo
}

type ITagKeyRepo interface {
	MCreateTagKeys(ctx context.Context, val []*entity2.TagKey, opt ...db.Option) error
	GetTagKey(ctx context.Context, spaceID, id int64, opts ...db.Option) (*entity2.TagKey, error)
	MGetTagKeys(ctx context.Context, param *entity2.MGetTagKeyParam, opts ...db.Option) ([]*entity2.TagKey, *pagination.PageResult, error)
	PatchTagKey(ctx context.Context, spaceID, id int64, patch *entity2.TagKey, opts ...db.Option) error
	DeleteTagKey(ctx context.Context, spaceID, id int64, opts ...db.Option) error
	UpdateTagKeysStatus(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, toStatus entity2.TagStatus, updateInfo bool, opts ...db.Option) error
	CountTagKeys(ctx context.Context, param *entity2.MGetTagKeyParam, opts ...db.Option) (int64, error)
}

type ITagValueRepo interface {
	MCreateTagValues(ctx context.Context, val []*entity2.TagValue, opts ...db.Option) error
	GetTagValue(ctx context.Context, spaceID, id int64, opts ...db.Option) (*entity2.TagValue, error)
	MGetTagValue(ctx context.Context, param *entity2.MGetTagValueParam, opts ...db.Option) ([]*entity2.TagValue, *pagination.PageResult, error)
	PatchTagValue(ctx context.Context, spaceID, id int64, patch *entity2.TagValue, opts ...db.Option) error
	DeleteTagValue(ctx context.Context, spaceID, id int64, opts ...db.Option) error
	UpdateTagValuesStatus(ctx context.Context, spaceID, tagKeyID int64, versionNum int32, toStatus entity2.TagStatus, updateInfo bool, opts ...db.Option) error
}
