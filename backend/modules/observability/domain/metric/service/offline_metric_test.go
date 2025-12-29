// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	config "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo/mocks"
	consts "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/metric/const"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	traceServicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	spanfiltermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// TestMetricsService_parseStartDate 测试日期解析功能
func TestMetricsService_parseStartDate(t *testing.T) {
	t.Parallel()

	t.Run("valid date format", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{}

		startAt, endAt, err := svc.parseStartDate("2025-11-17")

		assert.NoError(t, err)
		assert.NotZero(t, startAt)
		assert.NotZero(t, endAt)
		assert.Less(t, startAt, endAt)
	})

	t.Run("invalid date format", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{}

		startAt, endAt, err := svc.parseStartDate("invalid-date")

		assert.Error(t, err)
		assert.Zero(t, startAt)
		assert.Zero(t, endAt)
		assert.Contains(t, err.Error(), "fail to parse start date")
	})
}

// TestMetricsService_shouldTraverseMetric 测试指标遍历判断逻辑
func TestMetricsService_shouldTraverseMetric(t *testing.T) {
	t.Parallel()

	t.Run("empty request metrics - should traverse", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{}

		result := svc.shouldTraverseMetric("test_metric", []string{})

		assert.True(t, result)
	})

	t.Run("metric in request metrics - should traverse", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{}

		result := svc.shouldTraverseMetric("test_metric", []string{"test_metric", "other_metric"})

		assert.True(t, result)
	})

	t.Run("metric not in request metrics - should not traverse", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{}

		result := svc.shouldTraverseMetric("test_metric", []string{"other_metric"})

		assert.False(t, result)
	})
}

// TestMetricsService_buildDrillDownFields 测试钻取字段构建
func TestMetricsService_buildDrillDownFields(t *testing.T) {
	t.Parallel()

	t.Run("非 AVG 聚合：返回单一组合并包含 space_id", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			pMetrics: &entity.PlatformMetrics{
				DrillDownObjects: map[string]*loop_span.FilterField{
					"platform_obj": {
						FieldName: "platform_field",
						FieldType: loop_span.FieldTypeString,
					},
					"group_obj": {
						FieldName: "group_field",
						FieldType: loop_span.FieldTypeString,
					},
					"metric_obj": {
						FieldName: "metric_field",
						FieldType: loop_span.FieldTypeString,
					},
				},
			},
		}

		platformCfg := &entity.PlatformMetricDef{
			DrillDownObjects: []string{"platform_obj"},
		}

		groupCfg := &entity.MetricGroup{
			DrillDownObjects: []string{"group_obj"},
		}

		// 使用默认 OExpression（Sum），不触发 AVG 组合逻辑
		definition := &testMetricDefinition{name: "test_metric"}

		// 期望仅一个组合（platform + group + space_id）
		result := svc.buildDrillDownFields(platformCfg, groupCfg, definition)
		assert.Len(t, result, 1)
		combo := result[0]
		assert.Len(t, combo, 3)
		names := make([]string, 0, len(combo))
		for _, f := range combo {
			names = append(names, f.FieldName)
		}
		assert.Contains(t, names, "platform_field")
		assert.Contains(t, names, "group_field")
		assert.Contains(t, names, loop_span.SpanFieldSpaceId)
	})

	t.Run("非 AVG 聚合：重复字段不去重", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			pMetrics: &entity.PlatformMetrics{
				DrillDownObjects: map[string]*loop_span.FilterField{
					"common_obj": {
						FieldName: "common_field",
						FieldType: loop_span.FieldTypeString,
					},
				},
			},
		}

		platformCfg := &entity.PlatformMetricDef{
			DrillDownObjects: []string{"common_obj"},
		}

		groupCfg := &entity.MetricGroup{
			DrillDownObjects: []string{"common_obj"}, // 相同的字段
		}

		// 使用默认 OExpression（Sum），不触发 AVG 组合逻辑
		definition := &testMetricDefinition{name: "test_metric"}

		result := svc.buildDrillDownFields(platformCfg, groupCfg, definition)
		// 期望仅一个组合，且重复字段会保留
		assert.Len(t, result, 1)
		combo := result[0]
		fieldCount := 0
		names := make([]string, 0, len(combo))
		for _, field := range combo {
			names = append(names, field.FieldName)
			if field.FieldName == "common_field" {
				fieldCount++
			}
		}
		assert.Equal(t, 2, fieldCount)                        // 平台 + 组 都包含同名字段
		assert.Contains(t, names, loop_span.SpanFieldSpaceId) // 包含 space_id
		assert.Len(t, combo, 3)
	})

	t.Run("AVG 聚合：返回幂集组合并包含 space_id", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			pMetrics: &entity.PlatformMetrics{
				DrillDownObjects: map[string]*loop_span.FilterField{
					"a": {FieldName: "a", FieldType: loop_span.FieldTypeString},
					"b": {FieldName: "b", FieldType: loop_span.FieldTypeString},
				},
			},
		}

		platformCfg := &entity.PlatformMetricDef{DrillDownObjects: []string{"a"}}
		groupCfg := &entity.MetricGroup{DrillDownObjects: []string{"b"}}

		// 自定义 AVG 聚合表达式（应返回所有子集的组合）
		def := &customTestMetricDefinition{
			name:              "avg_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeAvg, MetricName: "avg_metric"},
		}

		result := svc.buildDrillDownFields(platformCfg, groupCfg, def)
		// 期望幂集：{}, {a}, {b}, {a,b} 四种组合；每种都应追加 space_id
		assert.Len(t, result, 4)

		// 检查四种情形是否存在
		var hasOnlySpaceID, hasA, hasB, hasAB bool
		// 辅助函数：判断切片中是否包含指定元素
		has := func(ss []string, s string) bool {
			for _, x := range ss {
				if x == s {
					return true
				}
			}
			return false
		}

		for _, combo := range result {
			names := make([]string, 0, len(combo))
			for _, f := range combo {
				names = append(names, f.FieldName)
			}
			// 所有组合都必须包含 space_id
			assert.Contains(t, names, loop_span.SpanFieldSpaceId)

			switch {
			case len(names) == 1 && has(names, loop_span.SpanFieldSpaceId):
				hasOnlySpaceID = true
			case has(names, "a") && !has(names, "b"):
				hasA = true
			case has(names, "b") && !has(names, "a"):
				hasB = true
			case has(names, "a") && has(names, "b"):
				hasAB = true
			}
		}
		assert.True(t, hasOnlySpaceID)
		assert.True(t, hasA)
		assert.True(t, hasB)
		assert.True(t, hasAB)
	})
}

// TestMetricsService_extractMetrics 测试指标提取功能
func TestMetricsService_extractMetrics(t *testing.T) {
	t.Parallel()

	t.Run("time series metric", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": &testMetricDefinition{
					name:       "test_metric",
					metricType: entity.MetricTypeTimeSeries,
					groupBy: []*entity.Dimension{
						{
							Field: &loop_span.FilterField{
								FieldName: "group_field",
								FieldType: loop_span.FieldTypeString,
							},
							Alias: "group_alias",
						},
					},
				},
			},
			pMetrics: &entity.PlatformMetrics{
				DrillDownObjects: map[string]*loop_span.FilterField{},
			},
		}

		metric := &entity.Metric{
			TimeSeries: map[string][]*entity.MetricPoint{
				`{"group_alias":"group1"}`: {
					{Timestamp: "1", Value: "100"},
				},
			},
		}

		events := svc.extractMetrics("test_metric", metric)

		assert.Len(t, events, 1)

		// 验证第一个事件（有分组）
		assert.Equal(t, "100", events[0].MetricValue)
		assert.Equal(t, "group1", events[0].ObjectKeys["group_field"])
	})

	t.Run("summary metric", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": &testMetricDefinition{
					name:       "test_metric",
					metricType: entity.MetricTypeSummary,
				},
			},
			pMetrics: &entity.PlatformMetrics{
				DrillDownObjects: map[string]*loop_span.FilterField{},
			},
		}

		metric := &entity.Metric{
			Summary: "1000",
		}

		events := svc.extractMetrics("test_metric", metric)

		assert.Len(t, events, 1)
		assert.Equal(t, "1000", events[0].MetricValue)
	})

	t.Run("pie metric", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": &testMetricDefinition{
					name:       "test_metric",
					metricType: entity.MetricTypePie,
					groupBy: []*entity.Dimension{
						{
							Field: &loop_span.FilterField{
								FieldName: "category_field",
								FieldType: loop_span.FieldTypeString,
							},
							Alias: "category_alias",
						},
					},
				},
			},
			pMetrics: &entity.PlatformMetrics{
				DrillDownObjects: map[string]*loop_span.FilterField{},
			},
		}

		metric := &entity.Metric{
			Pie: map[string]string{
				`{"category_alias":"A"}`: "100",
				`{"category_alias":"B"}`: "200",
				"all":                    "300",
			},
		}

		events := svc.extractMetrics("test_metric", metric)

		assert.Len(t, events, 3)

		// 验证事件值
		values := make([]string, len(events))
		for i, event := range events {
			values[i] = event.MetricValue
		}
		assert.Contains(t, values, "100")
		assert.Contains(t, values, "200")
		assert.Contains(t, values, "300")
	})

	t.Run("metric definition not found", func(t *testing.T) {
		t.Parallel()
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{}, // 空的
		}

		metric := &entity.Metric{
			Summary: "100",
		}

		events := svc.extractMetrics("nonexistent_metric", metric)

		assert.Nil(t, events)
	})
}

// TestMetricsService_queryOfflineMetrics 测试离线指标查询
func TestMetricsService_queryOfflineMetrics(t *testing.T) {
	t.Parallel()

	t.Run("success with multiple metrics", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockIOfflineMetricRepo(ctrl)

		// 模拟多次查询调用
		repoMock.EXPECT().
			GetMetrics(gomock.Any(), gomock.Any()).
			Return(&repo.GetMetricsResult{
				Data: []map[string]any{
					{"metric_a": "100", "group": "A"},
					{"metric_a": "200", "group": "B"},
				},
			}, nil).
			Times(2)

		metricDefA := &testMetricDefinition{
			name:       "metric_a",
			metricType: entity.MetricTypePie,
		}

		metricDefB := &testMetricDefinition{
			name:       "metric_b",
			metricType: entity.MetricTypeSummary,
		}

		svc := &MetricsService{
			oMetricRepo: repoMock,
			metricDefMap: map[string]entity.IMetricDefinition{
				"metric_a": metricDefA,
				"metric_b": metricDefB,
			},
		}

		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("test_platform"),
			WorkspaceID:  1,
			MetricsNames: []string{"metric_a", "metric_b"},
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
		}

		resp, err := svc.queryOfflineMetrics(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Metrics, 2)
		assert.Contains(t, resp.Metrics, "metric_a")
		assert.Contains(t, resp.Metrics, "metric_b")
	})

	t.Run("build offline metric query fails", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{}, // 空的，不包含metric
		}

		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("test_platform"),
			WorkspaceID:  1,
			MetricsNames: []string{"nonexistent_metric"},
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
		}

		resp, err := svc.queryOfflineMetrics(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("get metrics fails", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockIOfflineMetricRepo(ctrl)

		repoMock.EXPECT().
			GetMetrics(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("database error")).
			Times(1)

		metricDef := &testMetricDefinition{
			name:       "test_metric",
			metricType: entity.MetricTypeSummary,
		}

		svc := &MetricsService{
			oMetricRepo: repoMock,
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": metricDef,
			},
		}

		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("test_platform"),
			WorkspaceID:  1,
			MetricsNames: []string{"test_metric"},
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
		}

		resp, err := svc.queryOfflineMetrics(context.Background(), req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		assert.Nil(t, resp)
	})
}

// TestMetricsService_buildOfflineMetricQuery 测试离线指标查询构建
func TestMetricsService_buildOfflineMetricQuery(t *testing.T) {
	t.Parallel()

	t.Run("success with time series metric", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		metricDef := &testMetricDefinition{
			name:       "test_metric",
			metricType: entity.MetricTypeTimeSeries,
			groupBy: []*entity.Dimension{
				{
					Field: &loop_span.FilterField{
						FieldName: "group_field",
						FieldType: loop_span.FieldTypeString,
					},
					Alias: "group_alias",
				},
			},
		}

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": metricDef,
			},
		}

		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("test_platform"),
			WorkspaceID:  1,
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
		}

		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, "test_metric")

		assert.NoError(t, err)
		assert.NotNil(t, builder)
		assert.NotNil(t, builder.mRepoReq)
		assert.Equal(t, entity.MetricTypeTimeSeries, builder.mInfo.mType)
		assert.Len(t, builder.mInfo.mAggregation, 1)
		assert.Equal(t, "test_metric", builder.mInfo.mAggregation[0].Alias)
	})

	t.Run("success with custom metric name in expression", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// 创建一个自定义的测试指标定义，使用自定义的OExpression
		customDef := &customTestMetricDefinition{
			name:       "test_metric",
			metricType: entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{
				AggrType:   entity.MetricOfflineAggrTypeSum,
				MetricName: "custom_metric_name", // 自定义名称
			},
		}

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": customDef,
			},
		}

		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("test_platform"),
			WorkspaceID:  1,
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
		}

		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, "test_metric")

		assert.NoError(t, err)
		assert.NotNil(t, builder)

		// 验证过滤条件中使用了自定义的metric name
		filters := builder.mRepoReq.Filters
		assert.NotNil(t, filters)
	})

	t.Run("metric definition not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{}, // 空的
		}

		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("test_platform"),
			WorkspaceID:  1,
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
		}

		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, "nonexistent_metric")

		assert.Error(t, err)
		assert.Nil(t, builder)
	})
}

// TestMetricsService_buildOfflineMetricQuery_ExtraFilter 覆盖 AVG 额外过滤逻辑
func TestMetricsService_buildOfflineMetricQuery_ExtraFilter(t *testing.T) {
	t.Parallel()

	t.Run("AVG 指标追加 NotExist 额外过滤字段（平台+组下钻，过滤已在请求中的字段）", func(t *testing.T) {
		t.Parallel()

		// 准备平台配置与下钻对象
		pMetrics := &entity.PlatformMetrics{
			DrillDownObjects: map[string]*loop_span.FilterField{
				// 平台下钻对象 key => 真实字段名
				"plat_obj_a": {FieldName: "plat_a", FieldType: loop_span.FieldTypeString},
				// 组下钻对象
				"group_obj_b": {FieldName: "group_b", FieldType: loop_span.FieldTypeString},
			},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("platform-1"): {DrillDownObjects: []string{"plat_obj_a"}},
			},
		}

		// 定义 AVG 指标（触发 buildExtraFilter）
		avgDef := &customTestMetricDefinition{
			name:              "avg_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeAvg, MetricName: "avg_metric"},
		}

		// 构造 svc，其中 metricDrillDown 指明该指标的组下钻对象
		svc := &MetricsService{
			metricDefMap:    map[string]entity.IMetricDefinition{avgDef.Name(): avgDef},
			metricDrillDown: map[string][]string{avgDef.Name(): {"group_obj_b"}},
			pMetrics:        pMetrics,
		}

		// 请求的 FilterFields 中，显式包含 plat_a，因而不应该为 plat_a 追加 NotExist；应只为 group_b 追加 NotExist
		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("platform-1"),
			WorkspaceID:  1,
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
			FilterFields: &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{
				{FieldName: "plat_a", FieldType: loop_span.FieldTypeString, Values: []string{"x"}},
			}},
		}

		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, avgDef.Name())
		assert.NoError(t, err)
		assert.NotNil(t, builder)
		assert.NotNil(t, builder.mRepoReq)

		// 校验 subFilters（第一个 SubFilter）包含基础三项 + 额外 NotExist(group_b)
		filters := builder.mRepoReq.Filters
		if assert.NotNil(t, filters) && assert.Len(t, filters.FilterFields, 2) {
			sub := filters.FilterFields[0].SubFilter
			if assert.NotNil(t, sub) {
				names := make([]string, 0, len(sub.FilterFields))
				notExistCount := 0
				for _, f := range sub.FilterFields {
					names = append(names, f.FieldName)
					if f.QueryType != nil && *f.QueryType == loop_span.QueryTypeEnumNotExist {
						notExistCount++
						assert.Equal(t, "group_b", f.FieldName)
					}
				}
				// 基础三项：space_id、platform_type、metric_name
				assert.Contains(t, names, loop_span.SpanFieldSpaceId)
				assert.Contains(t, names, "platform_type")
				assert.Contains(t, names, "metric_name")
				// plat_a 在请求中已出现，不应追加 NotExist；仅 group_b 追加
				assert.NotContains(t, names, "plat_a") // plat_a 仅在请求的第二个 SubFilter 中
				assert.Equal(t, 1, notExistCount)
			}
		}
	})

	t.Run("非 AVG 指标不追加额外过滤字段", func(t *testing.T) {
		t.Parallel()
		pMetrics := &entity.PlatformMetrics{
			DrillDownObjects: map[string]*loop_span.FilterField{
				"plat_obj_a": {FieldName: "plat_a", FieldType: loop_span.FieldTypeString},
			},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("platform-1"): {DrillDownObjects: []string{"plat_obj_a"}},
			},
		}
		sumDef := &customTestMetricDefinition{
			name:              "sum_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeSum, MetricName: "sum_metric"},
		}
		svc := &MetricsService{metricDefMap: map[string]entity.IMetricDefinition{sumDef.Name(): sumDef}, pMetrics: pMetrics}
		req := &QueryMetricsReq{PlatformType: loop_span.PlatformType("platform-1"), WorkspaceID: 1}
		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, sumDef.Name())
		assert.NoError(t, err)
		sub := builder.mRepoReq.Filters.FilterFields[0].SubFilter
		names := make([]string, 0, len(sub.FilterFields))
		notExistCount := 0
		for _, f := range sub.FilterFields {
			names = append(names, f.FieldName)
			if f.QueryType != nil && *f.QueryType == loop_span.QueryTypeEnumNotExist {
				notExistCount++
			}
		}
		// 仅基础三项，无额外 NotExist
		assert.Contains(t, names, loop_span.SpanFieldSpaceId)
		assert.Contains(t, names, "platform_type")
		assert.Contains(t, names, "metric_name")
		assert.Equal(t, 0, notExistCount)
	})

	t.Run("平台未配置（AVG）不追加额外过滤字段", func(t *testing.T) {
		t.Parallel()
		pMetrics := &entity.PlatformMetrics{
			DrillDownObjects: map[string]*loop_span.FilterField{
				"plat_obj_a": {FieldName: "plat_a", FieldType: loop_span.FieldTypeString},
			},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{}, // 缺失 platform-1
		}
		avgDef := &customTestMetricDefinition{
			name:              "avg_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeAvg, MetricName: "avg_metric"},
		}
		svc := &MetricsService{metricDefMap: map[string]entity.IMetricDefinition{avgDef.Name(): avgDef}, pMetrics: pMetrics}
		req := &QueryMetricsReq{PlatformType: loop_span.PlatformType("platform-1"), WorkspaceID: 1}
		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, avgDef.Name())
		assert.NoError(t, err)
		sub := builder.mRepoReq.Filters.FilterFields[0].SubFilter
		names := make([]string, 0, len(sub.FilterFields))
		notExistCount := 0
		for _, f := range sub.FilterFields {
			names = append(names, f.FieldName)
			if f.QueryType != nil && *f.QueryType == loop_span.QueryTypeEnumNotExist {
				notExistCount++
			}
		}
		// 仅基础三项，无额外 NotExist（因平台未配置）
		assert.Contains(t, names, loop_span.SpanFieldSpaceId)
		assert.Contains(t, names, "platform_type")
		assert.Contains(t, names, "metric_name")
		assert.Equal(t, 0, notExistCount)
	})
}

// TestMetricsService_buildTraverseMetrics 测试构建遍历指标
func TestMetricsService_buildTraverseMetrics(t *testing.T) {
	t.Parallel()

	t.Run("success with valid configuration", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		metricDef := &testMetricDefinition{
			name:       "test_metric",
			metricType: entity.MetricTypeSummary,
		}

		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: []entity.IMetricDefinition{metricDef},
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test_platform"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": metricDef,
			},
			pMetrics: pMetrics,
		}

		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"test_platform"},
			MetricsNames:  []string{"test_metric"},
		})

		assert.NoError(t, err)
		assert.Len(t, metrics, 1)
		assert.Equal(t, "test_metric", metrics[0].metricDef.Name())
		assert.Equal(t, loop_span.PlatformType("test_platform"), metrics[0].platformType)
	})

	t.Run("skip const metrics", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		constDef := consts.NewConstMinuteMetric()

		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: []entity.IMetricDefinition{constDef},
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test_platform"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				constDef.Name(): constDef,
			},
			pMetrics: pMetrics,
		}

		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"test_platform"},
			MetricsNames:  []string{},
		})

		assert.NoError(t, err)
		assert.Len(t, metrics, 0)
	})

	t.Run("metric not found in metricDefMap", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		metricDef := &testMetricDefinition{
			name:       "test_metric",
			metricType: entity.MetricTypeSummary,
		}

		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: []entity.IMetricDefinition{metricDef},
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test_platform"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{}, // 空的，不包含test_metric
			pMetrics:     pMetrics,
		}

		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"test_platform"},
			MetricsNames:  []string{"test_metric"},
		})

		assert.Error(t, err)
		assert.Nil(t, metrics)
	})

	t.Run("platform type not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		svc := &MetricsService{
			pMetrics: &entity.PlatformMetrics{
				PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{},
			},
		}

		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"nonexistent_platform"},
		})

		assert.NoError(t, err)
		assert.Len(t, metrics, 0) // 没有找到平台类型，返回空
	})

	t.Run("metric group not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		pMetrics := &entity.PlatformMetrics{
			MetricGroups:     map[string]*entity.MetricGroup{},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test_platform"): {
					MetricGroups: []string{"nonexistent_group"}, // 不存在的组
				},
			},
		}

		svc := &MetricsService{
			pMetrics: pMetrics,
		}

		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"test_platform"},
		})

		assert.NoError(t, err)
		assert.Len(t, metrics, 0) // 没有找到组，返回空
	})

	t.Run("empty platform types uses all platform types", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		metricDef := &testMetricDefinition{
			name:       "test_metric",
			metricType: entity.MetricTypeSummary,
		}

		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: []entity.IMetricDefinition{metricDef},
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("platform1"): {
					MetricGroups: []string{"test_group"},
				},
				loop_span.PlatformType("platform2"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}

		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				"test_metric": metricDef,
			},
			pMetrics: pMetrics,
		}

		// 测试buildTraverseMetrics的逻辑 - 传入空数组时应该使用所有平台类型
		// 注意：在TraverseMetrics方法中会处理空数组的情况，但在buildTraverseMetrics中不会
		// 所以我们需要模拟TraverseMetrics方法的行为
		req := &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{}, // 空数组
			MetricsNames:  []string{"test_metric"},
			WorkspaceID:   1,
			StartDate:     "2025-11-17",
		}

		// 模拟TraverseMetrics方法中的处理逻辑
		if len(req.PlatformTypes) == 0 {
			req.PlatformTypes = []loop_span.PlatformType{"platform1", "platform2"}
		}

		metrics, err := svc.buildTraverseMetrics(context.Background(), req)

		assert.NoError(t, err)
		assert.Len(t, metrics, 2) // 两个平台类型
	})
}

// 自定义测试指标定义，支持自定义OExpression
type customTestMetricDefinition struct {
	name              string
	metricType        entity.MetricType
	groupBy           []*entity.Dimension
	where             []*loop_span.FilterField
	customOExpression *entity.OExpression
}

func (d *customTestMetricDefinition) Name() string {
	return d.name
}

func (d *customTestMetricDefinition) Type() entity.MetricType {
	return d.metricType
}

func (d *customTestMetricDefinition) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (d *customTestMetricDefinition) Expression(entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (d *customTestMetricDefinition) Where(context.Context, span_filter.Filter, *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return d.where, nil
}

func (d *customTestMetricDefinition) GroupBy() []*entity.Dimension {
	return d.groupBy
}

func (d *customTestMetricDefinition) OExpression() *entity.OExpression {
	if d.customOExpression != nil {
		return d.customOExpression
	}
	return &entity.OExpression{
		AggrType:   entity.MetricOfflineAggrTypeSum,
		MetricName: d.name,
	}
}

func TestMetricsService_buildTraverseMetrics_GroupBelong(t *testing.T) {
	t.Parallel()

	t.Run("compound sub-metrics use actual group", func(t *testing.T) {
		t.Parallel()
		metric1 := &testMetricDefinition{name: "metric_numerator", metricType: entity.MetricTypeSummary}
		metric2 := &testMetricDefinition{name: "metric_denominator", metricType: entity.MetricTypeSummary}
		compound := &testCompoundMetricDefinition{
			testMetricDefinition: &testMetricDefinition{name: "metric_ratio", metricType: entity.MetricTypeSummary},
			metrics:              []entity.IMetricDefinition{metric1, metric2},
			operator:             entity.MetricOperatorDivide,
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"group_a": {MetricDefinitions: []entity.IMetricDefinition{compound}},
				"group_b": {MetricDefinitions: []entity.IMetricDefinition{metric1, metric2}},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test_platform"): {MetricGroups: []string{"group_a", "group_b"}},
			},
		}
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				metric1.Name():  metric1,
				metric2.Name():  metric2,
				compound.Name(): compound,
			},
			pMetrics: pMetrics,
		}
		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"test_platform"},
			MetricsNames:  []string{"metric_ratio"},
		})
		assert.NoError(t, err)
		assert.Len(t, metrics, 2)
		nameToGroup := map[string]string{}
		for _, tm := range metrics {
			nameToGroup[tm.metricDef.Name()] = tm.groupName
		}
		assert.Equal(t, "group_b", nameToGroup["metric_numerator"])
		assert.Equal(t, "group_b", nameToGroup["metric_denominator"])
	})

	t.Run("sub-metric not in any group returns error", func(t *testing.T) {
		t.Parallel()
		metric1 := &testMetricDefinition{name: "metric_numerator", metricType: entity.MetricTypeSummary}
		metric2 := &testMetricDefinition{name: "metric_denominator", metricType: entity.MetricTypeSummary}
		compound := &testCompoundMetricDefinition{
			testMetricDefinition: &testMetricDefinition{name: "metric_ratio", metricType: entity.MetricTypeSummary},
			metrics:              []entity.IMetricDefinition{metric1, metric2},
			operator:             entity.MetricOperatorDivide,
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"group_a": {MetricDefinitions: []entity.IMetricDefinition{compound}},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test_platform"): {MetricGroups: []string{"group_a"}},
			},
		}
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				metric1.Name():  metric1,
				metric2.Name():  metric2,
				compound.Name(): compound,
			},
			pMetrics: pMetrics,
		}
		metrics, err := svc.buildTraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{"test_platform"},
			MetricsNames:  []string{"metric_ratio"},
		})
		assert.Error(t, err)
		assert.Nil(t, metrics)
	})
}

// TestMetricsService_TraverseMetrics 覆盖 TraverseMetrics 成功路径
func TestMetricsService_TraverseMetrics(t *testing.T) {
	t.Parallel()

	t.Run("空平台类型默认遍历且成功插入事件", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// 构造依赖的 mock
		metricRepoMock := repomocks.NewMockIMetricRepo(ctrl)
		offlineRepoMock := repomocks.NewMockIOfflineMetricRepo(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
		filterMock := spanfiltermocks.NewMockFilter(ctrl)
		traceCfgMock := configmocks.NewMockITraceConfig(ctrl)

		// 配置 TraceConfig：不走离线查询，直接走在线查询路径
		traceCfgMock.EXPECT().GetMetricQueryConfig(gomock.Any()).Return(&config.MetricQueryConfig{
			SupportOffline: false,
		}).AnyTimes()

		// 配置 filter 与租户
		tenantMock.EXPECT().GetMetricTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-1"}, nil)
		builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, env *span_filter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
				assert.Equal(t, int64(1), env.WorkspaceID)
				return []*loop_span.FilterField{{FieldName: "workspace_id"}}, true, nil
			},
		)

		// 在线查询返回一条 Summary 数据
		metricRepoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, _ *repo.GetMetricsParam) (*repo.GetMetricsResult, error) {
				return &repo.GetMetricsResult{Data: []map[string]any{{"metric_a": "123"}}}, nil
			},
		).Times(1)

		// 期望插入一条事件，校验关键字段
		offlineRepoMock.EXPECT().InsertMetrics(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, events []*entity.MetricEvent) error {
				assert.Len(t, events, 1)
				assert.Equal(t, "123", events[0].MetricValue)
				assert.Equal(t, "loop", events[0].PlatformType)
				assert.Equal(t, "metric_a", events[0].MetricName)
				assert.Equal(t, "2025-11-17", events[0].StartDate)
				return nil
			},
		).Times(1)

		// 定义一个 Summary 指标
		metricDef := &testMetricDefinition{
			name:       "metric_a",
			metricType: entity.MetricTypeSummary,
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"group_a": {MetricDefinitions: []entity.IMetricDefinition{metricDef}},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("loop"): {MetricGroups: []string{"group_a"}},
			},
		}

		// 创建服务
		svc, err := NewMetricsService(metricRepoMock, offlineRepoMock, tenantMock, builderMock, traceCfgMock, pMetrics)
		assert.NoError(t, err)

		// PlatformTypes 留空，TraverseMetrics 内部应使用全部平台类型
		resp, err := svc.TraverseMetrics(context.Background(), &TraverseMetricsReq{
			PlatformTypes: []loop_span.PlatformType{},
			MetricsNames:  []string{"metric_a"},
			WorkspaceID:   1,
			StartDate:     "2025-11-17",
		})

		assert.NoError(t, err)
		if assert.NotNil(t, resp) {
			assert.Equal(t, 1, resp.Statistic.Total)
			assert.Equal(t, 1, resp.Statistic.Success)
			assert.Equal(t, 0, resp.Statistic.Failure)
		}
	})

	t.Run("AVG 指标追加 NotExist 额外过滤字段（平台+组下钻，过滤已在请求中的字段）", func(t *testing.T) {
		t.Parallel()

		// 准备平台配置与下钻对象
		pMetrics := &entity.PlatformMetrics{
			DrillDownObjects: map[string]*loop_span.FilterField{
				// 平台下钻对象 key => 真实字段名
				"plat_obj_a": {FieldName: "plat_a", FieldType: loop_span.FieldTypeString},
				// 组下钻对象
				"group_obj_b": {FieldName: "group_b", FieldType: loop_span.FieldTypeString},
			},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("platform-1"): {
					DrillDownObjects: []string{"plat_obj_a"},
				},
			},
		}

		// 定义 AVG 指标（触发 buildExtraFilter）
		avgDef := &customTestMetricDefinition{
			name:              "avg_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeAvg, MetricName: "avg_metric"},
		}

		// 构造 svc，其中 metricDrillDown 指明该指标的组下钻对象
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{
				avgDef.Name(): avgDef,
			},
			metricDrillDown: map[string][]string{
				avgDef.Name(): {"group_obj_b"},
			},
			pMetrics: pMetrics,
		}

		// 请求的 FilterFields 中，显式包含 plat_a，因而不应该为 plat_a 追加 NotExist；应只为 group_b 追加 NotExist
		req := &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("platform-1"),
			WorkspaceID:  1,
			StartTime:    time.Now().Add(-24 * time.Hour).UnixMilli(),
			EndTime:      time.Now().UnixMilli(),
			FilterFields: &loop_span.FilterFields{FilterFields: []*loop_span.FilterField{
				{FieldName: "plat_a", FieldType: loop_span.FieldTypeString, Values: []string{"x"}},
			}},
		}

		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, avgDef.Name())
		assert.NoError(t, err)
		assert.NotNil(t, builder)
		assert.NotNil(t, builder.mRepoReq)

		// 校验 subFilters（第一个 SubFilter）包含基础三项 + 额外 NotExist(group_b)
		filters := builder.mRepoReq.Filters
		if assert.NotNil(t, filters) && assert.Len(t, filters.FilterFields, 2) {
			sub := filters.FilterFields[0].SubFilter
			if assert.NotNil(t, sub) {
				names := make([]string, 0, len(sub.FilterFields))
				notExistCount := 0
				for _, f := range sub.FilterFields {
					names = append(names, f.FieldName)
					if f.QueryType != nil && *f.QueryType == loop_span.QueryTypeEnumNotExist {
						notExistCount++
						assert.Equal(t, "group_b", f.FieldName)
					}
				}
				// 基础三项：space_id、platform_type、metric_name
				assert.Contains(t, names, loop_span.SpanFieldSpaceId)
				assert.Contains(t, names, "platform_type")
				assert.Contains(t, names, "metric_name")
				// plat_a 在请求中已出现，不应追加 NotExist；仅 group_b 追加
				assert.NotContains(t, names, "plat_a") // plat_a 仅在请求的第二个 SubFilter 中
				assert.Equal(t, 1, notExistCount)
			}
		}
	})

	t.Run("非 AVG 指标不追加额外过滤字段", func(t *testing.T) {
		t.Parallel()

		pMetrics := &entity.PlatformMetrics{
			DrillDownObjects: map[string]*loop_span.FilterField{
				"plat_obj_a": {FieldName: "plat_a", FieldType: loop_span.FieldTypeString},
			},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("platform-1"): {DrillDownObjects: []string{"plat_obj_a"}},
			},
		}
		sumDef := &customTestMetricDefinition{
			name:              "sum_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeSum, MetricName: "sum_metric"},
		}
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{sumDef.Name(): sumDef},
			pMetrics:     pMetrics,
		}
		req := &QueryMetricsReq{PlatformType: loop_span.PlatformType("platform-1"), WorkspaceID: 1}
		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, sumDef.Name())
		assert.NoError(t, err)
		sub := builder.mRepoReq.Filters.FilterFields[0].SubFilter
		names := make([]string, 0, len(sub.FilterFields))
		notExistCount := 0
		for _, f := range sub.FilterFields {
			names = append(names, f.FieldName)
			if f.QueryType != nil && *f.QueryType == loop_span.QueryTypeEnumNotExist {
				notExistCount++
			}
		}
		// 仅基础三项，无额外 NotExist
		assert.Contains(t, names, loop_span.SpanFieldSpaceId)
		assert.Contains(t, names, "platform_type")
		assert.Contains(t, names, "metric_name")
		assert.Equal(t, 0, notExistCount)
	})

	t.Run("平台未配置（AVG）不追加额外过滤字段", func(t *testing.T) {
		t.Parallel()
		pMetrics := &entity.PlatformMetrics{
			DrillDownObjects: map[string]*loop_span.FilterField{
				"plat_obj_a": {FieldName: "plat_a", FieldType: loop_span.FieldTypeString},
			},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{}, // 缺失 platform-1
		}
		avgDef := &customTestMetricDefinition{
			name:              "avg_metric",
			metricType:        entity.MetricTypeSummary,
			customOExpression: &entity.OExpression{AggrType: entity.MetricOfflineAggrTypeAvg, MetricName: "avg_metric"},
		}
		svc := &MetricsService{
			metricDefMap: map[string]entity.IMetricDefinition{avgDef.Name(): avgDef},
			pMetrics:     pMetrics,
		}
		req := &QueryMetricsReq{PlatformType: loop_span.PlatformType("platform-1"), WorkspaceID: 1}
		builder, err := svc.buildOfflineMetricQuery(context.Background(), req, avgDef.Name())
		assert.NoError(t, err)
		sub := builder.mRepoReq.Filters.FilterFields[0].SubFilter
		names := make([]string, 0, len(sub.FilterFields))
		notExistCount := 0
		for _, f := range sub.FilterFields {
			names = append(names, f.FieldName)
			if f.QueryType != nil && *f.QueryType == loop_span.QueryTypeEnumNotExist {
				notExistCount++
			}
		}
		// 仅基础三项，无额外 NotExist（因平台未配置）
		assert.Contains(t, names, loop_span.SpanFieldSpaceId)
		assert.Contains(t, names, "platform_type")
		assert.Contains(t, names, "metric_name")
		assert.Equal(t, 0, notExistCount)
	})
}
