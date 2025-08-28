// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"
	"github.com/bytedance/gg/gvalue"
	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/tag/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ptrutil"
)

type TagKey struct {
	ID             int64           `json:"id,omitempty"`
	AppID          int32           `json:"app_id,omitempty"`
	SpaceID        int64           `json:"space_id,omitempty"`
	Version        *string         `json:"version,omitempty"`
	VersionNum     *int32          `json:"version_num,omitempty"`
	TagKeyID       int64           `json:"tag_key_id,omitempty"`
	TagKeyName     string          `json:"tag_key_name,omitempty"`
	Description    *string         `json:"description,omitempty"`
	Status         TagStatus       `json:"status,omitempty"`
	TagType        TagType         `json:"tag_type,omitempty"`
	TagTargetType  []TagTargetType `json:"tag_target_type,omitempty"`
	ParentKeyID    *int64          `json:"parent_key_id,omitempty"`
	TagValues      []*TagValue     `json:"tag_values,omitempty"`
	ChangeLogs     []*ChangeLog    `json:"change_logs,omitempty"`
	CreatedBy      *string         `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at,omitempty"`
	UpdatedBy      *string         `json:"updated_by,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at,omitempty"`
	TagContentType TagContentType  `json:"tag_content_type,omitempty"`
	ContentSpec    *TagContentSpec `json:"tag_content_spec,omitempty"`
}

// RetainTagKeyID 仅仅只保留TagKeyID信息
func (t *TagKey) RetainTagKeyID() {
	if t == nil {
		return
	}
	t.ID = 0
	t.AppID = 0
	t.SpaceID = 0
	t.Version = nil
	t.VersionNum = nil
	t.TagKeyName = ""
	t.Description = nil
	t.Status = ""
	t.TagType = ""
	t.TagTargetType = nil
	t.ParentKeyID = nil
	t.TagValues = nil
	t.ChangeLogs = nil
	t.CreatedBy = nil
	t.UpdatedBy = nil
	t.TagContentType = ""
	t.ContentSpec = nil
}

func (t *TagKey) ToPO() (*model.TagKey, error) {
	if t == nil {
		return nil, nil
	}
	res := &model.TagKey{
		ID:          t.ID,
		AppID:       t.AppID,
		SpaceID:     t.SpaceID,
		Version:     ptrutil.GetOrDefault(t.Version, ""),
		VersionNum:  t.VersionNum,
		TagKeyID:    t.TagKeyID,
		TagKeyName:  t.TagKeyName,
		Description: t.Description,
		Status:      string(t.Status),
		TagType:     string(t.TagType),
		TagTargetType: strings.Join(gslice.Map(t.TagTargetType, func(val TagTargetType) string {
			return string(val)
		}), ","),
		ParentKeyID: t.ParentKeyID,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedBy:   t.UpdatedBy,
		UpdatedAt:   t.UpdatedAt,
		ContentType: gptr.Of(string(t.TagContentType)),
	}
	buf, err := sonic.Marshal(t.ChangeLogs)
	if err != nil {
		return nil, err
	}
	res.ChangeLog = buf
	buf1, err := sonic.Marshal(t.ContentSpec)
	if err != nil {
		return nil, err
	}
	res.Spec = buf1
	return res, nil
}

func (t *TagKey) ToTagInfoDTO() *tag.TagInfo {
	if t == nil {
		return nil
	}
	return &tag.TagInfo{
		ID:             gptr.Of(t.ID),
		AppID:          gptr.Of(t.AppID),
		WorkspaceID:    gptr.Of(t.SpaceID),
		Version:        t.Version,
		VersionNum:     t.VersionNum,
		TagKeyID:       gptr.Of(t.TagKeyID),
		TagKeyName:     gptr.Of(t.TagKeyName),
		Description:    t.Description,
		Status:         gptr.Of(t.Status.ToDTO()),
		TagType:        gptr.Of(t.TagType.ToDTO()),
		ParentTagKeyID: t.ParentKeyID,
		TagValues:      gslice.Map(t.TagValues, (*TagValue).ToDTO),
		ChangeLogs:     gslice.Map(t.ChangeLogs, (*ChangeLog).ToDTO),
		DomainTypeList: gslice.Map(t.TagTargetType, TagTargetType.ToDTO),
		ContentType:    gptr.Of(t.TagContentType.ToDTO()),
		ContentSpec:    t.ContentSpec.ToDTO(),
		BaseInfo: &common.BaseInfo{
			CreatedBy: &common.UserInfo{UserID: t.CreatedBy},
			UpdatedBy: &common.UserInfo{UserID: t.UpdatedBy},
			CreatedAt: gptr.Of(t.CreatedAt.UnixMilli()),
			UpdatedAt: gptr.Of(t.UpdatedAt.UnixMilli()),
		},
	}
}

func (t *TagKey) SetVersionNum(versionNum int32) {
	if t == nil {
		return
	}
	t.VersionNum = gptr.Of(versionNum)
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetVersionNum(versionNum)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetSpaceID(spaceID int64) {
	if t == nil {
		return
	}
	t.SpaceID = spaceID
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetSpaceID(spaceID)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetAppID(appID int32) {
	if t == nil {
		return
	}
	t.AppID = appID
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetAppID(appID)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetCreatedBy(createdBy string) {
	if t == nil {
		return
	}
	t.CreatedBy = gptr.Of(createdBy)
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetCreatedBy(createdBy)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetUpdatedBy(updatedBy string) {
	if t == nil {
		return
	}
	t.UpdatedBy = gptr.Of(updatedBy)
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetUpdatedBy(updatedBy)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetCreatedAt(tt time.Time) {
	if t == nil {
		return
	}
	t.CreatedAt = tt
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetCreatedAt(tt)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetUpdatedAt(tt time.Time) {
	if t == nil {
		return
	}
	t.UpdatedAt = tt
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.SetUpdatedAt(tt)
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetStatus(val TagStatus) {
	if t == nil {
		return
	}
	t.Status = val
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.Status = val
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) SetTagKeyID(id int64) {
	if t == nil {
		return
	}
	t.TagKeyID = id
	now := t.TagValues
	for len(now) > 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			item.TagKeyID = id
			next = append(next, item.Children...)
		}
		now = next
	}
}

func (t *TagKey) Validate(spec *TagSpec) error {
	if t == nil {
		return errno.InvalidParamErrorf("tag key is nil")
	}
	if t.TagType == TagTypeUndefined {
		return errno.InvalidParamErrorf("tag type is undefined")
	}
	if t.TagContentType == TagContentTypeUndefined {
		return errno.InvalidParamErrorf("tag content type is undefined")
	}
	if err := t.validateContent(); err != nil {
		return err
	}
	if utf8.RuneCountInString(t.TagKeyName) > 50 {
		return errno.InvalidParamErrorf("length of tag name is more than 50")
	}
	if t.TagType == TagTypeTag && gvalue.IsZero(t.TagKeyName) {
		return errno.InvalidParamErrorf("tag name is empty")
	}
	if t.Description != nil && utf8.RuneCountInString(*t.Description) > 200 {
		return errno.InvalidParamErrorf("length of tag description is more than 200")
	}

	return validateTagValues(t.TagValues, spec.MaxHeight, spec.MaxWidth)
}

func (t *TagKey) validateContent() error {
	if t == nil {
		return nil
	}
	switch t.TagContentType {
	case TagContentTypeContinuousNumber, TagContentTypeFreeText:
		if len(t.TagValues) > 0 {
			return errno.InvalidParamErrorf("number of tag value is more than 0, content type: %s", t.TagContentType)
		}
	case TagContentTypeBoolean:
		if len(t.TagValues) != 2 {
			return errno.InvalidParamErrorf("number of tag values is illegal, length: %d", len(t.TagValues))
		}
	}
	return nil
}

// SplitTagValues 获取已经入库的TagValue和新TagValue
func (t *TagKey) SplitTagValues() (map[int64]*TagValue, []*TagValue) {
	if t == nil {
		return nil, nil
	}
	var newValues []*TagValue
	existedMap := make(map[int64]*TagValue, 0)
	now := t.TagValues
	for len(now) != 0 {
		var next []*TagValue
		for _, v := range now {
			item := v
			if gvalue.IsZero(item.TagValueID) {
				newValues = append(newValues, item)
			} else {
				existedMap[item.TagValueID] = item
			}
			next = append(next, item.Children...)
		}
		now = next
	}
	return existedMap, newValues
}

func (t *TagKey) CalculateChangeLogs(preTagKey *TagKey) ([]*ChangeLog, error) {
	if t == nil {
		return nil, errors.New("tag key is nil")
	}
	var res []*ChangeLog
	// tag key
	// create
	if preTagKey == nil {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTag,
			Operation:    TagOperationTypeCreate,
		})
		return res, nil
	}
	// update
	if t.TagType != preTagKey.TagType {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTagType,
			Operation:    TagOperationTypeUpdate,
			BeforeValue:  string(preTagKey.TagType),
			AfterValue:   string(t.TagType),
		})
	}
	if t.Status != preTagKey.Status {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTagStatus,
			Operation:    TagOperationTypeUpdate,
			BeforeValue:  string(preTagKey.Status),
			AfterValue:   string(t.Status),
		})
	}
	if t.TagKeyName != preTagKey.TagKeyName {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTagName,
			Operation:    TagOperationTypeUpdate,
			BeforeValue:  preTagKey.TagKeyName,
			AfterValue:   t.TagKeyName,
		})
	}
	if ptrutil.GetOrDefault(t.Description, "") != ptrutil.GetOrDefault(preTagKey.Description, "") {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTagDescription,
			Operation:    TagOperationTypeUpdate,
			BeforeValue:  ptrutil.GetOrDefault(preTagKey.Description, ""),
			AfterValue:   ptrutil.GetOrDefault(t.Description, ""),
		})
	}
	if t.TagContentType != preTagKey.TagContentType {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTagContentType,
			Operation:    TagOperationTypeUpdate,
			BeforeValue:  string(preTagKey.TagContentType),
			AfterValue:   string(t.TagContentType),
		})
	}

	// tag values
	preExistedMap, _ := preTagKey.SplitTagValues()
	nowExistedMap, nowNewList := t.SplitTagValues()
	// create
	for _, v := range nowNewList {
		res = append(res, &ChangeLog{
			ChangeTarget: TagChangeTargetTypeTagValueName,
			Operation:    TagOperationTypeCreate,
			AfterValue:   v.TagValueName,
			TargetValue:  v.TagValueName,
		})
	}
	// delete & update
	for k, v1 := range preExistedMap {
		v2, ok := nowExistedMap[k]
		// delete
		if !ok {
			res = append(res, &ChangeLog{
				ChangeTarget: TagChangeTargetTypeTagValueName,
				Operation:    TagOperationTypeDelete,
				BeforeValue:  v1.TagValueName,
				TargetValue:  v1.TagValueName,
			})
		} else {
			// update
			if v1.TagValueName != v2.TagValueName {
				res = append(res, &ChangeLog{
					ChangeTarget: TagChangeTargetTypeTagValueName,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  v1.TagValueName,
					AfterValue:   v2.TagValueName,
					TargetValue:  v2.TagValueName,
				})
			} else if v1.Status != v2.Status {
				res = append(res, &ChangeLog{
					ChangeTarget: TagChangeTargetTypeTagValueStatus,
					Operation:    TagOperationTypeUpdate,
					BeforeValue:  string(v1.Status),
					AfterValue:   string(v2.Status),
					TargetValue:  v2.TagValueName,
				})
			}
		}
	}
	return res, nil
}

func validateTagValues(val []*TagValue, maxHeight, maxWidth int) error {
	now := make([]*TagValue, 0)
	fallBacks := make([]*TagValue, 0)
	for idx := range val {
		if val[idx].TagValueName != consts.FallbackTagValueDefaultName && !val[idx].IsSystem {
			now = append(now, val[idx])
		} else {
			fallBacks = append(fallBacks, val[idx])
		}
	}
	if len(fallBacks) > 1 {
		return errno.InvalidParamErrorf("name %s is duplicate", consts.FallbackTagValueDefaultName)
	}
	var next []*TagValue
	height := 0
	width := 0
	mp := make(map[string]bool, 0)
	for len(now) != 0 {
		height += 1
		if height > maxHeight {
			return errno.InvalidParamErrorf("tag value height exceeds limit: %d", maxHeight)
		}
		width = gvalue.Max(width, len(now))
		if width > maxWidth {
			return errno.InvalidParamErrorf("tag value width exceeds limit: %d", maxWidth)
		}
		for _, v := range now {
			value := v
			if gvalue.IsZero(value.TagValueName) {
				return errno.InvalidParamErrorf("there is empty tag value")
			}
			tagValueName := value.TagValueName
			if ok := mp[tagValueName]; ok {
				return errno.InvalidParamErrorf("tag value is duplicated, tag value: %v", tagValueName)
			}
			if err := value.Validate(); err != nil {
				return err
			}
			mp[tagValueName] = true
			for idx := range value.Children {
				if value.Children[idx].TagValueName != consts.FallbackTagValueDefaultName && !value.Children[idx].IsSystem {
					next = append(next, value.Children[idx])
				}
			}
		}
		now = next
		next = []*TagValue{}
	}
	return nil
}

// ChangeLog 变更历史.
type ChangeLog struct {
	ChangeTarget TagChangeTargetType `json:"change_target,omitempty"`
	Operation    TagOperationType    `json:"operation,omitempty"`
	BeforeValue  string              `json:"before_value,omitempty"`
	AfterValue   string              `json:"after_value,omitempty"`
	TargetValue  string              `json:"target_value,omitempty"`
}

func (c *ChangeLog) ToDTO() *tag.ChangeLog {
	if c == nil {
		return nil
	}
	return &tag.ChangeLog{
		Target:      gptr.Of(c.ChangeTarget.ToDTO()),
		Operation:   gptr.Of(c.Operation.ToDTO()),
		BeforeValue: gptr.Of(c.BeforeValue),
		AfterValue:  gptr.Of(c.AfterValue),
		TargetValue: gptr.Of(c.TargetValue),
	}
}

type TagContentSpec struct {
	ContinuousNumberSpec *ContinuousNumberSpec `json:"continuous_number_spec,omitempty"`
}

func NewTagContentSpec(val *tag.TagContentSpec) *TagContentSpec {
	if val == nil {
		return nil
	}
	return &TagContentSpec{ContinuousNumberSpec: NewContinuousNumberSpecFromDTO(val.ContinuousNumberSpec)}
}

func (t *TagContentSpec) ToDTO() *tag.TagContentSpec {
	if t == nil {
		return nil
	}
	return &tag.TagContentSpec{ContinuousNumberSpec: t.ContinuousNumberSpec.ToDTO()}
}

type ContinuousNumberSpec struct {
	MinValue     *float64 `json:"min_value,omitempty"`
	MinValueDesc *string  `json:"min_value_desc,omitempty"`
	MaxValue     *float64 `json:"max_value,omitempty"`
	MaxValueDesc *string  `json:"max_value_desc,omitempty"`
}

func NewContinuousNumberSpecFromDTO(val *tag.ContinuousNumberSpec) *ContinuousNumberSpec {
	if val == nil {
		return nil
	}
	return &ContinuousNumberSpec{
		MinValue:     val.MinValue,
		MinValueDesc: val.MinValueDescription,
		MaxValue:     val.MaxValue,
		MaxValueDesc: val.MaxValueDescription,
	}
}

func (s *ContinuousNumberSpec) ToDTO() *tag.ContinuousNumberSpec {
	if s == nil {
		return nil
	}
	return &tag.ContinuousNumberSpec{
		MinValue:            s.MinValue,
		MinValueDescription: s.MinValueDesc,
		MaxValue:            s.MaxValue,
		MaxValueDescription: s.MaxValueDesc,
	}
}
