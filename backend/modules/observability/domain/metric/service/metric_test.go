// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	configmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config/mocks"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	traceServicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	spanfilter "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	spanfiltermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewMetricsService(t *testing.T) {
	t.Parallel()

	t.Run("success with unique metrics", func(t *testing.T) {
		t.Parallel()
		defs := []entity.IMetricDefinition{
			&testMetricDefinition{name: "metric_a", metricType: entity.MetricTypeSummary},
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: defs,
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}
		svc, err := NewMetricsService(nil, nil, nil, nil, nil, pMetrics)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("duplicate metric name", func(t *testing.T) {
		t.Parallel()
		defs := []entity.IMetricDefinition{
			&testMetricDefinition{name: "metric_a", metricType: entity.MetricTypeSummary},
			&testMetricDefinition{name: "metric_a", metricType: entity.MetricTypeSummary},
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: defs,
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("test"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}
		svc, err := NewMetricsService(nil, nil, nil, nil, nil, pMetrics)
		assert.Error(t, err)
		assert.Nil(t, svc)
	})
}

func TestMetricsService_QueryMetrics(t *testing.T) {
	t.Parallel()

	t.Run("time series success", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockIMetricRepo(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
		filterMock := spanfiltermocks.NewMockFilter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		tenantMock.EXPECT().GetMetricTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-1"}, nil)
		builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, env *spanfilter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
				assert.Equal(t, int64(1), env.WorkspaceID)
				return []*loop_span.FilterField{{FieldName: "workspace"}}, true, nil
			},
		)
		repoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, param *repo.GetMetricsParam) (*repo.GetMetricsResult, error) {
				assert.Equal(t, []string{"tenant-1"}, param.Tenants)
				assert.NotNil(t, param.Filters)
				return &repo.GetMetricsResult{
					Data: []map[string]any{{
						"time_bucket": "0",
						"metric_a":    "3",
					}},
				}, nil
			},
		)
		traceConfigMock.EXPECT().GetMetricQueryConfig(gomock.Any()).Return(&config.MetricQueryConfig{
			SupportOffline: false,
		}).AnyTimes()

		metricDef := &testMetricDefinition{
			name:       "metric_a",
			metricType: entity.MetricTypeTimeSeries,
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: []entity.IMetricDefinition{metricDef},
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("loop"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}

		svc, err := NewMetricsService(repoMock, nil, tenantMock, builderMock, traceConfigMock, pMetrics)
		assert.NoError(t, err)

		resp, err := svc.QueryMetrics(context.Background(), &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("loop"),
			WorkspaceID:  1,
			MetricsNames: []string{"metric_a"},
			Granularity:  entity.MetricGranularity1Hour,
			StartTime:    0,
			EndTime:      0,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "3", resp.Metrics["metric_a"].TimeSeries["all"][0].Value)
	})

	t.Run("filter returns nil", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repoMock := repomocks.NewMockIMetricRepo(ctrl)
		tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
		builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
		filterMock := spanfiltermocks.NewMockFilter(ctrl)
		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)

		tenantMock.EXPECT().GetMetricTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-1"}, nil)
		builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, false, nil)

		metricDef := &testMetricDefinition{
			name:       "metric_a",
			metricType: entity.MetricTypeTimeSeries,
		}
		pMetrics := &entity.PlatformMetrics{
			MetricGroups: map[string]*entity.MetricGroup{
				"test_group": {
					MetricDefinitions: []entity.IMetricDefinition{metricDef},
				},
			},
			DrillDownObjects: map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
				loop_span.PlatformType("loop"): {
					MetricGroups: []string{"test_group"},
				},
			},
		}

		traceConfigMock.EXPECT().GetMetricQueryConfig(gomock.Any()).Return(&config.MetricQueryConfig{
			SupportOffline: false,
		}).AnyTimes()

		svc, err := NewMetricsService(repoMock, nil, tenantMock, builderMock, traceConfigMock, pMetrics)
		assert.NoError(t, err)

		resp, err := svc.QueryMetrics(context.Background(), &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("loop"),
			WorkspaceID:  2,
			MetricsNames: []string{"metric_a"},
			Granularity:  entity.MetricGranularity1Hour,
			StartTime:    0,
			EndTime:      0,
		})
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Empty(t, resp.Metrics)
	})

	t.Run("metric definition not found", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
		pMetrics := &entity.PlatformMetrics{
			MetricGroups:       map[string]*entity.MetricGroup{},
			DrillDownObjects:   map[string]*loop_span.FilterField{},
			PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{},
		}

		traceConfigMock.EXPECT().GetMetricQueryConfig(gomock.Any()).Return(&config.MetricQueryConfig{
			SupportOffline: false,
		}).AnyTimes()

		svc, err := NewMetricsService(repomocks.NewMockIMetricRepo(ctrl), nil, tenantmocks.NewMockITenantProvider(ctrl), traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl), traceConfigMock, pMetrics)
		assert.NoError(t, err)

		resp, err := svc.QueryMetrics(context.Background(), &QueryMetricsReq{
			PlatformType: loop_span.PlatformType("loop"),
			WorkspaceID:  1,
			MetricsNames: []string{"unknown"},
			Granularity:  entity.MetricGranularity1Hour,
			StartTime:    0,
			EndTime:      0,
		})
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

type testMetricDefinition struct {
	name       string
	metricType entity.MetricType
	groupBy    []*entity.Dimension
	where      []*loop_span.FilterField
}

func (d *testMetricDefinition) Name() string {
	return d.name
}

func (d *testMetricDefinition) Type() entity.MetricType {
	return d.metricType
}

func (d *testMetricDefinition) Source() entity.MetricSource {
	return entity.MetricSourceInnerStorage
}

func (d *testMetricDefinition) Expression(entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (d *testMetricDefinition) Where(context.Context, spanfilter.Filter, *spanfilter.SpanEnv) ([]*loop_span.FilterField, error) {
	return d.where, nil
}

func (d *testMetricDefinition) GroupBy() []*entity.Dimension {
	return d.groupBy
}

func (d *testMetricDefinition) OExpression() *entity.OExpression {
	return &entity.OExpression{
		AggrType:   entity.MetricOfflineAggrTypeSum,
		MetricName: d.name,
	}
}

func TestDivideNumber(t *testing.T) {
	t.Parallel()
	t.Run("valid division", func(t *testing.T) {
		assert.Equal(t, "2.5", divideNumber("5", "2"))
	})
	t.Run("invalid inputs", func(t *testing.T) {
		assert.Equal(t, "", divideNumber("NaN", "2"))
		assert.Equal(t, "", divideNumber("5", "0"))
		assert.Equal(t, "", divideNumber("-1", "1"))
	})
}

func TestDivideTimeSeries(t *testing.T) {
	t.Parallel()
	seriesA := entity.TimeSeries{
		"group": {
			{Timestamp: "2", Value: "9"},
			{Timestamp: "1", Value: "10"},
		},
		"mismatch": {
			{Timestamp: "1", Value: "1"},
		},
	}
	seriesB := entity.TimeSeries{
		"group": {
			{Timestamp: "1", Value: "2"},
			{Timestamp: "2", Value: "0"},
		},
		"mismatch": {
			{Timestamp: "1", Value: "1"},
			{Timestamp: "2", Value: "1"},
		},
	}
	ret := divideTimeSeries(context.Background(), seriesA, seriesB)
	assert.Len(t, ret, 1)
	points := ret["group"]
	if assert.Len(t, points, 2) {
		assert.Equal(t, "1", points[0].Timestamp)
		assert.Equal(t, "5", points[0].Value)
		assert.Equal(t, "2", points[1].Timestamp)
		assert.Equal(t, "null", points[1].Value)
	}
}

func TestAddNumber(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "5", addNumber("2", "3"))
	assert.Equal(t, "0", addNumber("0", "0"))
	assert.Equal(t, "-1", addNumber("2", "-3"))
}

func TestGetMetricValue(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "123", getMetricValue(123))
	assert.Equal(t, "null", getMetricValue("NaN"))
	assert.Equal(t, "null", getMetricValue("+Inf"))
	assert.Equal(t, "null", getMetricValue("-Inf"))
}

func TestGetDaysBeforeTimeStamp(t *testing.T) {
	t.Parallel()
	result := getDaysBeforeTimeStamp(1)
	assert.NotZero(t, result)
}

func TestDivideTimeSeriesBySummary(t *testing.T) {
	t.Parallel()
	series := entity.TimeSeries{
		"group": {
			{Timestamp: "1", Value: "10"},
			{Timestamp: "2", Value: "20"},
		},
	}
	ret := divideTimeSeriesBySummary(context.Background(), series, "2")
	assert.Len(t, ret, 1)
	points := ret["group"]
	if assert.Len(t, points, 2) {
		assert.Equal(t, "5", points[0].Value)
		assert.Equal(t, "10", points[1].Value)
	}
}

func TestFormatSummaryData(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}

	// Test normal summary data
	data := []map[string]any{
		{"metric_a": "100"},
	}
	mInfo := &metricInfo{
		mType: entity.MetricTypeSummary,
		mAggregation: []*entity.Dimension{
			{Alias: "metric_a"},
		},
	}
	ret := svc.formatSummaryData(data, mInfo)
	assert.NotNil(t, ret)
	assert.Equal(t, "100", ret["metric_a"].Summary)
}

func TestFormatPieData(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}

	data := []map[string]any{
		{"group": "A", "metric_a": "100"},
		{"group": "B", "metric_a": "200"},
	}
	mInfo := &metricInfo{
		mType: entity.MetricTypePie,
		mAggregation: []*entity.Dimension{
			{Alias: "metric_a"},
		},
	}
	ret := svc.formatPieData(data, mInfo)
	assert.NotNil(t, ret)
	assert.Equal(t, "100", ret["metric_a"].Pie[`{"group":"A"}`])
	assert.Equal(t, "200", ret["metric_a"].Pie[`{"group":"B"}`])
}

func TestMergeMetrics(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}

	onlineMetrics := map[string]*entity.Metric{
		"metric_a": {
			Summary: "100",
		},
	}
	offlineMetrics := map[string]*entity.Metric{
		"metric_a": {
			Summary: "50",
		},
		"metric_b": {
			Summary: "200",
		},
	}

	ret := svc.mergeMetrics(onlineMetrics, offlineMetrics)
	assert.NotNil(t, ret)
	assert.Equal(t, "150", ret["metric_a"].Summary) // 100 + 50
	assert.Equal(t, "200", ret["metric_b"].Summary)
}

func TestMergeSummaryMetric(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}

	onlineMetric := &entity.Metric{Summary: "100"}
	offlineMetric := &entity.Metric{Summary: "50"}

	ret := svc.mergeSummaryMetric(onlineMetric, offlineMetric)
	assert.NotNil(t, ret)
	assert.Equal(t, "150", ret.Summary) // 100 + 50
}

func TestMergeTimeSeriesMetric(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}

	onlineMetric := &entity.Metric{
		TimeSeries: entity.TimeSeries{
			"group1": {{Timestamp: "1", Value: "100"}},
		},
	}
	offlineMetric := &entity.Metric{
		TimeSeries: entity.TimeSeries{
			"group1": {{Timestamp: "2", Value: "200"}},
			"group2": {{Timestamp: "3", Value: "300"}},
		},
	}

	ret := svc.mergeTimeSeriesMetric(onlineMetric, offlineMetric)
	assert.NotNil(t, ret)
	assert.Len(t, ret.TimeSeries["group1"], 2)
	assert.Len(t, ret.TimeSeries["group2"], 1)
}

func TestMergePieMetric(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}

	onlineMetric := &entity.Metric{
		Pie: map[string]string{"A": "100", "B": "200"},
	}
	offlineMetric := &entity.Metric{
		Pie: map[string]string{"B": "50", "C": "300"},
	}

	ret := svc.mergePieMetric(onlineMetric, offlineMetric)
	assert.NotNil(t, ret)
	assert.Equal(t, "100", ret.Pie["A"])
	assert.Equal(t, "250", ret.Pie["B"]) // 200 + 50
	assert.Equal(t, "300", ret.Pie["C"])
}

func TestPieMetrics(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}
	resp, err := svc.pieMetrics(context.Background(), []*QueryMetricsResp{
		{Metrics: map[string]*entity.Metric{
			"metric_a": {Summary: "1"},
		}},
		{Metrics: map[string]*entity.Metric{
			"metric_b": {Summary: "3"},
		}},
	}, "metric_pie")
	assert.NoError(t, err)
	metric := resp.Metrics["metric_pie"]
	if assert.NotNil(t, metric) {
		assert.Equal(t, map[string]string{
			"metric_a": "1",
			"metric_b": "3",
		}, metric.Pie)
	}
}

func TestQueryMetrics_EmptyRequest(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
	pMetrics := &entity.PlatformMetrics{
		MetricGroups:       map[string]*entity.MetricGroup{},
		DrillDownObjects:   map[string]*loop_span.FilterField{},
		PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{},
	}

	traceConfigMock.EXPECT().GetMetricQueryConfig(gomock.Any()).Return(&config.MetricQueryConfig{
		SupportOffline: false,
	}).AnyTimes()

	svc, err := NewMetricsService(nil, nil, nil, nil, traceConfigMock, pMetrics)
	assert.NoError(t, err)

	resp, err := svc.QueryMetrics(context.Background(), &QueryMetricsReq{
		MetricsNames: []string{},
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, resp.Metrics)
}

func TestQueryMetrics_CompoundMetric(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 创建复合指标定义
	metric1 := &testMetricDefinition{
		name:       "metric_numerator",
		metricType: entity.MetricTypeSummary,
	}
	metric2 := &testMetricDefinition{
		name:       "metric_denominator",
		metricType: entity.MetricTypeSummary,
	}
	compoundMetric := &testCompoundMetricDefinition{
		testMetricDefinition: &testMetricDefinition{
			name:       "metric_ratio",
			metricType: entity.MetricTypeSummary,
		},
		metrics:  []entity.IMetricDefinition{metric1, metric2},
		operator: entity.MetricOperatorDivide,
	}

	pMetrics := &entity.PlatformMetrics{
		MetricGroups: map[string]*entity.MetricGroup{
			"test_group": {
				MetricDefinitions: []entity.IMetricDefinition{metric1, metric2, compoundMetric},
			},
		},
		DrillDownObjects: map[string]*loop_span.FilterField{},
		PlatformMetricDefs: map[loop_span.PlatformType]*entity.PlatformMetricDef{
			loop_span.PlatformType("loop"): {
				MetricGroups: []string{"test_group"},
			},
		},
	}

	traceConfigMock := configmocks.NewMockITraceConfig(ctrl)
	traceConfigMock.EXPECT().GetMetricQueryConfig(gomock.Any()).Return(&config.MetricQueryConfig{
		SupportOffline: false,
	}).AnyTimes()

	svc, err := NewMetricsService(nil, nil, nil, nil, traceConfigMock, pMetrics)
	assert.NoError(t, err)

	// 测试复合指标查询 - 多个指标，应该报错
	resp, err := svc.QueryMetrics(context.Background(), &QueryMetricsReq{
		PlatformType: loop_span.PlatformType("loop"),
		WorkspaceID:  1,
		MetricsNames: []string{"metric_ratio", "other_metric"},
		Granularity:  entity.MetricGranularity1Hour,
		StartTime:    0,
		EndTime:      0,
	})
	assert.Error(t, err)
	assert.Nil(t, resp)
}

type testCompoundMetricDefinition struct {
	*testMetricDefinition
	metrics  []entity.IMetricDefinition
	operator entity.MetricOperator
}

func (d *testCompoundMetricDefinition) GetMetrics() []entity.IMetricDefinition {
	return d.metrics
}

func (d *testCompoundMetricDefinition) Operator() entity.MetricOperator {
	return d.operator
}
