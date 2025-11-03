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
	TraceID         string   `json:"trace_id"`
	StartTime       int64    `json:"start_time"`
	SpaceID         string   `json:"space_id"`
	AnnotationType  string   `json:"annotation_type"`
	AnnotationIndex []string `json:"annotation_index"`
	Key             string   `json:"key"`
	ValueType       string   `json:"value_type"`
	ValueString     string   `json:"value_string"`
	ValueLong       int64    `json:"value_long"`
	ValueFloat      float64  `json:"value_float"`
	ValueBool       bool     `json:"value_bool"`
	Reasoning       string   `json:"reasoning"`
	Correction      string   `json:"correction"`
	Metadata        string   `json:"metadata"`
	Status          string   `json:"status"`
	CreatedBy       string   `json:"created_by"`
	CreatedAt       uint64   `json:"created_at"`
	UpdatedBy       string   `json:"updated_by"`
	UpdatedAt       uint64   `json:"updated_at"`
	DeletedAt       uint64   `json:"deleted_at"`
	StartDate       string   `json:"start_date"`
}
