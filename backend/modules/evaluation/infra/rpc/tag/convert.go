// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TagValueDTO2DO(tagValue *tag.TagValue) *entity.TagValue {
	return &entity.TagValue{
		TagValueId:   ptr.From(tagValue.TagValueID),
		TagValueName: ptr.From(tagValue.TagValueName),
		Status:       ptr.From(tagValue.Status),
	}
}

func TagValueListDTO2DO(tagValues []*tag.TagValue) []*entity.TagValue {
	ret := make([]*entity.TagValue, 0, len(tagValues))
	for _, tagValue := range tagValues {
		ret = append(ret, TagValueDTO2DO(tagValue))
	}
	return ret
}

func TagDTO2DO(tagInfo *tag.TagInfo) *entity.TagInfo {
	if tagInfo == nil {
		return nil
	}

	tagInfoDO := &entity.TagInfo{
		TagKeyId:       ptr.From(tagInfo.TagKeyID),
		TagKeyName:     ptr.From(tagInfo.TagKeyName),
		Description:    ptr.From(tagInfo.Description),
		InActive:       ptr.From(tagInfo.Status) != "active",
		TagValues:      TagValueListDTO2DO(tagInfo.TagValues),
		TagContentType: entity.TagContentType(ptr.From(tagInfo.ContentType)),
		TagStatus:      ptr.From(tagInfo.Status),
	}

	if tagInfo.ContentSpec != nil && tagInfo.ContentSpec.ContinuousNumberSpec != nil {
		tagInfoDO.TagContentSpec = &entity.TagContentSpec{
			ContinuousNumberSpec: &entity.ContinuousNumberSpec{
				MinValue:            tagInfo.ContentSpec.ContinuousNumberSpec.MinValue,
				MinValueDescription: tagInfo.ContentSpec.ContinuousNumberSpec.MinValueDescription,
				MaxValue:            tagInfo.ContentSpec.ContinuousNumberSpec.MaxValue,
				MaxValueDescription: tagInfo.ContentSpec.ContinuousNumberSpec.MaxValueDescription,
			},
		}
	}

	return tagInfoDO
}

func TagListDTO2DO(tagInfos []*tag.TagInfo) []*entity.TagInfo {
	ret := make([]*entity.TagInfo, 0, len(tagInfos))
	for _, tagInfo := range tagInfos {
		ret = append(ret, TagDTO2DO(tagInfo))
	}
	return ret
}
