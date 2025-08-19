// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type GetTraceParam struct {
	Tenants            []string
	TraceID            string
	StartAt            int64 // ms
	EndAt              int64 // ms
	Limit              int32
	NotQueryAnnotation bool
	SpanIDs            []string
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
}

type ListSpansResult struct {
	Spans     loop_span.SpanList
	PageToken string
	HasMore   bool
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
	Tenant     string
	TTL        loop_span.TTL
	Annotation *loop_span.Annotation
}

//go:generate mockgen -destination=mocks/trace.go -package=mocks . ITraceRepo
type ITraceRepo interface {
	InsertSpans(context.Context, *InsertTraceParam) error
	ListSpans(context.Context, *ListSpansParam) (*ListSpansResult, error)
	GetTrace(context.Context, *GetTraceParam) (loop_span.SpanList, error)
	ListAnnotations(context.Context, *ListAnnotationsParam) (loop_span.AnnotationList, error)
	GetAnnotation(context.Context, *GetAnnotationParam) (*loop_span.Annotation, error)
	InsertAnnotation(context.Context, *InsertAnnotationParam) error
}
