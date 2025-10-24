package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	metricentity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	metricrepo "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	metricrepomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	traceservicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	spanfiltermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	gomock "go.uber.org/mock/gomock"
)

type stubMetricDef struct {
	name    string
	mType   metricentity.MetricType
	groupBy []*metricentity.Dimension
	where   []*loop_span.FilterField
	expr    *metricentity.Expression
}

func (s *stubMetricDef) Name() string {
	return s.name
}

func (s *stubMetricDef) Type() metricentity.MetricType {
	if s.mType == "" {
		return metricentity.MetricTypeTimeSeries
	}
	return s.mType
}

func (s *stubMetricDef) Source() metricentity.MetricSource {
	return metricentity.MetricSourceCK
}

func (s *stubMetricDef) Expression(metricentity.MetricGranularity) *metricentity.Expression {
	if s.expr != nil {
		return s.expr
	}
	return &metricentity.Expression{Expression: "sum(x)"}
}

func (s *stubMetricDef) Where(context.Context, span_filter.Filter, *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	return s.where, nil
}

func (s *stubMetricDef) GroupBy() []*metricentity.Dimension {
	return s.groupBy
}

type stubAdapterMetric struct {
	*stubMetricDef
	wrappers []metricentity.IMetricWrapper
}

func (s *stubAdapterMetric) Wrappers() []metricentity.IMetricWrapper {
	return s.wrappers
}

type stubWrapper struct {
	suffix string
}

func (w stubWrapper) Wrap(def metricentity.IMetricDefinition) metricentity.IMetricDefinition {
	return &stubMetricDef{name: def.Name() + "_" + w.suffix, mType: def.Type()}
}

type stubFillMetric struct {
	*stubMetricDef
	fill string
}

func (s *stubFillMetric) Interpolate() string {
	return s.fill
}

func TestNewMetricsServiceDuplicateName(t *testing.T) {
	def1 := &stubMetricDef{name: "dup"}
	def2 := &stubMetricDef{name: "dup"}
	_, err := NewMetricsService(nil, []metricentity.IMetricDefinition{def1, def2}, nil, nil)
	require.Error(t, err)
}

func TestNewMetricsServiceAdapterWrapped(t *testing.T) {
	adapter := &stubAdapterMetric{
		stubMetricDef: &stubMetricDef{name: "base"},
		wrappers:      []metricentity.IMetricWrapper{stubWrapper{suffix: "w1"}},
	}
	svc, err := NewMetricsService(nil, []metricentity.IMetricDefinition{adapter}, nil, nil)
	require.NoError(t, err)
	metricsSvc := svc.(*MetricsService)
	require.Contains(t, metricsSvc.metricDefMap, "base_w1")
	require.Len(t, metricsSvc.metricDefMap, 1)
}

func TestMetricsServiceQueryMetricsMetricNotFound(t *testing.T) {
	svc := &MetricsService{metricDefMap: map[string]metricentity.IMetricDefinition{}}
	_, err := svc.QueryMetrics(context.Background(), &QueryMetricsReq{MetricsNames: []string{"missing"}})
	require.Error(t, err)
}

func TestBuildMetricQueryEmptyBasicFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	builder := traceservicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
	filter := spanfiltermocks.NewMockFilter(ctrl)
	tenantProvider := mocks.NewMockITenantProvider(ctrl)
	metricDef := &stubMetricDef{name: "metric"}
	svc := &MetricsService{
		metricDefMap:   map[string]metricentity.IMetricDefinition{"metric": metricDef},
		buildHelper:    builder,
		tenantProvider: tenantProvider,
	}
	ctx := context.Background()
	now := time.Now()
	builder.EXPECT().BuildPlatformRelatedFilter(ctx, gomock.Any()).Return(filter, nil)
	tenantProvider.EXPECT().GetTenantsByPlatformType(ctx, gomock.Any()).Return([]string{"tenant"}, nil)
	filter.EXPECT().BuildBasicSpanFilter(ctx, gomock.Any()).Return(nil, false, nil)
	mBuilder, err := svc.buildMetricQuery(ctx, &QueryMetricsReq{
		PlatformType: loop_span.PlatformType("test"),
		WorkspaceID:  1,
		MetricsNames: []string{"metric"},
		StartTime:    now.Add(-time.Hour).UnixMilli(),
		EndTime:      now.UnixMilli(),
	})
	require.NoError(t, err)
	require.Nil(t, mBuilder)
}

func TestBuildMetricQueryWithDrillDown(t *testing.T) {
	ctrl := gomock.NewController(t)
	builder := traceservicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
	filter := spanfiltermocks.NewMockFilter(ctrl)
	metricRepo := metricrepomocks.NewMockIMetricRepo(ctrl)
	tenantProvider := mocks.NewMockITenantProvider(ctrl)
	groupDim := &metricentity.Dimension{Alias: "group"}
	metricDef := &stubMetricDef{
		name:    "metric",
		groupBy: []*metricentity.Dimension{groupDim},
		where:   []*loop_span.FilterField{{FieldName: "f"}},
	}
	svc := &MetricsService{
		metricRepo:     metricRepo,
		metricDefMap:   map[string]metricentity.IMetricDefinition{"metric": metricDef},
		buildHelper:    builder,
		tenantProvider: tenantProvider,
	}
	ctx := context.Background()
	builder.EXPECT().BuildPlatformRelatedFilter(ctx, gomock.Any()).Return(filter, nil)
	tenantProvider.EXPECT().GetTenantsByPlatformType(ctx, gomock.Any()).Return([]string{"tenant"}, nil)
	filter.EXPECT().BuildBasicSpanFilter(ctx, gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "base"}}, false, nil)
	metricRepo.EXPECT().GetMetrics(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, param *metricrepo.GetMetricsParam) (*metricrepo.GetMetricsResult, error) {
		require.Equal(t, []string{"tenant"}, param.Tenants)
		require.Len(t, param.Aggregations, 1)
		require.Equal(t, "metric", param.Aggregations[0].Alias)
		require.Len(t, param.GroupBys, 2)
		require.Equal(t, groupDim, param.GroupBys[0])
		require.Equal(t, "drill", param.GroupBys[1].Alias)
		require.NotNil(t, param.Filters)
		require.Equal(t, metricentity.MetricGranularity1Min, param.Granularity)
		return &metricrepo.GetMetricsResult{Data: []map[string]any{}}, nil
	}).Times(1)
	resp, err := svc.QueryMetrics(ctx, &QueryMetricsReq{
		PlatformType: loop_span.PlatformType("test"),
		WorkspaceID:  2,
		MetricsNames: []string{"metric"},
		StartTime:    0,
		EndTime:      60000,
		Granularity:  metricentity.MetricGranularity1Min,
		FilterFields: &loop_span.FilterFields{},
		DrillDownFields: []*loop_span.FilterField{
			{FieldName: "drill"},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Metrics)
}

func TestFormatTimeSeriesDataFill(t *testing.T) {
	fillMetric := &stubFillMetric{
		stubMetricDef: &stubMetricDef{name: "metric"},
		fill:          "fill",
	}
	svc := &MetricsService{
		metricDefMap: map[string]metricentity.IMetricDefinition{fillMetric.Name(): fillMetric},
	}
	intervals := metricentity.NewTimeIntervals(0, 120000, metricentity.MetricGranularity1Min)
	builder := &metricQueryBuilder{
		mInfo: &metricInfo{
			mType:        metricentity.MetricTypeTimeSeries,
			mAggregation: []*metricentity.Dimension{{Alias: "metric"}},
		},
		granularity: metricentity.MetricGranularity1Min,
		mRepoReq: &metricrepo.GetMetricsParam{
			StartAt: 0,
			EndAt:   120000,
		},
	}
	data := []map[string]any{
		{"time_bucket": intervals[0], "metric": "1"},
		{"time_bucket": intervals[1], "metric": 2},
	}
	metrics := svc.formatTimeSeriesData(data, builder)
	require.Contains(t, metrics, "metric")
	require.Contains(t, metrics["metric"].TimeSeries, "all")
	series := metrics["metric"].TimeSeries["all"]
	require.Len(t, series, len(intervals))
	require.Equal(t, "1", series[0].Value)
	require.Equal(t, "2", series[1].Value)
	require.Equal(t, "fill", series[2].Value)
}

func TestDivideNumber(t *testing.T) {
	require.Equal(t, "2", divideNumber("4", "2"))
	require.Equal(t, "", divideNumber("4", "0"))
	require.Equal(t, "", divideNumber("nan", "2"))
}

func TestDivideTimeSeries(t *testing.T) {
	timeSeriesA := metricentity.TimeSeries{
		"all": {
			{Timestamp: "1", Value: "4"},
			{Timestamp: "2", Value: "6"},
		},
	}
	timeSeriesB := metricentity.TimeSeries{
		"all": {
			{Timestamp: "1", Value: "2"},
			{Timestamp: "2", Value: "3"},
		},
	}
	result := divideTimeSeries(context.Background(), timeSeriesA, timeSeriesB)
	require.Contains(t, result, "all")
	require.Len(t, result["all"], 2)
	require.Equal(t, "2", result["all"][0].Value)
	require.Equal(t, "2", result["all"][1].Value)
}

func TestPieMetrics(t *testing.T) {
	svc := &MetricsService{}
	resp, err := svc.pieMetrics(context.Background(), []*QueryMetricsResp{
		{Metrics: map[string]*metricentity.Metric{"a": {Summary: "1"}}},
		{Metrics: map[string]*metricentity.Metric{"b": {Summary: "2"}}},
	}, "pie")
	require.NoError(t, err)
	require.Contains(t, resp.Metrics, "pie")
	require.Equal(t, "1", resp.Metrics["pie"].Pie["a"])
	require.Equal(t, "2", resp.Metrics["pie"].Pie["b"])
}

func TestGetMetricValue(t *testing.T) {
	require.Equal(t, "null", getMetricValue("NaN"))
	require.Equal(t, "1", getMetricValue(1))
}
