// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type TagContentType string

const (
	TagContentTypeCategorical      = "categorical"
	TagContentTypeBoolean          = "boolean"
	TagContentTypeContinuousNumber = "continuous_number"
	TagContentTypeFreeText         = "free_text"

	TagContentTextMaxLength = 1024
)

type TagInfo struct {
	TagKeyId       int64
	TagKeyName     string
	InActive       bool
	TagContentType TagContentType
	TagValues      []*TagValue
}

type TagValue struct {
	TagValueId   int64
	TagValueName string
	TagValues    []*TagValue
}

func (t *TagValue) CheckTagValueId(tagValId int64) error {
	if t.TagValueId == tagValId {
		return nil
	}
	for _, val := range t.TagValues {
		if val.CheckTagValueId(tagValId) == nil {
			return nil
		}
	}
	return fmt.Errorf("tag value %d not found in tag values", tagValId)
}

func (t *TagValue) GetTagValue(tagValId int64) *TagValue {
	if t.TagValueId == tagValId {
		return t
	}
	for _, val := range t.TagValues {
		if subVal := val.GetTagValue(tagValId); subVal != nil {
			return subVal
		}
	}
	return nil
}

func (t *TagInfo) CheckAnnotation(annotation *loop_span.Annotation) error {
	switch t.TagContentType {
	case TagContentTypeCategorical, TagContentTypeBoolean:
		if annotation.Value.ValueType != loop_span.AnnotationValueTypeLong {
			return fmt.Errorf("annotation value type not match long type")
		}
		return t.CheckTagValueId(annotation.Value.LongValue)
	case TagContentTypeContinuousNumber:
		if annotation.Value.ValueType != loop_span.AnnotationValueTypeDouble {
			return fmt.Errorf("annotation value type not match double type")
		}
	case TagContentTypeFreeText:
		if annotation.Value.ValueType != loop_span.AnnotationValueTypeString {
			return fmt.Errorf("annotation value type not match string type")
		} else if len(annotation.Value.StringValue) > TagContentTextMaxLength {
			return fmt.Errorf("annotation string value length too long")
		} else if len(annotation.Value.StringValue) == 0 {
			return fmt.Errorf("annotation string value empty")
		}
	default:
		return fmt.Errorf("unknown tag content type: %s", t.TagContentType)
	}
	return nil
}

func (t *TagInfo) CheckTagValueId(tagValId int64) error {
	for _, tagValue := range t.TagValues {
		if tagValue.CheckTagValueId(tagValId) == nil {
			return nil
		}
	}
	return fmt.Errorf("tag value %d not found in tag values", tagValId)
}

func (t *TagInfo) GetTagValue(tagValId int64) *TagValue {
	for _, tagValue := range t.TagValues {
		if val := tagValue.GetTagValue(tagValId); val != nil {
			return val
		}
	}
	return nil
}

//go:generate mockgen -destination=mocks/tag.go -package=mocks . ITagRPCAdapter
type ITagRPCAdapter interface {
	GetTagInfo(context.Context, int64, string) (*TagInfo, error)
	BatchGetTagInfo(context.Context, int64, []string) (map[int64]*TagInfo, error)
}
