// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

const (
	TagValueTypeUnknown TagValueType = 1
	TagValueTypeBool    TagValueType = 2
	TagValueTypeInt64   TagValueType = 3
	TagValueTypeFloat64 TagValueType = 4
	TagValueTypeString  TagValueType = 5
)

type TagValueType int

type Tag struct {
	ValueType TagValueType
	Key       string
	Value     *TagValue
}

type TagValue struct {
	VBool   *bool
	VLong   *int64
	VDouble *float64
	VStr    *string
}

func (p TagValueType) String() string {
	switch p {
	case TagValueTypeBool:
		return "Bool"
	case TagValueTypeInt64:
		return "I64"
	case TagValueTypeFloat64:
		return "F64"
	case TagValueTypeString:
		return "String"
	}
	return "<UNSET>"
}

func (t Tag) GetKey() string {
	return t.Key
}

func (t Tag) IsSetValue() bool {
	return t.Value != nil
}

func (t Tag) GetTagType() TagValueType {
	return t.ValueType
}

func SetTag(tagKey string, tagType TagValueType, tagValue any) (tag *Tag) {
	switch tagType {
	case TagValueTypeBool:
		boolValue, ok := tagValue.(bool)
		if !ok {
			return &Tag{}
		}
		tag = NewBoolTag(tagKey, boolValue)
	case TagValueTypeInt64:
		int64Value, ok := tagValue.(int64)
		if !ok {
			return &Tag{}
		}
		tag = NewLongTag(tagKey, int64Value)
	case TagValueTypeFloat64:
		float64Value, ok := tagValue.(float64)
		if !ok {
			return &Tag{}
		}
		tag = NewDoubleTag(tagKey, float64Value)
	case TagValueTypeString:
		stringValue, ok := tagValue.(string)
		if !ok {
			return &Tag{}
		}
		tag = NewStringTag(tagKey, stringValue)
	default:
		return &Tag{}
	}
	return tag
}

func (t Tag) validate() error {
	if t.GetKey() == "" {
		return fmt.Errorf("tag is blank")
	}
	if !t.IsSetValue() {
		return fmt.Errorf("value is not set, tag=%s", json.MarshalStringIgnoreErr(t))
	}
	return nil
}

func (t Tag) assertType(tagType TagValueType) error {
	if t.GetTagType() != tagType {
		return fmt.Errorf("unexpected tag type(%s), current tag type=%s, tag=%s", tagType.String(), t.GetTagType(), json.MarshalStringIgnoreErr(t))
	}
	return nil
}

func (t Tag) getBool() (bool, error) {
	if err := t.assertType(TagValueTypeBool); err != nil {
		return false, err
	}
	if err := t.validate(); err != nil {
		return false, err
	}
	return *t.Value.VBool, nil
}

func (t Tag) getI64() (int64, error) {
	if err := t.assertType(TagValueTypeInt64); err != nil {
		return int64(-1), err
	}
	if err := t.validate(); err != nil {
		return int64(-1), err
	}
	return *t.Value.VLong, nil
}

func (t Tag) getF64() (float64, error) {
	const invalidVal = float64(0.0)
	if err := t.assertType(TagValueTypeFloat64); err != nil {
		return invalidVal, err
	}
	if err := t.validate(); err != nil {
		return invalidVal, err
	}

	return *t.Value.VDouble, nil
}

func (t Tag) getString() (string, error) {
	if err := t.assertType(TagValueTypeString); err != nil {
		return "", err
	}
	if err := t.validate(); err != nil {
		return "", err
	}
	return *t.Value.VStr, nil
}

func (t Tag) GetStringValue() (string, error) {
	var tagStr string
	var err error
	switch t.GetTagType() {
	case TagValueTypeString:
		tagStr, err = t.getString()
		if err != nil {
			return "", err
		}
	case TagValueTypeBool:
		tagBool, err := t.getBool()
		if err != nil {
			return "", err
		}
		tagStr = strconv.FormatBool(tagBool)
	case TagValueTypeInt64:
		tagI64, err := t.getI64()
		if err != nil {
			return "", err
		}
		tagStr = strconv.FormatInt(tagI64, 10)
	case TagValueTypeFloat64:
		tagF64, err := t.getF64()
		if err != nil {
			return "", err
		}
		tagStr = strconv.FormatFloat(tagF64, 'f', -1, 64)
	}
	return tagStr, nil
}

func NewStringTag(key, val string) *Tag {
	return &Tag{
		Key:       key,
		ValueType: TagValueTypeString,
		Value: &TagValue{
			VStr: ptr.Of(val),
		},
	}
}

func NewLongTag(key string, val int64) *Tag {
	return &Tag{
		Key:       key,
		ValueType: TagValueTypeInt64,
		Value: &TagValue{
			VLong: ptr.Of(val),
		},
	}
}

func NewDoubleTag(key string, val float64) *Tag {
	return &Tag{
		Key:       key,
		ValueType: TagValueTypeFloat64,
		Value: &TagValue{
			VDouble: ptr.Of(val),
		},
	}
}

func NewBoolTag(key string, val bool) *Tag {
	return &Tag{
		Key:       key,
		ValueType: TagValueTypeBool,
		Value: &TagValue{
			VBool: ptr.Of(val),
		},
	}
}
