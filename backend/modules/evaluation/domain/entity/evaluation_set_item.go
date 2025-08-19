// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
)

type EvaluationSetItem struct {
	ID              int64     `json:"id,omitempty"`
	AppID           int32     `json:"app_id,omitempty"`
	SpaceID         int64     `json:"space_id,omitempty"`
	EvaluationSetID int64     `json:"evaluation_set_id,omitempty"`
	SchemaID        int64     `json:"schema_id,omitempty"`
	ItemID          int64     `json:"item_id,omitempty"`
	ItemKey         string    `json:"item_key,omitempty"`
	Turns           []*Turn   `json:"turns,omitempty"`
	BaseInfo        *BaseInfo `json:"base_info,omitempty"`
}

type Turn struct {
	ID            int64        `json:"id,omitempty"`
	FieldDataList []*FieldData `json:"field_data_list,omitempty"`
}

type FieldData struct {
	Key     string   `json:"key,omitempty"`
	Name    string   `json:"name,omitempty"`
	Content *Content `json:"content,omitempty"`
}

type ItemErrorGroup struct {
	Type    *ItemErrorType
	Summary *string
	// 错误条数
	ErrorCount *int32
	// 批量写入时，每类错误至多提供 5 个错误详情；导入任务，至多提供 10 个错误详情
	Details []*ItemErrorDetail
}

type ItemErrorType int64

const (
	// schema 不匹配
	ItemErrorType_MismatchSchema ItemErrorType = 1
	// 空数据
	ItemErrorType_EmptyData ItemErrorType = 2
	// 单条数据大小超限
	ItemErrorType_ExceedMaxItemSize ItemErrorType = 3
	// 数据集容量超限
	ItemErrorType_ExceedDatasetCapacity ItemErrorType = 4
	// 文件格式错误
	ItemErrorType_MalformedFile ItemErrorType = 5
	// 包含非法内容
	ItemErrorType_IllegalContent ItemErrorType = 6
	// 缺少必填字段
	ItemErrorType_MissingRequiredField ItemErrorType = 7
	// 数据嵌套层数超限
	ItemErrorType_ExceedMaxNestedDepth ItemErrorType = 8
	// 数据转换失败
	ItemErrorType_TransformItemFailed ItemErrorType = 9
	/* system error*/
	ItemErrorType_InternalError ItemErrorType = 100
)

func (p ItemErrorType) String() string {
	switch p {
	case ItemErrorType_MismatchSchema:
		return "MismatchSchema"
	case ItemErrorType_EmptyData:
		return "EmptyData"
	case ItemErrorType_ExceedMaxItemSize:
		return "ExceedMaxItemSize"
	case ItemErrorType_ExceedDatasetCapacity:
		return "ExceedDatasetCapacity"
	case ItemErrorType_MalformedFile:
		return "MalformedFile"
	case ItemErrorType_IllegalContent:
		return "IllegalContent"
	case ItemErrorType_MissingRequiredField:
		return "MissingRequiredField"
	case ItemErrorType_ExceedMaxNestedDepth:
		return "ExceedMaxNestedDepth"
	case ItemErrorType_TransformItemFailed:
		return "TransformItemFailed"
	case ItemErrorType_InternalError:
		return "InternalError"
	}
	return "<UNSET>"
}

func ItemErrorTypeFromString(s string) (ItemErrorType, error) {
	switch s {
	case "MismatchSchema":
		return ItemErrorType_MismatchSchema, nil
	case "EmptyData":
		return ItemErrorType_EmptyData, nil
	case "ExceedMaxItemSize":
		return ItemErrorType_ExceedMaxItemSize, nil
	case "ExceedDatasetCapacity":
		return ItemErrorType_ExceedDatasetCapacity, nil
	case "MalformedFile":
		return ItemErrorType_MalformedFile, nil
	case "IllegalContent":
		return ItemErrorType_IllegalContent, nil
	case "MissingRequiredField":
		return ItemErrorType_MissingRequiredField, nil
	case "ExceedMaxNestedDepth":
		return ItemErrorType_ExceedMaxNestedDepth, nil
	case "TransformItemFailed":
		return ItemErrorType_TransformItemFailed, nil
	case "InternalError":
		return ItemErrorType_InternalError, nil
	}
	return ItemErrorType(0), fmt.Errorf("not a valid ItemErrorType string")
}

type ItemErrorDetail struct {
	Message *string
	// 单条错误数据在输入数据中的索引。从 0 开始，下同
	Index *int32
	// [startIndex, endIndex] 表示区间错误范围, 如 ExceedDatasetCapacity 错误时
	StartIndex *int32
	EndIndex   *int32
}

type ItemSnapshotFieldMapping struct {
	FieldKey string `json:"field_key"`
	// float_map, int_map, string_map, tag_array
	MappingKey string `json:"mapping_key"`
	// tag_array时，无值
	MappingSubKey string `json:"mapping_subKey"`
}
