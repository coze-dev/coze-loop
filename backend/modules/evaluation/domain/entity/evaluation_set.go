// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
)

type EvaluationSet struct {
	ID                   int64                 `json:"id,omitempty"`
	AppID                int32                 `json:"app_id,omitempty"`
	SpaceID              int64                 `json:"space_id,omitempty"`
	Name                 string                `json:"name,omitempty"`
	Description          string                `json:"description,omitempty"`
	Status               DatasetStatus         `json:"status,omitempty"`
	Spec                 *DatasetSpec          `json:"spec,omitempty"`
	Features             *DatasetFeatures      `json:"features,omitempty"`
	ItemCount            int64                 `json:"item_count,omitempty"`
	ChangeUncommitted    bool                  `json:"change_uncommitted,omitempty"`
	EvaluationSetVersion *EvaluationSetVersion `json:"evaluation_set_version,omitempty"`
	LatestVersion        string                `json:"latest_version,omitempty"`
	NextVersionNum       int64                 `json:"next_version_num,omitempty"`
	BaseInfo             *BaseInfo             `json:"base_info,omitempty"`
	BizCategory          BizCategory           `json:"biz_category,omitempty"`
}

type DatasetSpec struct {
	MaxItemCount           int64 `json:"max_item_count,omitempty"`
	MaxFieldCount          int32 `json:"max_field_count,omitempty"`
	MaxItemSize            int64 `json:"max_item_size,omitempty"`
	MaxItemDataNestedDepth int32 `json:"max_item_data_nested_depth,omitempty"`
}

type DatasetFeatures struct {
	EditSchema   bool `json:"editSchema,omitempty"`
	RepeatedData bool `json:"repeatedData,omitempty"`
	MultiModal   bool `json:"multiModal,omitempty"`
}

type DatasetStatus int64

const (
	DatasetStatus_Available DatasetStatus = 1
	DatasetStatus_Deleted   DatasetStatus = 2
	DatasetStatus_Expired   DatasetStatus = 3
	DatasetStatus_Importing DatasetStatus = 4
	DatasetStatus_Exporting DatasetStatus = 5
	DatasetStatus_Indexing  DatasetStatus = 6
)

func (p DatasetStatus) String() string {
	switch p {
	case DatasetStatus_Available:
		return "Available"
	case DatasetStatus_Deleted:
		return "Deleted"
	case DatasetStatus_Expired:
		return "Expired"
	case DatasetStatus_Importing:
		return "Importing"
	case DatasetStatus_Exporting:
		return "Exporting"
	case DatasetStatus_Indexing:
		return "Indexing"
	}
	return "<UNSET>"
}

func DatasetStatusFromString(s string) (DatasetStatus, error) {
	switch s {
	case "Available":
		return DatasetStatus_Available, nil
	case "Deleted":
		return DatasetStatus_Deleted, nil
	case "Expired":
		return DatasetStatus_Expired, nil
	case "Importing":
		return DatasetStatus_Importing, nil
	case "Exporting":
		return DatasetStatus_Exporting, nil
	case "Indexing":
		return DatasetStatus_Indexing, nil
	}
	return DatasetStatus(0), fmt.Errorf("not a valid DatasetStatus string")
}

func DatasetStatusPtr(v DatasetStatus) *DatasetStatus { return &v }
func (p *DatasetStatus) Scan(value interface{}) (err error) {
	var result sql.NullInt64
	err = result.Scan(value)
	*p = DatasetStatus(result.Int64)
	return
}

func (p *DatasetStatus) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return int64(*p), nil
}

type BizCategory = string

const (
	BizCategoryFromOnlineTrace = "from_online_trace"
)
