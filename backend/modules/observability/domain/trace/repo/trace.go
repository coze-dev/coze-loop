// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type GetTraceParam struct {
	Tenants            []string
	TraceID            string
	LogID              string
	StartAt            int64 // ms
	EndAt              int64 // ms
	Limit              int32
	NotQueryAnnotation bool
	SpanIDs            []string
	OmitColumns        []string // omit specific columns
	SelectColumns      []string // select specific columns, default select all columns
	Filters            *loop_span.FilterFields
}

type ListSpansParam struct {
	Tenants            []string
	Filters            *loop_span.FilterFields
	StartAt            int64 // ms
	EndAt              int64 // ms
	Limit              int32
	DescByStartTime    bool
	PageToken          string
	NotQueryAnnotation bool
	OmitColumns        []string // omit specific columns
	SelectColumns      []string // select specific columns, default select all columns
}

type ListSpansResult struct {
	Spans     loop_span.SpanList
	PageToken string
	HasMore   bool
}

type GetPreSpanIDsParam struct {
	PreRespID string
}
type InsertTraceParam struct {
	Spans  loop_span.SpanList
	Tenant string
	TTL    loop_span.TTL
}

type GetAnnotationParam struct {
	Tenants []string
	ID      string
	StartAt int64 // ms
	EndAt   int64 // ms
}

type ListAnnotationsParam struct {
	Tenants         []string
	SpanID          string
	TraceID         string
	WorkspaceId     int64
	DescByUpdatedAt bool
	StartAt         int64 // ms
	EndAt           int64 // ms
}

type InsertAnnotationParam struct {
	Tenant         string
	TTL            loop_span.TTL
	Span           *loop_span.Span
	AnnotationType *loop_span.AnnotationType
}

type UpsertTrajectoryConfigParam struct {
	WorkspaceId int64
	Filters     string
	UserID      string
}

type GetTrajectoryConfigParam struct {
	WorkspaceId int64
}

type ListTrajectoryParam struct {
	Tenants     []string
	WorkspaceId int64
	TraceIDs    []string
	StartAt     *int64 // ms
}

//go:generate mockgen -destination=mocks/trace.go -package=mocks . ITraceRepo
type ITraceRepo interface {
	InsertSpans(context.Context, *InsertTraceParam) error
	ListSpans(context.Context, *ListSpansParam) (*ListSpansResult, error)
	GetPreSpanIDs(context.Context, *GetPreSpanIDsParam) (preSpanIDs, responseIDs []string, err error)
	GetTrace(context.Context, *GetTraceParam) (loop_span.SpanList, error)
	ListAnnotations(context.Context, *ListAnnotationsParam) (loop_span.AnnotationList, error)
	GetAnnotation(context.Context, *GetAnnotationParam) (*loop_span.Annotation, error)
	InsertAnnotations(context.Context, *InsertAnnotationParam) error
	UpsertTrajectoryConfig(context.Context, *UpsertTrajectoryConfigParam) error
	GetTrajectoryConfig(context.Context, GetTrajectoryConfigParam) (*entity.TrajectoryConfig, error)
}
