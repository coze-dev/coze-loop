// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package dao

import (
	"context"

	metrics_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

const (
	QueryTypeGetTrace  = "get_trace"
	QueryTypeListSpans = "list_spans"
)

type QueryParam struct {
	QueryType        string // for sql optimization
	Tables           []string
	AnnoTableMap     map[string]string
	StartTime        int64 // us
	EndTime          int64 // us
	Filters          *loop_span.FilterFields
	Limit            int32
	OrderByStartTime bool
	SelectColumns    []string
	OmitColumns      []string // omit specific columns
	Extra            map[string]string
}

type InsertParam struct {
	Table string
	Spans []*Span
}

// GetMetricsParam 指标查询参数
type GetMetricsParam struct {
	Tables       []string
	Aggregations []*metrics_entity.Dimension
	GroupBys     []*metrics_entity.Dimension
	Filters      *loop_span.FilterFields
	StartAt      int64
	EndAt        int64
	Granularity  metrics_entity.MetricGranularity
	Extra        map[string]string
	Source       string
}

//go:generate mockgen -destination=mocks/spans_dao.go -package=mocks . ISpansDao
type ISpansDao interface {
	Insert(context.Context, *InsertParam) error
	Get(context.Context, *QueryParam) ([]*Span, error)
	GetMetrics(ctx context.Context, param *GetMetricsParam) ([]map[string]any, error)
}

type Span struct {
	TraceID           string             `json:"trace_id"`
	SpanID            string             `json:"span_id"`
	SpaceID           string             `json:"space_id"`
	SpanType          string             `json:"span_type"`
	SpanName          string             `json:"span_name"`
	ParentID          string             `json:"parent_id"`
	Method            *string            `json:"method"`
	Psm               *string            `json:"psm"`
	Logid             *string            `json:"logid"`
	StartTime         int64              `json:"start_time"`
	CallType          *string            `json:"call_type"`
	Duration          int64              `json:"duration"`
	StatusCode        int32              `json:"status_code"`
	ObjectStorage     *string            `json:"object_storage"`
	Input             string             `json:"input"`
	Output            string             `json:"output"`
	LogicDeleteDate   int64              `json:"logic_delete_date"`
	ReserveCreateTime *string            `json:"reserve_create_time"`
	TagsBool          map[string]uint8   `json:"tags_bool"`
	TagsFloat         map[string]float64 `json:"tags_float"`
	TagsString        map[string]string  `json:"tags_string"`
	TagsLong          map[string]int64   `json:"tags_long"`
	TagsByte          map[string]string  `json:"tags_byte"`
	SystemTagsFloat   map[string]float64 `json:"system_tags_float"`
	SystemTagsLong    map[string]int64   `json:"system_tags_long"`
	SystemTagsString  map[string]string  `json:"system_tags_string"`
}
