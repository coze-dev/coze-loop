// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package dao

import (
	"context"
)

type InsertAnnotationParam struct {
	Table       string
	Annotations []*Annotation
}

type GetAnnotationParam struct {
	Tables    []string
	ID        string
	StartTime int64 // us
	EndTime   int64 // us
	Limit     int32
}

type ListAnnotationsParam struct {
	Tables          []string
	SpanIDs         []string
	StartTime       int64 // us
	EndTime         int64 // us
	DescByUpdatedAt bool
	Limit           int32
}

//go:generate mockgen -destination=mocks/annotation_dao.go -package=mocks . IAnnotationDao
type IAnnotationDao interface {
	Insert(context.Context, *InsertAnnotationParam) error
	Get(context.Context, *GetAnnotationParam) (*Annotation, error)
	List(context.Context, *ListAnnotationsParam) ([]*Annotation, error)
}

type Annotation struct {
	ID              string   `json:"id"`
	SpanID          string   `json:"span_id"`
	TraceID         string   `gorm:"column:trace_id;type:String;not null" json:"trace_id"`
	StartTime       int64    `gorm:"column:start_time;type:Int64;not null" json:"start_time"`
	SpaceID         string   `gorm:"column:space_id;type:String;not null" json:"space_id"`
	AnnotationType  string   `gorm:"column:annotation_type;type:String;not null" json:"annotation_type"`
	AnnotationIndex []string `gorm:"column:annotation_index;type:Array(String);not null" json:"annotation_index"`
	Key             string   `gorm:"column:key;type:String;not null" json:"key"`
	ValueType       string   `gorm:"column:value_type;type:String;not null" json:"value_type"`
	ValueString     string   `gorm:"column:value_string;type:String;not null" json:"value_string"`
	ValueLong       int64    `gorm:"column:value_long;type:Int64;not null" json:"value_long"`
	ValueFloat      float64  `gorm:"column:value_float;type:Float64;not null" json:"value_float"`
	ValueBool       bool     `gorm:"column:value_bool;type:Bool;not null" json:"value_bool"`
	Reasoning       string   `gorm:"column:reasoning;type:String;not null" json:"reasoning"`
	Correction      string   `gorm:"column:correction;type:String;not null" json:"correction"`
	Metadata        string   `gorm:"column:metadata;type:String;not null" json:"metadata"`
	Status          string   `gorm:"column:status;type:String;not null" json:"status"`
	CreatedBy       string   `json:"created_by"`
	CreatedAt       uint64   `json:"created_at"`
	UpdatedBy       string   `json:"updated_by"`
	UpdatedAt       uint64   `json:"updated_at"`
	DeletedAt       uint64   `json:"deleted_at"`
	StartDate       string   `json:"start_date"`
}
