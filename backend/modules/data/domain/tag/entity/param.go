// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/bytedance/gg/gslice"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/vdutil"
)

type MGetTagKeyParam struct {
	Paginator       *pagination.Paginator `validate:"required"`
	SpaceID         int64                 `validate:"required,gt=0"`
	IDs             []int64
	TagType         *TagType
	Status          []TagStatus
	TagKeyIDs       []int64
	CreatedBys      []string
	TagDomainTypes  []TagTargetType
	TagContentTypes []TagContentType

	// 二选一
	TagKeyName *string
	// 模糊匹配
	TagKeyNameLike string
}

func (p *MGetTagKeyParam) ToWhere() (*clause.Where, error) {
	if p == nil {
		return nil, errno.DAOParamIsNilError
	}
	if err := vdutil.Validate(p); err != nil {
		return nil, err
	}
	b := db.NewWhereBuilder()
	db.MaybeAddEqToWhere(b, p.SpaceID, "space_id", db.WhereWithIndex)
	db.MaybeAddInToWhere(b, p.IDs, "id")
	db.MaybeAddEqToWhere(b, p.TagType, "tag_type")
	db.MaybeAddInToWhere(b, p.Status, "status")
	db.MaybeAddInToWhere(b, p.TagKeyIDs, "tag_key_id")
	db.MaybeAddInToWhere(b, p.CreatedBys, "created_by")
	db.MaybeAddInToWhere(b, p.TagContentTypes, "content_type")
	db.MaybeAddMultiLikeToWhere(b, gslice.Map(p.TagDomainTypes, func(val TagTargetType) string {
		return string(val)
	}), "tag_target_type")
	db.MaybeAddEqToWhere(b, p.TagKeyName, "tag_key_name")
	db.MaybeAddLikeToWhere(b, p.TagKeyNameLike, "tag_key_name")
	return b.Build()
}

type MGetTagValueParam struct {
	Paginator  *pagination.Paginator `validate:"required"`
	SpaceID    int64                 `validate:"required,gt=0"`
	IDs        []int64
	Status     *TagStatus
	TagKeyID   *int64
	Version    *int32
	TagValueID []int64
}

func (p *MGetTagValueParam) ToWhere() (*clause.Where, error) {
	if p == nil {
		return nil, errno.DAOParamIsNilError
	}
	if err := vdutil.Validate(p); err != nil {
		return nil, err
	}
	if len(p.IDs) == 0 && p.TagKeyID == nil && len(p.TagValueID) == 0 {
		return nil, errno.DAOParamIsIllegalError
	}
	b := db.NewWhereBuilder()
	db.MaybeAddEqToWhere(b, p.SpaceID, "space_id", db.WhereWithIndex)
	db.MaybeAddInToWhere(b, p.IDs, "id")
	db.MaybeAddEqToWhere(b, p.Status, "status")
	db.MaybeAddEqToWhere(b, p.TagKeyID, "tag_key_id")
	db.MaybeAddInToWhere(b, p.TagValueID, "tag_value_id")
	db.MaybeAddEqToWhere(b, p.Version, "version_num")
	return b.Build()
}

type GetTagDetailReq struct {
	PageSize  int32
	PageNum   int32
	PageToken string
	TagKeyID  int64
	OrderBy   string
	IsAsc     bool
}

type GetTagDetailResp struct {
	TagKeys       []*TagKey
	Total         int64
	NextPageToken string
}
