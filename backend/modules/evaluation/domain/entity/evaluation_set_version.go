// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type EvaluationSetVersion struct {
	ID                  int64                `thrift:"id,1,optional" frugal:"1,optional,i64" json:"id,omitempty"`
	AppID               int32                `thrift:"app_id,2,optional" frugal:"2,optional,i32" json:"app_id,omitempty"`
	SpaceID             int64                `thrift:"space_id,3,optional" frugal:"3,optional,i64" json:"space_id,omitempty"`
	EvaluationSetID     int64                `thrift:"evaluation_set_id,4,optional" frugal:"4,optional,i64" json:"evaluation_set_id,omitempty"`
	Version             string               `thrift:"version,10,optional" frugal:"10,optional,string" json:"version,omitempty"`
	VersionNum          int64                `thrift:"version_num,11,optional" frugal:"11,optional,i64" json:"version_num,omitempty"`
	Description         string               `thrift:"description,12,optional" frugal:"12,optional,string" json:"description,omitempty"`
	EvaluationSetSchema *EvaluationSetSchema `thrift:"evaluation_set_schema,13,optional" frugal:"13,optional,EvaluationSetSchema" json:"evaluation_set_schema,omitempty"`
	ItemCount           int64                `thrift:"item_count,14,optional" frugal:"14,optional,i64" json:"item_count,omitempty"`
	BaseInfo            *BaseInfo            `thrift:"base_info,100,optional" frugal:"100,optional,common.BaseInfo" json:"base_info"`
	SharedInfo          *SharedResourceInfo  `json:"shared_info,omitempty"`
}
