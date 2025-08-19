// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/data/domain/tag"
)

func TestNewTagStatusFromDTO(t *testing.T) {
	tests := []struct {
		name   string
		input  *tag.TagStatus
		target TagStatus
	}{
		{
			name:   "input is nil",
			input:  nil,
			target: TagStatusUndefined,
		},
		{
			name:   "active",
			input:  gptr.Of(tag.TagStatusActive),
			target: TagStatusActive,
		},
		{
			name:   "inactive",
			input:  gptr.Of(tag.TagStatusInactive),
			target: TagStatusInactive,
		},
		{
			name:   "deprecated",
			input:  gptr.Of(tag.TagStatusDeprecated),
			target: TagStatusDeprecated,
		},
		{
			name:   "other",
			input:  gptr.Of(tag.TagStatus("123")),
			target: TagStatusUndefined,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, NewTagStatusFromDTO(tt.input))
		})
	}
}

func TestTagStatus_ToDTO(t *testing.T) {
	tests := []struct {
		name   string
		target tag.TagStatus
		input  *TagStatus
	}{
		{
			name:   "input is nil",
			input:  nil,
			target: "",
		},
		{
			name:   "active",
			input:  gptr.Of(TagStatusActive),
			target: tag.TagStatusActive,
		},
		{
			name:   "inactive",
			input:  gptr.Of(TagStatusInactive),
			target: tag.TagStatusInactive,
		},
		{
			name:   "deprecated",
			input:  gptr.Of(TagStatusDeprecated),
			target: tag.TagStatusDeprecated,
		},
		{
			name:   "other",
			input:  gptr.Of(TagStatus("123")),
			target: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, tt.input.ToDTO())
		})
	}
}

func TestNewTagTypeFromDTO(t *testing.T) {
	tests := []struct {
		name   string
		input  tag.TagType
		target TagType
	}{
		{
			name:   "tag",
			input:  tag.TagTypeTag,
			target: TagTypeTag,
		},
		{
			name:   "option",
			input:  tag.TagTypeOption,
			target: TagTypeOption,
		},
		{
			name:   "other",
			input:  "123",
			target: TagTypeUndefined,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, NewTagTypeFromDTO(tt.input))
		})
	}
}

func TestTagType_ToDTO(t *testing.T) {
	tests := []struct {
		name   string
		target tag.TagType
		input  *TagType
	}{
		{
			name:   "input is nil",
			input:  nil,
			target: "",
		},
		{
			name:   "tag",
			input:  gptr.Of(TagTypeTag),
			target: tag.TagTypeTag,
		},
		{
			name:   "option",
			input:  gptr.Of(TagTypeOption),
			target: tag.TagTypeOption,
		},
		{
			name:   "other",
			input:  gptr.Of(TagType("123")),
			target: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, tt.input.ToDTO())
		})
	}
}

func TestNewTagTargetTypeFromDTO(t *testing.T) {
	tests := []struct {
		name   string
		input  tag.TagDomainType
		target TagTargetType
	}{
		{
			name:   "data",
			input:  tag.TagDomainTypeData,
			target: TagTargetTypeDatasetItem,
		},
		{
			name:   "observe",
			input:  tag.TagDomainTypeObserve,
			target: TagTargetTypeObserve,
		},
		{
			name:   "evaluation",
			input:  tag.TagDomainTypeEvaluation,
			target: TagTargetTypeEvaluation,
		},
		{
			name:   "resource",
			input:  tag.TagDomainType("resource"),
			target: TagTargetTypeUndefined,
		},
		{
			name:   "other",
			input:  tag.TagDomainType("123"),
			target: TagTargetTypeUndefined,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, NewTagTargetTypeFromDTO(tt.input))
		})
	}
}

func TestTagTargetType_ToDTO(t *testing.T) {
	tests := []struct {
		name   string
		target tag.TagDomainType
		input  TagTargetType
	}{
		{
			name:   "resource",
			input:  TagTargetTypeResource,
			target: "",
		},
		{
			name:   "data",
			input:  TagTargetTypeDatasetItem,
			target: tag.TagDomainTypeData,
		},
		{
			name:   "observe",
			input:  TagTargetTypeObserve,
			target: tag.TagDomainTypeObserve,
		},
		{
			name:   "evaluation",
			input:  TagTargetTypeEvaluation,
			target: tag.TagDomainTypeEvaluation,
		},
		{
			name:   "other",
			input:  TagTargetType("123"),
			target: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, tt.input.ToDTO())
		})
	}
}

func TestTagChangeTargetType_ToDTO(t *testing.T) {
	tests := []struct {
		name   string
		target tag.ChangeTargetType
		input  *TagChangeTargetType
	}{
		{
			name:   "nil",
			input:  nil,
			target: "",
		},
		{
			name:   "tag",
			input:  gptr.Of(TagChangeTargetTypeTag),
			target: tag.ChangeTargetTypeTag,
		},
		{
			name:   "tag name",
			input:  gptr.Of(TagChangeTargetTypeTagName),
			target: tag.ChangeTargetTypeTagName,
		},
		{
			name:   "tag description",
			input:  gptr.Of(TagChangeTargetTypeTagDescription),
			target: tag.ChangeTargetTypeTagDescription,
		},
		{
			name:   "tag status",
			input:  gptr.Of(TagChangeTargetTypeTagStatus),
			target: tag.ChangeTargetTypeTagStatus,
		},
		{
			name:   "tag type",
			input:  gptr.Of(TagChangeTargetTypeTagType),
			target: tag.ChangeTargetTypeTagType,
		},
		{
			name:   "tag value name",
			input:  gptr.Of(TagChangeTargetTypeTagValueName),
			target: tag.ChangeTargetTypeTagValueName,
		},
		{
			name:   "tag value status",
			input:  gptr.Of(TagChangeTargetTypeTagValueStatus),
			target: tag.ChangeTargetTypeTagValueStatus,
		},
		{
			name:   "tag content type",
			input:  gptr.Of(TagChangeTargetTypeTagContentType),
			target: tag.ChangeTargetTypeTagContentType,
		},
		{
			name:   "other",
			input:  gptr.Of(TagChangeTargetType("123123")),
			target: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, tt.input.ToDTO())
		})
	}
}

func TestTagOperation_ToDTO(t *testing.T) {
	tests := []struct {
		name   string
		target tag.OperationType
		input  *TagOperationType
	}{
		{
			name:   "nil",
			input:  nil,
			target: "",
		},
		{
			name:   "update",
			input:  gptr.Of(TagOperationTypeUpdate),
			target: tag.OperationTypeUpdate,
		},
		{
			name:   "create",
			input:  gptr.Of(TagOperationTypeCreate),
			target: tag.OperationTypeCreate,
		},
		{
			name:   "delete",
			input:  gptr.Of(TagOperationTypeDelete),
			target: tag.OperationTypeDelete,
		},
		{
			name:   "other",
			input:  gptr.Of(TagOperationType("123")),
			target: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, tt.input.ToDTO())
		})
	}
}

func TestTagContentType_ToDTO(t *testing.T) {
	tests := []struct {
		name   string
		target tag.TagContentType
		input  *TagContentType
	}{
		{
			name:   "nil",
			input:  nil,
			target: "",
		},
		{
			name:   "categorical",
			input:  gptr.Of(TagContentTypeCategorical),
			target: tag.TagContentTypeCategorical,
		},
		{
			name:   "boolean",
			input:  gptr.Of(TagContentTypeBoolean),
			target: tag.TagContentTypeBoolean,
		},
		{
			name:   "continuous number",
			input:  gptr.Of(TagContentTypeContinuousNumber),
			target: tag.TagContentTypeContinuousNumber,
		},
		{
			name:   "free text",
			input:  gptr.Of(TagContentTypeFreeText),
			target: tag.TagContentTypeFreeText,
		},
		{
			name:   "other",
			input:  gptr.Of(TagContentType("123")),
			target: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, tt.input.ToDTO())
		})
	}
}

func TestNewTagContentTypeFromDTO(t *testing.T) {
	tests := []struct {
		name   string
		input  tag.TagContentType
		target TagContentType
	}{
		{
			name:   "categorical",
			input:  tag.TagContentTypeCategorical,
			target: TagContentTypeCategorical,
		},
		{
			name:   "boolean",
			input:  tag.TagContentTypeBoolean,
			target: TagContentTypeBoolean,
		},
		{
			name:   "continuous number",
			input:  tag.TagContentTypeContinuousNumber,
			target: TagContentTypeContinuousNumber,
		},
		{
			name:   "free text",
			input:  tag.TagContentTypeFreeText,
			target: TagContentTypeFreeText,
		},
		{
			name:   "other",
			input:  tag.TagContentType("123"),
			target: TagContentTypeUndefined,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.target, NewTagContentTypeFromDTO(tt.input))
		})
	}
}
