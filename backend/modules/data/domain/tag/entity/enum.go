// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
)

type TagStatus string

const (
	TagStatusUndefined  TagStatus = ""
	TagStatusActive     TagStatus = "active"
	TagStatusInactive   TagStatus = "inactive"
	TagStatusDeprecated TagStatus = "deprecated"
)

func NewTagStatusFromDTO(val *tag.TagStatus) TagStatus {
	if val == nil {
		return TagStatusUndefined
	}
	switch *val {
	case tag.TagStatusActive:
		return TagStatusActive
	case tag.TagStatusInactive:
		return TagStatusInactive
	case tag.TagStatusDeprecated:
		return TagStatusDeprecated
	default:
		return TagStatusUndefined
	}
}

func (t *TagStatus) ToDTO() tag.TagStatus {
	if t == nil {
		return tag.TagInfo_Status_DEFAULT
	}
	switch *t {
	case TagStatusActive:
		return tag.TagStatusActive
	case TagStatusInactive:
		return tag.TagStatusInactive
	case TagStatusDeprecated:
		return tag.TagStatusDeprecated
	default:
		return tag.TagInfo_Status_DEFAULT
	}
}

type TagType string

const (
	TagTypeUndefined TagType = ""
	TagTypeTag       TagType = "tag"
	TagTypeOption    TagType = "option"
)

func NewTagTypeFromDTO(val tag.TagType) TagType {
	switch val {
	case tag.TagTypeTag:
		return TagTypeTag
	case tag.TagTypeOption:
		return TagTypeOption
	default:
		return TagTypeUndefined
	}
}

func (t *TagType) ToDTO() tag.TagType {
	if t == nil {
		return tag.TagInfo_TagType_DEFAULT
	}
	switch *t {
	case TagTypeTag:
		return tag.TagTypeTag
	case TagTypeOption:
		return tag.TagTypeOption
	default:
		return tag.TagInfo_TagType_DEFAULT
	}
}

type TagTargetType string

const (
	TagTargetTypeUndefined   TagTargetType = ""
	TagTargetTypeResource    TagTargetType = "resource"
	TagTargetTypeDatasetItem TagTargetType = "dataset_item"
	TagTargetTypeObserve     TagTargetType = "observe"
	TagTargetTypeEvaluation  TagTargetType = "evaluation"
)

func NewTagTargetTypeFromDTO(val tag.TagDomainType) TagTargetType {
	switch val {
	case tag.TagDomainTypeData:
		return TagTargetTypeDatasetItem
	case tag.TagDomainTypeObserve:
		return TagTargetTypeObserve
	case tag.TagDomainTypeEvaluation:
		return TagTargetTypeEvaluation
	default:
		return TagTargetTypeUndefined
	}
}

func (t TagTargetType) ToDTO() tag.TagDomainType {
	switch t {
	case TagTargetTypeDatasetItem:
		return tag.TagDomainTypeData
	case TagTargetTypeEvaluation:
		return tag.TagDomainTypeEvaluation
	case TagTargetTypeObserve:
		return tag.TagDomainTypeObserve
	default:
		return ""
	}
}

type TagChangeTargetType string

const (
	TagChangeTargetTypeUndefined      TagChangeTargetType = ""
	TagChangeTargetTypeTag            TagChangeTargetType = "tag"
	TagChangeTargetTypeTagName        TagChangeTargetType = "tag_name"
	TagChangeTargetTypeTagDescription TagChangeTargetType = "tag_description"
	TagChangeTargetTypeTagStatus      TagChangeTargetType = "tag_status"
	TagChangeTargetTypeTagType        TagChangeTargetType = "tag_type"
	TagChangeTargetTypeTagValueName   TagChangeTargetType = "tag_value_name"
	TagChangeTargetTypeTagValueStatus TagChangeTargetType = "tag_value_status"
	TagChangeTargetTypeTagContentType TagChangeTargetType = "tag_content_type"
)

func (t *TagChangeTargetType) ToDTO() tag.ChangeTargetType {
	if t == nil {
		return tag.ChangeLog_Target_DEFAULT
	}
	switch *t {
	case TagChangeTargetTypeTag:
		return tag.ChangeTargetTypeTag
	case TagChangeTargetTypeTagName:
		return tag.ChangeTargetTypeTagName
	case TagChangeTargetTypeTagDescription:
		return tag.ChangeTargetTypeTagDescription
	case TagChangeTargetTypeTagValueName:
		return tag.ChangeTargetTypeTagValueName
	case TagChangeTargetTypeTagValueStatus:
		return tag.ChangeTargetTypeTagValueStatus
	case TagChangeTargetTypeTagStatus:
		return tag.ChangeTargetTypeTagStatus
	case TagChangeTargetTypeTagType:
		return tag.ChangeTargetTypeTagType
	case TagChangeTargetTypeTagContentType:
		return tag.ChangeTargetTypeTagContentType
	default:
		return tag.ChangeLog_Target_DEFAULT
	}
}

type TagOperationType string

const (
	TagOperationTypeUndefined TagOperationType = ""
	TagOperationTypeCreate    TagOperationType = "create"
	TagOperationTypeUpdate    TagOperationType = "update"
	TagOperationTypeDelete    TagOperationType = "delete"
)

func (t *TagOperationType) ToDTO() tag.OperationType {
	if t == nil {
		return tag.ChangeLog_Operation_DEFAULT
	}
	switch *t {
	case TagOperationTypeCreate:
		return tag.OperationTypeCreate
	case TagOperationTypeUpdate:
		return tag.OperationTypeUpdate
	case TagOperationTypeDelete:
		return tag.OperationTypeDelete
	default:
		return tag.ChangeLog_Operation_DEFAULT
	}
}

type TagContentType string

const (
	TagContentTypeUndefined        TagContentType = ""
	TagContentTypeCategorical      TagContentType = "categorical"
	TagContentTypeBoolean          TagContentType = "boolean"
	TagContentTypeContinuousNumber TagContentType = "continuous_number"
	TagContentTypeFreeText         TagContentType = "free_text"
)

func (t *TagContentType) ToDTO() tag.TagContentType {
	if t == nil {
		return tag.TagInfo_ContentType_DEFAULT
	}
	switch *t {
	case TagContentTypeCategorical:
		return tag.TagContentTypeCategorical
	case TagContentTypeBoolean:
		return tag.TagContentTypeBoolean
	case TagContentTypeContinuousNumber:
		return tag.TagContentTypeContinuousNumber
	case TagContentTypeFreeText:
		return tag.TagContentTypeFreeText
	default:
		return tag.TagInfo_ContentType_DEFAULT
	}
}

func NewTagContentTypeFromDTO(val tag.TagContentType) TagContentType {
	switch val {
	case tag.TagContentTypeCategorical:
		return TagContentTypeCategorical
	case tag.TagContentTypeBoolean:
		return TagContentTypeBoolean
	case tag.TagContentTypeContinuousNumber:
		return TagContentTypeContinuousNumber
	case tag.TagContentTypeFreeText:
		return TagContentTypeFreeText
	default:
		return TagContentTypeUndefined
	}
}
