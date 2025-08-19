// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
)

type TagValue struct {
	ID            int64
	AppID         int32
	SpaceID       int64
	TagKeyID      int64
	TagValueID    int64
	TagValueName  string
	Description   *string
	Status        TagStatus
	VersionNum    *int32
	ParentValueID int64
	Children      []*TagValue
	IsSystem      bool
	CreatedBy     *string
	CreatedAt     time.Time
	UpdatedBy     *string
	UpdatedAt     time.Time
}

func (t *TagValue) ToPO() *model.TagValue {
	if t == nil {
		return nil
	}
	return &model.TagValue{
		ID:            t.ID,
		AppID:         t.AppID,
		SpaceID:       t.SpaceID,
		TagKeyID:      t.TagKeyID,
		TagValueID:    t.TagValueID,
		TagValueName:  t.TagValueName,
		Description:   t.Description,
		Status:        string(t.Status),
		VersionNum:    t.VersionNum,
		ParentValueID: t.ParentValueID,
		CreatedBy:     t.CreatedBy,
		CreatedAt:     t.CreatedAt,
		UpdatedBy:     t.UpdatedBy,
		UpdatedAt:     t.UpdatedAt,
	}
}

func (t *TagValue) ToDTO() *tag.TagValue {
	if t == nil {
		return nil
	}
	return &tag.TagValue{
		ID:               gptr.Of(t.ID),
		AppID:            gptr.Of(t.AppID),
		WorkspaceID:      gptr.Of(t.SpaceID),
		TagKeyID:         gptr.Of(t.TagKeyID),
		TagValueID:       gptr.Of(t.TagValueID),
		TagValueName:     gptr.Of(t.TagValueName),
		Description:      t.Description,
		Status:           gptr.Of(t.Status.ToDTO()),
		VersionNum:       t.VersionNum,
		ParentTagValueID: gptr.Of(t.ParentValueID),
		Children:         gslice.Map(t.Children, (*TagValue).ToDTO),
		IsSystem:         gptr.Of(t.IsSystem),
		BaseInfo: &common.BaseInfo{
			CreatedBy: &common.UserInfo{UserID: t.CreatedBy},
			UpdatedBy: &common.UserInfo{UserID: t.UpdatedBy},
			CreatedAt: gptr.Of(t.CreatedAt.UnixMilli()),
			UpdatedAt: gptr.Of(t.UpdatedAt.UnixMilli()),
		},
	}
}

func (t *TagValue) SetVersionNum(versionNum int32) {
	if t == nil {
		return
	}
	t.VersionNum = gptr.Of(versionNum)
}

func (t *TagValue) SetSpaceID(spaceID int64) {
	if t == nil {
		return
	}
	t.SpaceID = spaceID
}

func (t *TagValue) SetAppID(appID int32) {
	if t == nil {
		return
	}
	t.AppID = appID
}

func (t *TagValue) SetCreatedBy(createdBy string) {
	if t == nil {
		return
	}
	t.CreatedBy = gptr.Of(createdBy)
}

func (t *TagValue) SetUpdatedBy(updatedBy string) {
	if t == nil {
		return
	}
	t.UpdatedBy = gptr.Of(updatedBy)
}

func (t *TagValue) SetCreatedAt(tt time.Time) {
	if t == nil {
		return
	}
	t.CreatedAt = tt
}

func (t *TagValue) SetUpdatedAt(tt time.Time) {
	if t == nil {
		return
	}
	t.UpdatedAt = tt
}

func (t *TagValue) SetStatus(val TagStatus) {
	if t == nil {
		return
	}
	t.Status = val
}

func (t *TagValue) Validate() error {
	if t == nil {
		return nil
	}
	runeCount := utf8.RuneCountInString(t.TagValueName)
	if runeCount > 50 {
		return errno.InvalidParamErrorf("length of tag value name is more than 50, tagValueName: %v", t.TagValueName)
	}
	return nil
}

func NewTagValueFromDTO(val *tag.TagValue, opts ...func(value *TagValue)) *TagValue {
	if val == nil {
		return nil
	}
	res := &TagValue{
		ID:            val.GetID(),
		AppID:         val.GetAppID(),
		SpaceID:       val.GetWorkspaceID(),
		VersionNum:    val.VersionNum,
		TagKeyID:      val.GetTagKeyID(),
		TagValueID:    val.GetTagValueID(),
		TagValueName:  val.GetTagValueName(),
		Description:   val.Description,
		Status:        NewTagStatusFromDTO(val.Status),
		ParentValueID: val.GetParentTagValueID(),
	}
	if val.GetBaseInfo() != nil {
		res.UpdatedAt = time.UnixMilli(val.GetBaseInfo().GetUpdatedAt())
		res.CreatedAt = time.UnixMilli(val.GetBaseInfo().GetCreatedAt())
		if val.GetBaseInfo().GetCreatedBy() != nil {
			res.CreatedBy = val.GetBaseInfo().GetCreatedBy().UserID
		}
		if val.GetBaseInfo().GetUpdatedBy() != nil {
			res.UpdatedBy = val.GetBaseInfo().GetUpdatedBy().UserID
		}
	}
	var children []*TagValue
	for _, v := range val.Children {
		item := v
		children = append(children, NewTagValueFromDTO(item, opts...))
	}
	res.Children = children
	for _, opt := range opts {
		if opt != nil {
			opt(res)
		}
	}
	return res
}
