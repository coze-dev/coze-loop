// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package otel

type ResourceScopeSpan struct {
	Resource *Resource             `json:"resource,omitempty"`
	Scope    *InstrumentationScope `json:"scope,omitempty"`
	Span     *Span                 `json:"span,omitempty"`
}

type LoopSpan struct {
	StartTime      int64  `json:"start_time"` // us
	SpanID         string `json:"span_id"`
	ParentID       string `json:"parent_id"`
	TraceID        string `json:"trace_id"`
	DurationMicros int64  `json:"duration_micros"` // us
	CallType       string `json:"call_type"`
	PSM            string `json:"psm"`
	LogID          string `json:"log_id"`
	WorkspaceID    string `json:"space_id"`
	SpanName       string `json:"span_name"`
	SpanType       string `json:"span_type"`
	Method         string `json:"method"`
	StatusCode     int32  `json:"status_code"`
	Input          string `json:"input"`
	Output         string `json:"output"`
	ObjectStorage  string `json:"object_storage"`

	SystemTagsString map[string]string  `json:"system_tags_string"`
	SystemTagsLong   map[string]int64   `json:"system_tags_long"`
	SystemTagsDouble map[string]float64 `json:"system_tags_double"`

	TagsString map[string]string  `json:"tags_string"`
	TagsLong   map[string]int64   `json:"tags_long"`
	TagsDouble map[string]float64 `json:"tags_double"`

	TagsBool map[string]bool   `json:"tags_bool"`
	TagsByte map[string]string `json:"tags_byte"`
}
