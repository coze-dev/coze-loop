// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

const MetricSourceOffline = "offline"

type GetMetricsParam struct {
	WorkSpaceID  string
	Tenants      []string
	Aggregations []*entity.Dimension
	GroupBys     []*entity.Dimension
	Filters      *loop_span.FilterFields
	StartAt      int64
	EndAt        int64
	Granularity  entity.MetricGranularity
	Source       string
}

type GetMetricsResult struct {
	Data []map[string]any
}

//go:generate mockgen -destination=mocks/metrics.go -package=mocks . IMetricRepo
type IMetricRepo interface {
	GetMetrics(ctx context.Context, param *GetMetricsParam) (*GetMetricsResult, error)
}

type IOfflineMetricRepo interface {
	IMetricRepo
	InsertMetrics(ctx context.Context, events []*entity.MetricEvent) error
}

// QueryFeedbackAggregationParam annotation 表 Feedback 聚合查询参数
type QueryFeedbackAggregationParam struct {
	Tenants   []string
	StartDate string // e.g. "2026-06-02"
}

// FeedbackAggregationRow annotation 表 Feedback 聚合查询结果行
type FeedbackAggregationRow struct {
	SpaceID        string
	AnnotationKey  string
	PSM            string
	AgentName      string
	FeedbackSource string // annotation_type
	ValueType      string
	ValueString    string
	Count          int64
	AvgFloat       float64
	MaxFloat       float64
	MinFloat       float64
}

// IAnnotationMetricRepo annotation 表 Feedback 指标聚合查询接口
type IAnnotationMetricRepo interface {
	// QueryFeedbackAggregation 离线 CronJob 聚合查询（按天）
	QueryFeedbackAggregation(ctx context.Context, param *QueryFeedbackAggregationParam) ([]*FeedbackAggregationRow, error)
	// QueryFeedbackOnlineMetrics 在线实时查询（按时间范围）
	QueryFeedbackOnlineMetrics(ctx context.Context, param *QueryFeedbackOnlineParam) (*GetMetricsResult, error)
}

// QueryFeedbackOnlineParam annotation 表在线查询参数
type QueryFeedbackOnlineParam struct {
	Tenants     []string
	WorkspaceID string
	StartTime   int64 // ms timestamp
	EndTime     int64 // ms timestamp
	MetricNames []string
	Filters     *loop_span.FilterFields
	Granularity entity.MetricGranularity
	DrillDownFields []*loop_span.FilterField
}
