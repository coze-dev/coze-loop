// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"slices"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// Feedback 指标名常量
const (
	MetricFeedbackCount             = "feedback_count"
	MetricFeedbackScoreAvg          = "feedback_score_avg"
	MetricFeedbackScoreMax          = "feedback_score_max"
	MetricFeedbackScoreMin          = "feedback_score_min"
	MetricFeedbackValueDistribution = "feedback_value_distribution"

	MetricGroupFeedback = "feedback"
)

// TraverseFeedbackMetricsReq Feedback 离线指标遍历请求
type TraverseFeedbackMetricsReq struct {
	StartDate string // e.g. "2026-06-02"
}

// TraverseFeedbackMetricsResp Feedback 离线指标遍历结果
type TraverseFeedbackMetricsResp struct {
	Total   int
	Success int
	Failure int
	Error   error
}

// TraverseFeedbackMetrics Feedback 离线指标遍历入口
func (m *MetricsService) TraverseFeedbackMetrics(ctx context.Context, req *TraverseFeedbackMetricsReq) (*TraverseFeedbackMetricsResp, error) {
	if m.annotationMetricRepo == nil {
		logs.CtxInfo(ctx, "annotationMetricRepo is nil, skip feedback metrics traverse")
		return &TraverseFeedbackMetricsResp{}, nil
	}

	resp := &TraverseFeedbackMetricsResp{}

	for platformType, platformDef := range m.pMetrics.PlatformMetricDefs {
		if !slices.Contains(platformDef.MetricGroups, MetricGroupFeedback) {
			continue
		}
		tenants, err := m.tenantProvider.GetMetricTenantsByPlatformType(ctx, platformType)
		if err != nil {
			logs.CtxError(ctx, "get tenants for feedback metrics failed, platformType=%s: %v", platformType, err)
			resp.Error = err
			continue
		}

		rows, err := m.annotationMetricRepo.QueryFeedbackAggregation(ctx, &repo.QueryFeedbackAggregationParam{
			Tenants:   tenants,
			StartDate: req.StartDate,
		})
		if err != nil {
			logs.CtxError(ctx, "query feedback aggregation failed, platformType=%s: %v", platformType, err)
			resp.Error = err
			continue
		}

		events := m.convertFeedbackRows(rows, req.StartDate, string(platformType))
		logs.CtxInfo(ctx, "feedback metrics: platformType=%s, %d rows aggregated, %d events generated", platformType, len(rows), len(events))

		if len(events) == 0 {
			continue
		}

		if err := m.oMetricRepo.InsertMetrics(ctx, events); err != nil {
			logs.CtxError(ctx, "insert feedback metrics failed, platformType=%s: %v", platformType, err)
			resp.Failure += len(events)
			resp.Error = err
			continue
		}

		resp.Total += len(events)
		resp.Success += len(events)
	}

	return resp, resp.Error
}

// convertFeedbackRows 将聚合结果行转为 MetricEvent 列表
func (m *MetricsService) convertFeedbackRows(rows []*repo.FeedbackAggregationRow, startDate string, platformType string) []*entity.MetricEvent {
	var events []*entity.MetricEvent

	for _, row := range rows {
		if row.SpaceID == "" || row.AnnotationKey == "" {
			continue
		}

		baseObjectKeys := map[string]string{
			"annotation_key":  row.AnnotationKey,
			"psm":             row.PSM,
			"agent_name":      row.AgentName,
			"feedback_source": row.FeedbackSource,
		}

		// feedback_count: 所有类型都写入
		events = append(events, &entity.MetricEvent{
			PlatformType: platformType,
			WorkspaceID:  row.SpaceID,
			StartDate:    startDate,
			MetricName:   MetricFeedbackCount,
			MetricValue:  strconv.FormatInt(row.Count, 10),
			ObjectKeys:   copyObjectKeys(baseObjectKeys),
		})

		// numeric 类型: 写入 avg/max/min
		if isNumericValueType(row.ValueType) {
			events = append(events, &entity.MetricEvent{
				PlatformType: platformType,
				WorkspaceID:  row.SpaceID,
				StartDate:    startDate,
				MetricName:   MetricFeedbackScoreAvg,
				MetricValue:  strconv.FormatFloat(row.AvgFloat, 'f', -1, 64),
				ObjectKeys:   copyObjectKeys(baseObjectKeys),
			})
			events = append(events, &entity.MetricEvent{
				PlatformType: platformType,
				WorkspaceID:  row.SpaceID,
				StartDate:    startDate,
				MetricName:   MetricFeedbackScoreMax,
				MetricValue:  strconv.FormatFloat(row.MaxFloat, 'f', -1, 64),
				ObjectKeys:   copyObjectKeys(baseObjectKeys),
			})
			events = append(events, &entity.MetricEvent{
				PlatformType: platformType,
				WorkspaceID:  row.SpaceID,
				StartDate:    startDate,
				MetricName:   MetricFeedbackScoreMin,
				MetricValue:  strconv.FormatFloat(row.MinFloat, 'f', -1, 64),
				ObjectKeys:   copyObjectKeys(baseObjectKeys),
			})
		}

		// category/boolean 类型: 写入 value_distribution
		if isCategoryValueType(row.ValueType) && row.ValueString != "" {
			objKeys := copyObjectKeys(baseObjectKeys)
			objKeys["value_string"] = row.ValueString
			events = append(events, &entity.MetricEvent{
				PlatformType: platformType,
				WorkspaceID:  row.SpaceID,
				StartDate:    startDate,
				MetricName:   MetricFeedbackValueDistribution,
				MetricValue:  strconv.FormatInt(row.Count, 10),
				ObjectKeys:   objKeys,
			})
		}
	}

	return events
}

// isNumericValueType 判断是否为 numeric 类型
func isNumericValueType(valueType string) bool {
	return valueType == "double" || valueType == "long" || valueType == "numeric" || valueType == "number"
}

// isCategoryValueType 判断是否为 category/boolean 类型
func isCategoryValueType(valueType string) bool {
	return valueType == "string" || valueType == "bool" || valueType == "category" || valueType == "boolean"
}

// copyObjectKeys 复制 map
func copyObjectKeys(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
